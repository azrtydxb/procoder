// bug.go is defect intake: a bug IS a story with a type — same directory,
// same lifecycle, same controller — plus the two things a defect must
// carry that a feature need not: a severity, and the non-negotiable
// regression-test criterion. Creation prints, like every creation here
// (P-CONTROL).

package backlog

import (
	"fmt"
	"time"

	"procoder/internal/textutil"
)

// bugTemplate is the story template with the defect headers after
// Sprint: - and a description that asks for what a triage actually needs:
// how to see the failure, and what right would have looked like.
const bugTemplate = `# %s

Status: open
Created: %s
Epic: %s
Sprint: -
Type: bug
Severity: %s

## Description

<!-- Reproduction steps: the shortest path from a clean state to the
     failure — commands, inputs, environment. -->

<!-- Observed vs expected: what actually happens, and what should
     happen instead. -->

## Acceptance criteria

<!-- Each criterion is testable. The first is non-negotiable — a bug is
     only fixed when a test would catch its return. -->

- [ ] a regression test pins the fix: red before the change, green after

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
`

// validSeverity recognises the four severities a bug may carry.
func validSeverity(s string) bool {
	switch s {
	case "s1", "s2", "s3", "s4":
		return true
	}
	return false
}

// Bug prints a bug story for the agent to write. Severity defaults to s3
// — most defects are ordinary — and anything outside s1..s4 is refused,
// never guessed. A bug may predate any epic, so --epic is optional; the
// header still writes Epic: - to keep the story shape uniform, and the
// board reads - as no-epic, not a broken link.
func Bug(root, title, epic, severity string, out func(string)) int {
	if severity == "" {
		severity = "s3"
	}
	if !validSeverity(severity) {
		out("invalid severity " + severity + " — pass one of s1|s2|s3|s4")
		return 2
	}
	if epic == "" {
		epic = "-"
	}
	slug := textutil.Slug(title)
	if slug != "" {
		slug = time.Now().UTC().Format("20060102") + "-" + slug
	}
	return printItem(root, KindStory, slug, title, func(now string) string {
		return fmt.Sprintf(bugTemplate, title, now, epic, severity)
	}, out)
}
