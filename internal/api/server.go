package api

import (
	"fmt"
	"io"
	"net"
	"os"
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
	// Notice is where the server says what it did. Stderr by default; a
	// test points it somewhere it can read.
	Notice io.Writer
	// Identity names the repository a request is about, so requests
	// against one are queued rather than run at once. Supplied rather than
	// computed here: the identity ladder lives in internal/store, which
	// this package would otherwise have to import to answer a transport
	// question.
	Identity func(cwd string) string

	queues queues
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

// Accept serves until the listener is closed.
func (s *Server) Accept(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return // the listener was closed; nothing else ends this loop
		}
		go s.serveConn(conn)
	}
}

// serveConn answers one request. One request per connection: the client
// makes one call and reads one answer, and a connection that carried more
// would need a correlation id to say which answer belonged to which.
func (s *Server) serveConn(conn net.Conn) {
	defer conn.Close()

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
	identity := ""
	if s.Identity != nil {
		identity = s.Identity(req.Cwd)
	}
	// Held across the whole command, not just its writes: the store's lock
	// is per file, and two requests interleaving between a read and a
	// write of the same ledger is exactly what it cannot see.
	s.queues.do(identity, func() {
		_ = WriteResponse(conn, Serve(req, s.Run))
	})
}

// refuse answers with an exit code and a reason on stderr, which is where
// a caller already looks for one. A refusal is never silent: a response
// with no explanation reads to a client exactly like a command that
// printed nothing, and one of those is a bug and the other is Tuesday.
func (s *Server) refuse(conn io.Writer, code int, reason string) {
	_ = WriteResponse(conn, Response{Protocol: Protocol, Exit: &code, Stderr: reason + "\n"})
}
