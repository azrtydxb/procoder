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

// An answer the caller already has counts as somebody being there, and a
// terminal is not the only way to be there.
//
// Over a socket there is no character device and never will be, so the
// terminal test is the right answer to the wrong question: what matters is
// whether there is an answer, not whether there is a tty.
//
// proved by: having CanAskWith ignore supplied — every asking command then
// takes the non-interactive path for a caller who had an answer, silently.
func TestSuppliedAnswerCountsAsAsking(t *testing.T) {
	yes, no := "yes", "no"
	if !CanAskWith(nil, &yes) {
		t.Error("a supplied yes is not being counted as an answer")
	}
	if !CanAskWith(nil, &no) {
		t.Error("a supplied no is a person declining, which is still an answer")
	}
	if CanAskWith(nil, nil) {
		t.Error("no terminal and no supplied answer is nobody there")
	}
	if !ReadYesFrom(nil, &yes) || ReadYesFrom(nil, &no) {
		t.Error("the supplied answer is not read through the one definition of consent")
	}
	if ReadYesFrom(nil, nil) {
		t.Error("no answer is not a yes")
	}
}
