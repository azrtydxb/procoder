// Package copilot captures what Copilot's auto-review found before it is
// fixed and forgotten. A finding that is read, patched, and never recorded
// teaches nobody: the class of defect escapes again, and the layer that
// should have caught it never adapts.
//
// Two rules shape everything here. Nothing reaches GitHub or the ledger
// until it has been through Sanitise — the user's code is theirs, and a
// finding is metadata about a failure, not a copy of the source that
// failed. And nothing acts without being asked: the prompt defaults to no,
// including when there is nobody there to answer.
package copilot

import "time"

// Finding is one Copilot auto-review issue as GitHub returned it, before
// any of it is fit to leave the machine.
type Finding struct {
	OriginalURL string
	Title       string
	Body        string
	Line        int
	Repo        string
	Created     time.Time
}

// Sanitised is a Finding with the user's code taken out of it: what is left
// describes the defect, where it was, and what to do about it. This is the
// only shape that may be published or written to the ledger.
type Sanitised struct {
	Title       string
	Body        string
	Line        int
	OriginalURL string
	Created     time.Time
}
