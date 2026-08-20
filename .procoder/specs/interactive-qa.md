# Spec: Interactive Q&A — `ask` Feature

Status: draft

## Problem

When procoder finds things that need human judgment, the answers are injected into the AI coder's context as plain text via hooks (PostToolUse `additionalContext`, SessionStart principles hook). The AI coder then tries to answer these itself — guessing about specs with `OPEN:` questions, about whether a linter finding is real, about documentation gaps, about security flags. The user never gets to answer; they just see text that looks like the problem was resolved when the coder made it up.

We need a structured Q&A flow where procoder actually asks the human, collects answers, and makes those answers available to the AI coder as ground truth.

## Users

- The human being asked. Today they never see the question: it goes into
  the coder's context, the coder answers it, and the answer reads like a
  resolution rather than a guess.
- The AI coder, which currently has to invent an answer or stall. With
  `ask` it has ground truth to work from, and an instruction to stop rather
  than guess when it does not.
- The reviewer of the resulting work, who can see which decisions a human
  actually made.

## In scope

- [R-01] `procoder ask` collects questions from all question-generating domains:
  - Spec: every unresolved `OPEN:` question with its spec name and the question text
  - Docs obligation: every documentation gap that was not cleared by an edit or commit message
  - Security: every secret flag (real secret or test credential?)
  - Lint: every blocking lint finding that needs judgment (true positive or false positive?)
- [R-02] `procoder ask` presents questions one at a time in the terminal when a TTY is available, using the existing `copilot.Prompt` interaction pattern
- [R-03] When no TTY is available, `procoder ask` writes all questions to `.procoder/ask/QA.md` (one question per section, numbered) and exits 1 with a clear instruction telling the AI coder to forward them
- [R-04] `procoder ask` writes answers to `.procoder/ask/answers.md` in a machine-readable format
- [R-05] `procoder ask --file <path>` accepts answers from a file instead of interactive input (enables the AI coder to submit human answers)
- [R-06] PostToolUse hook injects a Q&A section into `additionalContext` whenever there are pending questions, with explicit instructions that the AI coder must stop and ask, not guess
- [R-07] `procoder ask` output includes the answers (from `answers.md` if present) so the AI coder sees the human's decisions
- [R-08] The `principles` hook text includes a section about how the AI coder should behave when presented with questions

## Out of scope

- Procoder answering on the user's behalf, or inferring an answer from the
  repository. The whole point is that a guess stops being indistinguishable
  from a decision.
- A graphical or web interface. The terminal and a Markdown file are the
  two surfaces.
- Question generation itself: each domain already knows what it cannot
  decide; `ask` collects and normalises, it does not invent questions.
- Answer persistence across runs — [O-2] asks whether that is even wanted,
  and [N-04] currently says a re-run refreshes.

## Constraints

- [N-01] Each domain collects questions in its own format; `ask` normalises them into a uniform `Question{Source, ID, Text, Default}` struct
- [N-02] The interaction is non-blocking for the AI coder: if the coder cannot interact (no terminal), questions are written to file with clear instructions
- [N-03] The Q&A file format must be parseable by both humans and the AI coder (Markdown format with clear structure)
- [N-04] Re-running `procoder ask` clears previous answers and refreshes questions

## Interfaces

- `procoder ask [--file <path>]` — collects, asks, and records ([R-01],
  [R-05]).
- `.procoder/ask/QA.md` — the questions, one numbered section each, written
  when there is no terminal to ask ([R-03]).
- `.procoder/ask/answers.md` — the answers, machine-readable ([R-04]).
- The PostToolUse hook's `additionalContext`, which carries the pending
  questions and the instruction not to guess ([R-06]).
- The `principles` hook text, which tells the coder how to behave when it
  is handed a question ([R-08]).
- `Question{Source, ID, Text, Default}` — the normalised shape every
  domain's questions are mapped into ([N-01]).

## Data

`.procoder/ask/QA.md` and `.procoder/ask/answers.md` hold questions and the
human's answers. Both are repository state, not user code, and both are
readable by a person as well as by the coder ([N-03]).

The questions themselves quote what the domain found — a spec's `OPEN:`
line, a lint finding, a flagged secret. A flagged secret's _value_ must
never be written into either file: the question is whether the flag is
real, and the answer does not need the credential to be legible.

## Edge cases

- No questions at all: `ask` must be a quiet no-op, not an empty prompt.
- A terminal that goes away mid-run.
- An answers file that answers questions which no longer exist, or misses
  ones that do.
- A question whose text is longer than a terminal line.
- Two domains asking the same question about the same finding.
- `--file` pointed at a file that is not answer-shaped.

## Failure modes

- `.procoder/ask/` is unwritable: the questions must still be shown, and
  the run must say plainly that nothing was recorded.
- The answers file is unreadable or malformed: the questions stay
  unanswered rather than being treated as answered — unknown is never done.
- The hook cannot reach the question set: it says so rather than injecting
  an empty Q&A section that reads as "nothing to ask".

## Acceptance criteria

- [ ] C-01: `procoder ask` collects at least one question from each domain that generates them — verified by: Run on a repo with known questions per domain; verify output includes all
- [ ] C-02: Interactive path reads from stdin only when a TTY is available — verified by: Run with `/dev/null` input; must write to file, not hang
- [ ] C-03: File path is written to a stable location — verified by: Check `.procoder/ask/QA.md` and `.procoder/ask/answers.md` exist after run
- [ ] C-04: `--file` flag reads answers from specified file — verified by: Write test answers file; run `ask --file`; verify questions are answered
- [ ] C-05: Hook output contains a `[q-a]` section with explicit ask instructions — verified by: Run hook; verify output contains `== q&a` and "do NOT guess" text
- [ ] C-06: Spec's `check` incorporates answers from `answers.md` when re-running — verified by: Write answer in `answers.md`; run check; verify OPEN: questions are accepted
- [ ] C-07: Non-interactive execution exits 1 (questions unanswered) — verified by: Run without TTY; verify exit code is 1, exit 0 only when all answered
- [ ] C-08: Prodigy gate passes with answered questions — verified by: Run `procoder check` after `procoder ask`; must pass when all answers provided

## Open questions

- [O-1] Should `procoder ask` be a subcommand of `ask` or a standalone command at the top level? (Top-level: simpler, matches existing commands like `check`, `lint`)
- [O-2] Should answers persist across runs, or every run re-asks all questions?
- [O-3] Should the AI coder be able to submit answers inline in its response, or must it use a separate command?
