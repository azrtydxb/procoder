package docs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func write(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBrokenRelativeReferenceIsBlockingAndNamed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/setup.md", "exists\n")
	md := write(t, root, "README.md",
		"# X\n\nsee [setup](docs/setup.md) and [gone](docs/nope.md)\nand ![img](assets/missing.png)\n")

	got := RelativeRefs(root, md)
	if len(got) != 2 {
		t.Fatalf("want 2 findings, got %d: %+v", len(got), got)
	}
	for _, f := range got {
		if !f.Blocking {
			t.Fatalf("broken reference must be blocking: %+v", f)
		}
	}
	if got[0].Line != 3 || got[1].Line != 4 {
		t.Fatalf("wrong lines: %+v", got)
	}
}

func TestExternalAndAnchorLinksAreNotRelativeFindings(t *testing.T) {
	root := t.TempDir()
	md := write(t, root, "README.md",
		"[a](https://example.com/x) [b](#section) [c](mailto:x@y.z)\n")
	if got := RelativeRefs(root, md); len(got) != 0 {
		t.Fatalf("external/anchor links are not relative refs: %+v", got)
	}
}

func TestLinksInsideCodeFencesAreIgnored(t *testing.T) {
	root := t.TempDir()
	md := write(t, root, "README.md", "```\n[x](not/a/real/link.md)\n```\n")
	if got := RelativeRefs(root, md); len(got) != 0 {
		t.Fatalf("fenced example links must not be findings: %+v", got)
	}
}

func TestAnchorAndQueryAreStrippedFromFileTargets(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/a.md", "x\n")
	md := write(t, root, "README.md", "[a](docs/a.md#part)\n")
	if got := RelativeRefs(root, md); len(got) != 0 {
		t.Fatalf("anchored link to existing file is fine: %+v", got)
	}
}

// A file with diagrams and no compiler must read as NOT checked, never clean.
func TestMermaidWithoutCompilerIsBlockingNotSilent(t *testing.T) {
	root := t.TempDir()
	md := write(t, root, "README.md", "```mermaid\ngraph TD; A-->B\n```\n")
	t.Setenv("PATH", root) // nothing on PATH
	got := MermaidBlocks(root, md)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("want one blocking NOT-checked finding, got %+v", got)
	}
}

func TestMermaidCompileFailureIsBlockingWithTheToolsReason(t *testing.T) {
	skipOnWindows(t)
	root := t.TempDir()
	md := write(t, root, "README.md", "```mermaid\ngraph TD; A-->B\n```\n")
	bin := filepath.Join(root, "bin")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "mmdc"),
		[]byte("#!/bin/sh\necho 'Parse error on line 1' >&2\nexit 1\n"), 0o755)
	t.Setenv("PATH", bin)
	got := MermaidBlocks(root, md)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "Parse error") {
		t.Fatalf("want compile failure finding carrying the reason, got %+v", got)
	}
}

func TestMermaidCleanCompileIsSilent(t *testing.T) {
	skipOnWindows(t)
	root := t.TempDir()
	md := write(t, root, "README.md", "```mermaid\ngraph TD; A-->B\n```\n")
	bin := filepath.Join(root, "bin")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "mmdc"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", bin)
	if got := MermaidBlocks(root, md); len(got) != 0 {
		t.Fatalf("clean diagram must be silent: %+v", got)
	}
}

func TestDriftNamesBothSides(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/usage.md", "run internal/gate/gate.go for the gate\n")
	changed := []string{write(t, root, "internal/gate/gate.go", "package gate\n")}
	got := Drift(root, changed)
	if len(got) != 1 {
		t.Fatalf("want 1 drift finding, got %+v", got)
	}
	if got[0].Blocking {
		t.Fatal("drift is a judgment call — report, never block")
	}
	if !strings.Contains(got[0].Message, "internal/gate/gate.go") ||
		!strings.Contains(got[0].File, "usage.md") {
		t.Fatalf("drift must name doc and changed file: %+v", got[0])
	}
}

func TestRulesFileWinsOverDefaults(t *testing.T) {
	root := t.TempDir()
	write(t, root, RulesPath, `# rules

## Required docs

- README.md
- SECURITY.md

## Required badges

- coverage
`)
	r := LoadRules(root)
	if len(r.RequiredDocs) != 2 || r.RequiredDocs[1] != "SECURITY.md" {
		t.Fatalf("repo's required docs must win: %+v", r.RequiredDocs)
	}
	if len(r.RequiredBadges) != 1 || r.RequiredBadges[0] != "coverage" {
		t.Fatalf("repo's badges must win: %+v", r.RequiredBadges)
	}
	// absent section keeps the default
	if len(r.ReadmeSections) != 3 {
		t.Fatalf("absent section keeps defaults: %+v", r.ReadmeSections)
	}
}

func TestRequiredDocsAndBadgesAndStructureReport(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "# proj\n\n## deep dive\nwords\n")
	r := defaultRules()

	if got := RequiredDocs(root, r); len(got) != 2 { // CHANGELOG.md + rules file
		t.Fatalf("want CHANGELOG+rules missing, got %+v", got)
	}
	if got := Badges(root, r); len(got) != 2 { // ci + license
		t.Fatalf("want 2 badge findings, got %+v", got)
	}
	got := ReadmeStructure(root, r)
	if len(got) != 3 { // usp, badges, quick start
		t.Fatalf("want 3 structure findings, got %+v", got)
	}
	for _, f := range got {
		if f.Blocking {
			t.Fatalf("presentation findings report, never block: %+v", f)
		}
	}
}

func TestSellingReadmePassesTheStructureCheck(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", `# procoder

The harness that makes AI coders work like senior developers.

![CI](https://img.shields.io/badge/ci-passing-green) ![License](https://img.shields.io/badge/license-Apache--2.0-blue)

## Quick start

    procoder init
`)
	r := defaultRules()
	if got := ReadmeStructure(root, r); len(got) != 0 {
		t.Fatalf("this README sells; no findings expected: %+v", got)
	}
	if got := Badges(root, r); len(got) != 0 {
		t.Fatalf("badges present: %+v", got)
	}
}

func TestGoExportedSymbolWithoutDocIsReported(t *testing.T) {
	root := t.TempDir()
	f := write(t, root, "x.go", `package x

// Documented is fine.
func Documented() {}

func Naked() {}

type Bare struct{}
`)
	got := MissingAPIDocs([]string{f})
	if len(got) != 2 {
		t.Fatalf("want Naked+Bare reported, got %+v", got)
	}
	for _, g := range got {
		if g.Blocking {
			t.Fatal("API doc findings report, never block")
		}
	}
}

func TestPythonAndTSPublicSymbolsWithoutDocsAreReported(t *testing.T) {
	root := t.TempDir()
	py := write(t, root, "m.py", `def documented():
    """has one"""
    pass

def naked():
    pass

def _private():
    pass
`)
	ts := write(t, root, "m.ts", `/** documented */
export function ok() {}

export class Naked {}
`)
	if got := MissingAPIDocs([]string{py}); len(got) != 1 || !strings.Contains(got[0].Message, "naked") {
		t.Fatalf("python: want only naked, got %+v", got)
	}
	if got := MissingAPIDocs([]string{ts}); len(got) != 1 || !strings.Contains(got[0].Message, "Naked") {
		t.Fatalf("ts: want only Naked, got %+v", got)
	}
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("sh-script tool stubs cannot run on Windows; POSIX legs carry this coverage")
	}
}
