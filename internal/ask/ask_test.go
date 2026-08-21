package ask

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"procoder/internal/answers"
	"procoder/internal/gitx"
)

// repo builds a throwaway repository with a spec carrying open questions.
func repo(t *testing.T, questions ...string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# widget\n\nStatus: draft\n\n## Open questions\n\n"
	for _, q := range questions {
		body += "- " + q + "\n"
	}
	if err := os.WriteFile(filepath.Join(root, ".procoder", "specs", "widget.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func collect(t *testing.T) (func(string), *[]string) {
	t.Helper()
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

// C-05, and the reason answers are keyed to their question: an answer must
// survive a re-run, and must stop counting the moment the question changes,
// because the old answer was to a different question.
// proved by: keyed answers by position instead of by text — the reworded
// question then reads as answered and the human is never asked again.
func TestAnAnswerSurvivesUntilTheQuestionChanges(t *testing.T) {
	root := repo(t, "should the cache be per-user?")
	qs, _ := Collect(root)
	if len(qs) != 1 {
		t.Fatalf("one question, got %d: %+v", len(qs), qs)
	}
	store := Answers{qs[0].Key(): answers.Entry{Question: qs[0].Text, Answer: "per-user, keyed by account id"}}
	if err := WriteAnswers(root, qs, store, time.Now()); err != nil {
		t.Fatal(err)
	}

	// Same question: not asked again.
	loaded, err := answers.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if left := unansweredIn(root, loaded); len(left) != 0 {
		t.Errorf("an answered question must not be asked twice: %+v", left)
	}

	// Reworded: asked again, because the answer was to the old wording.
	reworded := repo(t, "should the cache be per-user or per-organisation?")
	if err := WriteAnswers(reworded, qs, store, time.Now()); err != nil {
		t.Fatal(err)
	}
	loaded, _ = answers.Load(reworded)
	if left := unansweredIn(reworded, loaded); len(left) != 1 {
		t.Errorf("a changed question is unanswered again, got %d: %+v", len(left), left)
	}
}

// C-02 and C-07: with nobody to ask, it writes the file, names the route back
// in, and exits 1 — it does not hang and it does not answer anything.
// proved by: returned 0 from the no-terminal path — a caller then cannot tell
// "nothing to decide" from "nobody was asked".
func TestWithNoTerminalItWritesTheFileAndSaysSo(t *testing.T) {
	root := repo(t, "is the retry budget three attempts?")
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer func() { _ = null.Close() }()

	out, lines := collect(t)
	if code := Run(root, null, out); code != 1 {
		t.Fatalf("unanswered questions exit 1, got %d: %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	for _, want := range []string{"no terminal", QuestionsFile, "--file", "Do NOT answer them yourself"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the instruction must carry %q: %s", want, joined)
		}
	}
	raw, err := os.ReadFile(Path(root, QuestionsFile))
	if err != nil {
		t.Fatalf("C-03: the questions file must exist: %v", err)
	}
	if !strings.Contains(string(raw), "retry budget") {
		t.Errorf("the question itself must be in the file: %s", raw)
	}
}

// C-04: the route back in. A file that cannot be read as answers is refused
// rather than half-recorded.
// proved by: recorded whatever parsed and reported success — a mistyped key
// then looks like a decision that landed.
func TestTheFileRouteRecordsAnswersAndRefusesNonsense(t *testing.T) {
	root := repo(t, "which database?")
	qs, _ := Collect(root)
	path := filepath.Join(root, "reply.md")
	body := "## Q1\n\n" + answers.KeyPrefix + qs[0].Key() + "\n" + answers.AnswerPrefix + "postgres, for the constraints\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	out, lines := collect(t)
	if code := FromFile(root, path, out); code != 0 {
		t.Fatalf("every question answered exits 0, got %d: %v", code, *lines)
	}
	loaded, _ := answers.Load(root)
	if loaded[qs[0].Key()].Answer != "postgres, for the constraints" {
		t.Errorf("the answer must be recorded against its question: %+v", loaded)
	}

	empty := filepath.Join(root, "nothing.md")
	if err := os.WriteFile(empty, []byte("I decided some things\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out2, lines2 := collect(t)
	if code := FromFile(root, empty, out2); code != 2 {
		t.Fatalf("a file with no answers is refused, got %d: %v", code, *lines2)
	}
	if !strings.Contains(strings.Join(*lines2, "\n"), "nothing was recorded") {
		t.Errorf("the refusal must say nothing landed: %v", *lines2)
	}
}

// A question wrapped over several lines is ONE question. Counting lines made
// a four-question section ask six times, each fragment with its own key, so
// answering the whole thing still left two halves outstanding.
// proved by: treated every non-empty line as a question — this reports three.
func TestAWrappedQuestionIsOneQuestion(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# widget\n\nStatus: draft\n\n## Open questions\n\n" +
		"- [O-1] Which sections are release blockers\n  and which are follow-ups? It depends on\n  other people's queues.\n"
	if err := os.WriteFile(filepath.Join(root, ".procoder", "specs", "widget.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	qs, _ := Collect(root)
	if len(qs) != 1 {
		t.Fatalf("one wrapped question is one question, got %d: %+v", len(qs), qs)
	}
	if !strings.Contains(qs[0].Text, "other people's queues") {
		t.Errorf("the whole question must be carried: %q", qs[0].Text)
	}
}

// An unreadable answers file is not an empty one: re-asking everything and
// then writing over the top would destroy decisions already made.
// proved by: treated the read error as an empty store — Run then asks again
// and records its answers over the unreadable file.
func TestAnUnreadableAnswersFileStopsEverything(t *testing.T) {
	root := repo(t, "anything?")
	if err := os.MkdirAll(Path(root, answers.File), 0o755); err != nil {
		t.Fatal(err) // a directory where the file belongs: unreadable as a file
	}
	out, lines := collect(t)
	if code := Run(root, nil, out); code != 2 {
		t.Fatalf("an unreadable record exits 2, got %d: %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "may already hold decisions") {
		t.Errorf("the refusal must say why it stopped: %v", *lines)
	}
}

// C-01: every domain that can pose a question is asked, and a flagged
// secret's VALUE never leaves the security domain — what a human is asked is
// whether the flag is real, and answering that does not need the credential.
// proved by: put the finding's message into the security question like the
// other domains — the planted value then appears in the question, the file
// and the terminal.
func TestASecretsValueNeverReachesTheQuestion(t *testing.T) {
	planted := "AKIA" + strings.Repeat("X", 4) + "SFODNN7EXAMPLE" // assembled, never a literal the scanners can match
	root := t.TempDir()
	qs := findingQuestions(root, "security", []gitx.Finding{{
		File:    "config/app.yml",
		Line:    12,
		Message: "possible AWS key " + planted + " committed",
	}}, "is this a real credential, or a test value that only looks like one?")
	if len(qs) != 1 {
		t.Fatalf("one finding, one question: %+v", qs)
	}
	if strings.Contains(qs[0].Text, planted) || strings.Contains(qs[0].Origin, planted) {
		t.Fatalf("the value must never be carried: %+v", qs[0])
	}
	if !strings.Contains(qs[0].Origin, "config/app.yml:12") {
		t.Errorf("but where it was found must be, or nobody can judge it: %+v", qs[0])
	}
	// The other domains DO carry their message: that is the evidence a human
	// judges, and none of them handle credentials.
	docsQ := findingQuestions(root, "docs", []gitx.Finding{{File: "a.go", Message: "no doc changed"}}, "real?")
	if !strings.Contains(docsQ[0].Text, "no doc changed") {
		t.Errorf("a docs question must carry its finding: %+v", docsQ[0])
	}
}

// C-03: a second run with nothing new to ask leaves both files alone. A tool
// that rewrites its own state on every read makes a diff nobody can trust.
// proved by: wrote the files unconditionally — the second run then rewrites
// them with a new timestamp and the tree is dirty for no reason.
func TestAsecondRunWithNothingNewWritesNothing(t *testing.T) {
	root := repo(t, "which cache?")
	qs, _ := Collect(root)
	if err := WriteAnswers(root, qs, Answers{qs[0].Key(): answers.Entry{Question: qs[0].Text, Answer: "per-user"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(Path(root, answers.File))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	out, lines := collect(t)
	if code := Run(root, nil, out); code != 0 {
		t.Fatalf("everything answered exits 0: %d %v", code, *lines)
	}
	after, err := os.Stat(Path(root, answers.File))
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("nothing to ask means nothing to write")
	}
	if _, err := os.Stat(Path(root, QuestionsFile)); !os.IsNotExist(err) {
		t.Error("and no questions file is written when there are no questions")
	}
}

// unansweredIn is the question set a repository still has open.
func unansweredIn(root string, store Answers) []Question {
	qs, _ := Collect(root)
	return Unanswered(qs, store)
}

// The key must not bind an answer to one machine's checkout. Security
// findings arrive as absolute paths (lint's are relative — the two sources
// already disagree), and hashing an absolute path into the key means a
// teammate, CI, or the same person after moving the clone is asked
// everything again, while the hook feeds absolute paths to the model.
// proved by: took f.File as-is — the origin then carries /Users/... and the
// key changes with the checkout directory.
func TestTheKeyDoesNotDependOnWhereTheCloneLives(t *testing.T) {
	here := t.TempDir()
	there := t.TempDir()
	finding := func(root string) []gitx.Finding {
		return []gitx.Finding{{File: filepath.Join(root, "internal", "app.go"), Line: 7, Message: "possible key"}}
	}
	a := findingQuestions(here, "security", finding(here), "real?")
	b := findingQuestions(there, "security", finding(there), "real?")
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("one finding each: %+v %+v", a, b)
	}
	if a[0].Origin != "internal/app.go:7" {
		t.Errorf("the origin must be repo-relative, got %q", a[0].Origin)
	}
	if a[0].Key() != b[0].Key() {
		t.Errorf("the same question in two clones is one question: %q vs %q", a[0].Key(), b[0].Key())
	}
}

// A record whose question is no longer being asked keeps the question it
// answered. Rebuilding the file from whatever is live destroyed the text of
// anything since reworded, leaving an answer nobody could interpret — and the
// file's own header promises you can edit it to change what procoder
// believes.
// proved by: wrote only the live questions — the reworded entry then reads
// "(no longer asked)" with no question text at all.
func TestTheRecordKeepsTheQuestionItAnswered(t *testing.T) {
	root := repo(t, "which cache?")
	qs, _ := Collect(root)
	store := Answers{qs[0].Key(): answers.Entry{Question: qs[0].Text, Answer: "per-user"}}
	if err := WriteAnswers(root, qs, store, time.Now()); err != nil {
		t.Fatal(err)
	}
	// The question is reworded, so the recorded one is no longer collected.
	reworded := repo(t, "which cache, and for how long?")
	live, _ := Collect(reworded)
	loaded, err := answers.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteAnswers(reworded, live, loaded, time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(Path(reworded, answers.File))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "which cache?") {
		t.Errorf("the question that was answered must survive:\n%s", raw)
	}
}
