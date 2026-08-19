package envsync

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// theSecret is planted as the VALUE of every env var in these fixtures. It
// must never appear in any output or in the state file — the whole security
// contract of this package in one string.
const theSecret = "s3cr3t-VALUE-that-must-never-be-printed"

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runEnv runs the package and returns the whole report as one string plus the
// exit code — output is what this package is, so tests read it as text.
func runEnv(t *testing.T, root string, sync bool) (string, int) {
	t.Helper()
	var lines []string
	code := Run(root, sync, func(s string) { lines = append(lines, s) })
	return strings.Join(lines, "\n"), code
}

func mustContain(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("missing %q in:\n%s", want, got)
	}
}

func TestChangedLockfileNamesItAndItsInstallCommand(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package-lock.json", `{"name":"a","lockfileVersion":3}`)
	if _, code := runEnv(t, root, true); code != 0 {
		t.Fatalf("--sync exit %d", code)
	}
	write(t, root, "package-lock.json", `{"name":"a","lockfileVersion":3,"packages":{}}`)

	got, code := runEnv(t, root, false)
	if code != 0 {
		t.Fatalf("report exit %d:\n%s", code, got)
	}
	mustContain(t, got, "package-lock.json")
	mustContain(t, got, "dependencies changed since your last sync")
	mustContain(t, got, "npm ci")
}

func TestGoneAndNewLockfilesAreBothNamed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package-lock.json", "one")
	if _, code := runEnv(t, root, true); code != 0 {
		t.Fatal("sync failed")
	}
	if err := os.Remove(filepath.Join(root, "package-lock.json")); err != nil {
		t.Fatal(err)
	}
	write(t, root, "Cargo.lock", "two")

	got, code := runEnv(t, root, false)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, got)
	}
	mustContain(t, got, "package-lock.json is gone since your last sync")
	mustContain(t, got, "Cargo.lock is new since your last sync — run cargo fetch")
}

func TestMigrationsAddedAreCounted(t *testing.T) {
	root := t.TempDir()
	write(t, root, "db/migrate/001_init.sql", "create table a();")
	if _, code := runEnv(t, root, true); code != 0 {
		t.Fatal("sync failed")
	}
	write(t, root, "db/migrate/002_users.sql", "create table users();")
	write(t, root, "db/migrate/003_posts.sql", "create table posts();")

	got, code := runEnv(t, root, false)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, got)
	}
	mustContain(t, got, "db/migrate/: 2 migration(s) added since your last sync")
	mustContain(t, got, "002_users.sql")
	mustContain(t, got, "003_posts.sql")
}

func TestMigrationsRemovedReadAsChangedNeverNegative(t *testing.T) {
	root := t.TempDir()
	write(t, root, "migrations/001.sql", "a")
	write(t, root, "migrations/002.sql", "b")
	if _, code := runEnv(t, root, true); code != 0 {
		t.Fatal("sync failed")
	}
	if err := os.Remove(filepath.Join(root, "migrations", "002.sql")); err != nil {
		t.Fatal(err)
	}

	got, code := runEnv(t, root, false)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, got)
	}
	mustContain(t, got, "migrations/: migration set changed since your last sync")
	if strings.Contains(got, "-1 migration") || strings.Contains(got, "added") {
		t.Fatalf("a removal must never read as an addition:\n%s", got)
	}
}

func TestNewEnvKeyIsNamedAndNoValueEverLeaks(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".env.example", "DATABASE_URL="+theSecret+"\nREDIS_URL="+theSecret+"\n")
	write(t, root, ".env", "DATABASE_URL="+theSecret+"\n")
	if _, code := runEnv(t, root, true); code != 0 {
		t.Fatal("sync failed")
	}

	got, code := runEnv(t, root, false)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, got)
	}
	mustContain(t, got, "new env var(s) declared: REDIS_URL")
	if strings.Contains(got, "DATABASE_URL") {
		t.Fatalf("a key the local .env already carries must not be reported:\n%s", got)
	}
	// the security contract, asserted: no value, from either file, anywhere
	if strings.Contains(got, theSecret) {
		t.Fatalf("SECRET VALUE LEAKED into the report:\n%s", got)
	}
}

func TestMissingLocalEnvSaysSoAndLeaksNothing(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".env.sample", "# a comment\n\nAPI_TOKEN="+theSecret+"\nnot a key line\n")
	if _, code := runEnv(t, root, true); code != 0 {
		t.Fatal("sync failed")
	}

	got, code := runEnv(t, root, false)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, got)
	}
	mustContain(t, got, "the local .env does not exist")
	mustContain(t, got, "new env var(s) declared: API_TOKEN")
	if strings.Contains(got, "a comment") || strings.Contains(got, theSecret) {
		t.Fatalf("comment text or a value reached the report:\n%s", got)
	}
}

func TestNoBaselineSaysSoThenSyncMakesASecondRunClean(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.sum", "example.com/x v1.0.0 h1:abc=\n")
	write(t, root, "db/migrate/001.sql", "a")

	got, code := runEnv(t, root, false)
	if code != 0 {
		t.Fatalf("no-baseline must exit 0, got %d:\n%s", code, got)
	}
	mustContain(t, got, "no sync baseline recorded — run `procoder env --sync` once your setup is done")
	mustContain(t, got, "go.sum")
	mustContain(t, got, "db/migrate/")

	if _, code := runEnv(t, root, true); code != 0 {
		t.Fatal("sync failed")
	}
	got, code = runEnv(t, root, false)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, got)
	}
	mustContain(t, got, "no changes since your last sync")
}

func TestCorruptStateExitsOneNamingTheFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package-lock.json", "x")
	write(t, root, StateFile, "{not json at all")

	got, code := runEnv(t, root, false)
	if code != 1 {
		t.Fatalf("corrupt state must exit 1, got %d:\n%s", code, got)
	}
	mustContain(t, got, StateFile)
	mustContain(t, got, "corrupt")
	mustContain(t, got, "--sync")
}

func TestUnknownStateVersionIsRefusedNotTreatedAsEmpty(t *testing.T) {
	root := t.TempDir()
	write(t, root, StateFile, `{"version":99,"synced_at":"2026-01-01T00:00:00Z"}`)

	got, code := runEnv(t, root, false)
	if code != 1 {
		t.Fatalf("unknown version must exit 1, got %d:\n%s", code, got)
	}
	mustContain(t, got, "version 99")
}

func TestSyncWritesExactlyOneFileAndNoValue(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package-lock.json", "x")
	write(t, root, ".env.example", "SECRET_KEY="+theSecret+"\n")
	write(t, root, ".env", "SECRET_KEY="+theSecret+"\n")
	before := treeFiles(t, root)

	got, code := runEnv(t, root, true)
	if code != 0 {
		t.Fatalf("--sync exit %d:\n%s", code, got)
	}
	added := addedPaths(before, treeFiles(t, root))
	if len(added) != 1 || added[0] != StateFile {
		t.Fatalf("--sync must write exactly one file (%s), added: %v", StateFile, added)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(StateFile)))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, theSecret) {
		t.Fatalf("SECRET VALUE LEAKED into the state file:\n%s", body)
	}
	mustContain(t, body, "SECRET_KEY") // key names are the point
	mustContain(t, body, `"version": 1`)
	if strings.Contains(got, theSecret) {
		t.Fatalf("SECRET VALUE LEAKED into the --sync report:\n%s", got)
	}
}

func TestUnreadableLockfileIsNotCheckedWhileOthersStillReport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 does not deny reads on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads everything; this test needs an ordinary user")
	}
	root := t.TempDir()
	write(t, root, "package-lock.json", "one")
	write(t, root, "go.sum", "one")
	if _, code := runEnv(t, root, true); code != 0 {
		t.Fatal("sync failed")
	}
	write(t, root, "go.sum", "two")
	if err := os.Chmod(filepath.Join(root, "package-lock.json"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "package-lock.json"), 0o644) })

	got, code := runEnv(t, root, false)
	if code != 1 {
		t.Fatalf("an uncheckable file must exit 1, got %d:\n%s", code, got)
	}
	mustContain(t, got, "package-lock.json: NOT checked")
	mustContain(t, got, "go.sum: dependencies changed since your last sync — run go mod download")
}

func TestEveryPrintedPathUsesForwardSlashes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/api/package-lock.json", "one")
	write(t, root, "services/api/db/migrate/001.sql", "a")
	write(t, root, "services/api/.env.example", "TOKEN="+theSecret+"\n")
	if _, code := runEnv(t, root, true); code != 0 {
		t.Fatal("sync failed")
	}
	write(t, root, "services/api/package-lock.json", "two")
	write(t, root, "services/api/db/migrate/002.sql", "b")

	got, code := runEnv(t, root, false)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, got)
	}
	if strings.Contains(got, `\`) {
		t.Fatalf("a backslash reached the output:\n%s", got)
	}
	mustContain(t, got, "services/api/package-lock.json")
	mustContain(t, got, "services/api/db/migrate/")
	mustContain(t, got, "services/api/.env.example")
	mustContain(t, got, "the local services/api/.env does not exist")
}

func TestEnvKeyParsingKeepsNamesAndDropsEverythingElse(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".env.template", strings.Join([]string{
		"# FOO=" + theSecret,
		"",
		"export BAR=" + theSecret,
		"BAZ=",
		"1BAD=" + theSecret,
		"NOEQUALS",
	}, "\n"))
	keys, err := envKeysOf(filepath.Join(root, ".env.template"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"BAR", "BAZ"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
}

// --- fixture helpers --------------------------------------------------------

// treeFiles is every file in the tree as a repo-relative slash path.
func treeFiles(t *testing.T, root string) map[string]bool {
	t.Helper()
	files := map[string]bool{}
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		files[filepath.ToSlash(rel)] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func addedPaths(before, after map[string]bool) []string {
	var added []string
	for p := range after {
		if !before[p] {
			added = append(added, p)
		}
	}
	return added
}

// A gitignored tree is not this project's environment: agent worktrees and
// vendored copies carry their own lockfiles, and describing them as drift
// buries the real answer. git owns that question, so the survey asks it.
func TestGitIgnoredTreesAreNotSurveyed(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	write(t, root, ".gitignore", "copies/\n")
	write(t, root, "package-lock.json", `{"lockfileVersion":3}`)
	write(t, root, "copies/nested/package-lock.json", `{"lockfileVersion":3}`)

	var lines []string
	if code := Run(root, false, func(s string) { lines = append(lines, s) }); code != 0 {
		t.Fatalf("exit %d: %v", code, lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "package-lock.json") {
		t.Fatalf("the tracked lockfile must be surveyed: %v", lines)
	}
	if strings.Contains(joined, "copies/") {
		t.Fatalf("a gitignored tree must not be surveyed: %v", lines)
	}
}
