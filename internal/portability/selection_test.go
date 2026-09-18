package portability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/gitx"
)

// proved by: forced selectedCopies to all; Kilo-only setup printed unrelated
// legacy and other-host paths and this test rejected the output.
func TestHostSetupPrintsOnlyNeededFilesWithoutWriting(t *testing.T) {
	root := t.TempDir()
	writeRules(t, root, Master, "# Rules\n\nBody.\n")
	var lines []string
	if code := Agents(root, func(s string) { lines = append(lines, s) }, "kilo"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	out := strings.Join(lines, "\n")
	if !strings.Contains(out, ".kilo/rules/procoder.md") || !strings.Contains(out, HostsFile) {
		t.Fatal(out)
	}
	for _, unwanted := range []string{".kilocode/", ".cursor/", "skills/procoder/", ".codex/"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("printed unrelated %s: %s", unwanted, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, HostsFile)); !os.IsNotExist(err) {
		t.Fatal("agents wrote a declaration")
	}
	if _, err := os.Stat(filepath.Join(root, ".kilo")); !os.IsNotExist(err) {
		t.Fatal("agents wrote integration files")
	}

	writeRules(t, root, HostsFile, `["kilo"]`)
	writeRules(t, root, ".kilo/rules/procoder.md", "# Rules\n\nBody.\n")
	if got := AgentsDrift(root); len(got) != 0 {
		t.Fatalf("single host: %+v", got)
	}
	lines = nil
	Agents(root, func(s string) { lines = append(lines, s) }, "cursor")
	out = strings.Join(lines, "\n")
	if !strings.Contains(out, `"kilo"`) || !strings.Contains(out, `"cursor"`) || !strings.Contains(out, ".cursor/rules/") {
		t.Fatal(out)
	}
	raw, _ := os.ReadFile(filepath.Join(root, HostsFile))
	if string(raw) != `["kilo"]` {
		t.Fatal("existing selection changed")
	}
}

// proved by: forced selectedCopies to all; the one declared missing Kilo copy
// became twelve missing-copy findings and this test failed.
func TestDeclaredHostsAndExistingCopiesAreChecked(t *testing.T) {
	root := t.TempDir()
	writeRules(t, root, Master, "# Rules\n")
	writeRules(t, root, HostsFile, `["kilo"]`)
	if got := AgentsDrift(root); len(got) != 1 || !got[0].Blocking || got[0].File != ".kilo/rules/procoder.md" {
		t.Fatalf("missing selected host: %+v", got)
	}
	writeRules(t, root, ".kilo/rules/procoder.md", "# Rules\n")
	writeRules(t, root, ".kilocode/rules/procoder.md", "stale legacy rules")
	if got := AgentsDrift(root); len(got) != 1 || got[0].File != ".kilocode/rules/procoder.md" {
		t.Fatalf("existing legacy host skipped: %+v", got)
	}
	for _, invalid := range []string{`null`, `[]`, `["typo"]`, `["all","kilo"]`, `["kilo","kilo"]`, `{broken`} {
		writeRules(t, root, HostsFile, invalid)
		if got := AgentsDrift(root); len(got) != 1 || got[0].File != HostsFile || !got[0].Blocking {
			t.Fatalf("invalid selection %s passed: %+v", invalid, got)
		}
	}
}

// proved by: made declaredHosts return nil without reading; a declared setup
// missing its master passed with no findings and this test failed.
func TestMissingMasterAndUnreadableSelectionAreNotClean(t *testing.T) {
	root := t.TempDir()
	writeRules(t, root, HostsFile, `["kilo"]`)
	if got := Check(root); len(got) != 1 || !got[0].Blocking {
		t.Fatalf("missing master passed: %+v", got)
	}
	if got := AgentsDrift(root); len(got) != 1 || !got[0].Blocking {
		t.Fatalf("missing master passed drift: %+v", got)
	}
	if err := os.Remove(filepath.Join(root, HostsFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, HostsFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Check(root); len(got) != 1 || got[0].File != HostsFile {
		t.Fatalf("unreadable selection passed: %+v", got)
	}
}

// proved by: made declaredHosts return nil without reading; the dangling
// declaration passed with no findings and this test failed.
func TestDanglingHostDeclarationCannotDisableChecks(t *testing.T) {
	root := t.TempDir()
	writeRules(t, root, Master, "# Rules\n")
	if err := os.Mkdir(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing.json", filepath.Join(root, HostsFile)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got := Check(root); len(got) != 1 || !got[0].Blocking {
		t.Fatalf("dangling declaration passed: %+v", got)
	}
	if code := Agents(root, func(string) {}, "kilo"); code != 2 {
		t.Fatalf("dangling declaration accepted: %d", code)
	}
}

// proved by: before the existing-copy precondition fix, Check returned no
// findings for the installed Kilo copy without its master and this test failed.
func TestExistingHostWithoutDeclarationStillRequiresMaster(t *testing.T) {
	root := t.TempDir()
	writeRules(t, root, ".kilo/rules/procoder.md", "# Existing contract\n")
	for name, findings := range map[string][]gitx.Finding{"check": Check(root), "drift": AgentsDrift(root)} {
		if len(findings) != 1 || !findings[0].Blocking || findings[0].File != Master {
			t.Fatalf("%s silently skipped existing integration without master: %+v", name, findings)
		}
	}
}

// proved by: omitted $ARGUMENTS in the setup command examples; both commands
// failed here before forwarding was added to the canonical text and twins.
func TestSetupCommandExamplesForwardArguments(t *testing.T) {
	root := repoRoot(t)
	for _, command := range []string{"agents", "init"} {
		for _, dir := range []string{"commands", ".opencode/command", ".kilo/commands"} {
			path := filepath.Join(root, dir, command+".md")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			if strings.Count(text, " "+command+" $ARGUMENTS") != 2 {
				t.Errorf("%s must forward arguments in both initial and repeat invocations", path)
			}
		}
	}
}
