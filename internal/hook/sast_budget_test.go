package hook

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"procoder/internal/security"
)

// The hook's SAST leg must give its "NOT run" verdict before the host kills
// the hook, and a direct check must keep the longer budget (#310).
// proved by: set HookSastBudget to 10m — the host's 120s timeout kills the
// hook first; set CheckSastBudget to HookSastBudget — a large repository's
// `procoder check` can never finish a scan.
func TestHookSastBudgetFitsTheHookTimeout(t *testing.T) {
	raw, err := os.ReadFile("../../hooks/claude-hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	var hostTimeout time.Duration
	for _, m := range cfg.Hooks["PreToolUse"] {
		for _, h := range m.Hooks {
			if strings.HasSuffix(h.Command, "hook pre-tool-use") {
				hostTimeout = time.Duration(h.Timeout) * time.Second
			}
		}
	}
	if hostTimeout == 0 {
		t.Fatal("no pre-tool-use hook with a timeout in hooks/claude-hooks.json")
	}
	if gateDeadline > hostTimeout {
		t.Errorf("gateDeadline %s exceeds the host's hook timeout %s", gateDeadline, hostTimeout)
	}
	if security.HookSastBudget >= gateDeadline {
		t.Errorf("HookSastBudget %s does not fit inside gateDeadline %s", security.HookSastBudget, gateDeadline)
	}
	if security.CheckSastBudget <= security.HookSastBudget {
		t.Errorf("CheckSastBudget %s is not longer than the hook's %s", security.CheckSastBudget, security.HookSastBudget)
	}
}
