package status

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// requireGit skips where git is absent — the CI test leg installs no tooling,
// and a fixture must never assume any.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	write(t, root, "README.md", "# fixture\n")
	run("add", "-A")
	run("commit", "-qm", "first")
	return root
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fullRepo is the fixture with everything on: a sprint with one done and one
// open story, an open todo task, a lessons ledger, and an index meta pinned to
// a commit that is not HEAD (so staleness has something to report).
func fullRepo(t *testing.T) string {
	root := gitRepo(t)
	write(t, root, ".procoder/backlog/sprints/001-ship.md", "# Ship it\n\nStatus: active\n")
	write(t, root, ".procoder/backlog/stories/s-done.md", "# Done story\n\nStatus: done 2026-08-01\nSprint: 001-ship\n")
	write(t, root, ".procoder/backlog/stories/s-open.md", "# Open story\n\nStatus: open\nSprint: 001-ship\n")
	write(t, root, ".procoder/todo/20260819-a-task.md", "# A task\n\nStatus: open\n")
	write(t, root, ".procoder/github/LESSONS.md", "# Lessons\n\n## 2026-08-01 review — a thing escaped\n\n- Adaptation: <none yet>\n")
	write(t, root, ".procoder/index/meta.json", `{"commit":"deadbee","built_at":"2026-08-01","files":3,"tags":9}`)
	return root
}

func find(lines []string, prefix string) string {
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

func TestReportOnAFullRepoIsAllComputedFacts(t *testing.T) {
	root := fullRepo(t)
	lines := report(root, testBudget)
	joined := strings.Join(lines, "\n")

	if got := find(lines, "branch:"); !strings.Contains(got, "main") || !strings.Contains(got, "default branch") {
		t.Fatalf("branch line does not compare against the default: %q\n%s", got, joined)
	}
	if got := find(lines, "head:"); got == "head:" || strings.Contains(got, "unknown") {
		t.Fatalf("head line has no commit: %q", got)
	}
	if got := find(lines, "dirty files:"); !strings.Contains(got, "dirty files: ") || strings.Contains(got, "unknown") {
		t.Fatalf("dirty count missing or unknown in a working repo: %q", got)
	}
	if got := find(lines, "sprint:"); !strings.Contains(got, "001-ship") || !strings.Contains(got, "1 of 2 stories done") {
		t.Fatalf("sprint line wrong: %q\n%s", got, joined)
	}
	if got := find(lines, "  open story:"); !strings.Contains(got, "s-open") {
		t.Fatalf("the open story is not listed: %q\n%s", got, joined)
	}
	if got := find(lines, "open tasks:"); !strings.Contains(got, "20260819-a-task") {
		t.Fatalf("open task not reported: %q", got)
	}
	if got := find(lines, "unlearned lessons:"); got != "unlearned lessons: 1" {
		t.Fatalf("unlearned lesson count wrong: %q", got)
	}
	if got := find(lines, "index:"); !strings.Contains(got, "STALE") || !strings.Contains(got, "deadbee") {
		t.Fatalf("index staleness not reported: %q", got)
	}
	if strings.Contains(joined, "omitted for speed") {
		t.Fatalf("a small fixture must not overrun the budget:\n%s", joined)
	}
}

// testBudget is deliberately far past anything these fixtures need. What
// they assert is WHAT a repository reports, and inheriting the production
// budget made them assert how fast git is on the machine running them —
// which is how internal/status went red on a loaded Windows runner over a
// change that touched one Markdown file.
const testBudget = 60 * time.Second

func TestReportOnABareRepoSaysWhatIsEmpty(t *testing.T) {
	root := gitRepo(t)
	lines := report(root, testBudget)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"sprint: none — no backlog yet",
		"open tasks: none",
		"unlearned lessons: none",
		"index: none",
	} {
		if find(lines, want) == "" {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
	if got := find(lines, "dirty files:"); got != "dirty files: none (clean tree)" {
		t.Fatalf("a freshly committed repo is clean: %q", got)
	}
}

func TestReportOutsideARepoIsUnknownWithTheReason(t *testing.T) {
	requireGit(t)
	lines := report(t.TempDir(), testBudget)
	for _, prefix := range []string{"branch:", "head:", "dirty files:"} {
		got := find(lines, prefix)
		if !strings.Contains(got, "unknown — ") {
			t.Fatalf("%s must be unknown WITH a reason outside a repo: %q", prefix, got)
		}
		if strings.TrimSpace(strings.SplitN(got, "unknown — ", 2)[1]) == "" {
			t.Fatalf("%s reports unknown with an empty reason: %q", prefix, got)
		}
	}
	// the rest of the report still computes — one missing source never
	// silences the others
	if find(lines, "open tasks:") == "" || find(lines, "index:") == "" {
		t.Fatalf("the file-backed lines must still print outside a repo:\n%s", strings.Join(lines, "\n"))
	}
}

func TestUnreadableTodoDirectoryIsUnknownNotZero(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads everything; the permission bit proves nothing")
	}
	root := gitRepo(t)
	dir := filepath.Join(root, ".procoder", "todo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Skip("cannot drop read permission here")
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := os.ReadDir(dir); err == nil {
		t.Skip("this filesystem ignores the permission bits")
	}
	if got := find(report(root, testBudget), "open tasks:"); !strings.Contains(got, "unknown") {
		t.Fatalf("an unreadable todo directory must read as unknown, not none: %q", got)
	}
}

// A long sprint must not bury the principles the same block carries, and the
// stories it stops naming must still be counted.
func TestALongSprintIsCappedAndCounted(t *testing.T) {
	root := gitRepo(t)
	write(t, root, ".procoder/backlog/sprints/001-ship.md", "# Ship it\n\nStatus: active\n")
	for i := 0; i < storyCap+3; i++ {
		write(t, root, ".procoder/backlog/stories/s-"+string(rune('a'+i))+".md",
			"# story\n\nStatus: open\nSprint: 001-ship\n")
	}
	lines := report(root, testBudget)
	named := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "  open story:") {
			named++
		}
	}
	if named != storyCap {
		t.Fatalf("named %d stories, cap is %d", named, storyCap)
	}
	if got := find(lines, "  … "); !strings.Contains(got, "3 more") {
		t.Fatalf("the stories beyond the cap are not counted: %q", got)
	}
}

func TestReportStaysInsideTheBudget(t *testing.T) {
	root := fullRepo(t)
	start := time.Now()
	Report(root)
	if elapsed := time.Since(start); elapsed > Budget {
		t.Fatalf("report took %s — the SessionStart budget is %s", elapsed, Budget)
	} else {
		t.Logf("report took %s (budget %s)", elapsed, Budget)
	}
}

func TestRunPrintsTheHeaderAndExitsZero(t *testing.T) {
	root := gitRepo(t)
	var got []string
	if code := Run(root, func(s string) { got = append(got, s) }); code != 0 {
		t.Fatalf("status exit %d — a report cannot fail", code)
	}
	if len(got) == 0 || got[0] != Header {
		t.Fatalf("the report must open with %q, got %v", Header, got)
	}
}

// The report names what the gate will not run, and stays silent when
// there is nothing to name.
// proved by: returned the line unconditionally — every single-language
// repository gets "gate defers to CI: " on every session, naming nothing.
func TestTheReportNamesWhatTheGateDefers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := deferredLine(root); got != "" {
		t.Errorf("nothing deferred, nothing said: %q", got)
	}

	if err := os.WriteFile(filepath.Join(root, "package.json"),
		[]byte(`{"scripts":{"test":"vitest run"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := deferredLine(root)
	if !strings.Contains(got, "js") {
		t.Errorf("the deferred suite must be named: %q", got)
	}
	// Named AND explained: "js" alone reads as a suite that failed.
	if !strings.Contains(got, "CI") {
		t.Errorf("the line must say who runs it instead: %q", got)
	}
}

// However the budget runs out, the report never claims a clean tree it
// did not verify. There are two ways out: each git call fails on its own
// deadline and the line says unknown WITH the reason, or the whole lookup
// misses the wall and the git lines are dropped with a note. Both are
// correct; what must never happen is the third thing — the sentence a
// clean repository would have produced, printed by a report that never
// asked.
//
// This is the path the Windows runner took. The product was right and
// the test was wrong: it inherited the production wall and then asserted
// the machine had beaten it.
// proved by: made dirtyLine fall back to "none (clean tree)" when git
// gave no answer — every timed-out report then claims a clean tree, and
// the session that reads it starts by believing there is nothing to
// commit.
func TestAnExpiredBudgetNeverClaimsACleanTree(t *testing.T) {
	root := fullRepo(t)
	// budget == reserve, so the git lookups are handed a zero deadline.
	lines := report(root, reserve)
	joined := strings.Join(lines, "\n")

	got := find(lines, "dirty files:")
	switch {
	case got == "":
		// dropped wholesale — then the report must say it dropped something
		if !strings.Contains(joined, "omitted for speed") {
			t.Errorf("state left out must be named:\n%s", joined)
		}
	case strings.Contains(got, "unknown"):
		// answered as unknown — then it must carry the reason
		if !strings.Contains(got, "—") {
			t.Errorf("unknown must carry its reason: %q", got)
		}
	default:
		t.Errorf("a report that could not ask git must not answer for it: %q", got)
	}

	// Whichever path it took, the lines that never needed git are there.
	if find(lines, "sprint:") == "" {
		t.Errorf("what could be computed is still reported:\n%s", joined)
	}
}

// A git call that did not answer is reported as unknown WITH the reason,
// never as the sentence a clean repository produces. Asserted directly on
// the line function rather than through Report, because Report races the
// whole lookup against the wall: whichever way that race falls is correct
// behaviour, so a test that goes through it can only ever check the
// invariant, not this branch. This is the branch.
// proved by: returned "dirty files: none (clean tree)" from the error
// path — a report that never reached git tells the session there is
// nothing to commit.
func TestAGitCallThatDidNotAnswerIsNotACleanTree(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // no time at all, deterministically

	got := dirtyLine(ctx, root)
	if strings.Contains(got, "clean tree") || strings.Contains(got, "none") {
		t.Fatalf("git never answered; this is not a clean tree: %q", got)
	}
	if !strings.Contains(got, "unknown") {
		t.Errorf("the line must say unknown: %q", got)
	}
	if !strings.Contains(got, "—") {
		t.Errorf("unknown must carry the reason git gave: %q", got)
	}

	// headLine takes the same deadline and must answer the same way — and
	// must say WHICH unknown. It used to answer "no commits yet" for every
	// failure, so a repository with a hundred commits whose git call timed
	// out was told it had none.
	h := headLine(ctx, root)
	if !strings.Contains(h, "unknown") {
		t.Errorf("head must be unknown when git did not answer: %q", h)
	}
	if strings.Contains(h, "no commits yet") {
		t.Errorf("this repository has a commit; the reason is the timeout: %q", h)
	}

	// And the genuinely empty repository still gets the sentence written
	// for it, rather than git's raw complaint.
	if e := headLine(context.Background(), t.TempDir()); !strings.Contains(e, "unknown") {
		t.Errorf("a directory that is not a repository is unknown: %q", e)
	}

	// branchLine is deliberately NOT asserted here: it reaches git through
	// gitx.CurrentBranch and gitx.DefaultBranch, which take no context, so
	// it answers whatever the deadline says. That is a real gap in the
	// budget rather than a property worth pinning — filed separately, and
	// asserting it here would pin the gap in place.
}
