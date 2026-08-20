# auto-copilot-leak: capture Copilot's auto-review findings as anonymised issues and lessons

Status: closed 2026-08-20
Created: 2026-08-20

## Goal

Close the hole in the lessons loop where a bot's review findings are read,
acted on, and then forgotten. Today Copilot's auto-review comments are
addressed in the PR and leave no trace: nothing records that the class of
mistake happened, so nothing adapts the layer that should have caught it,
and the same class escapes again.

By the end of this sprint `procoder copilot-leak` finds those findings,
strips every trace of the user's code from them, asks before it acts, and
turns what the user approves into GitHub issues and lessons entries with
placeholder adaptations that read as UNLEARNED until they are written. The
gate knows about the ledger, the docs describe the command, and rot guards
keep the two from drifting apart.

## Result

committed: 4
done: 4 (20260820-auto-copilot-leak-command-shell-sanitisation-prompt, 20260820-auto-copilot-leak-gate-docs-rot-guards, 20260820-auto-copilot-leak-issue-capture-copilot-leaks, 20260820-auto-copilot-leak-lessons-integration)
carried: 0

## Retro

What slowed us down: the sprint read 0 of 4 while two stories were fully
built. Two things hid it. The stories were written with Steps/Files/
Verification instead of the sections the close controller reads, so nothing
could judge them and nothing said so until someone tried to close one, one at
a time. And the backlog is versioned per branch, so `backlog board` on a
feature branch reported "0 open · 78 done" while thirty-four specced stories
sat on another branch — the board answered about the checkout and was read as
answering about the project.

What we change next sprint: an open story missing a required section now says
so on the board, sharing its section list with the close controller so the two
cannot disagree. Cross-branch blindness is not fixed — the board still reports
the checkout. The proposal on the table is a footer line naming the branch it
read and how many open stories the default branch carries that this one cannot
see, computed with one `git grep`, saying NOT compared when git cannot answer.

One adaptation worth keeping: write the ledger through the package that reads
it. internal/copilot had grown its own path, header and entry shape while
internal/lessons kept the reader and an unused writer. Two writers for one
file drift silently — the first symptom would have been a captured leak that
never showed up as unlearned. The fix that sticks is not the deduplication but
the test that carries a finding across the seam.
