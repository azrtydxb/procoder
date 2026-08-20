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

// A repo with AGENTS.md but no copies has not adopted the layer — silence.
// Once one copy exists (adoption), drift blocks and the other copies are
// reported missing as information.
func TestDriftBlocksAndMissingInformsOnceAdopted(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, Master), []byte("# rules\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if findings := Check(root); len(findings) != 0 {
		t.Fatalf("un-adopted layer (AGENTS.md alone) must be silent, got %+v", findings)
	}

	c := Copies[0]
	p := filepath.Join(root, c.Path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(c.Frontmatter+"# rules\n\nDIFFERENT body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocking, info := 0, 0
	for _, f := range Check(root) {
		if f.Blocking {
			blocking++
		} else {
			info++
		}
	}
	if blocking != 1 {
		t.Fatalf("a drifted copy must block exactly once, got %d", blocking)
	}
	if info != len(Copies)-1 {
		t.Fatalf("adopted layer must report the %d other copies missing, got %d", len(Copies)-1, info)
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

// opencodeTwin reproduces the generator's substitution rule, so the test
// pins actual content parity, not just existence: twin = command body with
// the Claude launcher swapped for the PATH binary.
func opencodeTwin(body string) string {
	// CRLF leveled first: a Windows checkout rewrites line endings, and
	// the multi-line phrase below would otherwise never match there
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, `"${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"`, "procoder")
	body = strings.ReplaceAll(body, "The launcher is: procoder",
		"The command below is the `procoder` binary on PATH.")
	body = strings.ReplaceAll(body, "The launcher for every procoder command below is:\nprocoder",
		"Every procoder command below is the `procoder` binary on PATH.")
	body = strings.ReplaceAll(body, "launcher.sh ", "procoder ")
	body = strings.ReplaceAll(body, "launcher.sh", "procoder")
	return body
}

// Commands whose content is Claude-specific and deliberately ships no
// OpenCode twin.
var opencodeSkip = map[string]bool{
	"update.md": true, // the Claude plugin self-update flow
}

// Every Claude command skill has a content-identical OpenCode twin (via
// the generation rule), and no orphan twin lingers after a command is
// removed or renamed.
func TestOpenCodeCommandParity(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "commands"))
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") || opencodeSkip[e.Name()] {
			continue
		}
		sources[e.Name()] = true
		src, err := os.ReadFile(filepath.Join(root, "commands", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		twin := filepath.Join(root, ".opencode/command", e.Name())
		raw, err := os.ReadFile(twin)
		if err != nil {
			t.Errorf("command %s has no .opencode/command twin — regenerate", e.Name())
			continue
		}
		if strings.ReplaceAll(string(raw), "\r\n", "\n") != opencodeTwin(string(src)) {
			t.Errorf("%s does not match the generation rule applied to commands/%s — regenerate", twin, e.Name())
		}
	}
	twins, err := os.ReadDir(filepath.Join(root, ".opencode/command"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range twins {
		if strings.HasSuffix(e.Name(), ".md") && !sources[e.Name()] {
			t.Errorf(".opencode/command/%s has no commands/ source — stale twin", e.Name())
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

// The Kilo command set is the OpenCode twin set: Kilo's CLI is an OpenCode
// fork and reads the same command markdown, so the two directories hold the
// same files and no orphan survives a command being renamed or removed.
// proved by: deleted one .kilo/commands file — the parity check names it.
func TestKiloCommandParity(t *testing.T) {
	root := repoRoot(t)
	sources, err := os.ReadDir(filepath.Join(root, ".opencode/command"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, e := range sources {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		want[e.Name()] = true
		src, err := os.ReadFile(filepath.Join(root, ".opencode/command", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		twin, err := os.ReadFile(filepath.Join(root, ".kilo/commands", e.Name()))
		if err != nil {
			t.Errorf("command %s has no .kilo/commands twin — regenerate", e.Name())
			continue
		}
		if normalize(string(twin)) != normalize(string(src)) {
			t.Errorf(".kilo/commands/%s differs from the OpenCode twin — regenerate", e.Name())
		}
	}
	twins, err := os.ReadDir(filepath.Join(root, ".kilo/commands"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range twins {
		if strings.HasSuffix(e.Name(), ".md") && !want[e.Name()] {
			t.Errorf(".kilo/commands/%s has no OpenCode source — stale twin", e.Name())
		}
	}
}

// One plugin source serves both hosts, byte for byte. They differ only in
// the extension each host scans for, and a fix applied to one that misses
// the other is precisely the drift this pins.
// proved by: appended a line to one copy — the check names the pair.
func TestPluginTwinIsIdentical(t *testing.T) {
	root := repoRoot(t)
	a, err := os.ReadFile(filepath.Join(root, ".opencode/plugins/procoder.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".kilo/plugin/procoder.js"))
	if err != nil {
		t.Fatal(err)
	}
	if normalize(string(a)) != normalize(string(b)) {
		t.Error(".kilo/plugin/procoder.js is not the .opencode/plugins/procoder.mjs source — copy it across")
	}
}
