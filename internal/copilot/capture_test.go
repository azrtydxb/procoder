package copilot

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"procoder/internal/lessons"
)

// stubGh puts a fake `gh` on PATH — a real executable, because Capture reaches
// the CLI the way a user's shell does, through PATH — and returns the file it
// records its argv in. exit is the code the stub answers with, so the same
// helper covers the succeeding and the failing GitHub.
func stubGh(t *testing.T, exit int, stderr string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a POSIX shell script")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> '" + log + "'; done\n"
	if stderr != "" {
		script += "printf '%s\\n' '" + stderr + "' >&2\n"
	}
	script += "printf 'https://github.com/example/widget/issues/9\\n'\n"
	script += "exit " + strconv.Itoa(exit) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// PATH is the stub and nothing else: a real gh installed on this machine
	// would otherwise answer these tests and quietly open issues
	t.Setenv("PATH", dir)
	return log
}

func finding() Sanitised {
	return Sanitised{
		Title:       "nil map written on the empty-config path",
		Body:        "A write to an unallocated map panics when no config file exists.",
		Line:        42,
		OriginalURL: "https://github.com/example/widget/issues/7",
		Created:     time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC),
	}
}

func ledger(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(LedgerPath)))
	if err != nil {
		t.Fatalf("ledger unreadable: %v", err)
	}
	return string(raw)
}

// proved by: dropping either half of Capture — the gh call or the ledger
// append — or writing an adaptation that does not start with "<", which would
// make a captured leak read as already learned.
func TestCaptureOpensAnIssueAndRecordsAnUnlearnedEntry(t *testing.T) {
	log := stubGh(t, 0, "")
	root := t.TempDir()

	issues, written, notes := Capture([]Sanitised{finding()}, root)
	if issues != 1 || written != 1 {
		t.Fatalf("one finding must produce one issue and one entry, got %d/%d (notes: %v)", issues, written, notes)
	}
	if len(notes) != 0 {
		t.Fatalf("a clean capture must report no failures: %v", notes)
	}

	argv, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("gh was never called: %v", err)
	}
	for _, want := range []string{"issue", "create", "--label", "auto-copilot", "copilot-leak"} {
		if !strings.Contains(string(argv), want+"\n") {
			t.Fatalf("gh argv must carry %q: %s", want, argv)
		}
	}

	text := ledger(t, root)
	entries := lessons.Parse(text)
	if len(entries) != 1 {
		t.Fatalf("the ledger must parse as exactly one lessons entry: %q", text)
	}
	if !strings.Contains(entries[0].Title, "https://github.com/example/widget/issues/7") ||
		!strings.Contains(entries[0].Title, "2026-08-20") {
		t.Fatalf("the heading must carry the date and the original URL: %q", entries[0].Title)
	}
	if !strings.HasPrefix(entries[0].Adaptation, "<") {
		t.Fatalf("a captured leak must read as UNLEARNED until a human writes the adaptation: %q", entries[0].Adaptation)
	}
	if !strings.Contains(text, "- Source: Copilot auto-review") {
		t.Fatalf("the entry must name its source: %q", text)
	}
}

// proved by: returning early from Capture when gh fails, which would lose the
// finding entirely — the ledger is the memory, the issue only announces it.
func TestIssueFailureStillWritesTheLedger(t *testing.T) {
	stubGh(t, 1, "gh: HTTP 403 rate limit exceeded")
	root := t.TempDir()

	issues, written, notes := Capture([]Sanitised{finding()}, root)
	if issues != 0 {
		t.Fatalf("a failing gh must not be counted as an issue created: %d", issues)
	}
	if written != 1 {
		t.Fatalf("the ledger must still record the finding: %d entries", written)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "rate limit") {
		t.Fatalf("the failure must be reported, never swallowed: %v", notes)
	}
	if len(lessons.Parse(ledger(t, root))) != 1 {
		t.Fatal("the entry must be readable by the lessons parser even when GitHub refused")
	}
}

// proved by: writing the ledger before creating the issues and returning on
// its error, which would silently drop the issues on a read-only tree.
func TestUnwritableLedgerStillCreatesTheIssues(t *testing.T) {
	log := stubGh(t, 0, "")
	root := t.TempDir()
	// a directory where the file belongs: the open fails, nothing else does
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(LedgerPath)), 0o755); err != nil {
		t.Fatal(err)
	}

	issues, written, notes := Capture([]Sanitised{finding()}, root)
	if issues != 1 {
		t.Fatalf("the issue must be created even when the ledger cannot be written: %d", issues)
	}
	if written != 0 {
		t.Fatalf("an unwritten ledger must not be reported as written: %d", written)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], LedgerPath) {
		t.Fatalf("the note must name the ledger that could not be written: %v", notes)
	}
	if strings.Contains(notes[0], "\\") {
		t.Fatalf("every emitted path uses forward slashes: %q", notes[0])
	}
	if _, err := os.ReadFile(log); err != nil {
		t.Fatalf("gh was never called: %v", err)
	}
}

// proved by: publishing a finding whose sanitised body is empty — an issue
// with no content, and a ledger entry that teaches nothing.
func TestEmptyBodyIsSkippedWithANote(t *testing.T) {
	log := stubGh(t, 0, "")
	root := t.TempDir()
	f := finding()
	f.Body = "   \n\t\n"

	issues, written, notes := Capture([]Sanitised{f}, root)
	if issues != 0 || written != 0 {
		t.Fatalf("an empty body must produce neither issue nor entry, got %d/%d", issues, written)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "skipped") {
		t.Fatalf("the skip must be reported: %v", notes)
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("gh must not be called for a finding with nothing safe to publish")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(LedgerPath))); err == nil {
		t.Fatal("no entry means no ledger file is created")
	}
}

// proved by: skipping the ModeCharDevice check, which would make Prompt read
// its answer out of a pipe — consent nobody gave, in CI or a hook.
func TestPromptRefusesWhenStdinIsNotATerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if _, err := w.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	said := 0
	if Prompt(r, func(string) { said++ }, 3, 24*time.Hour) {
		t.Fatal("a yes waiting in a pipe is not a human saying yes")
	}
	if said != 0 {
		t.Fatalf("nothing may be prompted when there is nobody to answer: %d line(s)", said)
	}
}

// proved by: widening the accepted answers — anything but a bare y or yes
// read as consent, and the binary would act on a shrug.
func TestOnlyYesCountsAsConsent(t *testing.T) {
	for _, yes := range []string{"y\n", "yes\n", " Y \n", "YES"} {
		if !isYes(yes) {
			t.Fatalf("%q is a human saying yes", yes)
		}
	}
	for _, no := range []string{"", "\n", "n\n", "no", "yeah", "y es", "sure", "1"} {
		if isYes(no) {
			t.Fatalf("%q must not read as yes", no)
		}
	}
	if Prompt(nil, func(string) {}, 1, time.Hour) {
		t.Fatal("no stdin at all is not a yes")
	}
}
