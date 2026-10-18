package config

import (
	"reflect"
	"strings"
	"testing"
)

// [lint] skip_tools lands as a list, and is a relaxation: a scoped-out
// linter is a weaker gate, and a weaker gate must never be quiet (#324).
// proved by: dropped the lint.skip_tools entry from relaxations — the
// skip reads like any default.
func TestSkipToolsIsAListAndPrintsAsARelaxation(t *testing.T) {
	cfg := Load(writeConfig(t, "[lint]\nskip_tools = [\"typescript-eslint\", \"ruff\"]\n"))
	if len(cfg.Problems) != 0 {
		t.Fatalf("a well-formed list is not a problem: %v", cfg.Problems)
	}
	if want := []string{"typescript-eslint", "ruff"}; !reflect.DeepEqual(cfg.LintSkipTools, want) {
		t.Errorf("LintSkipTools = %v, want %v", cfg.LintSkipTools, want)
	}
	var relaxed bool
	for _, s := range cfg.Settings {
		if s.Key == "lint.skip_tools" && s.Relaxed {
			relaxed = true
		}
	}
	if !relaxed {
		t.Error("skipping a linter must print as a relaxation")
	}
	empty := Load(writeConfig(t, "[lint]\nskip_tools = []\n"))
	for _, s := range empty.Settings {
		if s.Key == "lint.skip_tools" && s.Relaxed {
			t.Error("an empty list skips nothing and relaxes nothing")
		}
	}
}

// A bare string skips nothing; the writer believes it skipped something.
// proved by: accepted any value — `skip_tools = "eslint"` silently runs eslint.
func TestSkipToolsThatIsNotAListIsAProblem(t *testing.T) {
	cfg := Load(writeConfig(t, "[lint]\nskip_tools = \"eslint\"\n"))
	if len(cfg.Problems) != 1 || !strings.Contains(cfg.Problems[0].Reason, "list") {
		t.Fatalf("want one problem asking for a list, got %v", cfg.Problems)
	}
	if len(cfg.LintSkipTools) != 0 {
		t.Errorf("a rejected value must skip nothing: %v", cfg.LintSkipTools)
	}
}
