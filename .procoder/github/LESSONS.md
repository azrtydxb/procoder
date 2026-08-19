# Lessons — findings that escaped our own gates

One entry per finding caught downstream (bot review, human review,
production) — the escape is the bug; the finding is its symptom. Every
entry names which layer should have caught it and the adaptation that now
does. `procoder lessons` flags entries with no adaptation.

## 2026-08-19 PR#17 (Copilot) — path traversal via user-supplied task/spec names

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "user-supplied strings reaching a path"; todo.File/spec/plan validate plain basenames, pinned by traversal tests

## 2026-08-19 PR#17 (Copilot) — todo list silently skipped unreadable task files

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "any unreadable input silently skipped"; List surfaces unreadable tasks as findings

## 2026-08-19 PR#17 (Copilot) — time.Now called twice across the id/Created boundary

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "state computed twice that must agree"

## 2026-08-19 PR#17 (Copilot) — changelog code span split across lines

- Class: taste
- Missed by: rubric
- Adaptation: REVIEW.md rubric prose line "code spans unbroken"

## 2026-08-19 PR#18 (Copilot) — block-comment terminator leaked into debt ledger text

- Class: mechanical
- Missed by: test
- Adaptation: TestScanTrimsBlockCommentTerminators pins both */ and --> trims

## 2026-08-19 PR#18 (Copilot) — regex compiled once per task in plan check

- Class: mechanical
- Missed by: linter
- Adaptation: golangci baseline enables gocritic/staticcheck full set (SA6000 class); filesRe hoisted to package level

## 2026-08-19 PR#18 (CI) — gate job hung 10 minutes on the azure apt mirror, cancelled by its own timeout

- Class: mechanical
- Missed by: ci
- Adaptation: ci.yml repoints apt at archive.ubuntu.com with retries before any apt-get, so a dead Azure mirror flakes fast instead of eating the timeout

## 2026-08-19 self-scan (procoder debt) — our own test fixtures carried literal debt markers

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "test fixtures that trip our own scanners"; debt_test assembles the marker at runtime, same as the gitleaks fixture lesson
