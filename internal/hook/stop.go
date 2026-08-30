package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"procoder/internal/status"
	"procoder/internal/tools"

	"procoder/internal/store"
)

// StateDir is procoder-owned, per-clone derived state — the same precedent as
// the index. HandoffFile is the note a session leaves behind for the next one.
const (
	StateDir    = store.StateDir
	HandoffFile = "handoff.md"
)

// The facts block is delimited so the writer can replace it and nothing else:
// everything below the closing marker belongs to the agent and is never
// touched. Markers, not line counts — a note that grew a paragraph must still
// survive the next rewrite.
const (
	factsOpen  = "<!-- procoder:facts -->"
	factsClose = "<!-- /procoder:facts -->"
	notesHead  = "## Notes"
)

const notesHint = `<!-- Yours. What you were doing, what you decided, what comes next.
     The facts block above is rewritten every time a session ends; this
     heading and everything under it is never touched by procoder. -->`

// stopPayload is all this hook reads from the host's Stop or PreCompact
// event: where the session was working, when the caller has no root of its
// own. Everything else in the payload is conversation, and the note records
// repository state only.
type stopPayload struct {
	Cwd string `json:"cwd"`
	// LastAssistantMessage is the final assistant text of the turn. The
	// host supplies it on Stop precisely so a hook does not read the
	// transcript, which it documents as written asynchronously and lagging
	// the current turn.
	LastAssistantMessage string `json:"last_assistant_message"`
}

// Stop writes the handoff note at the end of a session (or before a
// compaction) and says nothing: a session ending must never be noisy, and a
// failed handoff must never be the reason a session breaks. Exit is 0 on
// every path, including the ones where nothing could be written.
func Stop(stdin io.Reader, root string) int {
	raw, _ := readAll(stdin) // tolerant: any JSON, or none at all
	if root == "" {
		root = rootFromPayload(raw)
	}

	old, _ := store.LoadHandoff(root) // absent is the first handoff, not an error
	// a note that could not be written is a lost note, never a broken session
	_ = store.SaveHandoff(root, []byte(handoff(root, string(old))))

	// The note is written first, on every path including this one: a
	// blocked turn must not also lose its handoff.
	//
	// A decision put to the user in prose does not end the turn. The rule
	// shipped in 3.2.0 and was broken the same day by the agent that wrote
	// it, which is the whole argument — a rule that depends on remembering
	// fails on the day somebody is busy.
	var p stopPayload
	_ = json.Unmarshal(raw, &p)
	if blocked, reason := unaskedDecision(root, p.LastAssistantMessage); blocked {
		fmt.Fprintln(Stderr, reason)
		return 2 // the host's documented way for a Stop hook to continue the conversation
	}
	return 0
}

// Stderr is where a blocking reason goes, and a variable so a test can
// read it. The host feeds this back to the model.
var Stderr io.Writer = os.Stderr

// rootFromPayload falls back to the host's working directory, then the
// process's, so a caller with no root still writes the note in the right
// repository.
func rootFromPayload(raw []byte) string {
	var p stopPayload
	if json.Unmarshal(raw, &p) == nil && p.Cwd != "" {
		return tools.RepoRoot(p.Cwd)
	}
	if wd, err := os.Getwd(); err == nil {
		return tools.RepoRoot(wd)
	}
	return "."
}

// handoff renders the note: a freshly computed facts block, then whatever
// notes the previous note carried, verbatim. Facts only — what the session
// was trying to do is the agent's line to write, never the binary's guess.
func handoff(root, old string) string {
	var b strings.Builder
	b.WriteString("# procoder handoff\n\n")
	b.WriteString(factsOpen + "\n")
	b.WriteString("generated: " + time.Now().UTC().Format(time.RFC3339) + "\n")
	for _, line := range status.Report(root) {
		b.WriteString(line + "\n")
	}
	b.WriteString(factsClose + "\n\n")
	b.WriteString(preservedNotes(old))
	return b.String()
}

// preservedNotes recovers the agent's half of the previous note. Normally
// that is everything after the closing marker; when the markers were
// hand-deleted the whole file is rewritten, and the notes section is whatever
// can still be found from its heading down. Nothing the agent wrote is ever
// dropped — when in doubt the text is kept.
func preservedNotes(old string) string {
	notes := ""
	if i := strings.Index(old, factsClose); i >= 0 {
		notes = strings.TrimLeft(old[i+len(factsClose):], "\n")
	} else if i := strings.Index(old, notesHead); i >= 0 {
		notes = old[i:]
	}
	if strings.TrimSpace(notes) == "" {
		return notesHead + "\n\n" + notesHint + "\n"
	}
	if !strings.HasSuffix(notes, "\n") {
		notes += "\n"
	}
	return notes
}
