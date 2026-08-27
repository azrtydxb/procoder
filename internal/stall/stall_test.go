package stall

import (
	"strings"
	"testing"
)

const story = `# A story

Status: open
Created: 2026-08-27
Epic: something

## Description

The work to do.

## Acceptance criteria

- [ ] the first thing
- [ ] the second thing

## Evidence

<!-- Filled at close time. -->
`

// Cosmetic edits are not progress. A file whose timestamp moved, whose
// prose was reworded, whose list was reordered is a file that changed and
// work that did not — and a report counting those as movement is what
// makes a stalled story look busy.
//
// proved by: Semantic made to hash the whole text — every one of these
// produces a different hash and the distinction disappears.
func TestCosmeticEditsDoNotChangeTheMeaning(t *testing.T) {
	base := Semantic(story)
	for name, edited := range map[string]string{
		"a new timestamp":  strings.Replace(story, "Created: 2026-08-27", "Created: 2026-09-01", 1),
		"reworded prose":   strings.Replace(story, "The work to do.", "The work that needs doing, restated at length.", 1),
		"extra whitespace": strings.Replace(story, "## Description", "## Description\n\n", 1),
		"a reordered epic": strings.Replace(story, "Epic: something", "Epic: something-else", 1),
	} {
		if got := Semantic(edited); got != base {
			t.Errorf("%s changed the semantic hash — cosmetic edits read as progress", name)
		}
	}
}

// And the three things that ARE progress must each move it, or the report
// says nothing ever advances.
//
// proved by: each of the three parts dropped from Semantic in turn — the
// change it represents stops registering.
func TestRealProgressChangesTheMeaning(t *testing.T) {
	base := Semantic(story)
	for name, edited := range map[string]string{
		"a criterion checked": strings.Replace(story, "- [ ] the first thing", "- [x] the first thing", 1),
		"the status moved":    strings.Replace(story, "Status: open", "Status: done", 1),
		"evidence written":    strings.Replace(story, "<!-- Filled at close time. -->", "Proved by TestSomething; the suite is green.", 1),
	} {
		if got := Semantic(edited); got == base {
			t.Errorf("%s did not change the semantic hash — real progress reads as a stall", name)
		}
	}
}

// The template's own comment is not evidence. A story that has never been
// touched would otherwise look like one carrying evidence from the moment
// it was seeded.
//
// proved by: the comment-stripping loop removed from hasEvidence — the
// untouched template reports as having evidence.
func TestTheTemplateCommentIsNotEvidence(t *testing.T) {
	if hasEvidence(story) {
		t.Fatal("a story whose Evidence section holds only its template comment reports as having evidence")
	}
	written := strings.Replace(story, "<!-- Filled at close time. -->", "The suite is green.", 1)
	if !hasEvidence(written) {
		t.Fatal("real evidence was not recognised")
	}
}

// A file git cannot answer for is not evidence of progress OR of a stall.
// Reporting it as unstalled would be the quiet half of the same mistake.
//
// proved by: the error branch in Check changed to `continue` — the
// unreadable file vanishes from the report entirely.
func TestAFileGitCannotAnswerForIsReported(t *testing.T) {
	var lines []string
	// A directory with no git repository at all: every log fails.
	Check(t.TempDir(), []string{"no/such/file.md"}, 3, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "NOT checked") {
		t.Fatalf("a file git could not answer for was passed over silently:\n%s", joined)
	}
}
