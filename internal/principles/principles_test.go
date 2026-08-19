package principles

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"procoder/internal/status"
)

// hostEnv puts the process in one host's environment: every sniffed variable
// is set explicitly, so an ambient one cannot decide the answer.
func hostEnv(t *testing.T, want string) {
	t.Helper()
	for _, v := range []string{"COPILOT_PLUGIN_DATA", "CLAUDE_PLUGIN_ROOT", "PLUGIN_DATA", "QODER_SESSION_ID"} {
		t.Setenv(v, "")
	}
	switch want {
	case "copilot":
		t.Setenv("COPILOT_PLUGIN_DATA", "x")
	case "codex":
		t.Setenv("PLUGIN_DATA", "x")
	case "qoder":
		t.Setenv("QODER_SESSION_ID", "x")
	}
}

func hookOutput(t *testing.T, want, root string) string {
	t.Helper()
	hostEnv(t, want)
	var got []string
	if code := RunHook(root, func(s string) { got = append(got, s) }); code != 0 {
		t.Fatalf("hook exit %d — a SessionStart hook must never fail the session", code)
	}
	return strings.Join(got, "\n")
}

// The status block must arrive in every envelope: a host that gets the
// principles without the state is a session that opens blind, which is the
// whole failure this work exists to close.
func TestHookCarriesTheStatusBlockInEveryEnvelope(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		host    string
		extract func(string) string
	}{
		{"claude", func(s string) string { return s }},
		{"copilot", func(s string) string {
			var m map[string]string
			if json.Unmarshal([]byte(s), &m) != nil {
				t.Fatalf("copilot envelope is not JSON: %s", s)
			}
			return m["additionalContext"]
		}},
		{"codex", hookSpecific(t)},
		{"qoder", hookSpecific(t)},
	} {
		t.Run(tc.host, func(t *testing.T) {
			text := tc.extract(hookOutput(t, tc.host, root))
			if !strings.Contains(text, "Engineering principles") {
				t.Fatalf("%s envelope lost the principles text:\n%s", tc.host, text)
			}
			if !strings.Contains(text, status.Header) {
				t.Fatalf("%s envelope carries no status block:\n%s", tc.host, text)
			}
			if i, j := strings.Index(text, "Engineering principles"), strings.Index(text, status.Header); j < i {
				t.Fatalf("%s envelope puts the state before the principles", tc.host)
			}
			if !strings.Contains(text, "open tasks:") {
				t.Fatalf("%s envelope has the header but no facts:\n%s", tc.host, text)
			}
		})
	}
}

func hookSpecific(t *testing.T) func(string) string {
	return func(s string) string {
		t.Helper()
		var m struct {
			HookSpecificOutput struct {
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal([]byte(s), &m) != nil {
			t.Fatalf("envelope is not JSON: %s", s)
		}
		return m.HookSpecificOutput.AdditionalContext
	}
}

// `procoder principles` is the plain read of the philosophy and stays that:
// the state block belongs to the session hook, not to the document.
func TestPlainRunStaysPrinciplesOnly(t *testing.T) {
	var got []string
	if code := Run(t.TempDir(), func(s string) { got = append(got, s) }); code != 0 {
		t.Fatalf("principles exit %d", code)
	}
	if text := strings.Join(got, "\n"); strings.Contains(text, status.Header) {
		t.Fatalf("`procoder principles` must not carry the status block:\n%s", text)
	}
}

// The SessionStart path is what a session waits on, so it is timed here on
// the repository the hook actually runs in.
func TestSessionStartStaysInsideTheBudget(t *testing.T) {
	hostEnv(t, "claude")
	start := time.Now()
	RunHook("../..", func(string) {})
	elapsed := time.Since(start)
	if elapsed > status.Budget {
		t.Fatalf("SessionStart took %s — the budget is %s", elapsed, status.Budget)
	}
	t.Logf("SessionStart took %s (budget %s)", elapsed, status.Budget)
}
