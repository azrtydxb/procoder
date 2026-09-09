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
	// Pi wants the envelope the other envelope hosts get; it is named here so
	// that is a decision rather than an accident of the Claude default, which a
	// later change to that default would otherwise move silently.
	Pi Host = "pi"
)

// Env is the environment a command was called with. A map rather than the
// process's own, because one daemon serves several sessions: what the
// caller sent is what the command sees, and its own environment is
// whichever session happened to start it.
type Env map[string]string

// Read is os.Getenv over a request's environment, and the zero Env answers
// empty for every key rather than panicking.
func (e Env) Read(key string) string { return e[key] }

// envKeys is every variable procoder reads. A request carries these and a
// caller that sends more is not an error — the set will grow.
var envKeys = []string{
	"COPILOT_PLUGIN_DATA",
	"PLUGIN_DATA",
	"QODER_SESSION_ID",
	"PI_CODING_AGENT",
	"CLAUDE_PLUGIN_ROOT",
	"VIRTUAL_ENV",
	"PROCODER_PUPPETEER_CONFIG",
	"PATH",
}

// ProcessEnv is this process's answer to envKeys — what the CLI sends when
// it is its own caller.
func ProcessEnv() Env {
	e := make(Env, len(envKeys))
	for _, k := range envKeys {
		if v := os.Getenv(k); v != "" {
			e[k] = v
		}
	}
	return e
}

// Detect sniffs this process's environment. Unknown environments answer
// Claude — raw-stdout context is the least surprising default.
func Detect() Host { return DetectIn(ProcessEnv()) }

// DetectIn sniffs the environment it is given and nothing else.
func DetectIn(env Env) Host {
	if env.Read("COPILOT_PLUGIN_DATA") != "" || isVSCodeCopilotRoot(env.Read("CLAUDE_PLUGIN_ROOT")) {
		return Copilot
	}
	if env.Read("PLUGIN_DATA") != "" {
		return Codex
	}
	if env.Read("QODER_SESSION_ID") != "" {
		return Qoder
	}
	// Last among the known hosts. pi exports PI_CODING_AGENT into its own
	// process, and therefore into every launcher its adapter spawns; a host
	// that sets both its own variable and pi's is being run inside pi, and the
	// outer host is the one whose envelope matters.
	if env.Read("PI_CODING_AGENT") != "" {
		return Pi
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
	// A terminal never sends EOF, so io.ReadAll below would sit there
	// until the deadline expired — measured at 2.06s — and then return
	// nothing anyway. That is pure cost: a hook payload always arrives on
	// a pipe, so a character device means nobody is sending one.
	//
	// It matters beyond somebody typing the command to see what it does.
	// A host that invoked the hook with a terminal attached would pay the
	// deadline on every single session start, for an answer that was
	// never coming. Raised in review on #184.
	if f, ok := stdin.(interface{ Stat() (os.FileInfo, error) }); ok {
		if info, err := f.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return ""
		}
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
