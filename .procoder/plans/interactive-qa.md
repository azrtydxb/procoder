# Plan: Interactive Q&A — `ask` Feature

## Goal

Ship `procoder ask`: the questions every domain already knows it cannot
answer, collected, put to a human, and recorded where the coder and the
gate can both read the decision. Done when a question reaches a person
instead of being guessed at, and the answer outlives the session.

## Context

Procoder's hooks inject plain text findings into the AI coder's context. When those findings contain questions (spec `OPEN:` markers, docs obligations, security flags, lint judgments), the AI coder guesses rather than asking the human. This feature introduces `procoder ask` — a structured Q&A flow that actually collects human answers and re-injects them as ground truth.

## Architecture

Create a new `internal/ask/ask` package that:

- Provides a `Collect(root string)` function returning questions from all domains
- Normalises heterogeneous question sources into a uniform `Question` struct
- Handles TTY detection (uses existing `copilot.Prompt` pattern)
- Writes questions to `.procoder/ask/QA.md` and answers to `.procoder/ask/answers.md`

The `ask` command lives at the top level (matching `check`, `lint`, etc.). It is NOT behind any config policy — when there are questions, they must be asked.

## Constraints

- Pure Go stdlib; package `internal/ask`.
- P-CONTROL: `ask` prints and records answers; it never edits a spec, a
  lint config or any file a domain owns.
- A flagged secret's value never reaches `QA.md`, `answers.md` or the
  terminal — the question is whether the flag is real, and the answer
  does not need the credential to be legible.
- Answers are keyed to a fingerprint of the question text (D-2), so an
  unchanged question is never asked twice and a changed one always is.
- No terminal is not a no: with no TTY, `ask` writes the file and names
  the `--file` route rather than asking a pipe.
- `[ask] policy = "report" | "block"` follows D-OVERRIDE like every other
  domain policy, defaulting to report.

## Tasks

## Task 1: Create `internal/ask/ask.go` — Question struct and collect framework

Files: `internal/ask/ask.go` (new)
Steps:

- [ ] Define `Question` struct: `Source` (spec/docs/security/lint), `ID` (unique per question), `SpecName` (for spec questions), `Text` (the question), `Default` (suggested answer), `Answered` (bool)
- [ ] Define `Questions []Question` as the collection type
- [ ] Implement `Collect(root string) Questions` that calls each domain's question collector
- [ ] Use `copilot.CanAsk(os.Stdin)` rather than writing a terminal check: it already answers this question, and it already knows that `/dev/null` is a character device and therefore nobody to ask — a fact this feature would otherwise rediscover the hard way
- [ ] Implement `WriteFile(qs Questions, root string) error` that writes questions to `.procoder/ask/QA.md`
- [ ] Implement the read-write pattern: TTY -> interactive prompt; no TTY -> write file

**Evidence:** Package compiles. `Collect` returns at least one Question per domain. `TTY()` correctly detects terminal.

## Task 2: Implement question collectors per domain

Files: `internal/ask/ask.go` (edit, add collector functions)
Steps:

- [ ] Spec collector: read each `.procoder/specs/*.md` file and return one Question per line of real content in its "Open questions" section — NOT only `OPEN:`-prefixed lines. `spec.Check` blocks on anything left in that section whatever it is called, and a collector that only saw the prefix would miss the `- [O-1] …` shape two specs in this repository actually used, leaving the human unable to answer what the gate is refusing
- [ ] Docs obligation collector: call `docs.Obligation(root, changed, "", false)` (with block=false since we want findings to surface questions, not block), return one Question per finding
- [ ] Security collector: call `security.SecretsChangedFiles(root, changed)` with block=false, filter for flagged secrets that have uncertain classification, return one Question per flag
- [ ] Lint collector: call `lint.Files(root, changed, false)` with block=false, filter for findings in non-obvious files, return one Question per finding (or skip if trivial)
- [ ] Each collector returns `[]Question` with proper Source, ID, and Text fields

**Evidence:** Each collector returns zero or more Questions. No panics on missing domains. All collectors handle empty input gracefully.

## Task 3: Implement interactive prompt flow

Files: `internal/ask/ask.go` (edit, add interactive section)
Steps:

- [ ] Implement `promptQuestion(q Question) string` — prints the question text with source label, waits for stdin input
- [ ] Implement `runInteractive(qs Questions, out func(string)) int` — iterates questions, prints each one, reads answer, returns exit code
- [ ] Use the existing `copilot.Prompt` interaction pattern for consistency (terminal check, yes/no parsing) but extend to free-text input
- [ ] For each question: print source + ID + text, read answer line, write to answers map
- [ ] If answer == "skip" or empty, record as "skip" (not auto-answered)
- [ ] After all questions, print summary line: "answered: N / N questions"

**Evidence:** Interactive run collects answers for all questions. Empty input is recorded as skip, not accepted as answer.

## Task 4: Implement `ask` command at top level

Files: `cmd/procoder/main.go` (edit, add `ask` subcommand)
Steps:

- [ ] Add `"ask"` to the command switch in `run()`
- [ ] `askCmd` function:

- Parse flags: `--file` (path to answers file)
- Call `ask.Collect(root)` to get all questions
- If no questions, print "no questions to ask" and exit 0
- If `--file` flag, load answers from file, write to `answers.md`, exit 0
- If TTY, call `runInteractive` to prompt user
- If no TTY, call `WriteFile` to write `QA.md`, exit 1
- Print final summary
  4.3 Add `ask [--file <path>]` to the usage text (around line 280, near `test` or `principles`)
  4.4 Wire root resolution from the command context

**Evidence:** `procoder ask` runs without error on any repo. `procoder ask` with questions prints them. `procoder ask` without questions exits cleanly. `procoder ask --file ans.md` reads answers from file.

## Task 5: Write `QA.md` and `answers.md` formats

Files: `internal/ask/file.go` (new)
Steps:

- [ ] Define `.procoder/ask/QA.md` format:

```markdown
# Procoder — Questions to Answer

Status: open
Date: 2026-08-20

## Q-1: [Source] spec-name — Short label

**Source:** spec | docs | security | lint
**Text:** Full question text here?

<!-- Answer with your decision, e.g.:
Answer: We will use PostgreSQL because ...
Answer: skip (not a secret — it's a test key)
Answer: skip (intentional lint)
-->
```

- [ ] Define `.procoder/ask/answers.md` format:

```markdown
# Procoder — Answers

Status: open
Date: 2026-08-20

## Q-1: [Source] spec-name — Short label

Answer: Full human answer text here.
```

- [ ] Implement `parseQA(root string) map[string]string` — reads answers.md, returns id -> answer mapping
- [ ] Implement `writeQA(qs Questions, root string) error` — writes QA.md with all questions
- [ ] Implement `writeAnswers(answers map[string]string, root string) error` — writes answers.md

**Evidence:** Both files parse correctly. `parseQA` returns correct mapping. File writing survives directory creation (`.procoder/ask/` may not exist).

## Task 6: Update PostToolUse hook to inject Q&A section

Files: `internal/hook/hook.go` (edit)
Steps:

- [ ] After the existing checks (format, docs, drift, secrets, lint), call `askCollect(root, file)` to get questions related to the just-written file
- [ ] If there are questions, add a section:

```
== q&a

The following question(s) need human input. Do NOT guess — stop and ask the user:

[list questions here]

Answer them via: `procoder ask --file .procoder/ask/answers.md`
Then run `procoder check` again.
```

- [ ] If there is NO TTY, write questions to `QA.md` and add:

```
== q&a

Procoder found questions that need human input. These are in `.procoder/ask/QA.md` — read them and answer them in `.procoder/ask/answers.md`, then re-run `procoder check` without `--no-verify`.
```

**Evidence:** When questions exist, output contains `== q&a` section with human instruction. When no questions, no Q&A section is added.

## Task 7: Update principles hook with Q&A behavior instructions

Files: `internal/principles/principles.go` (edit)
Steps:

- [ ] Add a new section to the principles Default text called "Asking the user":

```
## Asking the user

When procoder sends a question — whether via a `@HUMAN:` marker in `additionalContext`,
a `== q&a` section, or a question file (`.procoder/ask/QA.md`) — it means the answer
requires a human decision you cannot make.

- Do NOT guess. Do NOT invent answers. Do NOT proceed without the answer.
- Stop processing and ask the human the exact question text as written.
- Get the human's answer, then either:
  a) Run `procoder ask --file .procoder/ask/answers.md` to submit it, OR
  b) Write the answer directly into `.procoder/ask/answers.md` following the format
- After the answer is recorded, re-run `procoder check` to verify.
```

**Evidence:** Principles output includes "Asking the user" section. The section is present in both `Run` and `RunHook` output.

## Task 8: Update AGENTS.md and skills/procoder/SKILL.md

Files: `AGENTS.md` (edit), `skills/procoder/SKILL.md` (edit)
Steps:

- [ ] In AGENTS.md's "Communicating" section, add a note about asking questions (or better: add a dedicated "Asking the user" subsection)
- [ ] In SKILL.md, add the `ask` workflow: when the AI coder encounters a procoder question, it must stop and ask
- [ ] Reference the `.procoder/ask/` directory as the canonical Q&A location
- [ ] Document that answers in `answers.md` override domain judgment during `procoder check`

**Evidence:** Both files contain the Q&A workflow. AI coders reading these files know to stop and ask.

## Task 9: Update `procoder release` config and tests

Files: `.procoder/config.toml` (edit), `cmd/procoder/main.go` tests
Steps:

- [ ] Add `.procoder/ask/` directory files to `[release] files` if they exist (they won't by default, but the config should not break if they do)
- [ ] Create tests for `internal/ask/ask.go`:

- Test `Collect` on repo with known questions
- Test `Collect` on clean repo (zero questions)
- Test `TTY()` returns true/false appropriately
- Test file write/read round-trip
- Test `--file` flag loading
  9.3 Run `go test ./internal/ask/...` — all tests pass

**Evidence:** Tests cover all `ask` package functions. `procoder release` validates the repo with `ask` files present.

## Task 10: Final validation

Files: the whole tree — this task writes nothing of its own and runs the
repository's own checks over what tasks 1-9 wrote.

Steps:

- [ ] Run `procoder ask` on procoder's own repo — verify it collects questions (or zero if clean)
- [ ] Run `procoder ask` on a fixture repo with unresolved spec questions — verify they are collected
- [ ] Run `procoder check` after `procoder ask` with answers — the answered spec questions no longer block `spec check` (D-5), while an unanswered one still does
- [ ] Run `procoder ask` without TTY — verify file is written and exit code is 1
- [ ] Run `procoder format` on new files
- [ ] Run `procoder lint` — no blocking findings
- [ ] Run `procoder check` — gate must pass

**Evidence:** `procoder check`, `procoder test`, `procoder lint` all pass.

## Dependencies

- Task 1 creates the foundation that Task 2 builds on
- Task 3 depends on Task 1 (needs Question struct and TTY check)
- Task 4 depends on Task 1 (needs Collect function)
- Task 5 can be parallel with Task 1 (file format definition)
- Task 6, 7, 8 are independent once the ask package exists
- Task 9 and 10 are last (validation)

## Risk Assessment

| Risk                                                               | Impact                                                 | Mitigation                                                                                                                                                                                                                                                              |
| ------------------------------------------------------------------ | ------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Questions become too noisy (every lint finding creates a question) | Blocks Task 2 — users will ignore the feature          | Start conservative: only collect questions that truly need human judgment (spec, docs obligation, security flags); skip trivial lint                                                                                                                                    |
| Interactive prompt blocks the AI coder's session                   | Blocks Task 3 — the coder hangs waiting for user input | Design the TTY check carefully: only prompt when a terminal is truly available; otherwise fall back to file                                                                                                                                                             |
| Adding Q&A to every hook output bloats the context                 | Blocks Task 6 — context window pressure                | Only add Q&A section when there are ACTUAL questions (not every hook run); keep it brief                                                                                                                                                                                |
| Answers file becomes stale across sessions                         | Blocks Task 4 — stale answers lead to wrong decisions  | Key each answer to a fingerprint of its question text: an unchanged question is never re-asked, a changed one is asked again, and a stale answer counts for nothing (D-2). Clearing on every run was the earlier plan and would re-ask everything at each session start |
| Spec OPEN: parsing is fragile                                      | Blocks Task 2 — wrong questions collected              | Use the existing `openRe` regex from `internal/spec/spec.go`; reuse the same parser                                                                                                                                                                                     |
