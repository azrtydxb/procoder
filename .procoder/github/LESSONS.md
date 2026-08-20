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
- Adaptation: ci.yml replaces /etc/apt/apt-mirrors.txt (the runner's real mirror source — rewriting sources files did nothing, proven by a second identical hang) with the canonical archive plus fail-fast retries

## 2026-08-19 PR#19 (Copilot) — temp-config Close error swallowed, baseline claimed applied

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric extended — "a failed Close/flush after a write IS a failed write"; lintGo folds the Close error into the honesty path

## 2026-08-19 PR#19 (Copilot) — docs claimed only .golangci.yml wins; heading dropped the file's directory

- Class: taste
- Missed by: rubric
- Adaptation: REVIEW.md rubric prose line sharpened — "names and paths in docs match the code exactly (every variant, full paths)"

## 2026-08-19 Pascal — eleven releases shipped against a README still describing release one

- Class: judgment
- Missed by: rubric
- Adaptation: three layers — `## README must mention` families in the docs rules (blocking, mechanical floor), the mandatory docs-impact question in /procoder:pr (judgment ceiling), and the product-story line in this rubric; plus the full README/site rewrite

## 2026-08-19 user review — docs written in diary voice (personal names, project anecdotes)

- Class: taste
- Missed by: rubric
- Adaptation: docs-voice standard written into .procoder/docs/RULES.md Guidance and a rubric line in REVIEW.md; all pages swept to professional product voice

## 2026-08-19 self-scan (procoder debt) — our own test fixtures carried literal debt markers

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "test fixtures that trip our own scanners"; debt_test assembles the marker at runtime, same as the gitleaks fixture lesson

## 2026-08-19 PR#33 (CI windows leg) — printed paths and test assertions used filepath.Join

- Class: mechanical
- Missed by: rubric (and twice: the first fix caught the prints, not the
  seed printout or the assertions — a class is not closed by fixing
  instances)
- Adaptation: REVIEW.md rubric line "paths in OUTPUT built with
  filepath.Join"; every backlog print site is ToSlash, pinned by the
  package tests running on the Windows CI leg

## 2026-08-19 PR#34 (CI windows leg) — seeded story filenames exceeded Windows' path limit

- Class: mechanical
- Missed by: test (and by me: the first failure was misdiagnosed as a
  checkout flake and rerun instead of read — an 11-second failure is a
  checkout failure, and checkout failures have causes)
- Adaptation: slugify caps at 60 characters at a word boundary in every
  domain that names files from titles (backlog, todo, adr), pinned by
  TestSlugifyCapsLongTitles in each; existing over-long story files
  renamed with references updated

## 2026-08-19 PR#36 (CI, all three platforms) — a test assumed the machine's toolchain

- Class: mechanical
- Missed by: test
- Adaptation: a "clean gate" fixture can only be honestly clean with
  NOTHING changed — with changed files present, a missing formatter or
  scanner is UNCHECKED, and unchecked is never clean (the product
  working correctly). TestCleanGateLetsTheCommitThrough now commits its
  fixture first, and the whole hook suite is verified under a stripped
  PATH that mimics the CI test leg

## 2026-08-19 sprint 004 (live verification) — a new walk kept its own ignore list

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "a file-discovery walk that keeps its
  own skip list instead of asking git what it ignores"; envsync now calls
  git ls-files --ignored and TestGitIgnoredTreesAreNotSurveyed pins it

## 2026-08-20 perf pass — the docs obligation could fire unclearably

- Class: judgment
- Missed by: test
- Adaptation: procoder's own store (.procoder/) was excluded from CLEARING
  an obligation but not from RAISING one, so a bug story naming the file
  it fixes demanded documentation no edit to that story could supply.
  Exclusion is symmetric now, pinned by
  TestStateMarkdownRaisesNoObligationItCannotClear

## 2026-08-20 perf pass — a benchmark that measured an early return

- Class: mechanical
- Missed by: test
- Adaptation: Drift skips changed files that do not exist, so a fixture
  that never created them timed a nil return at 2µs and looked fast.
  Every benchmark here now asserts the work happened before timing it
  (the guard in BenchmarkDriftOverATypicalCorpus)

## 2026-08-20 audit — a whole-tree sweep asked diff-scoped questions

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "a check whose question is about a
  CHANGE, reused over a whole-tree sweep"; docs.CollectSweep drops drift
  and the obligation for audit, pinned by
  TestSweepDropsTheDiffScopedDocumentationQuestions

## 2026-08-20 audit — a swallowed walk error made "could not look" read as "nothing there"

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "a directory walk that swallows the
  ROOT's error"; infra, docs and maintain distinguish an unreadable root
  (no survey) from a skippable subdirectory, pinned by
  TestUnwalkableTreeIsNotSurveyedRatherThanEmpty

## 2026-08-20 tdd sweep — the debt ledger cried rot over sound debt

- Class: mechanical
- Missed by: test
- Adaptation: a marker's revisit condition usually lands on a
  continuation line; debt.scanFile judges the whole comment block now,
  pinned by TestTriggerOnAContinuationLineCounts plus a guard test that
  a marker with no condition anywhere is still flagged

## 2026-08-20 tdd sweep — a walk-root fix shipped incomplete, twice

- Class: judgment
- Missed by: test
- Adaptation: the 0.32.3 root-error distinction was applied to infra and
  docs but not to maintain's predicate, so its recorded walk error was
  always nil. Writing the test for the behaviour is what found it — the
  rubric line already exists; the adaptation is that a fix applied to
  "the places that do X" now gets a test per place, not per class

## 2026-08-20 0.32.9 (docs) — a Markdown table shipped with no header row

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "rendered output was LOOKED AT"; the
  generated skills table now carries its header, and the rule that
  produced the escape — a green `mkdocs build --strict` was treated as
  proof the page reads correctly — is named in the rubric line itself

## 2026-08-20 0.32.9 (docs) — a link's anchor matched no heading, and --strict stayed green

- Class: mechanical
- Missed by: gate
- Adaptation: `procoder docs` now resolves the anchor as well as the
  file, reproducing Python-Markdown's toc slug (explicit `{#id}` and raw
  HTML ids count; a target that cannot be read yields no verdict rather
  than a false positive). Six mutations proved, all killed; the exact
  link that shipped is caught, and 218 Markdown files produce no false
  positive

## 2026-08-20 0.32.10 (docs) — an element reported hidden was still painted

- Class: judgment
- Missed by: rubric
- Adaptation: same rubric line. The filter set `hidden` and the scripted
  assertion read `hidden === true`, so the check passed while Material's
  `display` rule kept the table header on screen. An assertion about the
  DOM is not an assertion about the render

## 2026-08-20 PR#52 (Copilot) — an empty-notes guard tested file size, not content

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "guards test the property they claim
  to test". The release job refused notes with `[ ! -s ]`, which is size
  — a changelog section of blank lines is a non-empty file and would
  have published a Release saying nothing, which is exactly what the
  guard and the comment beside it promised to prevent. Now
  `grep -q '[^[:space:]]'`, proved both ways before pushing

## 2026-08-20 v1.0.1 (CI) — release assets uploaded under one colliding name

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "a third-party tool's semantics come
  from its own help or docs". `gh release create path#label` was read as
  renaming the asset; it sets the display label, so all five binaries
  uploaded as `procoder` and the second collided. The binaries are staged
  under distinct names now, with a guard that counts them and refuses at
  four. The deeper miss is what was verified: the awk extraction was run
  against the real changelog, the upload was assumed — the part that
  could not be tested locally is the part that broke

