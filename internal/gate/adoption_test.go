package gate

import (
	"os"
	"path/filepath"
	"testing"
)

func tmp(t *testing.T) string { return t.TempDir() }

func put(t *testing.T, dir, name, body string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// S-1: adoption is a property of the repository in front of you, never of
// the machine running the gate. A .procoder/ directory is the plainest
// possible statement of it.
//
// proved by: `return Adopted, ".procoder/ is here"` → `return Universal,...`
// (want adopted got universal).
func TestProcoderDirectoryMeansAdopted(t *testing.T) {
	dir := tmp(t)
	put(t, dir, ".procoder/config.toml", "")
	if s, _ := ScopeFor(dir, ""); s != Adopted {
		t.Fatalf("a repository with .procoder/ read as %s", s)
	}
}

// S-1: a repository that names procoder in its AGENTS.md has adopted it
// even without the directory — that file is how an agent is told to use it.
//
// proved by: `agentsNamesProcoder(root)` → `false` (want adopted got
// universal).
func TestAgentsFileNamingProcoderMeansAdopted(t *testing.T) {
	dir := tmp(t)
	put(t, dir, "AGENTS.md", "# Agents\n\nRun `procoder check` before committing.\n")
	if s, _ := ScopeFor(dir, ""); s != Adopted {
		t.Fatalf("a repository whose AGENTS.md names procoder read as %s", s)
	}
}

// The other side of it, and the whole point of #172: an AGENTS.md that
// belongs to some other tool is not an adoption of procoder.
//
// proved by: in agentsNamesProcoder, the strings.Contains call → `true`
// (want universal got adopted).
func TestAgentsFileAboutSomethingElseIsNotAdoption(t *testing.T) {
	dir := tmp(t)
	put(t, dir, "AGENTS.md", "# Agents\n\nThis project uses Copilot and its own review bot.\n")
	if s, _ := ScopeFor(dir, ""); s != Universal {
		t.Fatalf("somebody else's AGENTS.md read as %s", s)
	}
}

// A bare clone of somebody else's project is the default case, and it must
// default to the narrow gate rather than to procoder's house style.
//
// proved by: the final `return Universal, ...` → `return Adopted, ...`
// (want universal got adopted).
func TestARepositoryWithNeitherIsUniversal(t *testing.T) {
	dir := tmp(t)
	put(t, dir, "main.go", "package main\n")
	if s, _ := ScopeFor(dir, ""); s != Universal {
		t.Fatalf("a repository with no sign of procoder read as %s", s)
	}
}

// S-5: either mode can be forced. Config wins over everything, because it
// is the repository speaking rather than the shell that happened to run.
//
// proved by: move the config check below the env check (want universal got
// adopted — the env var would win).
func TestConfigScopeOverridesEverything(t *testing.T) {
	dir := tmp(t)
	put(t, dir, ".procoder/config.toml", "")
	t.Setenv(ScopeEnv, "adopted")
	s, why := ScopeFor(dir, "universal")
	if s != Universal {
		t.Fatalf("config scope was ignored: got %s (%s)", s, why)
	}
}

// The environment variable is the escape hatch for a one-off run — it must
// beat detection, and be beaten by config.
//
// proved by: delete the os.Getenv(ScopeEnv) branch (want adopted got
// universal).
func TestEnvironmentOverridesDetection(t *testing.T) {
	dir := tmp(t)
	put(t, dir, "main.go", "package main\n")
	t.Setenv(ScopeEnv, "adopted")
	if s, _ := ScopeFor(dir, ""); s != Adopted {
		t.Fatalf("%s was ignored: got %s", ScopeEnv, s)
	}
}

// A misspelled override must not silently pick a mode. It falls through to
// detection, which is the answer the repository itself gives.
//
// The fixture must be a repository detection calls ADOPTED, because
// parseScope's default return value is Universal: in a non-adopting
// repository an accepted typo and a correct fallthrough both give
// universal and the test would prove nothing.
//
// Which fixture can tell the two apart is decided by that zero value —
// exactly the shape of the sprint-021 defect this repository now has a
// spec check for. When the value flipped in review, this fixture had to
// flip with it.
//
// proved by: in parseScope, `return Universal, false` → `return Universal,
// true` (the typo is taken as a mode, want adopted got universal).
func TestAnUnreadableOverrideFallsBackToDetection(t *testing.T) {
	dir := tmp(t)
	put(t, dir, ".procoder/config.toml", "")
	if s, why := ScopeFor(dir, "adoptedd"); s != Adopted {
		t.Fatalf("a typo in the override was taken as a mode: got %s (%s)", s, why)
	}
}

// S-5 again: the reason is printed, not just the mode. A gate that has
// quietly halved itself and does not say why is the same failure as one
// that reports nothing.
//
// proved by: `return Adopted, ".procoder/ is here"` → `return Adopted, ""`
// (want a non-empty reason).
func TestScopeAlwaysExplainsItself(t *testing.T) {
	for _, tc := range []struct{ name, cfg, file string }{
		{"detected adopted", "", ".procoder/config.toml"},
		{"detected universal", "", "main.go"},
		{"forced", "universal", ".procoder/config.toml"},
	} {
		dir := tmp(t)
		put(t, dir, tc.file, "")
		if _, why := ScopeFor(dir, tc.cfg); why == "" {
			t.Errorf("%s: the gate chose a scope and gave no reason", tc.name)
		}
	}
}
