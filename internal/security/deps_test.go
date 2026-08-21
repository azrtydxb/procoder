package security

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stubOsv puts an osv-scanner on PATH that reports nothing, so these
// tests measure WHEN the scan runs rather than what a real scanner finds.
func stubOsv(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a POSIX shell script")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "osv-scanner"),
		[]byte("#!/bin/sh\necho '{\"results\":[]}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// The scan answers about the manifests, so it runs when the commit
// touches one and not otherwise. Running it on every commit would report
// the same vulnerabilities forever at nearly a second each time, which is
// how a check becomes something people route around.
// proved by: dropped the touchesManifest guard — a commit editing a
// comment pays for a full dependency scan and is told what it was told
// last time.
func TestTheScanRunsWhenAManifestChanges(t *testing.T) {
	stubOsv(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := DepsChanged(root, []string{filepath.Join(root, "README.md")}); got != nil {
		t.Errorf("a commit touching no manifest must not scan: %+v", got)
	}
	// "Ran and found nothing" and "never ran" are both an empty slice, so
	// the scan is made observable: a package.json declaring dependencies
	// with no lockfile is a gap only a real run reports.
	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"dependencies":{"left-pad":"1.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DepsChanged(root, []string{filepath.Join(root, "package.json")})
	var ran bool
	for _, f := range got {
		if strings.Contains(f.Message, "no lockfile") {
			ran = true
		}
	}
	if !ran {
		t.Errorf("touching a manifest must actually run the scan: %+v", got)
	}
	// And a commit that touches nothing relevant still does not run it,
	// now that a run would be visible.
	if others := DepsChanged(root, []string{filepath.Join(root, "README.md")}); others != nil {
		t.Errorf("an unrelated commit must still skip the scan: %+v", others)
	}
}

// Every name osv-scanner accepts triggers the scan, plus the package.json
// whose missing lockfile Deps reports on. A list that drifts from
// DepManifests would leave an ecosystem silently unscanned at the gate
// while `security --deep` still covered it.
// proved by: hard-coded a short list instead of deriving it from
// DepManifests — Cargo.lock and composer.lock stop triggering, and Rust
// and PHP repositories commit dependency changes unscanned.
func TestEveryManifestNameTriggersTheScan(t *testing.T) {
	root := t.TempDir()
	for _, m := range append([]string{"package.json"}, DepManifests...) {
		if !touchesManifest(root, []string{filepath.Join(root, m)}) {
			t.Errorf("%s must trigger the dependency scan", m)
		}
	}
	for _, other := range []string{"main.go", "README.md", "go.sum"} {
		if touchesManifest(root, []string{filepath.Join(root, other)}) {
			t.Errorf("%s is not a manifest osv-scanner reads", other)
		}
	}
}

// A path that arrives relative must trigger the scan too — the same file
// cannot mean two things depending on how it was named.
// proved by: matched on the raw string instead of gitx.RepoRel — an
// absolute path fails path.Base's expectations or a relative one is
// dropped, depending which way it is written.
func TestAManifestTriggersHoweverThePathArrived(t *testing.T) {
	root := t.TempDir()
	if !touchesManifest(root, []string{filepath.Join(root, "go.mod")}) {
		t.Error("absolute form must trigger")
	}
	if !touchesManifest(root, []string{"go.mod"}) {
		t.Error("relative form must trigger")
	}
	if !touchesManifest(root, []string{filepath.FromSlash("services/api/go.mod")}) {
		t.Error("a nested relative manifest must trigger")
	}
}
