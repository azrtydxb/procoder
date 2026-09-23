package security

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"procoder/internal/gitx"
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

// Agent tooling installs into the working tree — .kilocode/ and friends
// each drop a package-lock.json — and the walk scanned them, so an
// advisory against a package nobody here chose blocked every commit in
// the repository until someone else shipped a fix.
// proved by: restored the plain filepath.Walk — .kilocode/package-lock.json
// comes back in the scan set and the gate blocks on a dependency the
// repository does not have.
func TestGitignoredManifestsAreNotThisRepositorysDependencies(t *testing.T) {
	root := gitRepoForManifests(t)
	write := func(p, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", ".kilocode/\n")
	write("go.mod", "module x\n")
	write(filepath.FromSlash("web/app/package-lock.json"), "{}")
	write(filepath.FromSlash(".kilocode/package-lock.json"), "{}")
	write(filepath.FromSlash("vendor/x/go.mod"), "module vendor\n")
	if out, err := exec.Command("git", "-C", root, "add", "go.mod", "vendor/x/go.mod").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	got := manifestsIn(root)
	want := []string{"go.mod", "web/app/package-lock.json"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("manifests found:\n got %v\nwant %v", got, want)
	}
}

func TestEmptyGitInventoryDoesNotFallBackToIgnoredManifests(t *testing.T) {
	root := gitRepoForManifests(t)
	// Ignore the ignore file too: ls-files must successfully return no paths.
	for name, body := range map[string]string{
		".gitignore": "*\n", "package-lock.json": "{}",
		"package.json":   `{"dependencies":{"example":"1"}}`,
		"pyproject.toml": "[project]\ndependencies = [\"example\"]\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, got := range map[string][]string{"scan": manifestsIn(root), "npm": npmGaps(root), "python": pythonGaps(root)} {
		if len(got) != 0 {
			t.Errorf("%s included ignored files: %v", name, got)
		}
	}
}

func TestDeletedTrackedManifestsAreNotScanned(t *testing.T) {
	root := gitRepoForManifests(t)
	path := filepath.Join(root, "package-lock.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "add", "package-lock.json").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := manifestsIn(root); len(got) != 0 {
		t.Fatalf("deleted manifest passed to scanner: %v", got)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := manifestsIn(root); len(got) != 0 {
		t.Fatalf("directory replacing manifest passed to scanner: %v", got)
	}
}

func TestGitGapChecksShareManifestOwnership(t *testing.T) {
	root := gitRepoForManifests(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".kilocode/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"tracked", "untracked", ".kilocode", "vendor/x"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		for name, body := range map[string]string{
			"package.json":   `{"dependencies":{"example":"1"}}`,
			"pyproject.toml": "[project]\ndependencies = [\"example\"]\n",
		} {
			if err := os.WriteFile(filepath.Join(root, dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if out, err := exec.Command("git", "-C", root, "add", "tracked", "vendor").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	for name, got := range map[string][]string{"package.json": npmGaps(root), "pyproject.toml": pythonGaps(root)} {
		want := "tracked/" + name + ",untracked/" + name
		if strings.Join(got, ",") != want {
			t.Errorf("%s gaps: got %v, want %s", name, got, want)
		}
	}
}

func TestIgnoredLockfilesCannotCoverVisibleDependencies(t *testing.T) {
	root := gitRepoForManifests(t)
	for name, body := range map[string]string{
		".gitignore":        "package-lock.json\npoetry.lock\n",
		"package.json":      `{"workspaces":["web"],"dependencies":{"example":"1"}}`,
		"pyproject.toml":    "[project]\ndependencies = [\"example\"]\n",
		"package-lock.json": "{}",
		"poetry.lock":       "# ignored\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "web/package.json"), []byte(`{"dependencies":{"example":"1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := npmGaps(root); strings.Join(got, ",") != "package.json,web/package.json" {
		t.Fatalf("ignored sibling/workspace lockfile hid npm gaps: %v", got)
	}
	if got := pythonGaps(root); strings.Join(got, ",") != "pyproject.toml" {
		t.Fatalf("ignored sibling lockfile hid Python gap: %v", got)
	}
}

func TestGitManifestPathsAreNotQuoted(t *testing.T) {
	root := gitRepoForManifests(t)
	const dir = "space and caf\u00e9"
	if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, dir, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := manifestsIn(root); len(got) != 1 || got[0] != dir+"/go.mod" {
		t.Fatalf("Git-quoted path lost: %v", got)
	}
}

func gitRepoForManifests(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed — there is no gate file set to read")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return root
}

// gitIn runs git in root with an identity, so a fixture can commit on a
// machine that has none configured.
func gitIn(t *testing.T, root string, args ...string) {
	t.Helper()
	full := append([]string{"-C", root, "-c", "user.email=t@example.com", "-c", "user.name=t",
		"-c", "commit.gpgsign=false"}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// writeIn writes a repo-relative, slash-separated path under root.
func writeIn(t *testing.T, root, p, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// osvFlagging stubs osv-scanner with one that reports a vulnerable sharp
// 0.35.2 for every -L file containing "vulnerable", and nothing for the
// rest, naming each file by an absolute, symlink-resolved source path as
// osv-scanner does — so a test can tell exactly which files were scanned.
// It uses shell builtins only, so it still runs when PATH holds nothing
// else. Returns the directory holding the stub.
func osvFlagging(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a POSIX shell script")
	}
	bin := t.TempDir()
	const script = `#!/bin/sh
sep=''
printf '{"results":['
while [ $# -gt 0 ]; do
  if [ "$1" = -L ]; then
    shift
    hit=''
    while IFS= read -r line || [ -n "$line" ]; do
      case $line in *vulnerable*) hit=1 ;; esac
    done < "$1"
    if [ -n "$hit" ]; then
      printf '%s{"source":{"path":"%s/%s","type":"lockfile"},"packages":[{"package":{"name":"sharp","version":"0.35.2"},"vulnerabilities":[{"id":"GHSA-test"}],"groups":[{"max_severity":"8.9"}]}]}' "$sep" "$(pwd -P)" "$1"
      sep=','
    fi
  fi
  shift
done
printf ']}'
`
	if err := os.WriteFile(filepath.Join(bin, "osv-scanner"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return bin
}

// vulnFindings keeps the osv-scanner verdicts, dropping gap and
// nothing-found notes.
func vulnFindings(fs []gitx.Finding) []gitx.Finding {
	var out []gitx.Finding
	for _, f := range fs {
		if strings.Contains(f.Message, "known vulnerability") {
			out = append(out, f)
		}
	}
	return out
}

// #293: the commit that fixed a lockfile was refused by the gate, naming
// the versions the fix removed. Those versions lived in two places: HEAD's
// lockfile, and a second worktree nested in the checkout
// (.kilo/worktrees/<name>/, detached at the previous commit, not ignored).
// The scan reads the lockfile on disk (see the test below), so HEAD was
// never the source; 3.6.0's filepath.Walk descended into the nested
// worktree. A nested worktree or repository is another checkout: its
// lockfile is not what this commit carries, and nothing on this branch can
// change it. `procoder security` looked innocent only because it runs the
// dependency scan under --deep alone.
// proved by: dropped the nested-checkout skip from the fallback walk — the
// walk hands .kilo/worktrees/old/package-lock.json and
// tools/other/package-lock.json to the scanner, and the gate blocks the fix
// on sharp 0.35.2; on the pre-#285 code the git-inventory leg fails the
// same way.
func TestNestedCheckoutsAreNotThisCommitsDependencies(t *testing.T) {
	root := gitRepoForManifests(t)
	stubDir := osvFlagging(t)
	const lock = "package-lock.json"
	writeIn(t, root, lock, `{"sharp":"0.35.2 vulnerable"}`)
	gitIn(t, root, "add", lock)
	gitIn(t, root, "commit", "-q", "-m", "vulnerable")
	// The stale second checkout, pinned to the vulnerable commit.
	gitIn(t, root, "worktree", "add", "-q", "--detach", filepath.FromSlash(".kilo/worktrees/old"), "HEAD")
	// A nested repository of its own.
	gitIn(t, root, "init", "-q", "tools/other")
	writeIn(t, root, "tools/other/package-lock.json", `{"x":"vulnerable"}`)
	// The fix, in the working tree and the index alike.
	writeIn(t, root, lock, `{"sharp":"0.35.4"}`)
	gitIn(t, root, "add", lock)

	check := func(how string) {
		t.Helper()
		if got := manifestsIn(root); strings.Join(got, ",") != lock {
			t.Errorf("%s: the scan set reaches past this checkout: %v", how, got)
		}
		if got := vulnFindings(DepsChanged(root, []string{filepath.Join(root, lock)})); len(got) > 0 {
			t.Errorf("%s: the commit fixing the lockfile is blocked on a version it removes: %+v", how, got)
		}
	}
	check("git inventory")
	// Git unavailable: the filesystem walk must stop at the same boundary.
	t.Setenv("PATH", stubDir)
	check("filesystem walk")
}

// The scan judges the lockfile the commit carries, not HEAD's: a vulnerable
// version over a clean commit is reported, and reported against the file
// it came from, so a finding can be traced to its lockfile.
// proved by: dropped the source-path mapping — the finding carries no File
// and the reader of #293 is left guessing which lockfile it meant.
func TestTheScanReadsTheWorkingTreeNotHead(t *testing.T) {
	root := gitRepoForManifests(t)
	osvFlagging(t)
	const lock = "web/package-lock.json"
	writeIn(t, root, lock, `{"sharp":"0.35.4"}`)
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "clean")
	writeIn(t, root, lock, `{"sharp":"0.35.2 vulnerable"}`)

	got := vulnFindings(DepsChanged(root, []string{filepath.Join(root, filepath.FromSlash(lock))}))
	if len(got) != 1 {
		t.Fatalf("a vulnerable lockfile in the working tree must be reported once: %+v", got)
	}
	if got[0].File != lock || !got[0].Blocking {
		t.Errorf("finding must block and name the lockfile it came from: %+v", got[0])
	}
}
