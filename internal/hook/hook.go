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
	"path/filepath"
	"strings"
	"time"

	"procoder/internal/actions"
	"procoder/internal/ask"
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

// The host does not deliver an unbounded hook payload. Past roughly two
// kilobytes it writes the output to a file and inlines only a PREVIEW of
// the first 2KB, so anything after that reaches the agent as a path it has
// to go and read — which nothing makes it do.
//
// That is worse than losing the tail. This hook's format part says "review
// it and write it to the file", and a truncated formatted body under that
// instruction is a file with its end cut off. It also silently drops
// whatever comes after the format part, and a secret finding is one of the
// things that comes after.
//
// Measured, not documented: two previews observed at exactly 2KB, one on
// SessionStart stdout at 9.9KB of output and one on this hook's
// additionalContext at 10.7KB. The threshold at which the host starts
// persisting is NOT established — only that 2KB is what it inlines when it
// does. So 2KB is treated as the whole budget, which is safe whatever the
// persist threshold turns out to be.
const maxContextBytes = 2000

// maxInlineFormatted is the formatted body's own share of that budget. It
// is the only part that can be arbitrarily large — a findings line is tens
// of bytes and a file is not — so without a share of its own it starves
// everything after it.
const maxInlineFormatted = 900

// askPart carries the questions no domain can answer for itself into the
// place the coder actually reads. Without it these reach the coder as a
// finding it feels obliged to resolve, and it resolves them by inventing an
// answer — which is indistinguishable from a decision once written down.
//
// The instruction is the point, not the list: the coder is told to stop and
// relay, and told the route a human answer comes back through.
func askPart(root string) string {
	pending, err := ask.Pending(root)
	if err != nil {
		// Not silence: the coder is about to carry on without knowing there
		// are questions nobody can read.
		return "== q&a: questions NOT collected — " + err.Error() + "\nDo not treat this as 'nothing to ask'."
	}
	if len(pending) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "== q&a: %d question(s) only the user can answer\n", len(pending))
	limit := len(pending)
	if limit > 5 {
		limit = 5
	}
	for _, q := range pending[:limit] {
		fmt.Fprintf(&b, "  - %s %s\n", q.Label(), oneLine(q.Text, 160))
	}
	if len(pending) > limit {
		fmt.Fprintf(&b, "  … and %d more\n", len(pending)-limit)
	}
	b.WriteString("Do NOT guess at these and do NOT answer them yourself: an invented answer\n")
	b.WriteString("reads as a decision once it is written down. Put them to the user, write\n")
	b.WriteString("their answers into the file, and record them with `procoder ask --file`.")
	return b.String()
}

// oneLine flattens a question to a single line of at most n characters.
//
// The loop above was written as one line per question and was not: a
// question derived from a decision record carries the whole record as its
// text, blank lines and all, so five questions could run to eight
// kilobytes and crowd every other finding out of the payload.
func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

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
	add(askPart(root))
	msg := fit(parts)

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
	// each secret stays its own paragraph, exactly as before the refactor
	return strings.Join(b, "\n\n")
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

// fit joins the parts the agent will actually receive, and says so when it
// had to leave some out.
//
// Parts are added in the order they were built — which is deliberate and
// documented at each call site — and the first one that does not fit ends
// the message. Nothing is truncated mid-part: half a finding is a finding
// whose meaning cannot be trusted, and half a formatted file is a file
// somebody may write back.
//
// The notice is the point. A hook that quietly delivered four findings out
// of seven would be the same silent-green this tool exists to remove.
func fit(parts []string) string {
	var kept []string
	used := 0
	dropped := 0
	for _, p := range parts {
		// +2 for the blank line between parts.
		if used > 0 && used+len(p)+2 > maxContextBytes {
			dropped++
			continue
		}
		if used == 0 && len(p) > maxContextBytes {
			dropped++
			continue
		}
		kept = append(kept, p)
		used += len(p) + 2
	}
	if dropped > 0 {
		kept = append(kept, fmt.Sprintf(
			"== %d further finding(s) NOT shown — the host delivers only the first %d bytes of a hook's output. Run `procoder check` to see all of them.",
			dropped, maxContextBytes))
	}
	return strings.Join(kept, "\n\n")
}

// message turns a Result into what the agent is told. Clean and OutOfScope are
// silent here — the write hook speaks only when there is something to act on;
// the counting of skipped files is `procoder check`'s job, where a human reads
// the totals.
func message(res format.Result) string {
	switch res.Verdict {
	case format.Unformatted:
		if len(res.Formatted) > maxInlineFormatted {
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
