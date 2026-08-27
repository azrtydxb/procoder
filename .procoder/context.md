# Vocabulary

What this project calls things. Written by hand, never generated — the
value is the word the team agreed on, which is not always the identifier in
the code.

## the gate

Everything `procoder check` runs over a change before it becomes a commit:
formatting, git hygiene, secrets, lint, the domains a repository has opted
into. The gate answers about the CHANGE; CI answers about the tree.

## the quality chain

analysis → spec → plan → backlog → sprint, each link with a controller that
refuses rather than advises. What makes it a chain is that a link will not
advance on a document the previous one has not finished.

## P-CONTROL

The binary prints, the agent writes. procoder computes and reports; it does
not modify repository content. The single exception is `procoder prune
--apply`, which removes superseded copies of the plugin from a cache — not
repository content, and never without being asked twice.

## no silent green

A check that could not run must never read as one that passed. An
unreachable scanner, an unreadable file, a tool that is not installed:
each is reported as NOT known, never as clean.

## adopted, universal

The two gate scopes. A repository with `.procoder/` or an `AGENTS.md`
naming procoder has ADOPTED it and gets every check. Any other repository
is somebody else's, and gets only what is true anywhere — with the checks
that read file content narrowed to the lines the commit wrote.

## a decision

A choice that blocks other work and is not the agent's to make. Recorded in
`.procoder/ask/decisions.md` and put to a person; distinct from a task,
which is a deliverable. What closes a decision is an answer.

## proved by

The mutation a test's comment names: the change that must make that test
fail. A test with no `proved by:` has not been shown to catch anything.
