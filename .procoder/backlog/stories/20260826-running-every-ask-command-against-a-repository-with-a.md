# Running every `ask` command against a repository with a decisions file leaves every file on disk byte-identical, the decisions file included.

Status: done
Created: 2026-08-26
Epic: decisions-reach-the-user
Sprint: 022-a-decision-the-agent-cannot-make-reaches-the-user

## Description

P-CONTROL, the rule the whole tool rests on: the binary prints, the agent
writes. A decisions queue is the most natural place in the codebase to
break it — a `procoder ask --raise "..."` reads better on the command
line than telling an agent to write a file.

Done when procoder reads that file and never writes it.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Running every `ask` command against a repository with a decisions file leaves every file on disk byte-identical, the decisions file included.

## Evidence

`TestCollectingDecisionsWritesNothing` compares the file's bytes AND its
modification time across a `Collect`. Killed by adding an `os.WriteFile`
to `decisionQuestions` that rewrites what it just read.

Confirmed by hand too: `shasum` of `.procoder/ask/decisions.md` unchanged
across `procoder ask`, `procoder ask --file`, and `procoder check`.

No `--raise` flag exists, and the reason is recorded in the spec's
Decisions section rather than left to be rediscovered.
