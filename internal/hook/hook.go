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

	res := format.Check(p.ToolInput.FilePath)
	msg := message(res)

	// A workflow file gets actionlint's findings as well — the write that
	// introduced a broken workflow is the cheapest moment to hear about it.
	if actions.IsWorkflowFile(p.ToolInput.FilePath) {
		if lint := actions.Lint([]string{p.ToolInput.FilePath}); len(lint) > 0 {
			var b []string
			for _, f := range lint {
				b = append(b, fmt.Sprintf("  line %d: %s", f.Line, f.Message))
			}
			part := fmt.Sprintf("procoder [actions]: %s has workflow problems (actionlint) — investigate and fix:\n%s",
				p.ToolInput.FilePath, strings.Join(b, "\n"))
			if msg != "" {
				msg += "\n\n"
			}
			msg += part
		}
	}
	// Domain 5, same moment: a Markdown write gets its reference and diagram
	// findings now; a code write gets the doc-map — which docs mention this
	// file — so the prose is verified before the work is called done.
	root := tools.RepoRoot(filepath.Dir(p.ToolInput.FilePath))
	// keep the code index current for the file just written — maintenance of
	// procoder-owned derived state, only when an index already exists
	codeindex.Refresh(root, p.ToolInput.FilePath)
	if docs.IsMarkdownFile(p.ToolInput.FilePath) {
		if df := docs.CheckFile(root, p.ToolInput.FilePath); len(df) > 0 {
			var b []string
			for _, f := range df {
				b = append(b, fmt.Sprintf("  line %d: %s", f.Line, f.Message))
			}
			part := fmt.Sprintf("procoder [docs]: %s has documentation problems — investigate and fix:\n%s",
				p.ToolInput.FilePath, strings.Join(b, "\n"))
			if msg != "" {
				msg += "\n\n"
			}
			msg += part
		}
	} else {
		if drift := docs.Drift(root, []string{p.ToolInput.FilePath}); len(drift) > 0 {
			var b []string
			for _, f := range drift {
				b = append(b, "  "+f.File+": "+f.Message)
			}
			part := "procoder [docs]: documentation mentions the file you just changed — verify it is still true, update it if not:\n" + strings.Join(b, "\n")
			if msg != "" {
				msg += "\n\n"
			}
			msg += part
		}
		// Domain 2: the file's canonical linter answers in the same turn —
		// findings are diagnoses; the agent judges and fixes what is real
		if lf := lint.Files(root, []string{p.ToolInput.FilePath}, config.Load(root).LintBlock); len(lf) > 0 {
			var b []string
			for _, f := range lf {
				b = append(b, fmt.Sprintf("  %s:%d: %s", f.File, f.Line, f.Message))
			}
			part := "procoder [lint]: findings in the file you just wrote — investigate, judge, and fix what is real:\n" + strings.Join(b, "\n")
			if msg != "" {
				msg += "\n\n"
			}
			msg += part
		}
	}

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
