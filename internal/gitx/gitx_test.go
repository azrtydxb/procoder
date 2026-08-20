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

// FilesUnder scopes to the directory and honours gitignore — the set a
// directory argument to `procoder lint` expands into.
func TestFilesUnderScopesAndHonoursGitignore(t *testing.T) {
	dir := repo(t)
	os.MkdirAll(filepath.Join(dir, "web", "node_modules"), 0o755)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "web", "app.js"), []byte("x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "web", "node_modules", "dep.js"), []byte("x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "top.js"), []byte("x\n"), 0o644)

	got := FilesUnder(dir, "web")
	if len(got) != 1 || got[0] != filepath.Join(dir, "web", "app.js") {
		t.Fatalf("want only web/app.js (untracked in, gitignored and out-of-dir excluded), got %v", got)
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

// One case per trailer a host is known to write. The two hosts a procoder user
// is most likely to be sitting in — Codex and VS Code — wrote lines the gate
// used to wave through, which left those users with the policy and none of the
// enforcement.
//
// proved by: deleting the Codex entry from aiIdentities — the Codex trailer
// went unfound and this test failed. Same with the Copilot entry deleted.
func TestAttributionBlocksEveryRecognisedIdentity(t *testing.T) {
	bad := []struct{ message, identity string }{
		{"fix parser\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>", "Claude"},
		{"fix parser\n\nGenerated with Claude Code", "Claude"},
		{"fix parser\n\n🤖 automated", "a robot emoji"},
		{"fix parser\n\nCo-authored-by: Codex <noreply@openai.com>", "Codex"},
		{"fix parser\n\nCo-authored-by: Copilot <copilot@github.com>", "Copilot"},
		{"fix parser\n\nCo-authored-by: Cursor Agent <agent@cursor.com>", "Cursor"},
		{"fix parser\n\nMade with Cursor", "Cursor"},
		{"fix parser\n\nCo-Authored-By: Devin <158243242+devin-ai-integration[bot]@users.noreply.github.com>", "Devin"},
		{"fix parser\n\nCo-authored-by: gemini-code-assist[bot] <176961590+gemini-code-assist[bot]@users.noreply.github.com>", "Gemini"},
		{"fix parser\n\nCo-authored-by: aider (gpt-4) <noreply@aider.chat>", "aider"},
	}
	for _, c := range bad {
		got := Attribution([]string{c.message})
		if len(got) != 1 || !got[0].Blocking {
			t.Fatalf("not caught: %q -> %+v", c.message, got)
		}
		// Naming the identity is what makes a false positive arguable: the
		// reader can point at the list entry that was wrong about them.
		if !strings.Contains(got[0].Message, c.identity) {
			t.Errorf("finding must name %q: %q", c.identity, got[0].Message)
		}
	}
}

// The other half of the rule, and the half that decides whether anyone leaves
// the gate switched on. Co-Authored-By predates AI coders by a decade: pair
// programming, a patch carried on someone's behalf and a squashed contribution
// all use it correctly, and so does a human who happens to work at an AI lab or
// to be called Devin. Blocking those teaches users to bypass the gate, which
// costs more than the trailers it would have caught.
//
// proved by: replacing the Claude entry's inTrailer with a bare `.*` so any
// co-author line matched — every case below fired and this test failed.
func TestAttributionLeavesLegitimateCoAuthorsAlone(t *testing.T) {
	clean := []string{
		"fix parser\n\nHandles the empty-input case the old loop skipped.",
		"fix parser\n\nCo-authored-by: Jane Roe <jane@example.com>",
		// A noreply address belonging to a person, not to a vendor.
		"fix parser\n\nCo-authored-by: Jane Roe <12345+janeroe@users.noreply.github.com>",
		// A squash carrying everyone who touched the branch.
		"feat: the importer\n\nCo-authored-by: Jane Roe <jane@example.com>\nCo-authored-by: Sam Poe <sam@example.com>",
		// A human at an AI lab is a person; only the vendor's noreply mailbox
		// is the tool.
		"fix parser\n\nCo-authored-by: Jane Roe <jane@openai.com>",
		"fix parser\n\nCo-authored-by: Jane Roe <jane@anthropic.com>",
		"fix parser\n\nCo-authored-by: Devin Marsh <devin@example.com>",
	}
	for _, m := range clean {
		if got := Attribution([]string{m}); len(got) != 0 {
			t.Errorf("false positive on a legitimate message %q: %+v", m, got)
		}
	}
}

// The quoted match is the whole trailer, not the fragment that happened to
// satisfy the pattern: a reader shown half an address cannot grep their own
// history for the line the gate is refusing.
//
// proved by: dropping the `[^\n]*` tail from aiIdentity.pattern — the finding
// quoted "Co-authored-by: Codex <noreply@openai" and this test failed.
func TestAttributionQuotesTheWholeTrailer(t *testing.T) {
	got := Attribution([]string{"fix parser\n\nCo-authored-by: Codex <noreply@openai.com>"})
	if len(got) != 1 || !strings.Contains(got[0].Message, "Co-authored-by: Codex <noreply@openai.com>") {
		t.Fatalf("the finding must quote the trailer in full: %+v", got)
	}
}

// Amending removes the line the gate found; it does not stop the host that
// wrote it from writing it again, so the finding has to name the setting too.
// Without the pointer a user on a default-configured host meets this same
// blocking finding on every commit and has nowhere to go from the output.
//
// proved by: dropping the remedy clause from the message — the finding still
// blocked, still quoted the offending line, and this test failed.
func TestAttributionFindingNamesTheRemedyNotOnlyTheSymptom(t *testing.T) {
	got := Attribution([]string{"fix parser\n\nCo-Authored-By: Claude <noreply@anthropic.com>"})
	if len(got) != 1 {
		t.Fatalf("want exactly one finding, got %+v", got)
	}
	if !strings.Contains(got[0].Message, attributionRemedyURL) {
		t.Errorf("the finding must point at the per-host settings: %q", got[0].Message)
	}
	if !strings.Contains(got[0].Message, "--amend") {
		t.Errorf("the immediate action must survive alongside the remedy: %q", got[0].Message)
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

// A document whose SUBJECT is merge conflicts cannot show a learner what
// one looks like while the check treats every marker as a defect. The
// exemption is explicit, greppable and carries a reason, in the spirit of
// gitleaks:allow.
// proved by: made ConflictMarkers ignore the allow line — the exempt file
// then reports its markers again.
func TestAnExplicitAllowWithAReasonExemptsTheFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tutorial.md")
	os.WriteFile(p, []byte(
		"<!-- procoder:allow-conflict-markers teaching what a conflict looks like -->\n"+
			"```\n<<<<<<< HEAD\nmine\n=======\ntheirs\n>>>>>>> branch\n```\n"), 0o644)
	if got := ConflictMarkers([]string{p}); len(got) != 0 {
		t.Fatalf("an explicit allow with a reason must exempt the file, got %+v", got)
	}
}

// The other half: without the allow, the same content still blocks — and
// being inside a fenced code block changes nothing, because a real
// conflict lands inside a fence often enough that skipping fences would
// be a silent miss.
// proved by: made ConflictMarkers skip fenced blocks — this then passes
// while a genuine conflict in a documented code sample goes unreported.
func TestMarkersInAFenceStillBlockWithoutAnAllow(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tutorial.md")
	os.WriteFile(p, []byte("```\n<<<<<<< HEAD\nmine\n=======\ntheirs\n>>>>>>> branch\n```\n"), 0o644)
	if got := ConflictMarkers([]string{p}); len(got) != 2 {
		t.Fatalf("markers in a fence still block without an allow, got %d: %+v", len(got), got)
	}
}

// An allow with no reason is someone silencing the check, not documenting
// an exception. It does not count.
// proved by: dropped the reason requirement — a bare token then silences
// the file, which is the bypass this design exists to prevent.
func TestAnAllowWithoutAReasonDoesNotExempt(t *testing.T) {
	dir := t.TempDir()
	for _, bare := range []string{
		"<!-- procoder:allow-conflict-markers -->\n",
		"# procoder:allow-conflict-markers\n",
		"<!-- procoder:allow-conflict-markers    -->\n",
	} {
		p := filepath.Join(dir, "x.md")
		os.WriteFile(p, []byte(bare+"<<<<<<< HEAD\nmine\n>>>>>>> branch\n"), 0o644)
		if got := ConflictMarkers([]string{p}); len(got) != 2 {
			t.Errorf("%q must not exempt anything, got %d findings", bare, len(got))
		}
	}
}

// A reason is prose, and prose starts with whatever it starts with — an
// implementation that sniffs the first character rejects a perfectly good
// one. This pins the reason as "any non-empty text".
// proved by: restored the first-character check — a reason beginning with
// a dash is then read as no reason at all.
func TestAReasonMayStartWithAnyCharacter(t *testing.T) {
	dir := t.TempDir()
	for _, reason := range []string{"- teaching the reader", "-> see ADR 0002", "(illustration)"} {
		p := filepath.Join(dir, "x.md")
		os.WriteFile(p, []byte("<!-- procoder:allow-conflict-markers "+reason+" -->\n"+
			"<<<<<<< HEAD\nmine\n>>>>>>> branch\n"), 0o644)
		if got := ConflictMarkers([]string{p}); len(got) != 0 {
			t.Errorf("reason %q must exempt the file, got %d findings", reason, len(got))
		}
	}
}
