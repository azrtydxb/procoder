# auto-copilot-leak: capture Copilot's auto-review findings as anonymised issues and lessons

Status: active
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
