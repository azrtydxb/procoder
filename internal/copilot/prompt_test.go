package copilot

import (
	"os"
	"testing"
)

// /dev/null is a character device, so a mode test alone reports a terminal
// for the most common way of saying there is none. Found live:
// `version --check < /dev/null` printed the prompt and read the EOF as a
// no — the right outcome for the wrong reason, and the next caller to trust
// CanAsk would act on the wrong half.
// proved by: dropped the SameFile check — this test then reports a terminal
// on /dev/null.
func TestDevNullIsNotATerminal(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer func() { _ = null.Close() }()
	if CanAsk(null) {
		t.Error("/dev/null is nobody to ask")
	}

	// A pipe is not one either, and neither is nothing at all.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	if CanAsk(r) {
		t.Error("a pipe carries no human")
	}
	if CanAsk(nil) {
		t.Error("no file, no terminal")
	}
}
