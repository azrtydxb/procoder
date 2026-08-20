package host

import "testing"

// clear blanks every variable Detect consults, so a case declares its whole
// world. The test process itself runs under an AI agent that may well export
// CLAUDE_PLUGIN_ROOT, and a case that only sets what it cares about would
// otherwise inherit the developer's machine and answer differently in CI.
func clear(t *testing.T) {
	t.Helper()
	for _, k := range []string{"COPILOT_PLUGIN_DATA", "CLAUDE_PLUGIN_ROOT", "PLUGIN_DATA", "QODER_SESSION_ID"} {
		t.Setenv(k, "")
	}
}

// Detect decides which JSON envelope every hook emits, so a wrong answer
// breaks hooks silently on that host — one case per signal, and one per
// collision between signals, because the ORDER of the checks is the contract.
//
// proved by: reordering Detect so the PLUGIN_DATA (Codex) check runs before
// the Copilot check — the "copilot wins over codex" case then returns codex.
func TestDetectSelectsHostBySignalAndPrecedence(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Host
	}{
		{
			name: "nothing set falls back to claude",
			env:  nil,
			want: Claude,
		},
		{
			name: "COPILOT_PLUGIN_DATA names copilot",
			env:  map[string]string{"COPILOT_PLUGIN_DATA": "/some/where"},
			want: Copilot,
		},
		{
			name: "PLUGIN_DATA alone names codex",
			env:  map[string]string{"PLUGIN_DATA": "/some/where"},
			want: Codex,
		},
		{
			name: "QODER_SESSION_ID alone names qoder",
			env:  map[string]string{"QODER_SESSION_ID": "abc123"},
			want: Qoder,
		},
		{
			name: "vs code copilot is spotted by its plugin root",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/Users/x/.vscode/extensions/agent-plugins/procoder"},
			want: Copilot,
		},
		{
			name: "claude's own plugin root is not mistaken for copilot",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/Users/x/.claude/plugins/procoder"},
			want: Claude,
		},
		{
			// Copilot exports the Claude variable too, so a root that carries
			// only ONE of the two markers must not be enough to claim copilot.
			name: "agent-plugins outside .vscode stays claude",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/opt/agent-plugins/procoder"},
			want: Claude,
		},
		{
			name: ".vscode without agent-plugins stays claude",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/Users/x/.vscode/extensions/procoder"},
			want: Claude,
		},
		{
			// Copilot sets both; whichever check runs first decides, and it
			// must be copilot's.
			name: "copilot wins over codex",
			env:  map[string]string{"COPILOT_PLUGIN_DATA": "/c", "PLUGIN_DATA": "/p"},
			want: Copilot,
		},
		{
			name: "vs code copilot root wins over codex",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/x/.vscode/agent-plugins", "PLUGIN_DATA": "/p"},
			want: Copilot,
		},
		{
			name: "codex wins over qoder",
			env:  map[string]string{"PLUGIN_DATA": "/p", "QODER_SESSION_ID": "abc"},
			want: Codex,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clear(t)
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			if got := Detect(); got != c.want {
				t.Errorf("Detect() = %q, want %q", got, c.want)
			}
		})
	}
}

// The four host names are written into hook envelopes and matched by string
// elsewhere, so they are wire values: renaming one is a breaking change, not
// a refactor.
//
// proved by: changing the Copilot constant to "github-copilot" — the case
// below fails, as does every consumer that switches on the literal.
func TestHostNamesAreTheWireValues(t *testing.T) {
	for _, c := range []struct {
		got  Host
		want string
	}{
		{Claude, "claude"},
		{Codex, "codex"},
		{Copilot, "copilot"},
		{Qoder, "qoder"},
	} {
		if string(c.got) != c.want {
			t.Errorf("host constant = %q, want %q", c.got, c.want)
		}
	}
}
