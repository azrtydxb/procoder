package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHome builds a plugin cache under a temporary HOME. registry is
// written verbatim, so a test can hand it something unparseable.
func fakeHome(t *testing.T, registry string, versions ...string) string {
	t.Helper()
	home := t.TempDir()
	cache := filepath.Join(home, ".claude", "plugins", "cache", "procoder", "procoder")
	for _, v := range versions {
		if err := os.MkdirAll(filepath.Join(cache, v), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cache, v, "blob"), make([]byte, 2048), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if registry != "" {
		p := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(registry), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // windows
	return home
}

const goodRegistry = `{"version":2,"plugins":{"procoder@procoder":[{"version":"3.1.0","installPath":"x"}]}}`

func runPrune(t *testing.T, apply bool) (int, string) {
	t.Helper()
	var b strings.Builder
	code := pruneCmd(apply, func(s string) { b.WriteString(s + "\n") })
	return code, b.String()
}

// S-2 at the command surface: bare `prune` exits 0, names the set, and
// leaves every directory where it was. The domain test proves Compute does
// not delete; this proves the COMMAND does not, which is what a person
// actually types.
//
// proved by: `pruneCmd(apply, ...)` made to call plugincache.Apply
// regardless of apply (the directories vanish from a bare run).
func TestBarePruneReportsAndRemovesNothing(t *testing.T) {
	home := fakeHome(t, goodRegistry, "3.1.0", "3.0.0", "2.0.1", "1.4.0")
	code, out := runPrune(t, false)
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "would remove") || !strings.Contains(out, "--apply") {
		t.Errorf("the report did not say what it would do or how:\n%s", out)
	}
	cache := filepath.Join(home, ".claude", "plugins", "cache", "procoder", "procoder")
	for _, v := range []string{"3.1.0", "3.0.0", "2.0.1", "1.4.0"} {
		if _, err := os.Stat(filepath.Join(cache, v)); err != nil {
			t.Errorf("%s was removed by a command that only reports", v)
		}
	}
}

// S-5 at the command surface: the exit code the criterion actually names.
// The domain returns an error; what CI and a person's shell read is this.
//
// proved by: `return 2` in pruneCmd's Compute-error branch → `return 0`
// (a refusal reads as success).
func TestARefusalExitsTwoAndRemovesNothing(t *testing.T) {
	for _, tc := range []struct{ name, registry string }{
		{"absent", ""},
		{"unparseable", "{not json at all"},
		{"procoder not listed", `{"version":2,"plugins":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := fakeHome(t, tc.registry, "3.1.0", "3.0.0", "2.0.1")
			code, out := runPrune(t, true)
			if code != 2 {
				t.Errorf("exit %d for %s, want 2\n%s", code, tc.name, out)
			}
			cache := filepath.Join(home, ".claude", "plugins", "cache", "procoder", "procoder")
			for _, v := range []string{"3.1.0", "3.0.0", "2.0.1"} {
				if _, err := os.Stat(filepath.Join(cache, v)); err != nil {
					t.Errorf("%s was removed despite the refusal", v)
				}
			}
		})
	}
}

// S-6: --apply says what went and how much came back, and the figure is
// the summed size of what was actually removed.
//
// proved by: the reclaimed value in the summary replaced with plan.Bytes
// (identical here, so the test also pins the fixture where they differ —
// see the domain's TestTheReclaimedFigureIsWhatActuallyWent).
func TestApplyNamesWhatWentAndWhatCameBack(t *testing.T) {
	fakeHome(t, goodRegistry, "3.1.0", "3.0.0", "2.0.1", "1.4.0")
	code, out := runPrune(t, true)
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	for _, want := range []string{"removed  2.0.1", "removed  1.4.0", "reclaimed", "keeping 3.1.0, 3.0.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report did not contain %q:\n%s", want, out)
		}
	}
	// 2 directories x 2048 bytes: stated so a silent change to the fixture
	// cannot quietly change what this asserts.
	if !strings.Contains(out, "4 KB") {
		t.Errorf("the reclaimed figure was not the 4 KB actually removed:\n%s", out)
	}
}

// S-1: nothing to do says so, rather than printing an empty report that
// reads like a failure.
//
// proved by: the `len(plan.Removable) == 0` branch removed (an empty
// "would remove" list is printed with a 0-version summary).
func TestNothingToRemoveSaysSo(t *testing.T) {
	fakeHome(t, goodRegistry, "3.1.0")
	code, out := runPrune(t, false)
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "nothing to remove") {
		t.Errorf("a cache with nothing to sweep did not say so:\n%s", out)
	}
}
