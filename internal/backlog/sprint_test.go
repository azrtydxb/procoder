package backlog

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

const openStoryBody = `# %s

Status: open
Created: 2026-08-19
Epic: auth
Sprint: %s

## Description

real enough for a test

## Acceptance criteria

- [x] it works

## Evidence

it worked
`

func story(t *testing.T, root, id, sprint string) string {
	t.Helper()
	body := strings.Replace(openStoryBody, "%s", id, 1)
	body = strings.Replace(body, "%s", sprint, 1)
	return writeItem(t, root, KindStory, id, body)
}

func TestSprintOpenPrintsAndRefusesSecond(t *testing.T) {
	root := t.TempDir()
	out, lines := collect()
	if code := SprintOpen(root, "auth mvp", out); code != 0 {
		t.Fatalf("open: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "sprints/001-auth-mvp.md") || !strings.Contains(joined, "Status: active") {
		t.Fatalf("open must print the sprint path and template: %v", *lines)
	}
	if !strings.Contains(joined, "sprint pull") {
		t.Fatalf("open must point at the next step: %v", *lines)
	}
	// P-CONTROL: printed, not written
	if _, err := os.Stat(root + "/" + Dir); !os.IsNotExist(err) {
		t.Fatal("open must not write files")
	}
	// a hand-written active sprint blocks a second open
	writeItem(t, root, KindSprint, "001-auth-mvp", "# auth mvp\n\nStatus: active\n")
	out2, lines2 := collect()
	if code := SprintOpen(root, "second goal", out2); code != 1 {
		t.Fatalf("second open must refuse: exit %d %v", code, *lines2)
	}
	if !strings.Contains(strings.Join(*lines2, "\n"), "001-auth-mvp") {
		t.Fatalf("refusal must name the active sprint: %v", *lines2)
	}
}

func TestSprintOpenEmptyGoal(t *testing.T) {
	out, lines := collect()
	if code := SprintOpen(t.TempDir(), "  ", out); code != 2 {
		t.Fatalf("empty goal must be usage error: exit %d %v", code, *lines)
	}
}

func TestSprintOpenBlockedByUnreadableSprintFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 is not a thing on windows")
	}
	root := t.TempDir()
	p := writeItem(t, root, KindSprint, "001-old", "# old\n\nStatus: closed 2026-01-01\n")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(p, 0o644) }()
	out, lines := collect()
	if code := SprintOpen(root, "new goal", out); code != 1 {
		t.Fatalf("unreadable sprint must block open: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "unreadable") {
		t.Fatalf("refusal must say why: %v", *lines)
	}
}

func TestSprintOpenSequenceNumbering(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindSprint, "001-done-sprint", "# done sprint\n\nStatus: closed 2026-01-01\n")
	out, lines := collect()
	if code := SprintOpen(root, "next goal", out); code != 0 {
		t.Fatalf("open: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "sprints/002-next-goal.md") {
		t.Fatalf("sequence must continue past 001: %v", *lines)
	}
}

func TestSprintPullMixedBatch(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindSprint, "001-mvp", "# mvp\n\nStatus: active\n")
	good := story(t, root, "20260819-login", "-")
	writeItem(t, root, KindStory, "20260819-done", "# done story\n\nStatus: done 2026-08-01\nEpic: auth\nSprint: -\n")
	out, lines := collect()
	code := SprintPull(root, []string{"20260819-login", "20260819-done", "20260819-missing"}, out)
	if code != 1 {
		t.Fatalf("a batch with refusals must exit 1: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "pulled 20260819-login") {
		t.Fatalf("the good story must still be pulled: %v", *lines)
	}
	if !strings.Contains(joined, "already done") || !strings.Contains(joined, "no such story") {
		t.Fatalf("each refusal needs its reason: %v", *lines)
	}
	raw, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Sprint: 001-mvp") {
		t.Fatalf("pull must rewrite the story's Sprint line: %s", raw)
	}
}

func TestSprintPullRefusesStoryAlreadyInASprint(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindSprint, "002-mvp", "# mvp\n\nStatus: active\n")
	story(t, root, "20260819-taken", "001-old")
	out, lines := collect()
	if code := SprintPull(root, []string{"20260819-taken"}, out); code != 1 {
		t.Fatalf("already-committed story must refuse: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "001-old") {
		t.Fatalf("refusal must name the sprint the story is in: %v", *lines)
	}
}

func TestSprintPullWithoutActiveSprint(t *testing.T) {
	out, lines := collect()
	if code := SprintPull(t.TempDir(), []string{"x"}, out); code != 1 {
		t.Fatalf("pull needs an active sprint: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "sprint open") {
		t.Fatalf("refusal must point at sprint open: %v", *lines)
	}
}

func TestSprintCarryRewritesAndRequiresReason(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindSprint, "001-mvp", "# mvp\n\nStatus: active\n")
	p := story(t, root, "20260819-slow", "001-mvp")
	out, lines := collect()
	if code := SprintCarry(root, "20260819-slow", "", out); code != 2 {
		t.Fatalf("empty reason must be usage error: exit %d %v", code, *lines)
	}
	out2, lines2 := collect()
	if code := SprintCarry(root, "20260819-slow", "blocked on the API redesign", out2); code != 0 {
		t.Fatalf("carry: exit %d %v", code, *lines2)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "Sprint: -") {
		t.Fatalf("carry must send the story back to the backlog: %s", text)
	}
	if !strings.Contains(text, "Carried: 001-mvp — blocked on the API redesign") {
		t.Fatalf("carry must record the sprint and the reason: %s", text)
	}
}

func TestSprintCarryRefusesStoryOutsideTheSprint(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindSprint, "002-mvp", "# mvp\n\nStatus: active\n")
	story(t, root, "20260819-free", "-")
	out, lines := collect()
	if code := SprintCarry(root, "20260819-free", "some reason", out); code != 1 {
		t.Fatalf("only committed stories carry: exit %d %v", code, *lines)
	}
}

func TestSprintStatusCountsDoneAndCarried(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindSprint, "001-mvp", "# auth mvp\n\nStatus: active\n")
	story(t, root, "20260819-open", "001-mvp")
	writeItem(t, root, KindStory, "20260819-shipped", "# shipped\n\nStatus: done 2026-08-19\nEpic: auth\nSprint: 001-mvp\n")
	writeItem(t, root, KindStory, "20260819-back", "# back\n\nStatus: open\nEpic: auth\nSprint: -\nCarried: 001-mvp — too big\n")
	out, lines := collect()
	if code := SprintStatus(root, out); code != 0 {
		t.Fatalf("status: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "001-mvp") || !strings.Contains(joined, "auth mvp") {
		t.Fatalf("status must name the sprint and goal: %v", *lines)
	}
	if !strings.Contains(joined, "[x] 20260819-shipped") || !strings.Contains(joined, "[ ] 20260819-open") {
		t.Fatalf("each committed story shows its state: %v", *lines)
	}
	if !strings.Contains(joined, "1 of 2 stories done") || !strings.Contains(joined, "1 carried back") {
		t.Fatalf("status must count done and carried: %v", *lines)
	}
}

func TestSprintStatusWithoutActiveSprint(t *testing.T) {
	out, lines := collect()
	if code := SprintStatus(t.TempDir(), out); code != 1 {
		t.Fatalf("no active sprint is exit 1: exit %d %v", code, *lines)
	}
}

func TestSprintCloseRefusesThenSucceedsAfterCarry(t *testing.T) {
	root := t.TempDir()
	sp := writeItem(t, root, KindSprint, "001-mvp", "# mvp\n\nStatus: active\nCreated: 2026-08-19\n\n## Goal\n\nship it\n")
	story(t, root, "20260819-open", "001-mvp")
	writeItem(t, root, KindStory, "20260819-shipped", "# shipped\n\nStatus: done 2026-08-19\nEpic: auth\nSprint: 001-mvp\n")

	out, lines := collect()
	if code := SprintClose(root, out); code != 1 {
		t.Fatalf("close must refuse while a committed story is open: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "20260819-open") || !strings.Contains(joined, "sprint carry") {
		t.Fatalf("refusal must name the story and the way out: %v", *lines)
	}

	out2, lines2 := collect()
	if code := SprintCarry(root, "20260819-open", "ran out of sprint", out2); code != 0 {
		t.Fatalf("carry: exit %d %v", code, *lines2)
	}
	out3, lines3 := collect()
	if code := SprintClose(root, out3); code != 0 {
		t.Fatalf("close after carry: exit %d %v", code, *lines3)
	}
	raw, err := os.ReadFile(sp)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "Status: closed ") {
		t.Fatalf("close must rewrite the status: %s", text)
	}
	if !strings.Contains(text, "## Result") ||
		!strings.Contains(text, "committed: 2") ||
		!strings.Contains(text, "done: 1 (20260819-shipped)") ||
		!strings.Contains(text, "carried: 1 (20260819-open)") {
		t.Fatalf("close must write the honest tally: %s", text)
	}
}

func TestSprintCloseZeroStories(t *testing.T) {
	root := t.TempDir()
	sp := writeItem(t, root, KindSprint, "001-empty", "# empty\n\nStatus: active\n")
	out, lines := collect()
	if code := SprintClose(root, out); code != 0 {
		t.Fatalf("an empty sprint closes with the zero summary: exit %d %v", code, *lines)
	}
	raw, err := os.ReadFile(sp)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "committed: 0") || !strings.Contains(text, "done: 0") || !strings.Contains(text, "carried: 0") {
		t.Fatalf("zero summary missing: %s", text)
	}
}
