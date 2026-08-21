package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".procoder", "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// A setting procoder cannot apply must block. Silently falling back to the
// default lets a team believe a setting is in force when it never was —
// which is the failure configurability would otherwise introduce, and the
// same shape as a gate that reports green over code nothing read.
// proved by: restored the switch's silent fall-through for unknown keys —
// `polcy = "block"` is accepted as configured, does nothing, and the gate
// says nothing.
func TestASettingProcoderCannotApplyBlocks(t *testing.T) {
	root := writeConfig(t, "[lint]\npolcy = \"block\"\nthis line is broken\n")
	cfg := Load(root)
	if len(cfg.Problems) != 2 {
		t.Fatalf("both the typo and the broken line must be reported: %+v", cfg.Problems)
	}
	var blocking int
	for _, f := range cfg.Findings() {
		if f.Blocking {
			blocking++
		}
	}
	if blocking != 2 {
		t.Errorf("a setting that could not be applied must block, got %d blocking", blocking)
	}
	// The line number is the point: a person has to find it.
	if cfg.Problems[0].Line != 2 {
		t.Errorf("the problem must name its line, got %d", cfg.Problems[0].Line)
	}
}

// Weakening is allowed and visible; strengthening is allowed and silent.
// A line for every tightened setting would train the reader to skim the
// place the relaxations appear.
// proved by: made isRelaxed return true for any value differing from the
// default — raising the SAST bar then prints a relaxation line for a
// repository that made its gate stricter.
func TestLoweringADefaultPrintsAndRaisingOneDoesNot(t *testing.T) {
	relaxed := Load(writeConfig(t, "[git]\ncommit_gate = \"report\"\n"))
	var found bool
	for _, f := range relaxed.Findings() {
		if strings.Contains(f.Message, "relaxed: git.commit_gate") {
			found = true
			if f.Blocking {
				t.Error("a relaxation the repository chose must not block — that would make the setting useless")
			}
			if !strings.Contains(f.Message, "no longer stopped") {
				t.Errorf("the line must say what the relaxation costs: %q", f.Message)
			}
		}
	}
	if !found {
		t.Error("lowering a default must print")
	}

	// WARNING is a LOWER bar than ERROR: more findings block. That is a
	// strengthening and must be silent.
	strict := Load(writeConfig(t, "[security]\nsast_blocks_at = \"WARNING\"\n"))
	for _, f := range strict.Findings() {
		if strings.Contains(f.Message, "relaxed") {
			t.Errorf("strengthening must print nothing: %q", f.Message)
		}
	}
}

// An unrecognised severity is named and the default used — the run still
// reports findings rather than silently blocking on nothing.
// proved by: accepted any string as a severity — `sast_blocks_at =
// "SEVERE"` then ranks below every real severity and nothing ever blocks.
func TestAnUnrecognisedSeverityIsNamedAndTheDefaultUsed(t *testing.T) {
	cfg := Load(writeConfig(t, "[security]\nsast_blocks_at = \"SEVERE\"\n"))
	if cfg.SastBlocksAt != defaultSastBlocksAt {
		t.Errorf("an unknown severity must leave the default in force, got %q", cfg.SastBlocksAt)
	}
	if len(cfg.Problems) != 1 || !strings.Contains(cfg.Problems[0].Reason, "severity") {
		t.Fatalf("it must be named: %+v", cfg.Problems)
	}
}

// The story that protects everyone who is happy as things are: a
// repository that changes nothing behaves exactly as it did. Every source
// reads "default", nothing is relaxed, and nothing blocks.
// proved by: gave any setting a non-default effective value — a repo with
// no config.toml then shows a source that is not "default".
func TestARepositoryWithNoConfigIsAllDefaults(t *testing.T) {
	root := t.TempDir()
	cfg := Load(root)
	if len(cfg.Problems) != 0 {
		t.Errorf("no config is not a problem: %+v", cfg.Problems)
	}
	if len(cfg.Findings()) != 0 {
		t.Errorf("a repository that configured nothing must produce no config findings: %+v", cfg.Findings())
	}
	if len(cfg.Settings) == 0 {
		t.Fatal("the effective settings must still be listed")
	}
	for _, s := range cfg.Settings {
		if s.Source != "default" {
			t.Errorf("%s came from %q, want default", s.Key, s.Source)
		}
		if s.Relaxed {
			t.Errorf("%s cannot be relaxed when nothing was configured", s.Key)
		}
	}

	// And `procoder config` says so plainly.
	var buf bytes.Buffer
	if code := Report(root, &buf); code != 0 {
		t.Errorf("a repository with no config is not an error, exit %d", code)
	}
	if !strings.Contains(buf.String(), "default") {
		t.Errorf("the report must name the source:\n%s", buf.String())
	}
}
