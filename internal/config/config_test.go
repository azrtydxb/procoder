package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".procoder", "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestMissingFileMeansDefaults(t *testing.T) {
	cfg := Load(t.TempDir())
	if cfg.BlockDefaultBranch || cfg.MaxFileMB != 5 {
		t.Fatalf("defaults wrong: %+v", cfg)
	}
}

func TestValuesAreRead(t *testing.T) {
	dir := write(t, t.TempDir(), `
# comment
[git]
default_branch_policy = "block"
max_file_mb = 25   # generous
`)
	cfg := Load(dir)
	if !cfg.BlockDefaultBranch || cfg.MaxFileMB != 25 {
		t.Fatalf("got %+v", cfg)
	}
}

func TestGarbageLinesAreSkippedNotGuessed(t *testing.T) {
	dir := write(t, t.TempDir(), `
[git]
max_file_mb = not-a-number
default_branch_policy = "report"
`)
	cfg := Load(dir)
	if cfg.MaxFileMB != 5 || cfg.BlockDefaultBranch {
		t.Fatalf("got %+v", cfg)
	}
	// Skipped, and no longer silently: a value procoder could not use is
	// reported, or the writer believes a limit is in force that is not.
	if len(cfg.Problems) != 1 {
		t.Fatalf("the unusable value must be reported: %+v", cfg.Problems)
	}
	if !strings.Contains(cfg.Problems[0].Reason, "whole number") {
		t.Errorf("the reason must say what was wanted: %q", cfg.Problems[0].Reason)
	}
}

// The whole knob surface at once: every key the design documents, each landing
// in its own field. A case label typo or a copy-pasted assignment shows up
// here as one wrong field rather than as silent behaviour months later.
// proved by: pointing the "docs.policy" case at cfg.LintBlock, and separately
// pointing "maintain.funlen_lines" at cfg.FunlenStatements.
func TestEveryPolicyKeyLandsInItsOwnField(t *testing.T) {
	dir := write(t, t.TempDir(), `
[git]
default_branch_policy = "block"
max_file_mb = 12
commit_gate = "report"

[lint]
policy = "block"

[test]
policy = "block"

[docs]
policy = "block"

[ci]
pin_actions_policy = "block"

[sprint]
retro = "off"

[bench]
threshold = 25

[maintain]
gocyclo = 14
funlen_lines = 80
funlen_statements = 45

[debt]
marker = "tech-debt:"

[release]
files = ["Cargo.toml", "package.json"]
`)
	cfg := Load(dir)
	checks := []struct {
		field string
		got   any
		want  any
	}{
		{"BlockDefaultBranch", cfg.BlockDefaultBranch, true},
		{"MaxFileMB", cfg.MaxFileMB, 12},
		{"CommitGate", cfg.CommitGate, "report"},
		{"LintBlock", cfg.LintBlock, true},
		{"TestBlock", cfg.TestBlock, true},
		{"DocsBlock", cfg.DocsBlock, true},
		{"PinActions", cfg.PinActions, true},
		{"SprintRetroOff", cfg.SprintRetroOff, true},
		{"BenchThreshold", cfg.BenchThreshold, 25},
		{"Gocyclo", cfg.Gocyclo, 14},
		{"FunlenLines", cfg.FunlenLines, 80},
		{"FunlenStatements", cfg.FunlenStatements, 45},
		{"DebtMarker", cfg.DebtMarker, "tech-debt:"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.field, c.got, c.want)
		}
	}
	if len(cfg.ReleaseFiles) != 2 || cfg.ReleaseFiles[0] != "Cargo.toml" || cfg.ReleaseFiles[1] != "package.json" {
		t.Errorf("ReleaseFiles = %q, want [Cargo.toml package.json]", cfg.ReleaseFiles)
	}
}

// A repository that never wrote a config gets the documented defaults: nothing
// blocks by surprise on upgrade, and the commit gate is on.
// proved by: initialising cfg.CommitGate to "" in Load, and separately
// initialising DebtMarker to "todo:".
func TestAbsentFileGivesTheDocumentedDefaults(t *testing.T) {
	cfg := Load(t.TempDir())
	if cfg.CommitGate != "block" {
		t.Errorf("CommitGate = %q, want %q — the commit gate is on by default", cfg.CommitGate, "block")
	}
	if cfg.DebtMarker != "debt:" {
		t.Errorf("DebtMarker = %q, want %q", cfg.DebtMarker, "debt:")
	}
	if cfg.MaxFileMB != 5 {
		t.Errorf("MaxFileMB = %d, want 5", cfg.MaxFileMB)
	}
	for _, c := range []struct {
		field string
		got   bool
	}{
		{"DocsBlock", cfg.DocsBlock}, {"LintBlock", cfg.LintBlock}, {"TestBlock", cfg.TestBlock},
		{"BlockDefaultBranch", cfg.BlockDefaultBranch}, {"PinActions", cfg.PinActions},
		{"SprintRetroOff", cfg.SprintRetroOff},
	} {
		if c.got {
			t.Errorf("%s = true, want false — procoder never blocks by surprise", c.field)
		}
	}
	if cfg.Gocyclo != 0 || cfg.FunlenLines != 0 || cfg.FunlenStatements != 0 || cfg.BenchThreshold != 0 {
		t.Errorf("thresholds should be zero (meaning: the domain's own default), got %+v", cfg)
	}
	if cfg.ReleaseFiles != nil {
		t.Errorf("ReleaseFiles = %q, want nil — nothing declared, nothing verified", cfg.ReleaseFiles)
	}
}

// "block" is the only word that turns a policy on. Anything else — the
// documented "report", a typo, a half-written value — leaves it reporting.
// proved by: rewriting the lint.policy case as `cfg.LintBlock = value !=
// "report"`, which turns the typo'd value into a blocking gate.
func TestOnlyBlockTurnsAPolicyOn(t *testing.T) {
	dir := write(t, t.TempDir(), `
[lint]
policy = "blocking"

[docs]
policy = "report"

[test]
policy = ""

[ci]
pin_actions_policy = "warn"
`)
	cfg := Load(dir)
	if cfg.LintBlock || cfg.DocsBlock || cfg.TestBlock || cfg.PinActions {
		t.Errorf("a policy that does not read exactly \"block\" must not block: %+v", cfg)
	}
}

// The commit gate has three legal words; an unknown one keeps the default
// rather than disabling the interception by accident.
// proved by: dropping the `value == "block" || value == "report" || value ==
// "off"` guard so any value is taken verbatim.
func TestCommitGateKeepsTheDefaultOnAnUnknownValue(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"report", "report"},
		{"off", "off"},
		{"block", "block"},
		{"disabled", "block"},
		{"", "block"},
	} {
		dir := write(t, t.TempDir(), "[git]\ncommit_gate = \""+tc.value+"\"\n")
		if got := Load(dir).CommitGate; got != tc.want {
			t.Errorf("commit_gate = %q -> CommitGate %q, want %q", tc.value, got, tc.want)
		}
	}
}

// Several sections use the bare key "policy". The section header is what tells
// them apart, so it must follow the file down, not stick at the first one.
// proved by: making the section header assignment fire only while section is
// still "" — the [docs] block then lands on lint's field.
func TestTheSectionHeaderScopesKeysOfTheSameName(t *testing.T) {
	dir := write(t, t.TempDir(), `
[lint]
policy = "report"

[docs]
policy = "block"
`)
	cfg := Load(dir)
	if cfg.LintBlock {
		t.Error("LintBlock = true, but [lint] said report — the [docs] section leaked into it")
	}
	if !cfg.DocsBlock {
		t.Error("DocsBlock = false, but [docs] said block")
	}
}

// Three value spellings must reach the same field value: quoted, bare, and a
// value trailing an inline comment.
// proved by: dropping `value = strings.Trim(value, "\"")`, which leaves the
// quoted spellings carrying their quotes.
func TestQuotedBareAndCommentedValuesAgree(t *testing.T) {
	for name, body := range map[string]string{
		"quoted":   "[debt]\nmarker = \"TODO:\"\n",
		"bare":     "[debt]\nmarker = TODO:\n",
		"comment":  "[debt]\nmarker = \"TODO:\" # the marker debt harvests\n",
		"padded":   "[debt]\n   marker   =   \"TODO:\"   \n",
		"bare+cmt": "[debt]\nmarker = TODO: # harvested\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got := Load(write(t, t.TempDir(), body)).DebtMarker; got != "TODO:" {
				t.Errorf("DebtMarker = %q, want %q", got, "TODO:")
			}
		})
	}
}

// A key written twice is a person editing: the later line is the one they
// meant.
// proved by: guarding the git.max_file_mb case with `if cfg.MaxFileMB ==
// defaultMaxFileMB`, so the first assignment sticks.
func TestTheLastAssignmentWins(t *testing.T) {
	dir := write(t, t.TempDir(), `
[git]
max_file_mb = 7
commit_gate = "off"
max_file_mb = 40
commit_gate = "report"
`)
	cfg := Load(dir)
	if cfg.MaxFileMB != 40 {
		t.Errorf("MaxFileMB = %d, want 40 — the later line is the intent", cfg.MaxFileMB)
	}
	if cfg.CommitGate != "report" {
		t.Errorf("CommitGate = %q, want %q", cfg.CommitGate, "report")
	}
}

// release.files is the one list in the config, and the release gate verifies
// exactly what it names. Brackets are what makes it a list; anything else is
// not one, and an empty slot is not a filename.
// proved by: dropping the `strings.Trim(p, "\"")` in parseList (entries keep
// their quotes), and separately dropping the `if !strings.HasPrefix(value,
// "[")` guard so a plain string becomes a one-element list.
func TestReleaseFilesParsesOnlyBracketedLists(t *testing.T) {
	for name, tc := range map[string]struct {
		line string
		want []string
	}{
		"spaced":     {`files = [ "a.toml" ,  "b.json" ]`, []string{"a.toml", "b.json"}},
		"tight":      {`files = ["a.toml","b.json"]`, []string{"a.toml", "b.json"}},
		"empty slot": {`files = ["a.toml", , "b.json"]`, []string{"a.toml", "b.json"}},
		"single":     {`files = ["VERSION"]`, []string{"VERSION"}},
		"empty list": {`files = []`, nil},
		"not a list": {`files = "a.toml"`, nil},
		"commented":  {`files = ["a.toml"] # the one file`, []string{"a.toml"}},
	} {
		t.Run(name, func(t *testing.T) {
			got := Load(write(t, t.TempDir(), "[release]\n"+tc.line+"\n")).ReleaseFiles
			if len(got) != len(tc.want) {
				t.Fatalf("ReleaseFiles = %q, want %q", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("ReleaseFiles[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// A threshold of zero or below is not a threshold. Taking one literally would
// mark every benchmark a regression and flag every function as too complex, so
// the domain's own default must survive the bad value.
// proved by: returning n from atoiOr whenever err == nil, dropping the n <= 0
// test, and separately dropping the `n > 0` term from the max_file_mb case.
func TestNonPositiveNumbersLeaveTheDefaultsStanding(t *testing.T) {
	dir := write(t, t.TempDir(), `
[bench]
threshold = -5

[maintain]
gocyclo = 0
funlen_lines = -1

[git]
max_file_mb = 0
`)
	cfg := Load(dir)
	if cfg.BenchThreshold != 0 || cfg.Gocyclo != 0 || cfg.FunlenLines != 0 {
		t.Errorf("a non-positive threshold must read as unset (0), got %+v", cfg)
	}
	if cfg.MaxFileMB != 5 {
		t.Errorf("MaxFileMB = %d, want the default 5 — 0 MB would flag every file", cfg.MaxFileMB)
	}
}

// An empty marker would make `procoder debt` match every comment in the tree.
// proved by: dropping the `if value != ""` guard around cfg.DebtMarker.
func TestAnEmptyDebtMarkerKeepsTheDefault(t *testing.T) {
	dir := write(t, t.TempDir(), "[debt]\nmarker = \"\"\n")
	if got := Load(dir).DebtMarker; got != "debt:" {
		t.Errorf("DebtMarker = %q, want the default %q", got, "debt:")
	}
}

// A typo in this key silently decides which methodology governs the
// repository, which is more consequential than most: a mistyped "bmad"
// leaves a repository that plans elsewhere being governed by procoder's
// own chain and wondering why its artifacts are ignored.
// proved by: accepted any value into cfg.PlanningMethod — "nonsense"
// becomes the effective method, no Problem names the line, and the
// repository is governed by a method that does not exist.
func TestAnUnknownPlanningMethodIsAProblemAndTheDefaultRuns(t *testing.T) {
	root := t.TempDir()
	write := func(body string) Config {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".procoder", "config.toml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return Load(root)
	}

	cfg := write("[planning]\nmethod = \"nonsense\"\n")
	if len(cfg.Problems) != 1 {
		t.Fatalf("a value procoder cannot act on is exactly one Problem: %+v", cfg.Problems)
	}
	if cfg.Problems[0].Line != 2 {
		t.Errorf("the Problem must name the line: %+v", cfg.Problems[0])
	}
	if !strings.Contains(cfg.Problems[0].Reason, "procoder, bmad") {
		t.Errorf("and say what procoder does know: %q", cfg.Problems[0].Reason)
	}
	// The run continues on the default rather than on the typo.
	if cfg.Planning() != "procoder" {
		t.Errorf("an unusable value falls back to the default: %q", cfg.Planning())
	}

	// Both documented values are accepted, and neither is a Problem.
	for _, m := range PlanningMethods {
		cfg := write("[planning]\nmethod = \"" + m + "\"\n")
		if len(cfg.Problems) != 0 {
			t.Errorf("%s is a method procoder knows: %+v", m, cfg.Problems)
		}
		if cfg.Planning() != m {
			t.Errorf("%s must be the effective method, got %q", m, cfg.Planning())
		}
	}

	// And a repository that said nothing gets the default without having
	// to spell it, so no caller can disagree about what the default is.
	if got := (Config{}).Planning(); got != "procoder" {
		t.Errorf("the unset default is procoder: %q", got)
	}
}
