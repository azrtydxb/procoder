package main

import (
	"strings"
	"testing"

	"procoder/internal/docs"
	"procoder/internal/gitx"
)

// The usage text and the docs-coverage command list are pinned to each
// other: a command added to one without the other fails here, so the
// documentation completeness check can never silently drift from reality.
func TestUsageAndCoverageListAgree(t *testing.T) {
	for _, cmd := range docs.Commands {
		if !strings.Contains(usage, "\n  "+cmd) {
			t.Errorf("usage text does not list %q", cmd)
		}
	}
	// and the reverse: a command added to usage must join the coverage list
	listed := map[string]bool{}
	for _, cmd := range docs.Commands {
		listed[cmd] = true
	}
	for _, line := range strings.Split(usage, "\n") {
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if !listed[fields[0]] {
			t.Errorf("usage lists %q but docs.Commands does not — CommandCoverage would miss it", fields[0])
		}
	}
}

// Four commands — ci, infra, security, lint — printed their findings
// with four copies of the same loop, which is how three of them drift
// while the fourth is fixed. One printer, one behaviour.
func TestPrintFindingsRendersLocationMarkAndCount(t *testing.T) {
	root := "/repo"
	var lines []string
	code := printFindings(root, "demo", []gitx.Finding{
		{File: "/repo/a.go", Line: 12, Message: "a thing"},
		{File: "/repo/b.go", Message: "a whole-file thing", Blocking: true},
		{Message: "no file at all"},
	}, func(s string) { lines = append(lines, s) })

	want := []string{
		"  info  a.go:12  a thing",
		"  BLOCK b.go  a whole-file thing",
		"  info  no file at all",
		"procoder demo: 3 finding(s) (1 blocking)",
	}
	if len(lines) != len(want) {
		t.Fatalf("want %d lines, got %d:\n%s", len(want), len(lines), strings.Join(lines, "\n"))
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d:\n got %q\nwant %q", i, lines[i], w)
		}
	}
	if code != 1 {
		t.Errorf("a blocking finding must exit 1, got %d", code)
	}
}

// proved by: returned 1 unconditionally — information-only reports then
// fail the caller's build over findings that are explicitly judgment.
func TestPrintFindingsWithoutBlockingExitsZero(t *testing.T) {
	var lines []string
	code := printFindings("/repo", "demo", []gitx.Finding{{File: "/repo/a.go", Message: "just so you know"}},
		func(s string) { lines = append(lines, s) })
	if code != 0 {
		t.Fatalf("information is not failure, got exit %d\n%s", code, strings.Join(lines, "\n"))
	}
}

// A path outside the repository stays as it is: relativising it would
// produce ../../.. noise that helps nobody locate anything.
// proved by: dropped the HasPrefix("..") guard — the finding then reads
// as a long climb out of the repository.
func TestPrintFindingsLeavesOutsidePathsAlone(t *testing.T) {
	var lines []string
	printFindings("/repo", "demo", []gitx.Finding{{File: "/elsewhere/x.go", Line: 3, Message: "m"}},
		func(s string) { lines = append(lines, s) })
	// exact, not Contains: "../elsewhere/x.go:3" contains the absolute form
	// as a substring, so a Contains assertion here proves nothing
	if want := "  info  /elsewhere/x.go:3  m"; lines[0] != want {
		t.Errorf("an outside path is printed as given:\n got %q\nwant %q", lines[0], want)
	}
}
