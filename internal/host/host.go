// Package host detects which AI coding agent procoder is running under,
// so hooks can answer in the shape each host expects. Detection is env-var
// sniffing, ordered: Copilot also sets CLAUDE_PLUGIN_ROOT, so it is
// checked first; VS Code's Copilot sets CLAUDE_PLUGIN_ROOT but not
// COPILOT_PLUGIN_DATA, hence the path heuristic.
package host

import (
	"os"
	"strings"
)

// Host names the running agent.
type Host string

const (
	Claude  Host = "claude"
	Codex   Host = "codex"
	Copilot Host = "copilot"
	Qoder   Host = "qoder"
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
