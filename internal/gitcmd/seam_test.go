package gitcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"procoder/internal/config"
	"procoder/internal/gitx"
)

// The seam the whole planning setting rests on: it moves planning, and
// moves nothing else. That is the only reason a repository which plans
// elsewhere would install procoder at all, and the tempting design — a
// spectrum where the setting dials procoder back by degrees — erodes it
// one exception at a time.
//
// Asserted by comparing every finding the gate produces about the CODE
// across both settings. The planning domain's own findings are excluded
// on purpose: those are the setting doing its job, and including them
// would make this test assert the two are the same command rather than
// that governance is untouched.
// proved by: made any governance leg consult cfg.Planning() — gate the
// docs obligation on it, or skip lint under "bmad" — and the two lists
// stop matching, where every other test in the tree stays green because
// each leg is still correct in isolation.
func TestGovernanceIsUntouchedByThePlanningMethod(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")

	write := func(name, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A tree with something for the governance legs to find, and a BMad
	// installation so the "bmad" run is honoured rather than refused.
	write("a.go", "package x\n\nfunc F() int { return 1 }\n")
	write("README.md", "# fixture\n\n[missing](./nope.md)\n")
	write("_bmad/manifest.yaml", "version: \"6.11.0\"\n")
	write("_bmad-output/implementation-artifacts/sprint-status.yaml",
		"development_status:\n  1-1: done\n")
	run("add", "-A")
	run("commit", "-qm", "first")

	changed := []string{filepath.Join(root, "a.go"), filepath.Join(root, "README.md")}

	// Only the findings that are NOT the planning domain's own. Those are
	// the setting working; everything else is governance, which must not
	// notice the setting at all.
	governance := func(method string) []string {
		cfg := config.Load(root)
		cfg.PlanningMethod = method
		var out []string
		for _, f := range CollectFor(root, cfg, changed, "") {
			if isPlanning(f) {
				continue
			}
			out = append(out, f.Message)
		}
		return out
	}

	asProcoder := governance("procoder")
	asBmad := governance("bmad")

	if len(asProcoder) == 0 {
		t.Fatal("the fixture must produce findings, or this comparison proves nothing")
	}
	if !reflect.DeepEqual(asProcoder, asBmad) {
		t.Errorf("the planning method must not change what the gate says about the code:\n procoder: %v\n bmad:     %v",
			asProcoder, asBmad)
	}
}

// isPlanning identifies the planning domain's own findings by the tag
// every domain already puts at the end of its message.
func isPlanning(f gitx.Finding) bool {
	return strings.Contains(f.Message, "(planning)")
}
