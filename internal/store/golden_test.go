// Package store_test holds the parity harness.
//
// It is an EXTERNAL test package on purpose. The harness drives the real
// entrypoints — status, principles, the stop hook, the config report — and
// those packages import internal/store. An in-package test importing them
// back would be an import cycle.
package store_test

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/config"
	"procoder/internal/hook"
	"procoder/internal/principles"
	"procoder/internal/status"
)

var update = flag.Bool("update", false, "rewrite the golden files from the binary named by PROCODER_GOLDEN_BIN")

// fixture builds a small repository with a fixed git history.
//
// The dates are pinned because a golden that moves with the calendar is a
// golden that fails tomorrow for a reason nobody changed.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	for _, d := range []string{
		".procoder/specs", ".procoder/backlog/stories", ".procoder/backlog/sprints",
		".procoder/todo", ".procoder/adr",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		".procoder/specs/thing.md": "# thing\n\nStatus: complete\n\n## Problem\n\nA problem.\n",
		".procoder/config.toml":    "[ask]\npolicy = \"report\"\n",
		"main.go":                  "package main\n\nfunc main() {}\n",
		"README.md":                "# readme\n\nText.\n",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run := func(env []string, args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(nil, "init", "-q", "-b", "main")
	run(nil, "add", "-A")
	run([]string{
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	}, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "seed")
	return root
}

// replaceLinePrefix rewrites the whole of any line starting with prefix,
// so a nondeterministic value can be held out of a golden without losing
// the fact that the line is printed at all.
func replaceLinePrefix(text, prefix, with string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			lines[i] = with
		}
	}
	return strings.Join(lines, "\n")
}

// underClaudeHost blanks every variable host.Detect consults, runs, and puts
// them back.
//
// The principles golden is the Claude Code shape — raw text, no envelope — and
// Detect answers from the environment, so this golden was always quietly
// asserting that the developer's shell was not some other host. It is not a
// hypothetical: run the suite from inside a pi session and PI_CODING_AGENT is
// exported, the default branch is chosen, and the same code prints a JSON
// envelope where the golden holds prose.
func underClaudeHost(run func() string) string {
	keys := []string{"COPILOT_PLUGIN_DATA", "CLAUDE_PLUGIN_ROOT", "PLUGIN_DATA", "QODER_SESSION_ID", "PI_CODING_AGENT"}
	saved := map[string]string{}
	for _, k := range keys {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	defer func() {
		for _, k := range keys {
			if v := saved[k]; v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()
	return run()
}

// captures are the outputs the harness compares. Each runs in process.
//
// What is NOT here is as deliberate as what is. `procoder check`, the
// pre-tool-use hook and the post-tool-use hook all exec gitleaks, semgrep,
// golangci-lint or the test runner, whose presence and version vary by
// machine. A golden of those would pin THIS laptop rather than procoder's
// behaviour, and would fail in CI for reasons no change caused.
var captures = map[string]func(root string) string{
	"status": func(root string) string {
		var b strings.Builder
		status.Run(root, func(s string) { b.WriteString(s + "\n") })
		return b.String()
	},
	"principles-hook": func(root string) string {
		return underClaudeHost(func() string {
			var b strings.Builder
			principles.RunHook(root, strings.NewReader("{}"), func(s string) { b.WriteString(s + "\n") })
			return b.String()
		})
	},
	"handoff": func(root string) string {
		hook.Stop(strings.NewReader("{}"), root)
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(".procoder/state/handoff.md")))
		if err != nil {
			return "NO HANDOFF: " + err.Error()
		}
		// The note stamps the wall clock. That the line is THERE is the
		// behaviour; its value is not, and leaving it in would make this
		// golden fail one second after it was written. Caught by
		// TestCapturesAreDeterministic, which is why that test exists.
		return replaceLinePrefix(string(raw), "generated: ", "generated: <time>")
	},
	"config": func(root string) string {
		var b bytes.Buffer
		config.Report(root, &b)
		// The identity falls to the path rung in a fixture with no remote,
		// and a temp path is different on every run and every machine. The
		// rung is the behaviour under test; the path is not.
		var out strings.Builder
		for _, line := range strings.Split(b.String(), "\n") {
			if strings.HasPrefix(line, "repo identity") {
				out.WriteString("repo identity  <root>  (" + line[strings.Index(line, "(")+1:])
			} else {
				out.WriteString(line)
			}
			out.WriteString("\n")
		}
		return out.String()
	},
}

// proved by: any change to what these commands print, anywhere in the
// twenty-five packages the seam touches, fails this with a diff.
//
// The goldens for status and handoff were captured from c4bb353 — the commit
// before internal/store existed — so they assert that moving every read and
// write behind the store changed nothing.
//
// config and principles-hook are NOT parity goldens, and each stopped being
// one for a stated reason. config gained the repo identity line on purpose.
// principles-hook gained the receipt check and end marker on purpose, once
// the host was measured inlining only the first 2KB of a 10KB payload — the
// output HAD to change, because the old one was arriving four fifths missing
// with nothing saying so. Both guard drift from here rather than parity with
// before.
//
// Regenerating either of the remaining two is how a parity assertion stops
// asserting anything. Do not.
func TestMigrationOutputUnchanged(t *testing.T) {
	if *update {
		t.Skip("-update writes goldens; run TestUpdateGoldens instead")
	}
	for name, capture := range captures {
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", "golden", name+".txt"))
			if err != nil {
				t.Fatalf("no golden for %s: %v", name, err)
			}
			got := capture(fixture(t))
			if got != string(want) {
				t.Errorf("output drifted.\n--- want ---\n%s\n--- got ---\n%s", want, got)
			}
		})
	}
}

// proved by: a command whose output moves between two runs would make every
// golden a record of one roll of the dice, and would fail for everybody
// afterwards for no reason anybody changed.
func TestCapturesAreDeterministic(t *testing.T) {
	for name, capture := range captures {
		t.Run(name, func(t *testing.T) {
			first := capture(fixture(t))
			second := capture(fixture(t))
			if first != second {
				t.Errorf("two runs differ, so a golden cannot mean anything.\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
		})
	}
}

// TestUpdateGoldens rewrites the golden files from the CURRENT code. It runs
// only under -update and is how the config and principles-hook goldens were
// made; the other two
// were captured from the pre-store binary and must not be regenerated
// casually, because regenerating them is exactly how a parity assertion
// stops asserting anything.
func TestUpdateGoldens(t *testing.T) {
	if !*update {
		t.Skip("run with -update to rewrite goldens")
	}
	dir := filepath.Join("testdata", "golden")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, capture := range captures {
		if parityGoldens[name] {
			// Enforced, not asked for. A comment saying "do not" is the
			// only thing that stood between -update and the two captures
			// whose whole value is that they came from a binary built
			// before internal/store existed — and this branch has just
			// regenerated a third, which is exactly when the other two are
			// most at risk.
			t.Logf("skipped %s — it asserts parity with c4bb353 and must not be regenerated", name)
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(capture(fixture(t))), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", name)
	}
}

// parityGoldens are the captures taken from c4bb353, before internal/store
// existed. Regenerating one is how a parity assertion stops asserting
// anything, so -update refuses them.
var parityGoldens = map[string]bool{"status": true, "handoff": true}
