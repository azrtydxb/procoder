package config

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func reportRepo(t *testing.T, cfgBody string, remotes map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if cfgBody != "" {
		if err := os.WriteFile(filepath.Join(dir, ".procoder", "config.toml"), []byte(cfgBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	args := [][]string{{"init", "-q", "-b", "main"}}
	for name, url := range remotes {
		args = append(args, []string{"remote", "add", name, url})
	}
	for _, a := range args {
		if out, err := exec.Command("git", append([]string{"-C", dir}, a...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", a, err, out)
		}
	}
	return dir
}

// proved by: printing the identity without its rung leaves an unexpected key
// with nothing to explain it, which is the whole reason the rung is carried.
func TestConfigPrintsIdentityRung(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cfg     string
		remotes map[string]string
		wantKey string
		wantSrc string
	}{
		{"config", "[service]\nrepo = \"acme/widgets\"\n",
			map[string]string{"origin": "https://host/o/r.git"},
			"acme/widgets", "[service] repo in .procoder/config.toml"},
		{"origin", "",
			map[string]string{"origin": "https://host/o/r.git", "fork": "https://host/me/r.git"},
			"host/o/r", "origin remote"},
		{"remote", "",
			map[string]string{"upstream": "https://host/o/r.git", "fork": "https://host/me/r.git"},
			"host/me/r", "first remote alphabetically: fork"},
		{"path", "", nil, "", "no remote — repository root path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := reportRepo(t, tc.cfg, tc.remotes)
			var buf bytes.Buffer
			Report(root, &buf)

			var line string
			for _, l := range strings.Split(buf.String(), "\n") {
				if strings.HasPrefix(l, "repo identity") {
					line = l
					break
				}
			}
			if line == "" {
				t.Fatalf("output has no \"repo identity\" line:\n%s", buf.String())
			}
			if tc.wantKey != "" && !strings.Contains(line, tc.wantKey) {
				t.Errorf("line %q does not carry the key %q", line, tc.wantKey)
			}
			if !strings.Contains(line, tc.wantSrc) {
				t.Errorf("line %q does not name the rung %q", line, tc.wantSrc)
			}
		})
	}
}

// proved by: adding the identity to cfg.Settings instead would make it a
// setting with a default that can be relaxed, which it is not.
func TestIdentityIsNotASetting(t *testing.T) {
	root := reportRepo(t, "", map[string]string{"origin": "https://host/o/r.git"})
	for _, s := range Load(root).Settings {
		if strings.Contains(s.Key, "identity") || s.Key == "service.repo" {
			t.Fatalf("identity leaked into the settings table as %q", s.Key)
		}
	}
}
