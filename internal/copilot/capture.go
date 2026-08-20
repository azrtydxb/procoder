package copilot

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"procoder/internal/lessons"
)

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
		when := f.Created
		if when.IsZero() {
			when = time.Now()
		}
		head := strings.TrimSpace(f.Title)
		if head == "" {
			head = firstLine(f.Body)
		}
		if err := lessons.RecordCopilotEntry(root, head, originOf(f), f.Body, when); err != nil {
			notes = append(notes, filepath.ToSlash(lessons.CopilotLeaksPath)+" NOT written for "+originOf(f)+
				" — "+err.Error()+fmt.Sprintf("; %d issue(s) were still created", issuesCreated))
			continue
		}
		lessonsWritten++
	}
	return issuesCreated, lessonsWritten, notes
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
