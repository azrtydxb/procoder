// Package host detects which AI coding agent procoder is running under,
// so hooks can answer in the shape each host expects. Detection is env-var
// sniffing, ordered: Copilot also sets CLAUDE_PLUGIN_ROOT, so it is
// checked first; VS Code's Copilot sets CLAUDE_PLUGIN_ROOT but not
// COPILOT_PLUGIN_DATA, hence the path heuristic.
package host

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"
)

// Host names the running agent.
type Host string

const (
	// Claude Code reads a hook's raw stdout.
	Claude Host = "claude"
	// Codex wants the hookSpecificOutput envelope plus a systemMessage.
	Codex Host = "codex"
	// Copilot wants a bare additionalContext object.
	Copilot Host = "copilot"
	// Qoder wants the hookSpecificOutput envelope without the systemMessage.
	Qoder Host = "qoder"
)

// Detect sniffs the environment. Unknown environments answer Claude —
// raw-stdout context is the least surprising default.
func Detect() Host {
	if os.Getenv("COPILOT_PLUGIN_DATA") != "" || isVSCodeCopilotRoot(os.Getenv("CLAUDE_PLUGIN_ROOT")) {
		return Copilot
	}
	if os.Getenv("PLUGIN_DATA") != "" {
		return Codex
	}
	if os.Getenv("QODER_SESSION_ID") != "" {
		return Qoder
	}
	return Claude
}

// isVSCodeCopilotRoot spots VS Code's Copilot, which reuses the Claude
// plugin-root variable: the path carries an agent-plugins segment under a
// .vscode directory.
func isVSCodeCopilotRoot(root string) bool {
	return root != "" && strings.Contains(root, "agent-plugins") && strings.Contains(root, ".vscode")
}

// SessionSource is why the session started, as the host reports it on the
// SessionStart hook's stdin: "startup", "resume", "clear" or "compact".
// Empty when there is no payload, it cannot be read in time, or it does not
// carry the field.
//
// It exists because the principles injection is expensive — roughly 7KB of
// text, re-sent verbatim on every matched start. On resume and compact that
// text is already in the conversation, and one working day of resumed
// sessions was measured at ~187k tokens of repeated injection (#175).
//
// The read is deadline-guarded and never fatal. A SessionStart hook that
// blocks holds the whole session open, and one that fails should fall back
// to saying too much rather than too little: an unknown source is treated
// as a fresh start by every caller here, so the rules arrive in full rather
// than being replaced by a pointer to rules the model has not seen.
func SessionSource(stdin io.Reader) string {
	if stdin == nil {
		return ""
	}
	type readResult struct {
		data []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		// A SessionStart payload is a few hundred bytes; the cap is only
		// there so a wedged writer cannot grow this without bound.
		data, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
		ch <- readResult{data, err}
	}()
	var raw []byte
	select {
	case res := <-ch:
		if res.err != nil {
			return ""
		}
		raw = res.data
	case <-time.After(sessionSourceDeadline):
		return ""
	}
	var payload struct {
		Source string `json:"source"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(payload.Source))
}

// Resumed reports whether this start already has the session's earlier
// context in it — the two sources where re-injecting the full text repeats
// what the model can already see. Anything else, including an unreadable
// payload, is a fresh start.
func Resumed(source string) bool {
	return source == "resume" || source == "compact"
}

// sessionSourceDeadline is short on purpose: this runs before a session can
// begin, and a host that sends nothing must cost that session nothing.
const sessionSourceDeadline = 2 * time.Second
