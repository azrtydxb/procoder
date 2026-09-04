package api

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A daemon speaking another protocol is not used, and the command has not
// run. The caller is told; nothing is run behind its back.
//
// proved by: having Do return the response anyway — the caller then gets
// another release's behaviour with nothing saying so.
func TestVersionSkewIsRefused(t *testing.T) {
	path := filepath.Join(shortDir(t), "s.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	// A daemon from another build: it answers with a protocol this one
	// does not speak.
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := ReadRequest(conn); err != nil {
			return
		}
		code := 0
		_ = WriteResponse(conn, Response{Protocol: Protocol + 1, Exit: &code, Stdout: "from the future"})
	}()

	_, err = Client{Path: path, Version: "1.0.0"}.Do(Request{Argv: []string{"check"}})
	if !errors.Is(err, ErrVersionSkew) {
		t.Fatalf("want ErrVersionSkew, got %v", err)
	}
}

// A socket file with nothing behind it is reported quickly rather than
// waited on: the command is not going to run, and the caller should hear
// that in milliseconds.
func TestDeadSocketCostsNothing(t *testing.T) {
	path := filepath.Join(shortDir(t), "s.sock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err := Client{Path: path}.Do(Request{Argv: []string{"check"}})
	if !errors.Is(err, ErrNoDaemon) {
		t.Fatalf("want ErrNoDaemon, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("a dead socket cost %v — a hook would lose its session to that", elapsed)
	}
}

// An absent socket is the ordinary case on a machine with no daemon, and
// it is not an error worth printing twice.
func TestAbsentSocketIsNoDaemon(t *testing.T) {
	_, err := Client{Path: filepath.Join(shortDir(t), "nothing.sock")}.Do(Request{Argv: []string{"check"}})
	if !errors.Is(err, ErrNoDaemon) {
		t.Fatalf("want ErrNoDaemon, got %v", err)
	}
}

// The round trip carries everything a command answered with.
func TestClientCarriesTheWholeResponse(t *testing.T) {
	path, _ := testServer(t, func(req Request, stdout, stderr io.Writer) (int, *Result) {
		io.WriteString(stdout, "o")
		io.WriteString(stderr, "e")
		return 4, &Result{Kind: KindFindings, Findings: []Finding{{File: "a.go", Domain: "lint"}}}
	})
	res, err := Client{Path: path}.Do(Request{Argv: []string{"lint"}, Cwd: "/x"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if res.Stdout != "o" || res.Stderr != "e" || res.Exit == nil || *res.Exit != 4 {
		t.Fatalf("the response did not survive: %+v", res)
	}
	if res.Result == nil || len(res.Result.Findings) != 1 || res.Result.Findings[0].Domain != "lint" {
		t.Fatalf("the result did not survive: %+v", res.Result)
	}
}
