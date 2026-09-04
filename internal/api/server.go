package api

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// Server answers requests on one socket.
//
// One Server per door. The exec socket is a second Server with Exec set,
// which is what keeps the boundary structural rather than a condition
// somebody has to remember to write.
type Server struct {
	// Run is the command layer's runner. Supplied rather than imported:
	// this package knows the envelope and the socket, and nothing about
	// the dispatch on the other side.
	Run Runner
	// Version is this build's. A client from a different one is refused
	// rather than served stale behaviour.
	Version string
	// Exec says this door serves the commands that execute what a
	// repository declared. False everywhere except the exec socket.
	Exec bool
	// Notice is where the server says what it served.
	//
	// A line per request, and it is not decoration. The client falls back
	// to running in-process on every failure, silently and by design, so
	// the output of a served request and a fallback are identical — which
	// leaves no way to answer "is the daemon actually being used?" except
	// by asking the daemon. This is that answer.
	//
	// Stderr by default; a test points it somewhere it can read.
	Notice io.Writer
	// Identity names the repository a request is about, so requests
	// against one are queued rather than run at once. Supplied rather than
	// computed here: the identity ladder lives in internal/store, which
	// this package would otherwise have to import to answer a transport
	// question.
	Identity func(cwd string) string

	// Idle is how long a repository's warm state outlives its last
	// request. Zero means DefaultIdle.
	Idle time.Duration

	queues queues
	jobs   jobs
	warm   warm
}

// Listen opens the socket at path, replacing a stale one.
//
// The chmod is separate from the listen because net.Listen creates the
// socket with the process umask applied, which on a default umask is 0755
// — world-connectable, which is the one thing this design must not be.
// There is a window between the two calls; it is closed by the directory
// being 0700, which is why RunDir insists on that mode rather than
// assuming it.
func (s *Server) Listen(path string) (net.Listener, error) {
	// A socket file with nobody behind it is what a daemon that died
	// leaves. Connecting to it fails, so removing it is safe; removing a
	// LIVE one would not be, which is why the caller dials first.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("procoder: could not clear the stale socket at %s (%v)", path, err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("procoder: could not listen on %s (%v)", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		l.Close()
		return nil, fmt.Errorf("procoder: could not secure %s (%v) — refusing to serve on a socket anyone can open", path, err)
	}
	return l, nil
}

// Accept serves until the listener is closed, or until the daemon has
// been holding nothing for a whole idle window.
//
// A daemon holding nothing has nothing to be warm for, and staying
// resident to serve a request that may never come is how a convenience
// becomes something a person has to remember to kill. Exiting is safe
// because starting is free: the next session's hook starts another, and a
// client that finds no daemon runs in-process.
func (s *Server) Accept(l net.Listener) {
	s.warm.window = s.Idle
	stop := make(chan struct{})
	defer close(stop)
	go s.evictUntilEmpty(l, stop)

	for {
		conn, err := l.Accept()
		if err != nil {
			return // the listener was closed; nothing else ends this loop
		}
		go s.serveConn(conn)
	}
}

// evictUntilEmpty drops expired repositories and closes the listener once
// none is left, which is what ends Accept.
//
// The sweep runs at a fraction of the window rather than on a timer per
// repository: one goroutine for the daemon is cheaper to reason about
// than one per checkout, and being a minute late to release an index
// costs nothing.
func (s *Server) evictUntilEmpty(l net.Listener, stop <-chan struct{}) {
	window := s.warm.idle()
	tick := window / 6
	if tick <= 0 {
		tick = time.Second
	}
	t := time.NewTicker(tick)
	defer t.Stop()

	started := time.Now()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			if s.warm.evict(now) > 0 {
				started = now
				continue
			}
			// Nothing held. A daemon that has just started holds nothing
			// too, so it gets a whole window to be given some work before
			// it decides nobody wants it.
			if now.Sub(started) >= window {
				l.Close()
				return
			}
		}
	}
}

// serveConn answers one request. One request per connection: the client
// makes one call and reads one answer, and a connection that carried more
// would need a correlation id to say which answer belonged to which.
func (s *Server) serveConn(conn net.Conn) {
	defer conn.Close()

	started := time.Now()
	req, err := ReadRequest(conn)
	if err != nil {
		s.refuse(conn, 2, err.Error())
		return
	}
	if req.Protocol != Protocol {
		s.refuse(conn, 2, fmt.Sprintf(
			"procoder: this daemon speaks protocol %d and the client speaks %d — run the same build on both sides",
			Protocol, req.Protocol))
		return
	}
	// #201's boundary, enforced at the door rather than trusted to the
	// caller. The work socket serves everything except the four commands
	// that run what a repository — or a prior agent session — declared;
	// they are served here only where this is the exec socket, which the
	// hooks are never told the address of.
	if Executes(req.Argv) && !s.Exec {
		s.noticef("refused %s — not served on this socket", strings.Join(req.Argv, " "))
		path, _ := ExecSocket()
		s.refuse(conn, 2, "procoder: "+strings.Join(req.Argv, " ")+
			" is not served on this socket — a path that runs what a repository declared"+
			" must not be reachable by something running unattended (#201).\n"+
			"It is served on the exec socket ("+path+") when [service] exec = true,"+
			" and by the binary itself always.")
		return
	}

	// A poll is not a command: it reads a job's accumulated answer and
	// must not queue behind the very command it is asking about.
	if req.Job != "" {
		_ = WriteResponse(conn, s.jobs.poll(req.Job))
		return
	}

	identity := ""
	if s.Identity != nil {
		identity = s.Identity(req.Cwd)
	}
	// Touching the repository is what keeps it warm, and what keeps the
	// daemon alive: a daemon serving work is never one holding nothing.
	s.warm.get(identity)

	if IsLongRunning(req.Argv) {
		s.noticef("started %s as a job", strings.Join(req.Argv, " "))
		// Started INSIDE the queue so the repository's serialisation still
		// holds, and answered immediately: the caller gets an id in
		// milliseconds and the command keeps running behind it.
		job := s.jobs.start(req, s.runQueued(identity))
		_ = WriteResponse(conn, Response{Protocol: Protocol, Job: &job})
		return
	}
	// Held across the whole command, not just its writes: the store's lock
	// is per file, and two requests interleaving between a read and a
	// write of the same ledger is exactly what it cannot see.
	s.queues.do(identity, func() {
		res := Serve(req, s.Run)
		exit := -1
		if res.Exit != nil {
			exit = *res.Exit
		}
		s.noticef("served %s (exit %d, %dms)", strings.Join(req.Argv, " "), exit, time.Since(started).Milliseconds())
		_ = WriteResponse(conn, res)
	})
}

// noticef says what the daemon did, when anybody is listening.
func (s *Server) noticef(format string, args ...any) {
	if s.Notice == nil {
		return
	}
	fmt.Fprintf(s.Notice, "procoder serve: "+format+"\n", args...)
}

// runQueued is the runner with the repository's queue around it, for a
// job — which runs after its connection is gone and so cannot be wrapped
// by the caller.
func (s *Server) runQueued(identity string) Runner {
	return func(req Request, stdout, stderr io.Writer) (code int, result *Result) {
		s.queues.do(identity, func() { code, result = s.Run(req, stdout, stderr) })
		return code, result
	}
}

// refuse answers with an exit code and a reason on stderr, which is where
// a caller already looks for one. A refusal is never silent: a response
// with no explanation reads to a client exactly like a command that
// printed nothing, and one of those is a bug and the other is Tuesday.
func (s *Server) refuse(conn io.Writer, code int, reason string) {
	_ = WriteResponse(conn, Response{Protocol: Protocol, Exit: &code, Stderr: reason + "\n"})
}
