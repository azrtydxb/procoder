package docs

import (
	"os"
	"os/exec"
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

// The keyword appearing in prose is not a badge: "our ci is great" plus an
// unrelated badge image must still count as a missing ci badge.
func TestBadgeKeywordInProseDoesNotCount(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", `# x

our ci is great and the license is Apache

![Coverage](https://img.shields.io/badge/coverage-97%25-green)
`)
	got := Badges(root, defaultRules())
	if len(got) != 2 {
		t.Fatalf("ci and license badges are both missing, got %+v", got)
	}
}

// The failure this pins: three releases shipped while the README's badge
// said 0.7.0 — prose claims aren't file paths, so drift never fired. The
// version tripwire makes a release without a reviewed README block.
func TestReadmeMustCarryTheCurrentVersion(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".claude-plugin/plugin.json", `{"name":"x","version":"0.8.2"}`)
	write(t, root, "README.md", "# x\n\n![Version](https://img.shields.io/badge/version-0.7.0-blue)\n")

	got := VersionSync(root)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "0.8.2") {
		t.Fatalf("stale version must block and name the current one: %+v", got)
	}

	write(t, root, "README.md", "# x\n\n![Version](https://img.shields.io/badge/version-0.8.2-blue)\n")
	if got = VersionSync(root); len(got) != 0 {
		t.Fatalf("matching version is silent: %+v", got)
	}

	// no declared version anywhere → nothing to hold the README to
	if got = VersionSync(t.TempDir()); len(got) != 0 {
		t.Fatalf("no version source must be silent: %+v", got)
	}

	// the tripwire covers every version-tracked doc, not just the README —
	// the Pages index shipped eight releases stale before this
	write(t, root, "docs/index.md", "# site\n\nCurrent version: 0.7.0\n")
	got = VersionSync(root)
	if len(got) != 1 || !strings.Contains(got[0].Message, "docs/index.md") {
		t.Fatalf("stale versioned doc must block by name: %+v", got)
	}
	write(t, root, "docs/index.md", "# site\n\nCurrent version: 0.8.2\n")
	if got = VersionSync(root); len(got) != 0 {
		t.Fatalf("current versioned doc is silent: %+v", got)
	}
}

// The failure this pins: eleven releases shipped against a README still
// describing release one — mention-in-corpus checks passed while the
// front page went stale. Declared families hold the NARRATIVE current.
func TestReadmeMustMentionDeclaredFamilies(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "# x\n\nWe have a commit gate and a code index.\n")

	r := Rules{ReadmeMentions: []string{"commit gate", "code index", "self-learning"}}
	got := ReadmeMentions(root, r)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "self-learning") {
		t.Fatalf("the one untold family must block by name: %+v", got)
	}

	// no declared families → the check is opt-in and silent
	if got := ReadmeMentions(root, Rules{}); len(got) != 0 {
		t.Fatalf("no declared families must be silent: %+v", got)
	}

	// case-insensitive: the story can capitalise
	write(t, root, "README.md", "# x\n\nThe Commit Gate, the Code Index, and the Self-Learning loop.\n")
	if got := ReadmeMentions(root, r); len(got) != 0 {
		t.Fatalf("capitalised mentions must count: %+v", got)
	}

	// word boundaries: a short family inside another word is not a mention —
	// "ci.yml" satisfies "ci" (dot is a boundary) but "specific" must not
	// satisfy "spec", or terse family names become vacuous
	r2 := Rules{ReadmeMentions: []string{"spec", "lint"}}
	write(t, root, "README.md", "# x\n\nA specific tool with eslint support.\n")
	got = ReadmeMentions(root, r2)
	if len(got) != 2 {
		t.Fatalf("substrings inside words must not count as mentions: %+v", got)
	}
	write(t, root, "README.md", "# x\n\nThe spec interview and the lint domain.\n")
	if got = ReadmeMentions(root, r2); len(got) != 0 {
		t.Fatalf("whole-word mentions must count: %+v", got)
	}
}

// The failure this pins: the changelog existed, so RequiredDocs was happy,
// but nothing forced an entry for the version being released — a bump
// without release notes shipped silently.
func TestChangelogMustCoverTheCurrentVersion(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".claude-plugin/plugin.json", `{"name":"x","version":"0.8.2"}`)
	write(t, root, "README.md", "# x\n\nversion-0.8.2\n")
	write(t, root, "CHANGELOG.md", "# Changelog\n\n## 0.8.1 — old news\n")

	got := VersionSync(root)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "## 0.8.2") {
		t.Fatalf("a version with no changelog entry must block: %+v", got)
	}

	write(t, root, "CHANGELOG.md", "# Changelog\n\n## 0.8.2 — the news\n\n## 0.8.1 — old news\n")
	if got = VersionSync(root); len(got) != 0 {
		t.Fatalf("covered version is silent: %+v", got)
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

// Gitignored scratch is not the repository's documentation: inside a git
// repo, only tracked and untracked-but-not-ignored Markdown is listed. The
// 65-files-in-a-10-doc-repo bug, pinned.
func TestGitignoredMarkdownIsNotScanned(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH; the walk fallback is covered by the other tests")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	write(t, root, "README.md", "# x\n")
	write(t, root, "scratch/notes.md", "private\n")
	write(t, root, ".gitignore", "scratch/\n")

	got := MarkdownFiles(root)
	var names []string
	for _, f := range got {
		names = append(names, filepath.Base(f))
	}
	if len(got) != 1 || names[0] != "README.md" {
		t.Fatalf("want exactly README.md (untracked counts, ignored does not), got %v", names)
	}
	for _, f := range got {
		if strings.Contains(f, "scratch") {
			t.Fatalf("gitignored markdown was scanned: %v", names)
		}
	}
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("sh-script tool stubs cannot run on Windows; POSIX legs carry this coverage")
	}
}
