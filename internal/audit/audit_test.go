package audit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"procoder/internal/adr"
	"procoder/internal/docs"
	"procoder/internal/gitx"
	"procoder/internal/security"
	"procoder/internal/tools"
)

// write plants a fixture file, creating its directory.
func write(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A BLOCK finding sitting past the section cap must still be shown:
// orderForDisplay puts blocking findings first, so truncation only ever
// drops info lines.
func TestOrderForDisplayBlockingSurvivesCap(t *testing.T) {
	var findings []gitx.Finding
	for i := range maxPerSection + 5 {
		findings = append(findings, gitx.Finding{Message: fmt.Sprintf("info %d", i)})
	}
	findings = append(findings, gitx.Finding{Blocking: true, Message: "block A"})
	findings = append(findings, gitx.Finding{Blocking: true, Message: "block B"})

	ordered := orderForDisplay(findings)
	if len(ordered) != len(findings) {
		t.Fatalf("ordering changed length: got %d, want %d", len(ordered), len(findings))
	}
	if ordered[0].Message != "block A" || ordered[1].Message != "block B" {
		t.Fatalf("blocking findings not first (stable): got %q, %q", ordered[0].Message, ordered[1].Message)
	}
	for i, f := range ordered[2:] {
		if want := fmt.Sprintf("info %d", i); f.Message != want {
			t.Fatalf("info order not stable at %d: got %q, want %q", i, f.Message, want)
		}
	}
}

// A sweep passes every tracked file, so the two diff-scoped documentation
// questions — does a doc mention what you changed, did this diff move
// public surface — would answer about everything at once. On this
// repository that was 129 findings burying nine real ones.
func TestSweepDropsTheDiffScopedDocumentationQuestions(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/thing\n\ngo 1.22\n")
	write(t, root, "internal/service/handler.go", "package service\n\n// Serve serves.\nfunc Serve() {}\n")
	write(t, root, "docs/guide.md", "# Guide\n\nSee internal/service/handler.go for the entry point.\n")
	files := []string{
		filepath.Join(root, "go.mod"),
		filepath.Join(root, "internal/service/handler.go"),
		filepath.Join(root, "docs/guide.md"),
	}
	for _, f := range docs.CollectSweep(root, files) {
		if strings.Contains(f.Message, "mentions changed file") ||
			strings.Contains(f.Message, "documentation obligation") {
			t.Fatalf("a sweep must not ask a diff-scoped question: %s", f.Message)
		}
	}
	// and the diff path still asks it, so nothing was lost
	var sawDrift bool
	for _, f := range docs.CollectOfflineFor(root, files, "", false) {
		if strings.Contains(f.Message, "mentions changed file") {
			sawDrift = true
		}
	}
	if !sawDrift {
		t.Fatal("the diff path must still report drift — the check moved, it did not die")
	}
}

// gitRepo is a fixture repository: the audit reads its file set from git
// itself, so without git there is nothing to audit and nothing to assert.
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed — the audit has no file set to read")
	}
	root := t.TempDir()
	cmd := exec.Command("git", "-C", root, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return root
}

// collect gathers the report the way the CLI prints it.
func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

// section returns the lines of the named section, header first, up to the
// next header.
func section(lines []string, header string) []string {
	var out []string
	in := false
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "## "+header):
			in = true
		case in && strings.HasPrefix(l, "## "):
			return out
		}
		if in {
			out = append(out, l)
		}
	}
	return out
}

func headerIndex(lines []string, prefix string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}

// A directory git knows nothing about cannot be audited, and the audit says
// so instead of reporting a clean tree it never read.
// proved by: returning 0 with the same message instead of 2 — the refusal
// started looking like a pass and the test failed.
func TestRunRefusesWhatIsNotAGitRepository(t *testing.T) {
	out, lines := collect()
	if code := Run(t.TempDir(), out); code != 2 {
		t.Fatalf("a non-repository must refuse: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "not a git repository") {
		t.Fatalf("the refusal must say why: %v", *lines)
	}
}

// The report is read top to bottom: the scope line, then formatting,
// hygiene, secrets and lint, and the scorecard last. A section that
// wandered — or vanished — would change what a triaging agent reads first.
// proved by: swapping the hygiene and lint report() calls — hygiene landed
// after lint and the ordering assertion failed.
func TestRunPrintsEverySectionInReadingOrder(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "notes.txt", "hello\n")

	out, lines := collect()
	Run(root, out)

	if !strings.HasPrefix((*lines)[0], "procoder audit over 1 file(s) in scope") {
		t.Fatalf("the scope line comes first: %q", (*lines)[0])
	}
	order := []string{"## formatting", "## hygiene", "## secrets", "## lint", "## scorecard"}
	prev := 0
	for _, h := range order {
		i := headerIndex(*lines, h)
		if i < 0 {
			t.Fatalf("%s is missing from the report:\n%s", h, strings.Join(*lines, "\n"))
		}
		if i < prev {
			t.Fatalf("%s appears out of order at line %d:\n%s", h, i, strings.Join(*lines, "\n"))
		}
		prev = i
	}
	last := headerIndex(*lines, "## scorecard")
	for _, l := range (*lines)[last+1:] {
		if strings.HasPrefix(l, "## ") {
			t.Fatalf("the scorecard closes the report, %q came after it:\n%s", l, strings.Join(*lines, "\n"))
		}
	}
}

// The scorecard is the number a triaging agent trusts: it must equal what
// the sections themselves reported, formatting included. A section whose
// blocks never reached the total would let a repo look closer to the gate
// than it is.
// proved by: deleting `totalBlocking += len(fmtFindings)` — the unchecked
// formatting blocks stopped counting and the test failed.
func TestScorecardTotalsEverySectionsBlockingFindings(t *testing.T) {
	root := gitRepo(t)
	// a conflict marker is a hygiene block, and the badly indented Go file is
	// a formatting one — unformatted where gofmt is installed, NOT checked
	// where it is not, blocking either way
	write(t, root, "merged.txt", "one\n<<<<<<< HEAD\ntwo\n")
	write(t, root, "main.go", "package main\n\nfunc main()  {\n\t\tprintln( \"hi\" )\n}\n")

	out, lines := collect()
	code := Run(root, out)

	sum := 0
	for _, l := range *lines {
		if n, ok := sectionBlocking(l); ok {
			sum += n
		}
	}
	fmtBlocks := countPrefix(section(*lines, "formatting"), "  BLOCK  ") + moreCount(section(*lines, "formatting"))
	if fmtBlocks == 0 {
		t.Fatalf("a badly formatted Go file is either unformatted or NOT checked:\n%s",
			strings.Join(section(*lines, "formatting"), "\n"))
	}
	sum += fmtBlocks

	total, ok := scorecardTotal(*lines)
	if !ok {
		t.Fatalf("the scorecard must print a blocking total:\n%s", strings.Join(*lines, "\n"))
	}
	if total != sum {
		t.Fatalf("scorecard says %d blocking, the sections reported %d:\n%s",
			total, sum, strings.Join(*lines, "\n"))
	}
	if total <= 0 {
		t.Fatalf("a conflict marker is a blocking finding:\n%s", strings.Join(*lines, "\n"))
	}
	if code != 1 {
		t.Fatalf("blocking findings mean exit 1, got %d", code)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "would NOT pass the procoder gate") {
		t.Fatalf("the verdict must be stated:\n%s", strings.Join(*lines, "\n"))
	}
}

// sectionBlocking reads the "N blocking" count off a section header.
func sectionBlocking(line string) (int, bool) {
	if !strings.HasPrefix(line, "## ") || !strings.HasSuffix(line, " blocking") {
		return 0, false
	}
	fields := strings.Fields(line)
	n, err := strconv.Atoi(fields[len(fields)-2])
	return n, err == nil
}

func scorecardTotal(lines []string) (int, bool) {
	for _, l := range lines {
		if strings.HasPrefix(l, "  blocking findings total: ") {
			n, err := strconv.Atoi(strings.TrimPrefix(l, "  blocking findings total: "))
			return n, err == nil
		}
	}
	return 0, false
}

func countPrefix(lines []string, prefix string) int {
	n := 0
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

// moreCount reads N off a "  … N more" line, or 0 when the section was not
// truncated.
func moreCount(lines []string) int {
	for _, l := range lines {
		if strings.HasPrefix(l, "  … ") && strings.HasSuffix(l, " more") {
			n, err := strconv.Atoi(strings.Fields(l)[1])
			if err == nil {
				return n
			}
		}
	}
	return 0
}

// The section cap keeps the report readable, and the truncation is honest:
// exactly the cap is printed and the remainder is counted, so the header
// total and the "… N more" line always add up.
// proved by: dropping the `shown = shown[:maxPerSection]` truncation — all
// findings printed while the tail line still claimed the remainder, and the
// arithmetic assertion failed.
func TestSectionCapTruncatesWithHonestArithmetic(t *testing.T) {
	root := gitRepo(t)
	for i := range 20 {
		write(t, root, fmt.Sprintf("merged%d.txt", i), "one\n<<<<<<< HEAD\ntwo\n")
	}

	out, lines := collect()
	Run(root, out)

	sec := section(*lines, "hygiene")
	if len(sec) == 0 {
		t.Fatalf("no hygiene section:\n%s", strings.Join(*lines, "\n"))
	}
	total := headerTotal(t, sec[0])
	if total <= maxPerSection {
		t.Fatalf("20 conflict markers must overflow the cap, header says: %s", sec[0])
	}
	shown := countPrefix(sec, "  BLOCK") + countPrefix(sec, "  info")
	if shown != maxPerSection {
		t.Fatalf("a capped section prints exactly %d findings, printed %d:\n%s",
			maxPerSection, shown, strings.Join(sec, "\n"))
	}
	if more := moreCount(sec); more != total-maxPerSection {
		t.Fatalf("the tail line says %d more, but %d - %d = %d:\n%s",
			more, total, maxPerSection, total-maxPerSection, strings.Join(sec, "\n"))
	}
}

// headerTotal reads the "N finding(s)" count off a section header.
func headerTotal(t *testing.T, header string) int {
	t.Helper()
	_, rest, ok := strings.Cut(header, "— ")
	if !ok {
		t.Fatalf("unparseable section header: %q", header)
	}
	n, err := strconv.Atoi(strings.Fields(rest)[0])
	if err != nil {
		t.Fatalf("unparseable section header: %q", header)
	}
	return n
}

// The cap must never hide a BLOCK behind information. The hygiene section
// emits its info findings (undocumented public functions) before the
// blocking one the templates half adds last, so a report that printed
// findings in the order they arrived would truncate the block away.
// proved by: replacing `shown := orderForDisplay(findings)` with
// `shown := findings` — the drifted-mirror BLOCK fell past the cap and the
// test failed.
func TestTheCapNeverHidesABlockingFinding(t *testing.T) {
	root := gitRepo(t)
	var body strings.Builder
	for i := range 20 {
		fmt.Fprintf(&body, "def handler%d():\n    return %d\n\n", i, i)
	}
	write(t, root, "app.py", body.String())
	// the drifted PR-template mirror: a blocking finding the templates half
	// appends after every one of those info lines
	write(t, root, ".procoder/github/PULL_REQUEST_TEMPLATE.md", "## What\n")
	write(t, root, ".github/PULL_REQUEST_TEMPLATE.md", "## Something else\n")

	out, lines := collect()
	Run(root, out)

	sec := section(*lines, "hygiene")
	if headerTotal(t, sec[0]) <= maxPerSection {
		t.Fatalf("the fixture must overflow the cap: %s", sec[0])
	}
	if !strings.Contains(strings.Join(sec, "\n"), "out of sync with") {
		t.Fatalf("the blocking mirror finding was truncated away:\n%s", strings.Join(sec, "\n"))
	}
	if !strings.HasPrefix(sec[1], "  BLOCK") {
		t.Fatalf("the block is what a reader must see first:\n%s", strings.Join(sec, "\n"))
	}
}

// Decision records are audited where a repository keeps them, and the
// section is simply absent where it does not — an empty "0 findings"
// heading would tell a repo without ADRs that it was judged on them.
// proved by: removing the os.Stat guard around the adr section — the
// repository with no .procoder/adr grew a decision-records heading and the
// test failed.
func TestDecisionRecordsSectionOnlyAppearsWhereTheRepoKeepsThem(t *testing.T) {
	bare := gitRepo(t)
	write(t, bare, "notes.txt", "hello\n")
	out, lines := collect()
	Run(bare, out)
	if headerIndex(*lines, "## decision records") >= 0 {
		t.Fatalf("a repo without ADRs is not judged on them:\n%s", strings.Join(*lines, "\n"))
	}

	kept := gitRepo(t)
	write(t, kept, "notes.txt", "hello\n")
	// a record that supersedes one that does not exist: rot like any other
	write(t, kept, filepath.Join(adr.Dir, "0001-choose-a-queue.md"),
		"# 1. Choose a queue\n\nDate: 2026-01-02\n\nStatus: Accepted\n\nSupersedes: 0009\n\n## Context\n\nWe need one.\n\n## Decision\n\nUse one.\n\n## Consequences\n\nIt runs.\n")
	out2, lines2 := collect()
	Run(kept, out2)
	sec := section(*lines2, "decision records")
	if len(sec) == 0 {
		t.Fatalf("a repo with ADRs must be audited on them:\n%s", strings.Join(*lines2, "\n"))
	}
	if headerTotal(t, sec[0]) == 0 {
		t.Fatalf("a dangling supersede is a finding: %s", sec[0])
	}
}

// Nothing blocking means the repository would pass the gate, and the audit
// says so and exits 0 — an audit that could only ever fail would be a
// ritual, not a verdict.
// proved by: returning 1 from the final branch of Run — the clean fixture
// started failing its own audit and the test failed.
func TestACleanRepositoryPassesItsOwnAudit(t *testing.T) {
	if tools.Resolve(security.Gitleaks, "") == "" {
		t.Skip("gitleaks is not installed — the secrets section is honestly NOT checked, which blocks")
	}
	root := gitRepo(t)
	// no formatter covers a .txt, no linter claims it, and it carries no
	// secret: every section has an answer and none of them blocks
	write(t, root, "notes.txt", "hello\n")

	out, lines := collect()
	code := Run(root, out)

	if code != 0 {
		t.Fatalf("nothing blocking means exit 0, got %d:\n%s", code, strings.Join(*lines, "\n"))
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "  blocking findings total: 0") ||
		!strings.Contains(joined, "would pass the procoder gate") {
		t.Fatalf("a clean audit must say so:\n%s", joined)
	}
	if strings.Contains(joined, "BLOCK") {
		t.Fatalf("a clean audit prints no blocks:\n%s", joined)
	}
}
