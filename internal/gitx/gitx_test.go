package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func commit(t *testing.T, dir, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", msg}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestConflictMarkersAreFoundWithLineNumbers(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte("ok\n<<<<<<< HEAD\nmine\n=======\ntheirs\n>>>>>>> branch\n"), 0o644)
	got := ConflictMarkers([]string{p})
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2 (the two ends)", len(got))
	}
	if got[0].Line != 2 || got[1].Line != 6 {
		t.Fatalf("lines = %d,%d want 2,6", got[0].Line, got[1].Line)
	}
	if !got[0].Blocking {
		t.Fatal("conflict markers must block")
	}
}

// A Markdown setext underline is ======= at line start and must NOT be a
// conflict finding on its own.
func TestMarkdownUnderlineIsNotAConflict(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	os.WriteFile(p, []byte("Title\n=======\nbody\n"), 0o644)
	if got := ConflictMarkers([]string{p}); len(got) != 0 {
		t.Fatalf("flagged a Markdown underline as a conflict: %+v", got)
	}
}

func TestJunkAndOversized(t *testing.T) {
	dir := t.TempDir()
	junk := filepath.Join(dir, ".DS_Store")
	os.WriteFile(junk, []byte("x"), 0o644)
	big := filepath.Join(dir, "big.bin")
	os.WriteFile(big, make([]byte, 6<<20), 0o644)
	small := filepath.Join(dir, "ok.txt")
	os.WriteFile(small, []byte("x"), 0o644)

	if got := JunkFiles([]string{junk, small}); len(got) != 1 || !got[0].Blocking {
		t.Fatalf("junk: %+v", got)
	}
	if got := Oversized([]string{big, small}, 5); len(got) != 1 || !got[0].Blocking {
		t.Fatalf("oversized: %+v", got)
	}
	if got := Oversized([]string{big}, 10); len(got) != 0 {
		t.Fatalf("under the raised threshold should pass: %+v", got)
	}
}

// Caches and garbage are junk wherever they hide: by name, by extension, or
// by living inside a cache directory. Deliberately shipped artifacts (dist/)
// are not junk.
func TestCachesAndGarbageAreJunk(t *testing.T) {
	junk := []string{
		"a/.lycheecache", "b/debug.log", "c/x.pyc", "d/.file.swp",
		"src/__pycache__/m.cpython-312.pyc", "web/node_modules/x/i.js",
		".venv/bin/python", ".pytest_cache/v/cache",
	}
	for _, f := range junk {
		if got := JunkFiles([]string{f}); len(got) != 1 || !got[0].Blocking {
			t.Fatalf("%s must be blocking junk: %+v", f, got)
		}
	}
	fine := []string{"dist/linux-amd64/procoder", "internal/gitx/gitx.go", "CHANGELOG.md"}
	if got := JunkFiles(fine); len(got) != 0 {
		t.Fatalf("shipped artifacts and source are not junk: %+v", got)
	}
}

// A repo whose ecosystems generate garbage needs a .gitignore that covers it;
// the finding names the missing entry and the marker that demands it.
func TestIgnoreCoverageNamesTheGapAndTheReason(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(root, "mkdocs.yml"), []byte("site_name: x"), 0o644)

	got := IgnoreCoverage(root)
	if len(got) != 1 || !strings.Contains(got[0].Message, ".gitignore is missing") {
		t.Fatalf("missing .gitignore must be one finding: %+v", got)
	}

	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules\n"), 0o644)
	got = IgnoreCoverage(root)
	if len(got) != 1 || !strings.Contains(got[0].Message, `"site"`) ||
		!strings.Contains(got[0].Message, "mkdocs.yml") {
		t.Fatalf("want exactly the site/mkdocs gap named: %+v", got)
	}
	for _, f := range got {
		if f.Blocking {
			t.Fatal("ignore coverage reports; the one-line fix is the agent's call")
		}
	}

	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules\nsite\n"), 0o644)
	if got = IgnoreCoverage(root); len(got) != 0 {
		t.Fatalf("covered repo must be silent: %+v", got)
	}
}

func TestAttributionBlocksInAllItsCostumes(t *testing.T) {
	bad := []string{
		"fix parser\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>",
		"fix parser\n\nGenerated with Claude Code",
		"fix parser\n\n🤖 automated",
	}
	for _, m := range bad {
		got := Attribution([]string{m})
		if len(got) == 0 || !got[0].Blocking {
			t.Fatalf("not caught: %q -> %+v", m, got)
		}
	}
	clean := "fix parser\n\nHandles the empty-input case the old loop skipped."
	if got := Attribution([]string{clean}); len(got) != 0 {
		t.Fatalf("false positive on a clean message: %+v", got)
	}
}

func TestSubjectShapeReportsButNeverBlocks(t *testing.T) {
	msgs := []string{
		strings.Repeat("x", 80),
		"subject\nbody with no blank line",
	}
	got := SubjectShape(msgs)
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(got), got)
	}
	for _, f := range got {
		if f.Blocking {
			t.Fatalf("subject shape must not block: %+v", f)
		}
	}
}

func TestUnpushedMessagesFallsBackToLastCommit(t *testing.T) {
	dir := repo(t)
	commit(t, dir, "first change")
	msgs := UnpushedMessages(dir)
	if len(msgs) != 1 || !strings.Contains(msgs[0], "first change") {
		t.Fatalf("msgs = %q, want the last commit", msgs)
	}
}

func TestOnDefaultBranchReportsAndCanBlock(t *testing.T) {
	dir := repo(t)
	commit(t, dir, "seed")
	report := OnDefaultBranch(dir, false)
	if len(report) != 1 || report[0].Blocking {
		t.Fatalf("report mode: %+v", report)
	}
	block := OnDefaultBranch(dir, true)
	if len(block) != 1 || !block[0].Blocking {
		t.Fatalf("block mode: %+v", block)
	}
	// on a feature branch: silence
	exec.Command("git", "-C", dir, "checkout", "-q", "-b", "feature").Run()
	if got := OnDefaultBranch(dir, true); len(got) != 0 {
		t.Fatalf("on a branch there is nothing to say: %+v", got)
	}
}
