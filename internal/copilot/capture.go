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

	// Lazily, and once: a run with nothing safe to publish must not touch
	// GitHub at all.
	labelled := false
	for _, f := range finds {
		if strings.TrimSpace(f.Body) == "" {
			// sanitisation removed everything it was given; an empty issue
			// teaches nobody and an empty entry is worse than none
			notes = append(notes, "skipped "+originOf(f)+" — nothing left after sanitisation, so there was nothing safe to publish")
			continue
		}
		// One instant for both halves, in one zone: the issue said
		// 2026-08-20T00:15+02:00 while the ledger said 2026-08-19 22:15 —
		// the same finding filed on two different days.
		when := f.Created.UTC()
		if f.Created.IsZero() {
			when = time.Now().UTC()
		}
		// The heading both halves carry, derived once: the issue and the
		// ledger entry are the same finding under two roofs.
		head := strings.TrimSpace(f.Title)
		if head == "" {
			head = firstLine(f.Body)
		}
		if bin != "" {
			if !labelled {
				notes = append(notes, ensureLabels(bin, root)...)
				labelled = true
			}
			if err := createIssue(bin, root, f, head, when); err != nil {
				notes = append(notes, "issue NOT created for "+originOf(f)+" — "+err.Error()+"; the ledger still records it")
			} else {
				issuesCreated++
			}
		}
		if err := lessons.RecordCopilotEntry(root, head, originOf(f), f.Body, when); err != nil {
			notes = append(notes, filepath.ToSlash(lessons.CopilotLeaksPath)+" NOT written for "+originOf(f)+
				" — "+err.Error()+fmt.Sprintf("; %d issue(s) created so far", issuesCreated))
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

// ensureLabels creates the two labels every captured issue carries. gh
// refuses `issue create --label` outright when a label does not exist in the
// repository ("could not add label: not found"), so on a repository adopting
// procoder the publish half fails for every finding until someone creates
// them by hand. --force makes this idempotent: existing labels are updated,
// not rejected. A failure here is reported, never fatal — the ledger is the
// memory and it is written either way.
func ensureLabels(bin, root string) []string {
	var failed []string
	var reason string
	for _, label := range []string{AutoLabel, OwnLabel} {
		ctx, cancel := context.WithTimeout(context.Background(), ghTimeout)
		cmd := exec.CommandContext(ctx, bin, "label", "create", label, // nosemgrep -- gh resolved from PATH with fixed subcommands
			"--description", "findings from Copilot auto-reviews, captured by procoder", "--force")
		cmd.Dir = root
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		cancel()
		if err != nil {
			failed = append(failed, label)
			reason = strings.TrimSpace(stderr.String())
		}
	}
	if len(failed) == 0 {
		return nil
	}
	// One note for both: a broken gh fails every label for the same reason,
	// and repeating it buries the issue failure that follows.
	return []string{"label(s) " + strings.Join(failed, ", ") + " NOT created — " + reason +
		"; create them by hand if the issue(s) below are refused"}
}

// createIssue opens one issue from an already-sanitised finding. Both labels
// are passed: `auto-copilot` is the family the finder queries, `copilot-leak`
// marks the ones procoder itself opened, so a later run cannot mistake our own
// issue for a fresh Copilot review and capture it again.
func createIssue(bin, root string, f Sanitised, title string, when time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), ghTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "issue", "create", // nosemgrep -- gh resolved from PATH with fixed subcommands
		"--title", title,
		"--body", issueBody(f, when),
		"--label", AutoLabel,
		"--label", OwnLabel)
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
func issueBody(f Sanitised, when time.Time) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(f.Body))
	b.WriteString("\n\n---\n\n")
	b.WriteString("- Source: Copilot auto-review\n")
	b.WriteString("- Original: " + originOf(f) + "\n")
	if f.Line > 0 {
		fmt.Fprintf(&b, "- Line: %d\n", f.Line)
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
