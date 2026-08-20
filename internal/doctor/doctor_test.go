package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// write creates rel under root, making the intermediate directories. The
// content is irrelevant to doctor: only the path's extension is surveyed.
func write(t *testing.T, root, rel string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// script writes an executable shell script at path. Used to stand in for a
// formatter binary so the "installed" leg does not depend on which real
// formatters happen to exist on the machine running the suite.
func script(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// register puts a tool in the shared extension table for the duration of one
// test. Real formatters make poor fixtures — whether gofmt or prettier is on
// this machine is exactly the thing under test — so doctor is fed invented
// extensions whose presence or absence the test controls completely.
func register(t *testing.T, ext string, tool *tools.Tool) {
	t.Helper()
	tools.ByExtension[ext] = tool
	t.Cleanup(func() { delete(tools.ByExtension, ext) })
}

// withExtras installs an ExtraTools hook and restores whatever was there.
func withExtras(t *testing.T, fn func(root string) []*tools.Tool) {
	t.Helper()
	prev := ExtraTools
	ExtraTools = fn
	t.Cleanup(func() { ExtraTools = prev })
}

func names(ts []*tools.Tool) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Name)
	}
	return out
}

// The survey decides which formatters the whole domain asks for, so both
// halves matter: an extension that is missed leaves a language unchecked, and
// a directory that is walked when it should not be (node_modules alone can
// hold more files than the repository) makes every doctor run crawl and
// invents requirements from vendored code.
//
// proved by: dropping strings.ToLower from ExtensionsIn — App.PY then lands
// as ".PY" and the expected set no longer matches; and separately by removing
// the skipDirs/dot-prefix branch, which drags .js, .rs, .css, .rb and .kt in
// from the vendored trees.
func TestExtensionsInSurveysSourceAndSkipsVendoredAndHiddenTrees(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"main.go",
		"pkg/App.PY", // casing is normalised: this is a .py file
		"README.md",
		"Makefile", // no extension at all — contributes nothing
		"node_modules/left-pad/index.js",
		".git/config.json",
		".hidden/secret.rs",
		"dist/bundle.css",
		"vendor/lib.rb",
		"target/out.kt",
		"__pycache__/mod.pyc",
	} {
		write(t, root, rel)
	}

	got := ExtensionsIn(root)
	want := map[string]bool{".go": true, ".py": true, ".md": true}
	if len(got) != len(want) {
		t.Fatalf("survey found %v, want exactly %v", got, want)
	}
	for ext := range want {
		if !got[ext] {
			t.Errorf("survey missed %s: %v", ext, got)
		}
	}
}

// An empty tree is not an error and not a guess: it simply has no extensions.
//
// proved by: seeding found with a default entry (e.g. found[".go"] = true) —
// the length check below catches the invented requirement.
func TestExtensionsInEmptyTreeFindsNothing(t *testing.T) {
	if got := ExtensionsIn(t.TempDir()); len(got) != 0 {
		t.Errorf("an empty tree needs no formatters, got %v", got)
	}
}

// RequiredTools is the shared survey doctor and init both start from. Two
// contracts ride on it: one entry per TOOL (a repo with .md and .json needs
// prettier once, not twice) and a stable sorted order, because init prints
// and installs from this list and a shuffling list makes its output
// unreviewable.
//
// proved by: keying the dedupe map by extension instead of by tool name —
// prettier appears twice for .md and .json and the expected slice no longer
// matches.
func TestRequiredToolsDedupesByToolAndSortsByName(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"a.go", "b.go", "x.py", "doc.md", "pkg.json"} {
		write(t, root, rel)
	}
	if got := names(RequiredTools(root)); !equal(got, []string{"gofmt", "prettier", "ruff"}) {
		t.Errorf("RequiredTools = %v, want [gofmt prettier ruff]", got)
	}
}

// Extra tools are the non-extension-driven ones (actionlint for workflows, gh
// for a GitHub remote). They must join the same list, in the same sorted
// order, and must not duplicate a tool the extensions already asked for.
//
// proved by: appending extras to the output slice after the sort instead of
// merging them into the map — "aaa-extra" lands last and gofmt appears twice.
func TestRequiredToolsMergesExtraToolsWithoutDuplicating(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.go")
	gofmt := tools.ByExtension[".go"]
	if gofmt == nil {
		t.Fatal("the tool table lost its .go entry")
	}
	withExtras(t, func(string) []*tools.Tool {
		return []*tools.Tool{{Name: "zz-extra"}, {Name: "aaa-extra"}, gofmt}
	})
	if got := names(RequiredTools(root)); !equal(got, []string{"aaa-extra", "gofmt", "zz-extra"}) {
		t.Errorf("RequiredTools = %v, want [aaa-extra gofmt zz-extra]", got)
	}
}

// A formatter this repository needs but does not have is the whole reason
// doctor exists: it must be named as a GAP, carry its manual install line,
// and — because a gate that cannot look is a gate with a hole in it — make
// the command exit non-zero.
//
// proved by: returning 0 instead of 1 at the end of Run — the exit-code
// assertion fails while every printed line still matches.
func TestRunReportsMissingFormatterAsGapAndExitsNonZero(t *testing.T) {
	root := t.TempDir()
	register(t, ".fakemissing", &tools.Tool{
		Name:    "procoder-absent-formatter",
		Install: "brew install nothing-real",
	})
	write(t, root, "src/thing.fakemissing")

	var buf bytes.Buffer
	code := Run(root, &buf)
	out := buf.String()

	if code != 1 {
		t.Errorf("a missing formatter must exit 1, got %d\n%s", code, out)
	}
	for _, want := range []string{
		"GAP", "procoder-absent-formatter", ".fakemissing",
		"missing", "procoder init", "brew install nothing-real",
		"1 formatter(s) missing",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("gap report is missing %q:\n%s", want, out)
		}
	}
}

// The installed case is the other half: the ok line, the tool's own version
// (first line only — several formatters print a banner), and exit 0.
//
// proved by: dropping the SplitN in version() so the whole of stdout is
// reported — "BANNER-LINE-TWO" then appears in the report and the assertion
// below fails.
func TestRunReportsInstalledFormatterAsOkWithItsVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake binary is a /bin/sh script")
	}
	root := t.TempDir()
	register(t, ".fakepresent", &tools.Tool{
		Name:        "procoder-present-formatter",
		VersionArgs: []string{"--version"},
		Install:     "never needed",
	})
	// tools.Resolve prefers the repo's own node_modules/.bin, which makes a
	// fixture binary possible without touching PATH.
	script(t, filepath.Join(root, "node_modules", ".bin", "procoder-present-formatter"),
		"echo 'fakefmt 9.9.9'; echo 'BANNER-LINE-TWO'")
	write(t, root, "src/thing.fakepresent")

	var buf bytes.Buffer
	code := Run(root, &buf)
	out := buf.String()

	if code != 0 {
		t.Errorf("an installed formatter must exit 0, got %d\n%s", code, out)
	}
	if strings.Contains(out, "GAP") {
		t.Errorf("an installed formatter must not be reported as a gap:\n%s", out)
	}
	for _, want := range []string{"ok", "procoder-present-formatter", ".fakepresent", "fakefmt 9.9.9",
		"every formatter this repository needs is installed"} {
		if !strings.Contains(out, want) {
			t.Errorf("ok report is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "BANNER-LINE-TWO") {
		t.Errorf("only the first line of --version belongs in the report:\n%s", out)
	}
}

// A tree with nothing formatter-covered is a clean answer, not a gap: doctor
// says so plainly and exits 0, so `procoder doctor` in a docs-only or empty
// repository never blocks anything.
//
// proved by: deleting the len(byTool) == 0 branch — Run then falls through to
// the "every formatter is installed" line, which is a different claim.
func TestRunOnTreeWithNoCoveredTypesSaysSoAndExitsZero(t *testing.T) {
	root := t.TempDir()
	write(t, root, "notes.unknownext")
	withExtras(t, nil)

	var buf bytes.Buffer
	if code := Run(root, &buf); code != 0 {
		t.Errorf("nothing to check exits 0, got %d", code)
	}
	if out := buf.String(); !strings.Contains(out, "no files in this tree have a formatter-covered type") {
		t.Errorf("doctor must say the tree has nothing to check, got:\n%s", out)
	}
}

// version() feeds a fixed-width column, so a tool with a chatty --version
// must not be able to wreck the report's shape; and a tool that has no
// version flag, or whose binary fails, still counts as present.
//
// proved by: raising the truncation limit from 60 to 80 — the truncation case
// then returns 80 characters and the literal expectation fails.
func TestVersionTruncatesAndDegradesGracefully(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake binaries are /bin/sh scripts")
	}
	dir := t.TempDir()

	chatty := filepath.Join(dir, "chatty")
	script(t, chatty, "printf '%s\\n' "+strings.Repeat("x", 100))
	broken := filepath.Join(dir, "broken")
	script(t, broken, "exit 3")

	// 100 x's in, 60 x's out — written literally rather than computed from the
	// constant the function uses.
	sixty := "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	if len(sixty) != 60 {
		t.Fatalf("the fixture itself is wrong: %d characters", len(sixty))
	}

	cases := []struct {
		name string
		bin  string
		tool *tools.Tool
		want string
	}{
		{
			name: "no version flag means presence is the answer",
			bin:  chatty,
			tool: &tools.Tool{Name: "x"},
			want: "present",
		},
		{
			name: "an over-long version line is cut to 60 characters",
			bin:  chatty,
			tool: &tools.Tool{Name: "x", VersionArgs: []string{"--version"}},
			want: sixty,
		},
		{
			name: "a binary that fails is still installed",
			bin:  broken,
			tool: &tools.Tool{Name: "x", VersionArgs: []string{"--version"}},
			want: "present (version unknown)",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := version(c.bin, c.tool); got != c.want {
				t.Errorf("version = %q, want %q", got, c.want)
			}
		})
	}
}

// Root falls back to the working directory's repository root; inside this
// source tree that is the procoder checkout, recognisable by its go.mod.
//
// proved by: returning "" instead of "." / the repo root from Root — the
// go.mod check below fails.
func TestRootFindsTheRepositoryCheckout(t *testing.T) {
	root := Root()
	if root == "" {
		t.Fatal("Root must always answer a usable directory")
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Errorf("Root() = %q, which holds no go.mod: %v", root, err)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// a is expected to be sorted already; comparing element-wise is the point.
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return sort.StringsAreSorted(b)
}
