package status

import (
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
	lines := Report(root)
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

func TestReportOnABareRepoSaysWhatIsEmpty(t *testing.T) {
	root := gitRepo(t)
	lines := Report(root)
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
	lines := Report(t.TempDir())
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
	if got := find(Report(root), "open tasks:"); !strings.Contains(got, "unknown") {
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
	lines := Report(root)
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
