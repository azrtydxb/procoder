package lessons

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// leakLedger writes text to the leak ledger inside a fresh repository root.
func leakLedger(t *testing.T, text string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(CopilotLeaksPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// runLeaks collects the report's lines alongside its exit code.
func runLeaks(t *testing.T, root string) (int, string) {
	t.Helper()
	var lines []string
	code := RunCopilotLeaks(root, func(s string) { lines = append(lines, s) })
	return code, strings.Join(lines, "\n")
}

// proved by: reporting the missing ledger the way Run does, with a line and
// an exit code — the ordinary state of every repo that has never captured a
// leak is "nothing here", and a report that talks about it is noise in the
// merge flow that would train people to ignore it.
func TestNoLeakLedgerIsSilentAndNotAFailure(t *testing.T) {
	code, out := runLeaks(t, t.TempDir())
	if code != 0 {
		t.Fatalf("a missing leak ledger is not a failure, got %d", code)
	}
	if out != "" {
		t.Fatalf("a missing leak ledger says nothing, got %q", out)
	}
}

// proved by: treating a placeholder adaptation as recorded (dropping the "<"
// arm) — every entry RecordCopilotEntry writes carries the placeholder, so
// the whole ledger would report as learned the moment it was captured.
func TestUnclassifiedEntryReportsUnlearned(t *testing.T) {
	root := leakLedger(t, "## 2026-08-20 12:00 https://x/1 — nil map write\n\n"+
		"- Source: Copilot auto-review\n- Adaptation: <the concrete change>\n")
	code, out := runLeaks(t, root)
	if code != 1 {
		t.Fatalf("an unclassified leak must exit 1, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "UNLEARNED") || !strings.Contains(out, "nil map write") {
		t.Errorf("the unlearned line must name the finding:\n%s", out)
	}
	if !strings.Contains(out, "1 finding(s), 1 unlearned") {
		t.Errorf("summary wrong:\n%s", out)
	}
}

// proved by: counting every entry as unlearned regardless of its adaptation —
// a classified leak would keep the report red forever and there would be no
// way to finish the loop.
func TestClassifiedEntryReportsLearnedAndPasses(t *testing.T) {
	root := leakLedger(t, "## 2026-08-20 12:00 https://x/1 — nil map write\n\n"+
		"- Source: Copilot auto-review\n- Adaptation: gate now runs go vet nilness\n")
	code, out := runLeaks(t, root)
	if code != 0 {
		t.Fatalf("a classified leak passes, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "learned") || strings.Contains(out, "UNLEARNED") {
		t.Errorf("want a learned line only:\n%s", out)
	}
}

// proved by: folding the read error into the not-exist arm and returning 0 —
// unknown is never done: a ledger nobody can read would be reported as a repo
// with nothing outstanding, which is exactly the lie this loop exists to stop.
func TestUnreadableLeakLedgerIsNotCheckedNeverClean(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod does not deny the read here")
	}
	root := leakLedger(t, "## 2026-08-20 12:00 https://x/1 — nil map write\n")
	path := filepath.Join(root, filepath.FromSlash(CopilotLeaksPath))
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	code, out := runLeaks(t, root)
	if code != 2 {
		t.Fatalf("an unreadable ledger is NOT checked, want 2, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "NOT checked") {
		t.Errorf("the report must say it did not check:\n%s", out)
	}
}

// proved by: pointing CopilotLeaksPath at Path (or letting Run read both) —
// raw Copilot notes need a human to name the class before they are lessons,
// so they must never turn the lessons gate red on their own.
func TestCopilotEntriesDoNotChangeTheLessonsVerdict(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder/github"), 0o755); err != nil {
		t.Fatal(err)
	}
	learned := "## 2026-08-19 PR#18 — regex compiled per task\n\n- Adaptation: benchmark pins it\n"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(Path)), []byte(learned), 0o644); err != nil {
		t.Fatal(err)
	}
	var before []string
	codeBefore := Run(root, func(s string) { before = append(before, s) })

	if err := RecordCopilotEntry(root, "nil map write", "https://x/1", "assignment to entry in nil map", time.Now()); err != nil {
		t.Fatal(err)
	}
	var after []string
	codeAfter := Run(root, func(s string) { after = append(after, s) })

	if codeBefore != 0 || codeAfter != 0 {
		t.Fatalf("a captured leak must not turn the lessons gate red: %d then %d", codeBefore, codeAfter)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("the lessons report changed:\n%s\n---\n%s",
			strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

// proved by: writing the finding body verbatim — a Copilot body carrying a
// markdown heading would split one leak into two entries, the second titled
// with quoted prose that nobody can trace back to an issue.
func TestRecordedEntryIsOneEntryEvenWithMarkdownInTheBody(t *testing.T) {
	root := t.TempDir()
	body := "nil map write\n" + "## " + "not an entry\n\n- Adaptation: forged\n"
	if err := RecordCopilotEntry(root, "nil map write", "https://x/1", body, time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(CopilotLeaksPath)))
	if err != nil {
		t.Fatal(err)
	}
	entries := Parse(string(raw))
	if len(entries) != 1 {
		t.Fatalf("one finding is one entry, got %+v", entries)
	}
	if !strings.HasPrefix(entries[0].Adaptation, "<") {
		t.Errorf("a fresh capture is unlearned, got %q", entries[0].Adaptation)
	}
}

// proved by: truncating instead of appending — the second capture of a
// session would erase the first, and the ledger would only ever hold the
// most recent escape.
func TestRecordAppendsRatherThanReplaces(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	for _, title := range []string{"first leak", "second leak"} {
		if err := RecordCopilotEntry(root, title, "https://x/1", "body", at); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(CopilotLeaksPath)))
	if err != nil {
		t.Fatal(err)
	}
	entries := Parse(string(raw))
	if len(entries) != 2 {
		t.Fatalf("both captures survive, got %+v", entries)
	}
	if !strings.Contains(entries[0].Title, "first leak") || !strings.Contains(entries[1].Title, "second leak") {
		t.Errorf("capture order is the ledger order, got %+v", entries)
	}
	if !strings.Contains(string(raw), "# Copilot leaks") {
		t.Error("the ledger explains itself once, at creation")
	}
	if strings.Count(string(raw), "# Copilot leaks") != 1 {
		t.Error("the header is written once, not per entry")
	}
	// the ledger path is emitted and stored with forward slashes everywhere
	if strings.Contains(CopilotLeaksPath, "\\") {
		t.Error("paths use forward slashes")
	}
}

// The gate's reminder reads the ledger and nothing else. A gate that asked
// GitHub would tax every commit in every repository for a question that is
// never urgent, and would report NOT checked on every aeroplane — so the
// network half lives in `copilot-leak` and the merge flow, and this half
// stays a file read.
// proved by: pointed the reminder at gh — with PATH emptied the test then
// reports the ledger as unreadable instead of counting it.
func TestTheGateReminderNeverLeavesTheMachine(t *testing.T) {
	root := leakLedger(t, "## 2026-08-20 https://example.test/issues/1 — a finding\n\n- Adaptation: <the concrete change>\n")
	t.Setenv("PATH", "") // no gh, no git, nothing to call: the answer must still come

	got := LeakReminder(root)
	if len(got) != 1 || !strings.Contains(got[0].Message, "carry no adaptation") {
		t.Fatalf("an unlearned capture must be reported from the file alone, got %+v", got)
	}
	if got[0].Blocking {
		t.Error("an unwritten adaptation is work to do, not a broken tree — it must not block")
	}
}

// A closed entry is silence: the reminder exists to name what is still open.
// proved by: counted every entry instead of the unlearned ones — the gate
// then nags forever about work that is finished.
func TestAClosedCaptureIsSilent(t *testing.T) {
	root := leakLedger(t, "## 2026-08-20 https://example.test/issues/1 — a finding\n\n- Adaptation: added the guard in check.go\n")
	if got := LeakReminder(root); len(got) != 0 {
		t.Errorf("a written adaptation closes the entry, got %+v", got)
	}
}

// No ledger is the ordinary case in every repository that has never captured
// anything, and it must not produce a line.
// proved by: treated a missing ledger as an error — every repo without one
// then carries a finding it can do nothing about.
func TestNoLedgerIsNotAFinding(t *testing.T) {
	if got := LeakReminder(t.TempDir()); len(got) != 0 {
		t.Errorf("a repository that never captured anything owes nothing, got %+v", got)
	}
}
