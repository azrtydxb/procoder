package portability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRules(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// `procoder agents` has always ended by printing "the gate blocks on
// drift", and docs/commands.md said the same. Neither was true: nothing
// in the gate, the hooks or CI ever asked. So Cursor, Windsurf, Cline,
// Kilo, Roo, Kiro, Codex and the rest could be reading rules this
// repository had stopped holding, while procoder reported clean — which
// is the exact failure the agent layer exists to prevent.
// proved by: returned nil from AgentsDrift — a drifted rule file passes
// the gate again, and the binary goes back to printing a promise it does
// not keep.
func TestDriftedRuleFilesBlock(t *testing.T) {
	root := t.TempDir()
	master := "# Rules\n\nAlways do the thing.\n"
	if err := os.WriteFile(filepath.Join(root, Master), []byte(master), 0o644); err != nil {
		t.Fatal(err)
	}
	// Every host missing: each is reported, and each blocks.
	got := AgentsDrift(root)
	if len(got) != len(Copies) {
		t.Fatalf("every host without a rule file must be reported: %d of %d", len(got), len(Copies))
	}
	for _, f := range got {
		if !f.Blocking {
			t.Errorf("a host reading nothing must block: %q", f.Message)
		}
	}

	// One host given content that differs is DRIFTED, and the message says
	// so rather than calling it missing — the reader has a file to fix,
	// not one to create.
	c := Copies[0]
	writeRules(t, root, c.Path, c.Frontmatter+"# Rules\n\nSomething else entirely.\n")
	var found bool
	for _, f := range AgentsDrift(root) {
		if f.File == c.Path {
			found = true
			if !strings.Contains(f.Message, "drifted") {
				t.Errorf("a file with different content has drifted, not gone: %q", f.Message)
			}
		}
	}
	if !found {
		t.Errorf("%s must still be reported", c.Path)
	}
}

// A file that matches is not a finding, and a repository with no
// AGENTS.md ships no agent layer and is asked nothing — or the check
// would block every repository that never opted in.
// proved by: reported on every host regardless of content — a repo with
// no AGENTS.md then cannot commit at all.
func TestMatchingFilesAndNoAgentLayerAreSilent(t *testing.T) {
	if got := AgentsDrift(t.TempDir()); len(got) != 0 {
		t.Errorf("no AGENTS.md means no agent layer: %+v", got)
	}

	root := t.TempDir()
	master := "# Rules\n\nAlways do the thing.\n"
	if err := os.WriteFile(filepath.Join(root, Master), []byte(master), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range Copies {
		writeRules(t, root, c.Path, c.Frontmatter+master)
	}
	if got := AgentsDrift(root); len(got) != 0 {
		t.Errorf("rule files that match must be silent: %+v", got)
	}
}
