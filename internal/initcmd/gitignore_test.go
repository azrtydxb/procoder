package initcmd

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// proved by: returned success without writing in IgnoreHosts; the empty-file
// case failed because no .gitignore was created.
func TestIgnoreHostsPreservesExistingRulesAndIsIdempotent(t *testing.T) {
	for _, original := range []string{"", "node_modules/", "# existing\r\n/.cursor/\r\n"} {
		t.Run(original, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".gitignore")
			if original != "" {
				if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := IgnoreHosts(root, io.Discard, []string{"all"}); err != nil {
				t.Fatal(err)
			}
			first, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(first), original) {
				t.Fatal("existing content changed")
			}
			if err := IgnoreHosts(root, io.Discard, []string{"all"}); err != nil {
				t.Fatal(err)
			}
			second, err := os.ReadFile(path)
			if err != nil || string(first) != string(second) {
				t.Fatalf("second init changed the file: %v", err)
			}
		})
	}
}

// proved by: made IgnoreHosts a successful no-op; git check-ignore rejected
// every selected host path rather than confirming it was ignored.
func TestIgnoreHostsGitSemantics(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := IgnoreHosts(root, io.Discard, []string{"all"}); err != nil {
		t.Fatal(err)
	}
	for _, pattern := range hostIgnores {
		path := strings.TrimPrefix(pattern, "/")
		if strings.HasSuffix(path, "/") {
			path += "rules/example.md"
		}
		if err := exec.Command("git", "-C", root, "check-ignore", "-q", path).Run(); err != nil {
			t.Errorf("%s is not ignored: %v", path, err)
		}
	}
	for _, path := range []string{".procoder/config.toml", ".procoder/specs/example.md", ".github/workflows/ci.yml", "skills/other/SKILL.md", "src/.cursor/example.md", "AGENTS.md"} {
		err := exec.Command("git", "-C", root, "check-ignore", "-q", path).Run()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Errorf("%s must remain trackable: %v", path, err)
		}
	}
}

// proved by: returned success before IgnoreHosts validation; a directory at
// .gitignore was accepted and this test failed.
func TestInitRefusesNonRegularGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := IgnoreHosts(root, io.Discard, []string{"kilo"}); err == nil {
		t.Fatal("init succeeded despite unwritable ignore file")
	}
}

// proved by: returned success before IgnoreHosts validation; the symlink was
// accepted instead of refused and this test failed.
func TestIgnoreHostsRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "ignore")
	if err := os.WriteFile(target, []byte("keep me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, ".gitignore")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := IgnoreHosts(root, io.Discard, []string{"all"}); err == nil {
		t.Fatal("symlink accepted")
	}
	raw, err := os.ReadFile(target)
	if err != nil || string(raw) != "keep me\n" {
		t.Fatalf("symlink target changed: %q, %v", raw, err)
	}
}

// proved by: made IgnoreHosts a successful no-op; the selected Kilo entry was
// absent and the first selection assertion failed.
func TestIgnoreOnlySelectedHostsAndAddWithoutRemoving(t *testing.T) {
	root := t.TempDir()
	if err := IgnoreHosts(root, io.Discard, []string{"kilo"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(raw), "/.kilo/\n") || strings.Contains(string(raw), "/.kilocode/") || strings.Contains(string(raw), "/.cursor/") {
		t.Fatalf("wrong hosts: %s", raw)
	}
	if err := IgnoreHosts(root, io.Discard, []string{"cursor"}); err != nil {
		t.Fatal(err)
	}
	added, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.HasPrefix(string(added), string(raw)) || !strings.Contains(string(added), "/.cursor/\n") {
		t.Fatal("additional host lost existing selection")
	}
}
