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
	write(t, root, "docs/a.md", "## Part\n\nx\n")
	md := write(t, root, "README.md", "[a](docs/a.md#part)\n[b](docs/a.md?v=2)\n")
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

	// whole-word matching: a family inside another word is not a mention —
	// "specific" must not satisfy "spec", "eslint" not "lint" — or terse
	// family names become vacuous
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

	// only narrative counts: a ci.yml badge URL or a link target is not
	// the README telling the reader about CI
	r3 := Rules{ReadmeMentions: []string{"ci"}}
	write(t, root, "README.md", "# x\n\n![CI](https://github.com/o/r/actions/workflows/ci.yml/badge.svg)\n[docs](https://example.com/ci/page)\n")
	if got = ReadmeMentions(root, r3); len(got) != 1 {
		t.Fatalf("badge URLs and link targets must not satisfy a family: %+v", got)
	}
	write(t, root, "README.md", "# x\n\n![CI](https://x/ci.yml/badge.svg)\n\nOur ci runs the same gate.\n")
	if got = ReadmeMentions(root, r3); len(got) != 0 {
		t.Fatalf("prose next to a badge must still count: %+v", got)
	}
}

// The failure this pins: a PR adding a docs page and linking its future
// site URL could never pass CI — the page 404s until the very deploy the
// PR triggers. Own-site links with a local source page are pending, not
// dead; genuinely missing pages still block.
func TestOwnSiteLinksPendingDeployAreNotDead(t *testing.T) {
	root := t.TempDir()
	write(t, root, "mkdocs.yml", "site_name: x\nsite_url: https://example.github.io/x/\n")
	write(t, root, "docs/getting-started.md", "# gs\n")

	if !localPageExists(root, "getting-started/") {
		t.Error("existing page must be recognised as pending, not dead")
	}
	if !localPageExists(root, "getting-started/#anchor") {
		t.Error("anchors strip before the lookup")
	}
	if localPageExists(root, "") {
		t.Error("no docs/index.md here — the site root is genuinely missing")
	}
	if localPageExists(root, "nope/") {
		t.Error("a missing page is dead, not pending")
	}
	if localPageExists(root, "../secret") {
		t.Error("traversal never resolves")
	}
	if got := siteURL(root); got != "https://example.github.io/x/" {
		t.Errorf("siteURL = %q", got)
	}
	if got := extractURL("* [404] <https://example.github.io/x/getting-started/> (at 12:1)"); got != "https://example.github.io/x/getting-started/" {
		t.Errorf("extractURL = %q", got)
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

// A link into another page can name a heading, and a heading that no
// longer exists leaves the reader at the top of the page wondering.
// mkdocs reports this at INFO, so --strict stays green and it ships —
// which is exactly how one shipped from this repository.
// proved by: kept the old behaviour of discarding the anchor — the dead
// one is then reported as fine.
func TestAnAnchorThatNoHeadingGeneratesIsBroken(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "target.md"), "# Page\n\n## Contract 1 — P-CONTROL: the agent stays in control\n\nbody\n")
	src := filepath.Join(dir, "src.md")
	mustWrite(t, src, "See [it](target.md#contract-2-the-agent-stays-in-control).\n")

	got := RelativeRefs(dir, src)
	if len(got) != 1 {
		t.Fatalf("want 1 finding for the dead anchor, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "anchor") || !got[0].Blocking {
		t.Errorf("finding must name the anchor and block: %+v", got[0])
	}
}

// The same link with the slug the heading actually generates is fine —
// em dash dropped, colon dropped — in either dialect: mkdocs collapses the
// run of separators the dropped characters leave, github.com keeps it, and
// the same page is read through both.
// proved by: made the slug keep punctuation — the correct link is then
// reported as broken, which is worse than not checking at all; and dropped
// the github spelling — every heading with an `&`, an em dash, or a colon
// then reads as a broken reference on the renderer most repositories use.
func TestTheRealSlugResolves(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "target.md"), "# Page\n\n## Contract 1 — P-CONTROL: the agent stays in control\n\n## Files & skills\n\n### DeepSeek: `reasoning_content`\n\n## Hello _world_\n")
	for _, link := range []string{
		"target.md#contract-1-p-control-the-agent-stays-in-control",
		"target.md#contract-1--p-control-the-agent-stays-in-control",
		"target.md#files-skills",
		"target.md#files--skills",
		"target.md#deepseek-reasoning_content",
		"target.md#hello-world",
		"target.md#page",
	} {
		src := filepath.Join(dir, "src.md")
		mustWrite(t, src, "See [it]("+link+").\n")
		if got := RelativeRefs(dir, src); len(got) != 0 {
			t.Errorf("%s must resolve, got %+v", link, got)
		}
	}
}

// An explicit id — attr_list `{#custom}` or a raw HTML anchor — counts.
// proved by: dropped explicit-id collection; a deliberate anchor is then
// called broken and the writer is told to fix something that works.
func TestExplicitIdsCount(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "target.md"),
		"# Page\n\n## Something {#custom-id}\n\n<a id=\"raw-html-anchor\"></a>\n")
	for _, link := range []string{"target.md#custom-id", "target.md#raw-html-anchor"} {
		src := filepath.Join(dir, "src.md")
		mustWrite(t, src, "See [it]("+link+").\n")
		if got := RelativeRefs(dir, src); len(got) != 0 {
			t.Errorf("%s must resolve, got %+v", link, got)
		}
	}
}

// A target that is not Markdown has no headings to check, and an anchor
// into it is not something this can judge — say nothing rather than
// something wrong.
// proved by: checked anchors on every target type; a link into an image
// or a source file is then reported broken.
func TestAnchorsIntoNonMarkdownAreNotJudged(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "diagram.svg"), "<svg></svg>\n")
	src := filepath.Join(dir, "src.md")
	mustWrite(t, src, "See [it](diagram.svg#layer1).\n")
	if got := RelativeRefs(dir, src); len(got) != 0 {
		t.Errorf("an anchor into a non-Markdown target is not judged, got %+v", got)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A target that exists but cannot be read yields no verdict. Answering
// "it has no anchors" would report every link into it as broken, which
// is a false positive that blocks the gate — the honesty rule pointed
// the other way round.
// proved by: made anchorIDs answer (empty, true) on a read error — the
// link into the unreadable page is then called broken.
func TestAnUnreadableTargetYieldsNoAnchorVerdict(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod does not deny the read here")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.md")
	mustWrite(t, target, "## Something\n")
	if err := os.Chmod(target, 0o000); err != nil {
		t.Skip("cannot make the file unreadable")
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o644) })
	if data, err := os.ReadFile(target); err == nil {
		t.Skipf("file is still readable (%d bytes) — the guard cannot be exercised", len(data))
	}

	src := filepath.Join(dir, "src.md")
	mustWrite(t, src, "See [it](target.md#anything-at-all).\n")
	if got := RelativeRefs(dir, src); len(got) != 0 {
		t.Errorf("an unreadable target must produce no anchor verdict, got %+v", got)
	}
}
