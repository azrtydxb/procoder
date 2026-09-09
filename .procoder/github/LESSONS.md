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
  anything other than five. The deeper miss is what was verified: the awk extraction was run
  against the real changelog, the upload was assumed — the part that
  could not be tested locally is the part that broke.

## 2026-08-20 PR#75 (Copilot) — the leak ledger report was written, tested, and reachable by nothing

- Class: judgment
- Missed by: maintain (dead-code tier)
- Adaptation: codeindex.Unused now counts test references separately and
  reports "referenced only by tests — surface wired nowhere" as its own
  tier, pinned by TestUnusedSeparatesTestOnlySurfaceFromLiveCode; REVIEW.md
  rubric line "a function whose only callers are its own tests is not wired";
  the flag itself is wired as `copilot-leak --from-copilot` and pinned
  against the usage text by TestCopilotLeakAcceptsEveryFlagTheUsageTextPromises

## 2026-08-20 PR#75 (Copilot) — one ledger file, two owners, two entry shapes

- Class: judgment
- Missed by: rubric
- Adaptation: REVIEW.md rubric line "one concept, one owner"; the reading
  package owns the path and the writer calls it, with
  TestWhatCaptureWritesTheLedgerReportReads carrying a captured finding
  across the package seam

## 2026-08-20 PR#75 (Copilot) — a label constant declared and then hardcoded beside itself

- Class: taste
- Missed by: rubric
- Adaptation: covered by the same "one concept, one owner" rubric line;
  createIssue passes AutoLabel and OwnLabel, and the second label exists
  because capture was re-capturing its own issues — pinned by
  TestOurOwnIssuesAreNeverCapturedAgain

## 2026-08-21 PR#82 (Copilot) — an inline code span opened and never closed

- Class: judgment
- Missed by: rubric
- Adaptation: docs.UnclosedSpans counts backticks per PARAGRAPH (not per
  line — CommonMark lets a span wrap, and per-line counting flagged 46
  correct spans in this tree) and reports a paragraph that opens one and
  never closes it, wired into docs.CheckFile and pinned by
  TestAnUnclosedSpanIsReported plus three false-positive guards. The
  REVIEW.md line "code spans unbroken" already existed and the pre-PR
  review applied it — the eye reads the intent and skips the missing
  character, so the rule needed counting rather than better wording.

## 2026-08-21 PR#82 (Copilot) — a hyphenated compound split across a line break in instruction text

- Class: taste
- Missed by: rubric
- Adaptation: reworded rather than rewrapped, and covered by the same
  paragraph-level reading the span check now applies; in agent-executed
  instruction text a stray hyphen is a plausible token rather than an
  obvious typo, which is why it survived a human-shaped review

## 2026-08-21 PR#134,#136 (CI, windows-latest) — a rooted literal used as a test repository root

- Class: portability
- Missed by: the local suite — macOS and Linux call `/repo` absolute, Windows does not
- Adaptation: `TestNoTestUsesARootedLiteralAsARepositoryRoot` in internal/audit reads the tree for `"/repo"`-shaped roots; `gitx.RepoRel` joins a non-absolute path onto the root, so the fixture was silently measuring its own path arithmetic rather than the function

## 2026-08-24 e2e-campaign (self) — "did the output contain X" read three absent tools as passes

- Class: mechanical
- Missed by: nothing — the campaign's own harness, which had no gate over it
- Adaptation: three separate checks written to hunt silent greens each
  produced one. The classifier claimed NOT RUN for any output containing
  "missing", so a finding about an absent PR template read as a check that
  never happened. The brew-formula check read `tools.go` by a path
  relative to a directory the script had already left, so grep matched
  nothing, the loop ran zero times, and it reported every formula valid.
  The catch test matched the planted file's name, so `UNCHECKED
cs/Sloppy.cs — csharpier is not installed` — which names the file and
  reports the opposite of a finding — counted as a catch.

  The shape is one shape: **a grep that finds nothing is indistinguishable
  from a grep that found nothing wrong.** Every place a verdict is derived
  from "does this text appear", the empty case has to be answered
  separately and first, and the pattern has to match the verdict rather
  than the subject — `unformatted  <file>`, not `<file>`.

  The over-correction is its own lesson: widening the absent-tool pattern
  to any "NOT ..." line naming the file turned Dart into a NOT RUN,
  because procoder separately reports "NOT linted — Dart: procoder has no
  linter for it yet" about a file whose formatter had caught the defect
  perfectly well. A false skip is as wrong as a false pass, and both were
  caught only by replaying the classifier over logs already on disk rather
  than trusting the second version.

## 2026-08-24 e2e-campaign (self) — a shell script edited while it was running re-executed part of itself

- Class: mechanical
- Missed by: nothing — the harness again, and the duplicate output nearly passed for a longer report
- Adaptation: bash reads a script incrementally from a byte offset rather
  than loading it whole, so rewriting the file underneath a running
  invocation moves what the next read returns. The docs pass came back
  with its P-CONTROL block executed twice and a pass count inflated by
  twenty-five, which looks exactly like a more thorough run. Nothing in
  the output says "this ran twice"; it was visible only because the
  section repeated verbatim and the total did not match what the script
  could produce.

  The rule: never edit a script that is currently executing — queue the
  edit, or copy the script and edit the copy. And when a harness reports a
  count, know what the maximum possible count is, because a number that is
  too HIGH is as much a defect as one that is too low and reads as better
  news.

## 2026-08-27 roadmap sweep (self) — "Closes #A, #B" closed only #A, twice

- Class: mechanical

- Adaptation: GitHub honours a closing keyword only before the number it
  immediately precedes. `Closes #194, #211, #209` closed #194 and left the
  other two open; `Closes #230, #231` closed #230 and left #231. Both read
  as correct in the commit message, and the issues sat open while every PR
  showed as merged — which is the worst shape for this failure, because the
  work IS done and nothing looks wrong until somebody lists open issues.

  The rule: repeat the keyword per issue — `Closes #194, closes #211,
closes #209` — and after merging a PR that claimed to close more than
  one, check the issues rather than the PR.

## 2026-08-27 roadmap sweep (self) — a timeout read as a cancellation, and I blamed my own pushes

- Class: judgment

- Adaptation: the gate job reported CANCELLED, and the step list showed the
  cancellation inside `procoder security --deep`. I concluded twice from
  that: first that my own `gh pr update-branch` pushes were killing the run
  through the concurrency group, then that semgrep was the expensive step.
  Both were wrong and both were checkable in one call.

  The timestamps said 10m09s against `timeout-minutes: 10` — a timeout,
  which GitHub surfaces as `cancelled`. And per-step timings said semgrep
  took 6 seconds while `gate over the tracked tree` took 6m47s: the
  cancelled step is simply the one that was RUNNING when the clock
  expired, which says nothing about what consumed the time.

  The rule: a cancelled job is a timeout until the timestamps say
  otherwise, and the step that shows as cancelled is the last one started,
  never by itself the slow one. Read per-step durations before naming a
  cause. This ledger already carried the same failure from 2026-08-19 —
  "gate job hung 10 minutes ... cancelled by its own timeout" — and the
  budget was never raised, so it recurred. A lesson recorded without the
  fix applied is a lesson that will be learned again.

## 2026-08-27 roadmap sweep (self) — a local speedup asserted as a CI speedup

- Class: judgment

- Adaptation: parallelising the tracked-tree format check cut 257.5s to
  153.0s on a ten-core laptop, and I told the maintainer it would take the
  CI job "from 9-plus minutes to 4-5". Then I measured: that step ran 508s,
  407s and 402s on recent main runs and 429s with the change — inside the
  existing spread, no improvement at all. The runners have far fewer cores
  and the CI bottleneck is not per-file process startup.

  The rule: a benchmark measures the machine it ran on. Before claiming an
  improvement somewhere else, measure it there, and get the baseline's
  SPREAD rather than one sample — a single 6m47s reading looked like a
  fixed baseline when the real range was 402-508s.

## 2026-08-30 state seam (self) — a lock that worked everywhere except the one platform nobody ran

- Class: mechanical

- Adaptation: `internal/store`'s lock treated any create error that was not
  `os.IsExist` as fatal. On Windows a file pending deletion reports "access
  is denied" instead, so ordinary contention became a hard failure and
  `TestConcurrentAppendsBothSurvive` lost five of twenty appends — the exact
  defect the package exists to prevent, on the platform it had never run
  on. A second one beside it: release removed the lock while the heartbeat
  could still have it open in `os.Chtimes`, which Windows refuses, so the
  lock survived and the next caller paid the whole timeout.

  What makes this worth a ledger entry is everything that did NOT catch it.
  Two fresh-context review passes read the whole diff. A 245-invocation
  equivalence sweep compared every command, every write path and nineteen
  broken-input scenarios against the pre-change binary. `go test -race`
  passed. All of it ran on macOS, and none of it could have found this.

  The rule: filesystem semantics are platform behaviour, not Go behaviour,
  and no amount of local evidence substitutes for the leg that runs
  elsewhere. REVIEW.md now names the four shapes — `os.IsExist` alone,
  removing a file another handle may hold, `os.Chmod` on a directory, and
  native-versus-slash path comparisons — so the rubric asks the question
  before CI has to.

## 2026-08-30 state seam (self) — committed goldens failed on line endings nobody wrote

- Class: mechanical

- Adaptation: the parity goldens are compared byte for byte against output
  procoder generates at runtime, and procoder emits LF everywhere. A
  Windows checkout converted the committed files to CRLF, so all four
  golden comparisons failed on a difference no change had made. Fixed with
  `.gitattributes` pinning them, rather than by teaching the comparison to
  normalise — a comparison taught to ignore a difference stops being a
  byte-for-byte comparison.

  The rule: any committed fixture compared byte for byte against generated
  output needs its line endings pinned. REVIEW.md now says so.

## 2026-08-30 state seam (self) — the same check-then-act race, twice in one branch

- Class: mechanical

- Adaptation: the pre-PR review found that judging a lock stale and removing
  it were two unbound operations, so two callers could both break one lock
  and both hold it. The fix serialised breaking behind an `O_EXCL` file. The
  VERIFICATION pass then found the identical shape reintroduced on that
  break file — a stat, then a remove, with nothing binding them — one commit
  after the first was fixed.

  Fixing an instance does not close a class. `removeIfSame` is now the one
  way this package removes anything it did not just create, and REVIEW.md
  carries check-then-act on a shared file as its own rubric line, because
  reading for "is this race fixed" is a different question from reading for
  "does this shape appear anywhere".

## 2026-09-02 hook delivery budget (self) — the fixes for a review are the code nobody reviews

- Class: judgment

- Adaptation: the pre-PR review of #266 found that a formatted body could
  starve the secret scan. The fix was a must-keep path, and that path
  shipped with two defects of its own: keep parts were placed against the
  _full_ budget rather than the reserved one, so the omission notice could
  push the payload past the constant it existed to enforce; and a keep part
  larger than the whole budget was dropped without being counted, so the one
  finding declared un-droppable vanished silently. Copilot found both on the
  PR.

  Neither is subtle. They are in eleven lines written in direct response to
  a review finding, on the path that finding said was the important one —
  and nothing read them, because the reviewer had already run. A review is a
  snapshot of the diff as it was; everything written to satisfy it is newer
  than the review.

  The same session had already shown the fix. During #249 a verification
  pass over the fix diff caught a race reintroduced _one commit_ after the
  original was fixed. That pass was not run here.

  The rule: an exception path added to satisfy a finding — a must-keep, a
  bypass, a special case — is the least-reviewed code in the change. Read
  those hunks last and hardest, or re-run the reviewer over the fix diff.
  REVIEW.md now carries it, alongside the budget-excludes-its-own-overhead
  shape that was the concrete form it took here.

