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

// A monorepo keeps one manifest per package. Scanning only the ones at
// the repository root reported clean over every package beneath it — in
// `security --deep` as well as at the gate — and once the gate began
// triggering on a nested manifest, a commit paid for a scan that could
// not look at the file that triggered it.
// proved by: restored the root-only os.Stat loop — services/api/go.mod
// and web/app/package-lock.json vanish from the scan, and a monorepo
// commits dependency changes unscanned while being charged for the scan.
func TestManifestsAreFoundBeneathTheRootToo(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"go.mod",
		filepath.FromSlash("services/api/go.mod"),
		filepath.FromSlash("web/app/package-lock.json"),
		// Not ours: a vendored copy and an installed package carry their
		// own manifests, and reporting on them means reporting code
		// nobody here can change.
		filepath.FromSlash("node_modules/evil/package-lock.json"),
		filepath.FromSlash("vendor/x/go.mod"),
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := manifestsIn(root)
	want := []string{"go.mod", "services/api/go.mod", "web/app/package-lock.json"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("manifests found:\n got %v\nwant %v", got, want)
	}
}

// The same gap one level down: a nested package.json declaring
// dependencies with no lockfile beside it is an unscannable package, and
// checking only the repository root reports the first and stays silent
// about the rest.
// proved by: called hasNpmDepsWithoutLockfile(root) directly again — the
// nested package is not reported and its dependencies go unscanned with
// nothing said.
func TestEveryPackageWithoutALockfileIsNamed(t *testing.T) {
	root := t.TempDir()
	write := func(p, body string) {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const withDeps = `{"dependencies":{"left-pad":"1.0.0"}}`
	write("package.json", withDeps)
	write(filepath.FromSlash("web/app/package.json"), withDeps)
	// This one has its lockfile, so it is scannable and not a gap.
	write(filepath.FromSlash("web/ok/package.json"), withDeps)
	write(filepath.FromSlash("web/ok/package-lock.json"), "{}")

	got := npmGaps(root)
	want := []string{"package.json", "web/app/package.json"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("packages with no lockfile:\n got %v\nwant %v", got, want)
	}
}
