---
description: "Copilot's auto-review escapes: find them since the last look, sanitise them, and turn each one into a lesson."
---

The user invoked /procoder:copilot-leak with arguments: $ARGUMENTS

Run:

    procoder copilot-leak $ARGUMENTS

(With no arguments it looks back 24 hours. `--since 6h` narrows the window to
any duration; `--quiet` reports the count and never prompts;
`--from-copilot` reads the captured ledger back instead of going to look for
more, and exits 1 while any finding is still unclassified.)

The command reads this repository's Copilot auto-review issues through `gh`:
the ones carrying the `auto-copilot` label, opened by a `copilot*[bot]`
author, or carrying Copilot's review quote block. A finding there is a bug
that got past our own gates, which makes it a lesson, not just a fix.

## Read the exit, not the mood

- Exit 0 with no findings means the window is genuinely empty — or that
  this repository has no GitHub remote, in which case there were never any
  auto-reviews to ask about.
- Exit 2 means the answer was no — either the prompt was declined, or the
  window was never queried at all.

A check that could not run is never reported as clean. `gh` missing, `gh`
unauthenticated, a network failure, or output the parser does not recognise
all end as NOT checked and exit 2. Fix the cause and re-run: `procoder init`
installs `gh`, and `gh auth login` authenticates it. Do not describe the
repository as free of Copilot findings on the strength of an exit 2.

When stdin is not a terminal there is nobody to answer, so the answer is no
and the run exits 2. That is a report, not an error — re-run it in a terminal
when you want the capture.

## Privacy is the command's job, and yours

Nothing reaches GitHub or the ledger unsanitised: fenced code is stripped,
secret-shaped strings are redacted, paths are made relative to the project.
What survives is the defect description, the file and line, and the fix
Copilot proposed. If sanitisation leaves a body empty, that finding is skipped
and said so — an empty body is never padded with the original text.

Hold the same line yourself. Never paste a raw Copilot body into an issue, a
commit message, or the chat to "add context"; quote the sanitised text the
command printed.

## After a capture

Captured findings land in `.procoder/github/COPILOT-LEAKS.md` as unlearned
entries. They are raw notes, not lessons yet. For each one:

1. Name the layer that should have caught it before the PR existed — a
   linter rule, the pre-PR rubric in `.procoder/github/REVIEW.md`, a quality
   controller, a pinning test, or CI.
2. Adapt that layer now, in this branch.
3. Promote the entry into `.procoder/github/LESSONS.md` with the adaptation
   named, and run `procoder lessons` — an entry left unlearned is not
   done.

Report to the user what was found, what was created, and what remains
unlearned.
