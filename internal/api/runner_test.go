package api

import (
	"fmt"
	"io"
	"testing"
)

// Serve keeps the two streams apart, because the domains already do:
// store.Notice, principles.Stderr and the stop hook's writer are separate
// channels, and merging them at the transport would lose the distinction
// the hook contract depends on.
//
// proved by: writing both into one buffer — a hook's context then arrives
// with its lock notices spliced into it.
func TestServeCapturesBothStreams(t *testing.T) {
	res := Serve(Request{Protocol: Protocol, Argv: []string{"anything"}},
		func(_ Request, stdout, stderr io.Writer) (int, *Result) {
			fmt.Fprint(stdout, "out")
			fmt.Fprint(stderr, "err")
			return 3, nil
		})
	if res.Stdout != "out" || res.Stderr != "err" {
		t.Fatalf("the streams were not kept apart: stdout %q stderr %q", res.Stdout, res.Stderr)
	}
	if res.Exit == nil || *res.Exit != 3 {
		t.Fatalf("the exit code did not survive: %v", res.Exit)
	}
	if res.Protocol != Protocol {
		t.Fatalf("the response carries no protocol version: %d", res.Protocol)
	}
}

// A command that reports no findings carries no result at all, which is
// not the same as reporting an empty list.
func TestServeWithoutFindingsCarriesNoResult(t *testing.T) {
	res := Serve(Request{Protocol: Protocol}, func(Request, io.Writer, io.Writer) (int, *Result) { return 0, nil })
	if res.Result != nil {
		t.Fatalf("a command that reports nothing returned a result: %+v", res.Result)
	}
}
