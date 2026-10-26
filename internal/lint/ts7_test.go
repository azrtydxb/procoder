package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// isolateCache points the user cache at a temp dir, so procoder's own
// typescript-eslint copy is whatever the test puts there — never the copy
// on the machine running the suite.
func isolateCache(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("LocalAppData", filepath.Join(home, "AppData", "Local"))
	return home
}

func installTypescript(t *testing.T, root, version string) {
	t.Helper()
	dir := filepath.Join(root, "node_modules", "typescript")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"typescript","version":"` + version + `"}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The project's TypeScript decides which typescript-eslint init installs:
// up to 6 the project's own, past it procoder's copy, because npm refuses
// typescript-eslint into a TypeScript 7 project (#324).
// proved by: TypescriptEslintFor returned TypescriptEslint unconditionally —
// a TS 7 project is told to run an install npm rejects with ERESOLVE.
func TestTypescriptPastThePeerRangeInstallsProcodersOwnParser(t *testing.T) {
	isolateCache(t)
	for _, tc := range []struct {
		version string
		want    *tools.Tool
	}{
		{"5.9.3", TypescriptEslint},
		{"6.0.3", TypescriptEslint},
		{"7.0.2", TypescriptEslintTS7},
		{"not-a-version", TypescriptEslint},
	} {
		root := t.TempDir()
		installTypescript(t, root, tc.version)
		if got := TypescriptEslintFor(root); got != tc.want {
			t.Errorf("TypeScript %s: init would run %v, want %v", tc.version, got.InstallVia, tc.want.InstallVia)
		}
	}
	// no TypeScript installed is "not known to be past the range"
	if got := TypescriptEslintFor(t.TempDir()); got != TypescriptEslint {
		t.Errorf("with no TypeScript installed the project-local install is the one to try, got %v", got.InstallVia)
	}
	args := strings.Join(TypescriptEslintTS7.InstallVia[0].Args, " ")
	// the args are built once at package load, so they carry the machine's
	// cache dir, not the one isolateCache points at
	owned := filepath.Join("procoder", "typescript-eslint-ts6")
	if !strings.Contains(args, "--prefix ") || !strings.Contains(args, owned) || !strings.Contains(args, "typescript@~6.0") {
		t.Errorf("the TS 7 install must land in procoder's own directory beside a TypeScript 6: %s", args)
	}
}

// A TS 7 project resolves procoder's copy ahead of one it forced in with
// --legacy-peer-deps, which would load TypeScript 7 into a parser that
// does not support it; and a TS 6 project never sees procoder's copy.
// proved by: dropped the owned-dir lookup from tsESLintEntry — the TS 7
// project resolves its own unsupported copy, or nothing.
func TestTheOwnedParserServesOnlyProjectsPastTheRange(t *testing.T) {
	isolateCache(t)
	owned := filepath.Join(ownedTSESLintDir(), "node_modules", "typescript-eslint", "dist", "index.js")
	if err := os.MkdirAll(filepath.Dir(owned), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(owned, []byte("export default {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ts7 := t.TempDir()
	installTypescript(t, ts7, "7.0.2")
	stubTypescriptEslint(t, ts7) // the --legacy-peer-deps copy
	if got := tsESLintEntry(ts7, filepath.Join(ts7, "src", "a.ts")); got != owned {
		t.Errorf("TS 7 project resolved %q, want procoder's copy %q", got, owned)
	}
	if got := tools.Resolve(TypescriptEslintTS7, ts7); got != owned {
		t.Errorf("doctor would resolve %q for the TS 7 project, want %q", got, owned)
	}

	ts6 := t.TempDir()
	installTypescript(t, ts6, "6.0.3")
	if got := tsESLintEntry(ts6, filepath.Join(ts6, "a.ts")); got != "" {
		t.Errorf("a TS 6 project without typescript-eslint must install its own, not borrow procoder's: %q", got)
	}
	stubTypescriptEslint(t, ts6)
	if got := tsESLintEntry(ts6, filepath.Join(ts6, "a.ts")); got == owned || got == "" {
		t.Errorf("a TS 6 project's own typescript-eslint must win: %q", got)
	}
}

// Without procoder's copy installed, a TS 7 project is NOT checked like any
// missing tool, and the remedy is init — which now has an install to run.
// proved by: returned nil when the owned copy is absent — TS 7 passes the
// gate unlinted.
func TestTS7WithoutTheOwnedParserIsNotChecked(t *testing.T) {
	if tools.Resolve(Eslint, "") == "" {
		t.Skip("eslint not installed")
	}
	isolateCache(t)
	root := t.TempDir()
	installTypescript(t, root, "7.0.2")
	src := filepath.Join(root, "a.ts")
	if err := os.WriteFile(src, []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Files(root, []string{src}, false)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, TypescriptEslint.Name) {
		t.Fatalf("want one blocking refusal naming typescript-eslint, got %+v", got)
	}
}

// A repository whose TypeScript lives in a subdirectory (an app under web/,
// nothing at the root) is past the range exactly like one with it at the
// root: doctor must check, and init install, procoder's own copy, and the
// copy must resolve for the repo and for a file under that subdirectory.
// proved by: TypescriptEslintFor looked only at root/node_modules — doctor
// reported typescript-eslint missing with the project-local npm install that
// npm refuses, and the owned copy was never resolved for the repo.
func TestTypescriptInASubdirectoryDecidesTheParser(t *testing.T) {
	isolateCache(t)
	owned := filepath.Join(ownedTSESLintDir(), "node_modules", "typescript-eslint", "dist", "index.js")
	if err := os.MkdirAll(filepath.Dir(owned), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(owned, []byte("export default {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	web := filepath.Join(root, "web")
	installTypescript(t, web, "7.0.2")
	if err := os.MkdirAll(filepath.Join(web, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := TypescriptEslintFor(root); got != TypescriptEslintTS7 {
		t.Errorf("TypeScript 7 under web/: init would run %v, want procoder's own copy", got.InstallVia)
	}
	if got := tools.Resolve(TypescriptEslintFor(root), root); got != owned {
		t.Errorf("doctor resolved %q for the repo, want %q", got, owned)
	}
	if got := tsESLintEntry(root, filepath.Join(web, "src", "a.ts")); got != owned {
		t.Errorf("a file under web/src resolved %q, want %q", got, owned)
	}

	// any location past the range decides, even beside one within it
	mixed := t.TempDir()
	installTypescript(t, filepath.Join(mixed, "legacy"), "5.9.3")
	installTypescript(t, filepath.Join(mixed, "apps", "web"), "7.0.2")
	if got := TypescriptEslintFor(mixed); got != TypescriptEslintTS7 {
		t.Errorf("one TypeScript 7 location among several: got %v", got.InstallVia)
	}

	// subdirectories within the range keep the project-local install
	within := t.TempDir()
	installTypescript(t, filepath.Join(within, "web"), "6.0.3")
	if got := TypescriptEslintFor(within); got != TypescriptEslint {
		t.Errorf("TypeScript 6 under web/ must install typescript-eslint itself, got %v", got.InstallVia)
	}
}
