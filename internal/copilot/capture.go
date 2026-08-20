package copilot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LedgerPath is the copilot ledger, beside LESSONS.md under D-HOME. It is
// deliberately NOT the lessons ledger: a raw Copilot finding is not yet a
// lesson — a human still has to name the class and the adaptation — and
// merging the two would let unclassified notes fail the lessons check.
const LedgerPath = ".procoder/github/COPILOT-LEAKS.md"

// unlearnedAdaptation is what every captured entry records instead of an
// adaptation. lessons.Parse reads the text after "Adaptation:", and
// lessons.Run counts an adaptation starting with "<" as UNLEARNED — so this
// placeholder makes a captured leak read as an open class until a human
// replaces it. The angle bracket is the contract; the words are for the human.
const unlearnedAdaptation = "<the concrete change that catches this class from now on>"

// ghTimeout bounds one `gh issue create`: a hung network call must not hold
// the capture open, and a capture that never returns loses the ledger too.
const ghTimeout = 60 * time.Second

// Capture publishes each sanitised finding as a GitHub issue and records it in
// the ledger. Only ever called after an explicit yes — see Prompt.
//
// The two halves are independent ON PURPOSE. Issue creation can fail (rate
// limit, expired auth) and the ledger must still be written, because the
// ledger is the memory; the issue is only its announcement. The ledger can be
// unwritable (read-only tree, a directory in the way) and the issues must
// still be created, for the same reason in reverse. Every failure of either
// half is reported through notes — a swallowed failure is a leak lost twice.
func Capture(finds []Sanitised, root string) (issuesCreated, lessonsWritten int, notes []string) {
	bin, err := exec.LookPath("gh")
	if err != nil {
		// not fatal: the ledger below is still worth writing
		notes = append(notes, "no issue created — gh is not installed (https://cli.github.com); the ledger still records the finding(s)")
		bin = ""
	}

	var entries []string
	for _, f := range finds {
		if strings.TrimSpace(f.Body) == "" {
			// sanitisation removed everything it was given; an empty issue
			// teaches nobody and an empty entry is worse than none
			notes = append(notes, "skipped "+originOf(f)+" — nothing left after sanitisation, so there was nothing safe to publish")
			continue
		}
		if bin != "" {
			if err := createIssue(bin, root, f); err != nil {
				notes = append(notes, "issue NOT created for "+originOf(f)+" — "+err.Error()+"; the ledger still records it")
			} else {
				issuesCreated++
			}
		}
		entries = append(entries, entry(f))
	}
	if len(entries) == 0 {
		return issuesCreated, 0, notes
	}
	if err := appendLedger(root, entries); err != nil {
		notes = append(notes, filepath.ToSlash(LedgerPath)+" NOT written — "+err.Error()+
			fmt.Sprintf("; %d issue(s) were still created", issuesCreated))
		return issuesCreated, 0, notes
	}
	return issuesCreated, len(entries), notes
}

// entry is one ledger record, in LESSONS.md's shape so lessons.Parse reads it
// without a second parser: a "## " heading, then the dashed fields.
func entry(f Sanitised) string {
	when := f.Created
	if when.IsZero() {
		when = time.Now()
	}
	head := strings.TrimSpace(f.Title)
	if head == "" {
		head = firstLine(f.Body)
	}
	origin := originOf(f)
	return fmt.Sprintf("## %s %s — %s\n\n- Source: Copilot auto-review\n- Original: %s\n- Adaptation: %s\n",
		when.Format("2006-01-02"), origin, head, origin, unlearnedAdaptation)
}

// originOf names the finding in messages and in the entry heading. A finding
// with no URL still gets an entry — losing it because GitHub gave us no link
// would be the failure this whole package exists to prevent.
func originOf(f Sanitised) string {
	if u := strings.TrimSpace(f.OriginalURL); u != "" {
		return u
	}
	return "(no original issue URL)"
}

func firstLine(body string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(body), "\n")
	return strings.TrimSpace(line)
}

// createIssue opens one issue from an already-sanitised finding. Both labels
// are passed: `auto-copilot` is the family the finder queries, `copilot-leak`
// marks the ones procoder itself opened, so a later run cannot mistake our own
// issue for a fresh Copilot review and capture it again.
func createIssue(bin, root string, f Sanitised) error {
	ctx, cancel := context.WithTimeout(context.Background(), ghTimeout)
	defer cancel()
	title := strings.TrimSpace(f.Title)
	if title == "" {
		title = firstLine(f.Body)
	}
	cmd := exec.CommandContext(ctx, bin, "issue", "create", // nosemgrep -- gh resolved from PATH with fixed subcommands
		"--title", title,
		"--body", issueBody(f),
		"--label", "auto-copilot",
		"--label", "copilot-leak")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("gh gave no answer in %s; the process was killed", ghTimeout)
	}
	if err != nil {
		return fmt.Errorf("%s", ghError(stderr.String()+stdout.String(), err))
	}
	return nil
}

// issueBody is the sanitised text plus its provenance. Nothing is added that
// the sanitiser has not already seen — the body must stay code-free.
func issueBody(f Sanitised) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(f.Body))
	b.WriteString("\n\n---\n\n")
	b.WriteString("- Source: Copilot auto-review\n")
	b.WriteString("- Original: " + originOf(f) + "\n")
	if f.Line > 0 {
		fmt.Fprintf(&b, "- Line: %d\n", f.Line)
	}
	when := f.Created
	if when.IsZero() {
		when = time.Now()
	}
	b.WriteString("- Captured: " + when.Format(time.RFC3339) + "\n")
	return b.String()
}

// ghError picks gh's own first line, keeping the `gh auth login` hint when it
// offered one — the fix belongs next to the reason.
func ghError(raw string, err error) string {
	first, hint := "", ""
	for _, l := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if first == "" {
			first = t
		}
		if strings.Contains(t, "gh auth login") {
			hint = t
		}
	}
	switch {
	case hint != "" && hint != first:
		return first + " (" + hint + ")"
	case first != "":
		return first
	default:
		return err.Error()
	}
}

// appendLedger adds the entries to the ledger, creating it (and its directory)
// with a header when it does not exist yet. Append, never rewrite: the ledger
// is history, and this binary must not be able to lose an older entry.
func appendLedger(root string, entries []string) error {
	path := filepath.Join(root, filepath.FromSlash(LedgerPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fresh := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fresh = true
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // the checked close is below; this one only covers the error paths
	var b strings.Builder
	if fresh {
		b.WriteString(ledgerHeader)
	}
	for _, e := range entries {
		b.WriteString("\n" + e)
	}
	if _, err := f.WriteString(b.String()); err != nil {
		return err
	}
	return f.Close()
}

// ledgerHeader introduces a fresh ledger. It carries no "## " line: Parse
// treats every one of those as an entry, so an example heading here would
// make an empty ledger report a phantom unlearned lesson.
const ledgerHeader = `# Copilot leaks — findings from Copilot auto-reviews

One entry per finding captured by ` + "`procoder copilot-leak`" + `, sanitised: no
user code, no secrets, no absolute paths. Each entry stays UNLEARNED until a
human classifies it and writes the adaptation that closes its class — then it
becomes a real entry in LESSONS.md.
`
