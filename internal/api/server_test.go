package api

import (
	"io"
	"net"
	"os"
	"path/filepath"
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

// testServer starts a server on a socket inside dir and returns its path.
func testServer(t *testing.T, run Runner) (string, *Server) {
	t.Helper()
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
