# Spec: Interactive Q&A — `ask` Feature

Status: complete

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
- Answering the same question twice. Answers persist and are keyed to the
  question that earned them ([D-2]).

## Constraints

- [N-01] Each domain collects questions in its own format; `ask` normalises them into a uniform `Question{Source, ID, Text, Default}` struct
- [N-02] The interaction is non-blocking for the AI coder: if the coder cannot interact (no terminal), questions are written to file with clear instructions
- [N-03] The Q&A file format must be parseable by both humans and the AI coder (Markdown format with clear structure)
- [N-04] Re-running `procoder ask` keeps answers already given and asks only what is new or changed ([D-2])

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

- [ ] C-01: `procoder ask` on a fixture carrying one question per generating
      domain — an unresolved spec question, an uncleared documentation
      obligation, a flagged secret, a blocking lint finding — prints all
      four, each naming its source, and a flagged secret's value appears
      nowhere in the output or in either file.
- [ ] C-02: With a terminal it asks one question at a time; with
      `/dev/null` or a pipe on stdin it asks nothing, writes
      `.procoder/ask/QA.md`, names the file and the `--file` route, and
      does not hang.
- [ ] C-03: `.procoder/ask/QA.md` and `.procoder/ask/answers.md` are
      written at those paths, and a second run with no new questions
      rewrites neither.
- [ ] C-04: `procoder ask --file <path>` records the answers in that file
      against the questions they belong to, and refuses a file it cannot
      parse rather than recording a partial reading.
- [ ] C-05: An answer persists: a question already answered is not asked
      again on the next run, and the same question with its text changed IS
      asked again — verified by a test that answers, re-runs, edits the
      question, and re-runs.
- [ ] C-06: The PostToolUse hook's `additionalContext` carries the pending
      questions and the instruction not to guess, and carries nothing when
      none are pending.
- [ ] C-07: `procoder ask` exits 1 while any question is unanswered and 0
      when all are, so a caller can tell the two apart.
- [ ] C-08: `[ask] policy = "block"` makes pending questions block
      `procoder check`; the default `report` lists them and leaves the
      gate's verdict unchanged — both verified by test.
- [ ] C-09: A spec question that has been ANSWERED in `answers.md` no
      longer blocks `spec check`: the spec reports COMPLETE with a note
      that the section still lists questions a human has decided. A
      question with no answer blocks exactly as it does today, and an
      answer whose question text has since changed does not count —
      verified by a test covering all three.

## Open questions

<!-- none — decisions recorded below -->

## Decisions

- [D-1] `procoder ask` is a top-level command, not a subcommand group. One
  verb, one job, beside `check`, `lint` and `test` — and the Interfaces
  section already assumed it. Variants ride on flags. ([O-1] resolved.)
- [D-2] Answers persist in `.procoder/ask/answers.md`, keyed by a
  fingerprint of the question text. An unchanged question is never asked
  twice; a question whose text changed is asked again, because the old
  answer was to a different question. This is the same mechanism that
  stopped spec drift crying wolf: hash the thing that matters, not the
  prose around it. It replaces [N-04], which said a re-run clears
  everything — that would have made the feature unusable at every session
  start. ([O-2] resolved.)
- [D-3] Without a terminal, the coder relays the questions to the human,
  writes the human's answers into a file, and runs `procoder ask --file
<path>`; the binary parses and records them. One route in, and the file
  is evidence of what was decided. The coder can fabricate that file — so
  can it fabricate anything it types — and this is the same trust boundary
  every other procoder input sits on, made visible rather than hidden.
  ([O-3] resolved.)
- [D-5] An answer recorded through `ask` resolves a spec's open question
  for `spec check`: an answered question no longer blocks, and the verdict
  is COMPLETE. This softens the rule that a question blocks while it sits
  in the section, and it is worth naming what that rule was protecting so
  the softening does not go further than intended: `backlog seed` gates on
  COMPLETE, so a spec must not be seedable while its design is undecided.
  Answered is decided — the decision simply lives in `answers.md` rather
  than in prose. What must NOT follow is the checker going quiet: an
  UNANSWERED question blocks exactly as before, an answer keyed to a
  question whose text has since changed does not count, and a spec that
  passes on answers says so out loud, so a reader is never told a section
  full of questions is finished. Rewriting them into the spec stays the
  tidier end state; it stops being the price of a green check.

- [D-4] `[ask] policy = "report" | "block"` in `.procoder/config.toml`,
  default `report`. A pending question is a request for judgement, not a
  defect, and blocking a commit on one stops work the human may not be
  awake to unblock. A repository that wants the hard stop sets `block`,
  following every other domain policy here.
