package portability

import (
	"os"
	"path/filepath"
	"runtime"
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
			// The blocking assertion belongs here too, not only on the
			// missing case: drift is the reason this function exists, and
			// a regression making it advisory would otherwise pass.
			if !f.Blocking {
				t.Errorf("a drifted rule file must block: %q", f.Message)
			}
		}
	}
	if !found {
		t.Errorf("%s must still be reported", c.Path)
	}
}

// An AGENTS.md that exists and cannot be read is not a repository without
// an agent layer. Returning nothing there would disable the whole drift
// check on a permission or IO error — unknown reported as clean, the one
// verdict this gate must never produce.
// proved by: returned nil for any read error again — an unreadable
// AGENTS.md silently switches the check off and every host passes.
func TestAnUnreadableMasterIsNotAnAbsentAgentLayer(t *testing.T) {
	// chmod 000 does not make a file unreadable on Windows, so the file
	// stays readable there, the drift check runs normally, and this test
	// would be asserting the opposite of what it is named for. The same
	// guard the adr sweep already uses, for the same reason.
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 does not make a file unreadable on Windows")
	}
	root := t.TempDir()
	path := filepath.Join(root, Master)
	if err := os.WriteFile(path, []byte("# Rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Skip("cannot make a file unreadable here: ", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if os.Geteuid() == 0 {
		t.Skip("root reads anything, so this cannot be exercised")
	}

	got := AgentsDrift(root)
	if len(got) != 1 {
		t.Fatalf("an unreadable master must be reported once, got %+v", got)
	}
	if !got[0].Blocking {
		t.Error("a check that could not run must block")
	}
	if !strings.Contains(got[0].Message, "unreadable") {
		t.Errorf("the reason must say what happened: %q", got[0].Message)
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
