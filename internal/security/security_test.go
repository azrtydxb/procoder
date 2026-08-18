package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// A planted high-entropy token must come back as a blocking finding that
// names the rule and location and NEVER echoes the secret itself.
func TestSecretIsBlockingAndNeverEchoed(t *testing.T) {
	if tools.Resolve(Gitleaks, "") == "" {
		t.Skip("gitleaks not installed; the not-checked path is tested below")
	}
	root := t.TempDir()
	// assembled at runtime so no scanner ever matches this SOURCE file —
	// the written fixture file is what must be caught
	token := "ghp_" + "x9J2mQ8v" + "LpZ4tR7n" + "W3kY6bC1" + "dF5gH0aSeD"
	f := filepath.Join(root, "cfg.py")
	os.WriteFile(f, []byte("token = \""+token+"\"\n"), 0o644)

	got := Secrets(root, []string{f})
	if len(got) != 1 || !got[0].Blocking || got[0].Line != 1 {
		t.Fatalf("want one blocking line-1 finding, got %+v", got)
	}
	if strings.Contains(got[0].Message, token) || strings.Contains(got[0].File, token) {
		t.Fatal("the secret value must never be echoed")
	}
	if !strings.Contains(got[0].Message, "rotate") {
		t.Fatalf("the finding must tell the agent to rotate: %+v", got[0])
	}
	if got[0].File != f {
		t.Fatalf("single-file scan must keep the path as given: %q", got[0].File)
	}
}

func TestCleanFileIsSilent(t *testing.T) {
	if tools.Resolve(Gitleaks, "") == "" {
		t.Skip("gitleaks not installed")
	}
	root := t.TempDir()
	f := filepath.Join(root, "ok.py")
	os.WriteFile(f, []byte("x = 1\n"), 0o644)
	if got := Secrets(root, []string{f}); len(got) != 0 {
		t.Fatalf("clean file must be silent: %+v", got)
	}
}

// Missing scanners are blocking NOT-checked, never silence — a security
// check that quietly didn't run is worse than a red one.
func TestMissingToolsReadAsBlockingNotChecked(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if got := Secrets("/tmp", []string{"/tmp"}); len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("missing gitleaks: %+v", got)
	}
	if got := Sast("/tmp"); len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("missing semgrep: %+v", got)
	}
	if got := Deps("/tmp"); len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("missing osv-scanner: %+v", got)
	}
}
