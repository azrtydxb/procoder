package portability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/host"
)

// repoRoot walks up from the test's working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above test dir")
		}
		dir = parent
	}
}

// The real repo must pass its own portability check — every rule copy in
// sync, every manifest at the plugin version, no forbidden paths.
func TestOwnRepoIsInSync(t *testing.T) {
	findings := Check(repoRoot(t))
	for _, f := range findings {
		t.Errorf("%s: %s", f.File, f.Message)
	}
}

// Every declared copy actually exists in the repo — the layer ships whole.
func TestEveryDeclaredCopyShips(t *testing.T) {
	root := repoRoot(t)
	for _, c := range Copies {
		if _, err := os.Stat(filepath.Join(root, c.Path)); err != nil {
			t.Errorf("%s (%s) missing from the repo", c.Path, c.Host)
		}
	}
}

// Drift in a copy must block; a missing copy is information.
func TestDriftBlocksAndMissingInforms(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, Master), []byte("# rules\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := Check(root)
	for _, f := range findings {
		if f.Blocking {
			t.Errorf("missing copies must not block: %s", f.Message)
		}
	}
	if len(findings) != len(Copies) {
		t.Fatalf("want one info finding per missing copy, got %d", len(findings))
	}

	c := Copies[0]
	p := filepath.Join(root, c.Path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(c.Frontmatter+"# rules\n\nDIFFERENT body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocking := 0
	for _, f := range Check(root) {
		if f.Blocking {
			blocking++
		}
	}
	if blocking != 1 {
		t.Fatalf("a drifted copy must block exactly once, got %d", blocking)
	}
}

// Frontmatter and line endings are host metadata, not drift.
func TestFrontmatterAndCRLFAreNotDrift(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, Master), []byte("# rules\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Copies[0] // cursor, has frontmatter
	p := filepath.Join(root, c.Path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	crlf := strings.ReplaceAll(c.Frontmatter+"# rules\n\nbody\n", "\n", "\r\n")
	if err := os.WriteFile(p, []byte(crlf), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range Check(root) {
		if f.Blocking {
			t.Errorf("frontmatter+CRLF copy flagged as drift: %s", f.Message)
		}
	}
}

// A stale manifest version must block.
func TestStaleManifestVersionBlocks(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(Master, "# rules\n")
	write(".claude-plugin/plugin.json", `{"name":"x","version":"1.2.3"}`)
	write(".codex-plugin/plugin.json", `{"name":"x","version":"1.2.2"}`)
	write("plugin.yaml", "name: x\nversion: 1.2.3\n")
	blocking := 0
	for _, f := range Check(root) {
		if f.Blocking {
			blocking++
			if !strings.Contains(f.Message, "1.2.2") {
				t.Errorf("stale version not named: %s", f.Message)
			}
		}
	}
	if blocking != 1 {
		t.Fatalf("exactly the stale manifest blocks, got %d", blocking)
	}
}

// The Gemini trap: a root hooks/hooks.json must never exist.
func TestForbiddenHooksJSONBlocks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, Master), []byte("# rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hooks/hooks.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range Check(root) {
		if f.Blocking && strings.Contains(f.Message, "must not exist") {
			found = true
		}
	}
	if !found {
		t.Fatal("hooks/hooks.json existence must block")
	}
}

// Every Claude command skill has its OpenCode twin, and no twin is stale:
// the twin is the same body with the launcher path swapped for the PATH
// binary — so it must never mention the Claude launcher.
func TestOpenCodeCommandParity(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "commands"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		twin := filepath.Join(root, ".opencode/command", e.Name())
		raw, err := os.ReadFile(twin)
		if err != nil {
			t.Errorf("command %s has no .opencode/command twin", e.Name())
			continue
		}
		if strings.Contains(string(raw), "CLAUDE_PLUGIN_ROOT") || strings.Contains(string(raw), "launcher.sh") {
			t.Errorf("%s still references the Claude launcher — regenerate it", twin)
		}
	}
}

// Host detection: ordering and the VS Code heuristic.
func TestHostDetection(t *testing.T) {
	clear := func() {
		for _, k := range []string{"COPILOT_PLUGIN_DATA", "PLUGIN_DATA", "QODER_SESSION_ID", "CLAUDE_PLUGIN_ROOT"} {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	clear()
	if got := host.Detect(); got != host.Claude {
		t.Errorf("bare env: want claude, got %s", got)
	}
	t.Setenv("PLUGIN_DATA", "/x")
	if got := host.Detect(); got != host.Codex {
		t.Errorf("PLUGIN_DATA: want codex, got %s", got)
	}
	t.Setenv("COPILOT_PLUGIN_DATA", "/y") // copilot also sets PLUGIN_DATA-like vars: copilot wins
	if got := host.Detect(); got != host.Copilot {
		t.Errorf("both set: want copilot, got %s", got)
	}
	clear()
	t.Setenv("CLAUDE_PLUGIN_ROOT", "/home/u/.vscode/extensions/agent-plugins/procoder")
	if got := host.Detect(); got != host.Copilot {
		t.Errorf("vscode copilot root: want copilot, got %s", got)
	}
	clear()
	t.Setenv("QODER_SESSION_ID", "abc")
	if got := host.Detect(); got != host.Qoder {
		t.Errorf("qoder: want qoder, got %s", got)
	}
}
