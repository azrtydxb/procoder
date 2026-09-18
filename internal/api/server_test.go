package api

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// shortDir is a temporary directory with a SHORT name.
//
// A unix socket path is capped at around 104 bytes on macOS and 108 on
// Linux, and t.TempDir() spends most of that on the test's own name —
// TestProtocolSkewIsRefusedWithAReason bound fine as a directory and
// failed as a socket, with "bind: invalid argument" and nothing saying
// why. The real path, ~/.procoder/run/procoder.sock, is nowhere near the
// cap; this is a test-fixture problem and it is solved here rather than by
// shortening the tests' names.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "pc")
	if err != nil {
		t.Fatalf("could not make a short temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// requireDaemon skips a test that needs a running daemon.
//
// The daemon does not run on Windows — Listen refuses, because the
// socket's permission bits are its only authentication and Windows cannot
// set them. Every test that opens one is therefore about a thing that
// platform does not have. What Windows DOES have is asserted:
// TestWindowsRefusesToServe checks the refusal, and every test in this
// package that does not need a socket keeps running there.
func requireDaemon(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the daemon does not run on Windows — see TestWindowsRefusesToServe")
	}
}

// testServer starts a server on a socket inside dir and returns its path.
func testServer(t *testing.T, run Runner) (string, *Server) {
	t.Helper()
	requireDaemon(t)
	path := filepath.Join(shortDir(t), "s.sock")
	srv := &Server{Run: run, Version: "test", Notice: io.Discard}
	l, err := srv.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go srv.Accept(l)
	t.Cleanup(func() { l.Close() })
	return path, srv
}

// The socket is openable by its user and by nobody else.
//
// net.Listen applies the process umask, which on a default umask leaves a
// world-connectable socket — the one thing this design must not be, since
// the permission bits are the whole authentication.
//
// proved by: removing the chmod — the socket comes back 0755 under a
// default umask and every user on the machine can drive procoder.
func TestServeSocketPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the daemon refuses to run on Windows — see TestWindowsRefusesToServe")
	}
	dir := shortDir(t)
	path := filepath.Join(dir, "s.sock")
	srv := &Server{Run: func(Request, io.Writer, io.Writer) (int, *Result) { return 0, nil }}
	l, err := srv.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the socket is not there: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("socket mode is %04o, want 0600", perm)
	}
}

// A socket left behind by a daemon that died is replaced, not refused.
func TestListenClearsAStaleSocket(t *testing.T) {
	requireDaemon(t)
	dir := shortDir(t)
	path := filepath.Join(dir, "s.sock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	srv := &Server{Run: func(Request, io.Writer, io.Writer) (int, *Result) { return 0, nil }}
	l, err := srv.Listen(path)
	if err != nil {
		t.Fatalf("a stale socket was not cleared: %v", err)
	}
	l.Close()
}

// The same command answers the same over the socket as it does in
// process. This is the transport's whole job.
func TestSocketExitMatchesInProcess(t *testing.T) {
	run := func(req Request, stdout, stderr io.Writer) (int, *Result) {
		io.WriteString(stdout, "argv="+req.Argv[0])
		io.WriteString(stderr, "cwd="+req.Cwd)
		return 7, &Result{Kind: KindFindings, Findings: []Finding{}}
	}
	path, _ := testServer(t, run)

	direct := Serve(Request{Protocol: Protocol, Argv: []string{"check"}, Cwd: "/x"}, run)

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	if err := WriteRequest(conn, Request{Protocol: Protocol, Argv: []string{"check"}, Cwd: "/x"}); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}
	over, err := ReadResponse(conn)
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}

	if over.Stdout != direct.Stdout || over.Stderr != direct.Stderr {
		t.Fatalf("the socket changed the bytes:\n direct: %q / %q\n socket: %q / %q",
			direct.Stdout, direct.Stderr, over.Stdout, over.Stderr)
	}
	if over.Exit == nil || *over.Exit != *direct.Exit {
		t.Fatalf("exit codes differ: %v vs %v", over.Exit, direct.Exit)
	}
	if over.Result == nil || over.Result.Kind != KindFindings {
		t.Fatalf("the result did not survive the socket: %+v", over.Result)
	}
}

// A client speaking another protocol is refused with a reason, never
// served stale behaviour and never answered with silence.
func TestProtocolSkewIsRefusedWithAReason(t *testing.T) {
	path, _ := testServer(t, func(Request, io.Writer, io.Writer) (int, *Result) {
		t.Error("the command ran for a client speaking another protocol")
		return 0, nil
	})
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := WriteRequest(conn, Request{Protocol: Protocol + 1, Argv: []string{"check"}}); err != nil {
		t.Fatal(err)
	}
	res, err := ReadResponse(conn)
	if err != nil {
		t.Fatal(err)
	}
	if res.Exit == nil || *res.Exit != 2 {
		t.Fatalf("want exit 2, got %v", res.Exit)
	}
	if res.Stderr == "" {
		t.Fatal("the refusal said nothing — a client cannot tell it from a command that printed nothing")
	}
}

// Windows does not get a daemon, because it cannot secure the socket.
//
// The permission bits are this design's only authentication, and os.Chmod
// on Windows sets one thing: the read-only bit. A socket created there
// comes back 0666 — openable by every account on the machine — which is
// the one thing this design must never be. Refusing costs Windows nothing
// it has today: every command runs in-process, which is the whole of
// procoder.
//
// proved by: dropping the GOOS check in Listen — the Windows CI job goes
// back to reporting "socket mode is 0666, want 0600", which is a daemon
// anyone on the box can drive.
func TestWindowsRefusesToServe(t *testing.T) {
	srv := &Server{Run: func(Request, io.Writer, io.Writer) (int, *Result) { return 0, nil }, Notice: io.Discard}
	l, err := srv.Listen(filepath.Join(shortDir(t), "s.sock"))
	if runtime.GOOS != "windows" {
		if err != nil {
			t.Fatalf("Listen failed where it should work: %v", err)
		}
		l.Close()
		return
	}
	if err == nil {
		l.Close()
		t.Fatal("Windows served on a socket it cannot secure")
	}
	if !strings.Contains(err.Error(), "Windows") || !strings.Contains(err.Error(), "in-process") {
		t.Errorf("the refusal must say why and what still works: %v", err)
	}
}

// A second daemon does not steal the first one's socket.
//
// Listen removed any file at the path before binding. If a daemon was
// already listening, that orphaned it: still running, holding a socket
// nothing can reach, while every client silently talked to the second.
//
// proved by: removing the listening() check in Listen — the first
// server's connection below stops being answered and the test's second
// Listen succeeds.
func TestASecondDaemonDoesNotStealTheSocket(t *testing.T) {
	requireDaemon(t)
	path := filepath.Join(shortDir(t), "s.sock")
	first := &Server{Run: func(Request, io.Writer, io.Writer) (int, *Result) { return 7, nil }, Notice: io.Discard}
	l, err := first.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	go first.Accept(l)

	second := &Server{Run: func(Request, io.Writer, io.Writer) (int, *Result) { return 0, nil }, Notice: io.Discard}
	if l2, err := second.Listen(path); err == nil {
		l2.Close()
		t.Fatal("a second daemon took the socket out from under a live one")
	} else if !strings.Contains(err.Error(), "already listening") {
		t.Errorf("the refusal does not say what is in the way: %v", err)
	}

	// The first daemon is still reachable, which is the point.
	res, err := Client{Path: path}.Do(Request{Argv: []string{"check"}})
	if err != nil {
		t.Fatalf("the original daemon was orphaned: %v", err)
	}
	if res.Exit == nil || *res.Exit != 7 {
		t.Fatalf("a different daemon answered: %v", res.Exit)
	}
}

// A daemon from another build is refused even when the protocol matches.
//
// The protocol can be identical between two releases whose behaviour is
// not, and that is the skew worth catching. The first version compared
// only the protocol and left Client.Version unused and Server.Version
// never sent, so two different builds were treated as compatible.
//
// proved by: dropping the version comparison in Client.Do — this test's
// mismatched daemon serves the request.
func TestADifferentBuildIsRefusedOnTheSameProtocol(t *testing.T) {
	requireDaemon(t)
	path := filepath.Join(shortDir(t), "s.sock")
	srv := &Server{
		Run:     func(Request, io.Writer, io.Writer) (int, *Result) { return 0, nil },
		Version: "3.5.0",
		Notice:  io.Discard,
	}
	l, err := srv.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go srv.Accept(l)

	_, err = Client{Path: path, Version: "3.6.0"}.Do(Request{Argv: []string{"check"}})
	if !errors.Is(err, ErrVersionSkew) {
		t.Fatalf("want ErrVersionSkew across builds, got %v", err)
	}
	if !strings.Contains(err.Error(), "3.5.0") || !strings.Contains(err.Error(), "3.6.0") {
		t.Errorf("the refusal names neither build: %v", err)
	}

	// Same build: served.
	if _, err := (Client{Path: path, Version: "3.5.0"}).Do(Request{Argv: []string{"check"}}); err != nil {
		t.Fatalf("a matching build was refused: %v", err)
	}
}

// A response may be far larger than a request. They are not the same
// risk: a request is what an unknown caller sends, a response is this
// daemon's own output, and `procoder audit` legitimately produces
// megabytes of it.
//
// proved by: capping the response read at MaxRequestBytes again — a big
// command's real answer is refused, with an error calling it a request.
func TestALargeResponseIsNotRefusedAsARequest(t *testing.T) {
	big := strings.Repeat("finding\n", (MaxRequestBytes/8)+1024)
	path, _ := testServer(t, func(_ Request, stdout, _ io.Writer) (int, *Result) {
		io.WriteString(stdout, big)
		return 0, nil
	})
	// `check`, not `audit`: audit answers with a job id rather than
	// output, so it would prove nothing about response size.
	res, err := Client{Path: path}.Do(Request{Argv: []string{"check"}})
	if err != nil {
		t.Fatalf("a large answer was refused: %v", err)
	}
	if len(res.Stdout) != len(big) {
		t.Fatalf("the answer was truncated: got %d bytes, want %d", len(res.Stdout), len(big))
	}
}
