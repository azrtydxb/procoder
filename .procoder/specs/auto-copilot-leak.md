# auto-copilot-leak

Status: draft

## Problem

The lessons loop exists but is incomplete: a real escape (a bug found by
Copilot in a Copilot review, by a human reviewer, in production) must be
manually written into `.procoder/github/LESSONS.md` by the agent during the
merge reflection step. If the agent forgets, skips, or glosses over a real
finding, the class of bug can repeat.

A specific gap is Copilot's auto-generated review issues. When Copilot
reviews a PR and automatically creates a GitHub issue (when the `auto-copilot`
label is present or the issue is opened by `copilot[bot]`), its findings
often surface defects that none of procoder's gates caught. The agent reads
the Copilot comment/issue, fixes the bug, and sometimes never records what
class of failure let it through — and therefore never adapts REVIEW.md
or any other layer to prevent the next occurrence.

Another gap is privacy: when the findings from a Copilot review get
captured in a GitHub issue or lesson, user code must never appear in the
issue body or lesson entry. What escapes must be metadata about the
failure, not the user's source.

## Users

- Pascal, who runs `procoder copilot-leak` (or a periodic hook) and wants a
  one-line prompt asking whether Copilot flagged a review since last check,
  and if so, an issue created automatically — with zero user code exposure.
- The agent mid-worktree, which wants to know `did a Copilot review escape`
  without manually scanning GitHub, and wants the adaptation wired in one
  step.

## In scope

### 1. New command: `procoder copilot-leak` [S-1]

A top-level command that:

- Reads the repo's default branch from `git remote` (uses existing
  `gitx.DefaultBranch` logic).
- Queries GitHub (via `gh`) for issues opened or updated in the last N
  hours (default 24) that came from Copilot's auto-review path:
  issues where the `auto-copilot` label exists, or the author is
  `copilot[bot]`, or the issue body contains a `---\n> **Copilot**` quote
  block (the Copilot review annotation format).
- Queries the same window for **pull request review comments**, through
  `gh api repos/{owner}/{repo}/pulls/comments?since=…`, keeping those from
  a Bot account whose login begins with `copilot`.

  Added after the command shipped and reported "no findings" while four
  real defects sat in a Copilot review of the branch that was extending
  it. Copilot's auto-review does not open issues on this repository or
  most others — it comments inline on the pull request — so the issue
  query alone could not see the thing the command is named for.

  The author test is NOT the issue test: measured against a real review,
  the bot posts review comments as `{"login":"Copilot","type":"Bot"}`,
  with no `[bot]` suffix, so the issue pattern matches none of them. Both
  halves of the new test are required — `type == "Bot"` alone takes every
  bot in the repository, and the login alone takes a person who chose the
  name.

- Either query failing means the command reports NOT checked. A count
  built from one source while the other went unread is the silent green
  this domain exists to catch.
- For each matching issue, extracts the finding text (the body, stripped
  of Copilot's header).
- Asks the user (stdout) a single prompt:
  ```
  copilot-leak: N finding(s) from Copilot auto-reviews since <time>.
  Create anonymised issue(s) under the `auto-copilot` label and record
  lesson(s)? [y/N]
  ```
- On yes:
  - For each finding, sanitises the text: strips file contents, replaces
    concrete file paths under the user's project with `src/<path>`,
    removes any variable values or secrets, and keeps only the defect
    description, file:line position (if any), and the suggested fix.
  - Creates a GitHub issue using `gh issue create` with:
    - Title: one-line summary derived from the finding
    - Label: `auto-copilot`
    - Body: the sanitised finding, plus a link back to the original
      Copilot issue (the one Copilot created), plus the timestamp.
  - Creates a lessons ledger entry (appends to the copilot-leak ledger
    beside `.procoder/github/LESSONS.md` — a dedicated tracking file if
    the open decision names one).
  - Prints a summary: how many issues created, how many lessons recorded.

### 2. Sanitisation [S-2]

A new function `sanitiseFinding(body string, root string) string` that:

- Removes or replaces raw code blocks (fenced code blocks in markdown).
- Redacts lines matching secret-like patterns (GitHub tokens, API keys,
  passwords, `ghp_`, `gho_`, `sk-`, `AIza`).
- Normalises file paths: `<root>` becomes `.` (relative to project).
- Preserves: the defect description, the file:line where the bug was,
  the action Copilot suggested (if present).
- Never includes: full file diffs, user's commit messages, email
  addresses, usernames (beyond `copilot[bot]` which is safe).

### 3. New file: `.procoder/github/COPILOT-LEAKS.md` [S-3]

A scratch ledger (not the formal LESSONS.md) that holds one entry per
captured escape. Each entry carries:

- Date + time
- Original Copilot issue URL (for traceability)
- Sanitised finding text
- Whether the lesson was learned (adaptation recorded)
- Whether a GitHub issue was created

This file is separate from LESSONS.md so the `procoder lessons` check
(which flags entries with `<`-prefixed adaptations) does not block on
raw Copilot notes that haven't been converted yet.

### 4. Integration: `procoder lessons --from-copilot` [S-4]

An optional flag on `procoder lessons` that lists the lessons learned
from Copilot reviews — entries in COPILOT-LEAKS.md that still have
placeholder adaptations (`<...>`) but have a matching GitHub issue that
can be used as context.

### 5. Hook integration: session start / merge [S-5]

The merge flow (`commands/merge.md`, section 2b) gains a step: at the
end of the reflection phase, if `copilot-leak` hasn't been run today,
it runs automatically (non-blocking: if no findings, silence; if
findings exist, prompt).

The session start hook can optionally run `copilot-leak --quiet` to
accumulate findings without prompting during development.

### 6. GitHub issue template [S-6]

A new file under `.github/ISSUE_TEMPLATE/copilot-leak.md`: a template
used when creating issues, carrying the `auto-copilot` label by default.

```markdown
## What escaped our gates

<!-- Sanitised Copilot review finding. No user code. -->

## File / line

<!-- Where the bug was, in the user's repo. -->

## What Copilot suggested

<!-- The fix or action proposed by Copilot. -->

## What should have caught this

<!-- One of: linter | rubric | controller | test | ci | nothing -->

## Adaptation needed

<!-- TODO: record the adaptation that closes this class in 48 hours -->
```

## Out of scope

- Auto-creating the Copilot auto-review issue (we do not create the
  Copilot issue; we read what's already there).
- Polling / continuous monitoring in the background (that is Part B).
- Sending the adaptation back to user repos (we don't manage external
  repos).
- Webhooks or HTTP listeners (no server required).
- Copilot Code's review thread parsing (only auto-issue path for now).
- Multi-repo: only the current repo.

## Constraints

- No direct GitHub API calls — all interaction via `gh` CLI (same as
  existing CI, docs, and security domains).
- No secrets in issue bodies or ledger files. Sanitisation is mandatory,
  not optional.
- `copilot-leak` never writes user code: it writes only to
  `.procoder/github/COPILOT-LEAKS.md` and opens issues.
- Pure Go stdlib for sanitisation. No external deps.
- The command is non-blocking when no findings exist (exit 0).
- The prompt uses stdin; if stdin is not a terminal, the default is
  N (silent pass).
- Every output uses forward slashes, on every platform.

## Interfaces

- `procoder copilot-leak [--since <duration>] [--quiet]`
  - `--since`: how far back to look (default `24h`; accepts the
    standard duration forms such as `6h`, plus plain day values like
    `2d` — the parser extends the standard Go one, so a `2d` value
    will not parse in stock Go code).
  - `--quiet`: do not prompt; just report findings count and exit 0.
  - Exit 0: no findings, or user declined, or --quiet.
  - Exit 2: stdin is a terminal and user declined (reporting exit, not an error).

- `internal/copilot` (new package):
  - `type Finding struct {OriginalURL string; Title string; Body string; Line int; Repo string; Created time.Time}`
  - `type Sanitised struct {Title string; Body string; Line int; OriginalURL string; Created time.Time}`
  - `func Find(root string, since time.Duration, out func(string)) []Finding`
    Queries `gh issue list` + `gh issue view` for matching issues.
  - `func Sanitise(f Finding, root string) Sanitised`
    Sanitises a finding for safe display.
  - `func Capture(finds []Sanitised, root string) (int, int, []string)`
    Creates GitHub issues (one per sanitised finding) and appends entries
    to COPILOT-LEAKS.md. Returns (issuesCreated, lessonsWritten, pathsChanged).
  - `func Prompt() bool`
    Prompts the user on stdin. Returns false if stdin is not a terminal.

- `internal/lessons` (extension):
  - New constant `CopilotLeaksPath = ".procoder/github/COPILOT-LEAKS.md"`
  - Function `RecordCopilotEntry(path string, sanitize Sanitised) error`
    Appends an unlearned entry to COPILOT-LEAKS.md.
  - Extension to `Run()` or a new `RunFromCopilot()` that processes entries
    from COPILOT-LEAKS.md and converts them into LESSONS.md entries.

- `internal/gate/gate.go` (extension to `RunWith`):
  - After collecting findings, call `copilot.FinalCheck(root)` which does
    a lightweight check for any new `auto-copilot` issues created in the
    last 5 minutes (freshly opened by Copilot just now). If any, appends
    an informational line. Non-blocking.

## Data

- `.procoder/github/COPILOT-LEAKS.md` — one entry per captured escape:

  ```markdown
  ## <date> <original-issue-url> — <one-line finding>

  - Source: Copilot auto-review
  - Original: <url>
  - Adaptation: <the concrete change that catches this class from now on>
  ```

  Shape mirrors LESSONS.md entries (same structure, different file).

- GitHub issues created with label `auto-copilot` and `copilot-leak`.

## Edge cases

- `gh` not installed: reports NOT checked, exits 2, does not block.
- No GitHub remote: exits 0 silently (nothing to query).
- `gh` auth required but not authenticated: exits 2, says to run
  `gh auth login`.
- Copilot issues that are not about code quality (e.g. feature requests,
  documentation suggestions) should be skipped — filter by body patterns.
- A user repo where Copilot's auto-review issue format differs (forked
  Copilot instance): the body-pattern matching is the best-effort filter.
  If no issues match the pattern, exit 0 silently.
- Sanitisation that over-strips: if a sanitised body is empty, skip
  issue creation for that finding and log a warning.
- Multiple findings in one Copilot issue: one GitHub issue per finding,
  but a single COPILOT-LEAKS.md entry referencing all findings.
- The user's project at a path with spaces: all paths normalised to
  `.` in sanitised output; the original path is only in metadata.
- No `gh issue create` — `gh` exists but `issue subcommand` is gated:
  the command should detect this, print a clear message, and exit 2.

## Failure modes

- `gh` output is malformed (new GitHub API response): NOT checked, exits 2.
- Network error during `gh issue list/view`: NOT checked, exits 2.
- Sanitisation fails (unrecognised markdown): raw body with a `NOTE:
sanitisation incomplete` prefix, and a warning.
- Issue creation fails (rate limit, auth): notes the failure, still
  writes to COPILOT-LEAKS.md so nothing is lost.
- COPILOT-LEAKS.md is unwritable: notes the failure, still opens
  GitHub issues.

## Acceptance criteria

- [x] [S-1] `copilot-leak` on a repo with no recent Copilot issues prints
      "copilot-leak: no findings since <24h>" and exits 0 —
      `TestNoMatchingIssuesIsAnAnswerNotAnUnknown` in `internal/copilot`
      asserts the empty window is an answer, not an unknown.
- [x] [S-1] `copilot-leak --since 6h` queries the last 6 hours only,
      verified in a fixture with recorded `gh` output by
      `TestTheWindowIsBothAskedForAndEnforced` in `internal/copilot`.
- [x] [S-1] [S-3] On a fixture where a Copilot auto-review issue exists with the
      `auto-copilot` label, `copilot-leak` prints the finding count,
      prompts, and on yes creates one GitHub issue with label
      `auto-copilot` and an entry in the copilot-leak ledger —
      `TestCaptureOpensAnIssueAndRecordsAnUnlearnedEntry` in
      `internal/copilot` walks the whole path.
- [x] [S-2] Sanitised bodies never contain raw code (fenced code blocks
      stripped), secrets (redacted), or full file paths (replaced with
      `.`) — `TestAdversarialSanitise` in `internal/copilot` injects a
      secret literal and greps the output.
- [x] [S-1] When stdin is not a terminal, `copilot-leak` does not prompt
      (defaults to N, exits 2) —
      `TestPromptRefusesWhenStdinIsNotATerminal` in `internal/copilot`.
- [ ] [S-1] `--quiet` flag suppresses prompting and only reports counts,
      exits 0 (the flag is declared in `cmd/procoder/flags.go` and the
      quiet path in `cmd/procoder/main.go`); fails if a quiet run ever
      prompts.
- [x] [S-3] [S-4] COPILOT-LEAKS.md entries have the same structure as LESSONS.md
      entries, and `copilot-leak --from-copilot` (or integrated into
      `procoder lessons`) reports them —
      `TestWhatCaptureWritesTheLedgerReportReads` in `internal/copilot`
      plus the report tests in `internal/lessons`.
- [x] [S-1] Every path in output uses forward slashes — asserted inside
      `TestUnwritableLedgerStillCreatesTheIssues` in `internal/copilot`.
- [x] [S-1] Usage text includes `copilot-leak`; `commands/copilot-leak.md`
      exists; `copilot-leak.md` has an OpenCode twin under
      `.opencode/command/` — `TestOpenCodeCommandParity` in
      `internal/portability` keeps them in step; fails if the command
      gains a flag the twin does not carry.
- [x] [S-5] The merge flow in `commands/merge.md` runs the copilot-leak
      step during its reflection phase (silent when nothing is found),
      and the session-start hook may run it quiet; fails if the step
      disappears from the merge command.
- [x] [S-6] An issue template for the `auto-copilot` label exists under
      the repository's issue-template directory as
      `.github/ISSUE_TEMPLATE/copilot-leak.yml`; fails if the template
      is removed and a created issue loses its label.

## Open questions

- Q1: Should COPILOT-LEAKS.md be merged into LESSONS.md, or kept
  separate? Keeping it separate means two files to check; merging
  means the lessons check sees raw Copilot findings that need
  classification first.
  -- A: Keep separate. Raw Copilot notes need a human step to become
  a real lesson (classify the class: mechanical/judgment/taste,
  name the adaptation). Merging skips that step.
- Q2: How often should auto-copilot checks run? Manual (`copilot-leak`)
  or automatic (hook/post-tool-use/merge)?
  -- A: Manual by default, automatic in merge flow only. A periodic
  background poll is Part B if the user wants it.
- Q3: The `copilot[bot]` — should we also check for `copilot[bot]` as
  issue author? Some instances use `copilot-preview[bot]`.
  -- A: Check for authors matching `copilot.*\[bot\]` regex. Cover
  both variants.
