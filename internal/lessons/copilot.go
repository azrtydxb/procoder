package lessons

// The Copilot leak ledger lives here rather than in internal/copilot for one
// reason: internal/copilot is the side that reaches GitHub, and it writes what
// it captured through this package. If this package imported it back for the
// Sanitised type, the two would form an import cycle — so the append below
// takes the fields it needs as plain values. The caller unpacks Sanitised.
//
// The ledger itself is deliberately NOT LESSONS.md. A raw Copilot finding is
// not yet a lesson: somebody has to name the failure class and the adaptation
// that closes it. Merging the two files would skip that step and make the
// lessons gate block on notes nobody has read yet, so this is a separate
// report over a separate file, and `procoder lessons` never sees these.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"procoder/internal/gitx"
)

// CopilotLeaksPath is the scratch ledger of Copilot auto-review findings,
// beside the other github rules files under D-HOME.
const CopilotLeaksPath = ".procoder/github/COPILOT-LEAKS.md"

// copilotHeader is written once, when the ledger is first created, so the file
// explains itself to the human who has to classify what is in it. Parse
// ignores prose, and nothing here starts a line with "## ".
const copilotHeader = `# Copilot leaks — auto-review findings awaiting classification

One entry per finding Copilot's auto-review caught, sanitised: metadata about
a failure, never the source that failed. An entry becomes a real lesson in
` + "`" + Path + "`" + ` only after a human names its class and its adaptation.
`

// RunCopilotLeaks prints the leak ledger's state and flags entries nobody has
// turned into an adaptation yet.
func RunCopilotLeaks(root string, out func(string)) int {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(CopilotLeaksPath)))
	if os.IsNotExist(err) {
		// nothing has been captured, which is the ordinary state — the ledger
		// is written by `procoder copilot-leak`, not by a template, so there
		// is no shape to print and nothing to say
		return 0
	}
	if err != nil {
		// unknown is never done: a ledger we cannot read is not an empty one
		out("copilot leak ledger NOT checked: " + err.Error())
		return 2
	}
	entries := Parse(string(raw))
	if len(entries) == 0 {
		out("copilot leak ledger has no entries yet — nothing escaped, or nothing was captured")
		return 0
	}
	unlearned := 0
	for _, e := range entries {
		if isUnlearned(e.Adaptation) {
			unlearned++
			out("  UNLEARNED  " + e.Title + " — captured but not classified; no adaptation recorded")
		} else {
			out("  learned    " + e.Title)
		}
	}
	out(fmt.Sprintf("procoder copilot-leak: %d finding(s), %d unlearned", len(entries), unlearned))
	if unlearned > 0 {
		return 1
	}
	return 0
}

// RecordCopilotEntry appends one captured finding to the leak ledger, with the
// adaptation left as a placeholder — the entry reads as unlearned until a
// human replaces it. title, url and finding must already be sanitised; this
// package never sees the raw Copilot body.
func RecordCopilotEntry(root, title, url, finding string, at time.Time) error {
	path := filepath.Join(root, filepath.FromSlash(CopilotLeaksPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // the checked close is below; this one only covers the error paths
	st, err := f.Stat()
	if err != nil {
		return err
	}
	var b strings.Builder
	if st.Size() == 0 {
		b.WriteString(copilotHeader)
	}
	b.WriteString("\n## " + at.UTC().Format("2006-01-02 15:04") + " " + oneLine(url) +
		" — " + oneLine(title) + "\n\n")
	b.WriteString("- Source: Copilot auto-review\n")
	b.WriteString("- Original: " + oneLine(url) + "\n")
	b.WriteString("- Finding: " + oneLine(finding) + "\n")
	b.WriteString("- Adaptation: <the concrete change that catches this class from now on>\n")
	if _, err := f.WriteString(b.String()); err != nil {
		return err
	}
	return f.Close()
}

// oneLine flattens a value onto a single ledger line. A finding body carries
// whatever Copilot wrote, and a line of it beginning "## " would parse as a
// second entry — one leak would silently become two, the extra one titled
// with someone else's prose.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// LeakReminder is the gate's half of the Copilot loop, and it is deliberately
// the offline half: it reads the ledger already on disk and says how many
// captured findings still carry no adaptation. It never asks GitHub. The gate
// runs on every commit, in CI, and on aeroplanes; a network call there would
// tax every commit for a question that is not urgent and would report NOT
// checked whenever gh was unavailable.
//
// Finding leaks that GitHub knows about and this repository does not is
// `procoder copilot-leak`'s job, and the merge flow runs it — at the moment
// the work is being reflected on, which is when the answer is worth having.
//
// The reminder never blocks: an unwritten adaptation is work to do, not a
// broken tree.
func LeakReminder(root string) []gitx.Finding {
	path := filepath.Join(root, filepath.FromSlash(CopilotLeaksPath))
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil // no ledger is the ordinary case, not a finding
	}
	if err != nil {
		return []gitx.Finding{{File: path,
			Message: CopilotLeaksPath + " NOT checked (" + err.Error() + ") — captured Copilot findings cannot be counted"}}
	}
	unlearned := 0
	for _, e := range Parse(string(raw)) {
		if isUnlearned(e.Adaptation) {
			unlearned++
		}
	}
	if unlearned == 0 {
		return nil
	}
	return []gitx.Finding{{File: path,
		Message: fmt.Sprintf("%d captured Copilot finding(s) in %s carry no adaptation — the class stays open until one is written",
			unlearned, CopilotLeaksPath)}}
}
