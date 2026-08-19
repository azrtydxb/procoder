// Package hook is the PostToolUse handler: the forced task of the formatting
// domain. It reads the tool payload, checks the file that was just written,
// and — per P-CONTROL — hands the agent the cleanly formatted code, never
// touching the file itself.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"path/filepath"

	"procoder/internal/actions"
	"procoder/internal/codeindex"
	"procoder/internal/config"
	"procoder/internal/docs"
	"procoder/internal/format"
	"procoder/internal/lint"
	"procoder/internal/security"
	"procoder/internal/tools"
)

type payload struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// The host writes the payload and closes the pipe. A pipe held open with no
// data must not hold the session open with it — the previous implementation's
// deadline only covered EAGAIN and a SessionStart hook was observed blocked
// for 31 minutes. Here the read runs in a goroutine and the select gives it a
// hard wall.
const stdinDeadline = 5 * time.Second

// Formatted output larger than this is not injected into the agent's context —
// a giant paste displaces the work the agent was doing. The agent is told the
// file is unformatted and where to get the full result instead. The threshold
// is generous: real source files the hook sees are almost always far below it.
const maxInlineBytes = 48 * 1024

// Run handles one PostToolUse event end to end. It never returns an error to
// the host: a broken hook must not break the user's session, so every failure
// path degrades to empty output.
func Run(stdin io.Reader, stdout io.Writer) int {
	raw, ok := readAll(stdin)
	if !ok {
		return 0 // could not read the payload inside the deadline; stay silent
	}
	var p payload
	if json.Unmarshal(raw, &p) != nil || p.ToolInput.FilePath == "" {
		return 0
	}

	var parts []string
	add := func(part string) {
		if part != "" {
			parts = append(parts, part)
		}
	}

	add(message(format.Check(p.ToolInput.FilePath)))
	add(workflowPart(p.ToolInput.FilePath))

	root := tools.RepoRoot(filepath.Dir(p.ToolInput.FilePath))
	// keep the code index current for the file just written — maintenance of
	// procoder-owned derived state, only when an index already exists
	codeindex.Refresh(root, p.ToolInput.FilePath)

	if docs.IsMarkdownFile(p.ToolInput.FilePath) {
		add(markdownPart(root, p.ToolInput.FilePath))
	} else {
		add(driftPart(root, p.ToolInput.FilePath))
		add(secretsPart(root, p.ToolInput.FilePath))
		add(lintPart(root, p.ToolInput.FilePath))
	}
	msg := strings.Join(parts, "\n\n")

	if msg == "" {
		return 0
	}
	var out hookOutput
	out.HookSpecificOutput.HookEventName = "PostToolUse"
	out.HookSpecificOutput.AdditionalContext = msg
	enc, err := json.Marshal(out)
	if err != nil {
		return 0
	}
	fmt.Fprintln(stdout, string(enc))
	return 0
}

// workflowPart runs actionlint on a written workflow file — the write that
// introduced a broken workflow is the cheapest moment to hear about it.
func workflowPart(file string) string {
	if !actions.IsWorkflowFile(file) {
		return ""
	}
	findings := actions.Lint([]string{file})
	if len(findings) == 0 {
		return ""
	}
	var b []string
	for _, f := range findings {
		b = append(b, fmt.Sprintf("  line %d: %s", f.Line, f.Message))
	}
	return fmt.Sprintf("procoder [actions]: %s has workflow problems (actionlint) — investigate and fix:\n%s",
		file, strings.Join(b, "\n"))
}

// markdownPart checks a written Markdown file's references and diagrams.
func markdownPart(root, file string) string {
	findings := docs.CheckFile(root, file)
	if len(findings) == 0 {
		return ""
	}
	var b []string
	for _, f := range findings {
		b = append(b, fmt.Sprintf("  line %d: %s", f.Line, f.Message))
	}
	return fmt.Sprintf("procoder [docs]: %s has documentation problems — investigate and fix:\n%s",
		file, strings.Join(b, "\n"))
}

// driftPart reports which docs mention the code file just changed, so the
// prose is verified before the work is called done.
func driftPart(root, file string) string {
	drift := docs.Drift(root, []string{file})
	if len(drift) == 0 {
		return ""
	}
	var b []string
	for _, f := range drift {
		b = append(b, "  "+f.File+": "+f.Message)
	}
	return "procoder [docs]: documentation mentions the file you just changed — verify it is still true, update it if not:\n" + strings.Join(b, "\n")
}

// secretsPart catches a secret at the moment it lands in a file — the
// cheapest possible time, before it reaches history.
func secretsPart(root, file string) string {
	var b []string
	for _, f := range security.SecretsChangedFiles(root, []string{file}) {
		if f.Line > 0 {
			b = append(b, fmt.Sprintf("procoder [security]: %s:%d: %s", f.File, f.Line, f.Message))
		}
	}
	return strings.Join(b, "\n")
}

// lintPart runs the file's canonical linter in the same turn — findings are
// diagnoses; the agent judges. Tool-missing and out-of-scope notes (Line 0)
// stay out: nagging every write is noise; gate and doctor say it once.
func lintPart(root, file string) string {
	var b []string
	for _, f := range lint.Files(root, []string{file}, config.Load(root).LintBlock) {
		if f.Line > 0 {
			b = append(b, fmt.Sprintf("  %s:%d: %s", f.File, f.Line, f.Message))
		}
	}
	if len(b) == 0 {
		return ""
	}
	return "procoder [lint]: findings in the file you just wrote — investigate, judge, and fix what is real:\n" + strings.Join(b, "\n")
}

// message turns a Result into what the agent is told. Clean and OutOfScope are
// silent here — the write hook speaks only when there is something to act on;
// the counting of skipped files is `procoder check`'s job, where a human reads
// the totals.
func message(res format.Result) string {
	switch res.Verdict {
	case format.Unformatted:
		if len(res.Formatted) > maxInlineBytes {
			return fmt.Sprintf(
				"procoder [format]: %s is not formatted (%s). The formatted result is %d bytes — too large to inline. Run `procoder format %q` to see it, then write it.",
				res.File, res.Tool, len(res.Formatted), res.File)
		}
		return fmt.Sprintf(
			"procoder [format]: %s is not formatted. Below is the output of %s for this file — review it and write it to the file (the file itself was NOT modified):\n\n%s",
			res.File, res.Tool, res.Formatted)
	case format.Unchecked:
		// Never let "could not check" read as clean.
		return fmt.Sprintf("procoder [format]: %s was NOT checked — %s", res.File, res.Reason)
	default:
		return ""
	}
}

func readAll(r io.Reader) ([]byte, bool) {
	type readResult struct {
		data []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(r, 4<<20))
		ch <- readResult{data, err}
	}()
	select {
	case res := <-ch:
		return res.data, res.err == nil
	case <-time.After(stdinDeadline):
		fmt.Fprintln(os.Stderr, "procoder: gave up reading the hook payload after", stdinDeadline)
		return nil, false
	}
}
