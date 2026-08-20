# The workflow

How work moves from an idea to a tagged release under Procoder's
discipline. The skills drive it; the rules live in
`.procoder/github/WORKFLOW.md` and are the repo's to edit.

## 1. Spec

`/procoder:spec` interviews the design gaps closed and writes
`.procoder/specs/<name>.md`. `procoder spec check` blocks while a
section is empty, an `OPEN:` question is unresolved, or an acceptance
criterion is untestable. Small, bounded work skips this — the skill
classifies first and says which path it took.

## 2. Plan

`/procoder:plan` turns the COMPLETE spec into `.procoder/plans/<name>.md`
for an engineer with zero context: Goal, Architecture, Constraints
verbatim, and `## Task N:` blocks with files, interfaces, and test-first
steps. `procoder plan check` blocks placeholders and tasks with no files
or steps.

## 3. Seed the backlog

`procoder backlog seed <spec>` decomposes the spec into an epic and one
story per acceptance criterion, fingerprinting the spec so the board can
flag drift later. Work not born from a spec goes to `procoder todo add`
instead — the same closing rigor, no project layer.

## 4. Open a sprint and pull

`procoder sprint open <goal>` — one active sprint at a time, and it
refuses while the last closed sprint's retro is empty. `sprint pull
<story-id>…` commits stories to it; `sprint status` shows the goal and
the done/total.

## 5. Build

Branch the work. A worktree per feature is the practice
`.procoder/github/WORKFLOW.md` prescribes and the pr/merge skills follow
— it is plain git, and Procoder automates none of it: no command creates
or removes a worktree.

While you write, the PostToolUse hook checks every file in the same turn
and hands back the formatted result and the domain findings; the binary
never edits the file. `procoder index` answers structural questions
faster than grep, and the hook keeps the index current.

## 6. Test

`procoder test` runs each detected ecosystem's canonical runner. NOT run
is never green. With `[test] policy = "block"` the suite joins the close
controllers: `todo close` and `backlog close story` refuse on a red or
unverifiable suite. `procoder bench` proves a performance claim against
the saved baseline, in Go.

## 7. Check

`procoder git` and `procoder check` must be clean: formatting, hygiene,
lint, secrets, docs, CI and infra rules all answer at once, through the
one code path CI also runs.

## 8. PR

The pr skill summarises the real diff, fills the PR template from
`.procoder/github/`, scrubs attribution, verifies the blast radius the
gate reported (`procoder index impact`), and shows everything before
`gh pr create`. Titles stay ≤ 72 characters — they become squash-commit
subjects.

Merge-gate polling is delegated to a watch-only background agent
following the watcher protocol: calibrate against previous runs of the
same workflow, poll per job in the foreground, report the first failure
immediately with its log excerpt, report every state change. The watcher
never fixes, replies, or merges — the main agent acts on its reports.

## 9. Reviews and merge

Every review thread — human or bot, Copilot included — is either fixed
(commit, push, reply saying what changed) or answered with a concrete
reason. Threads are resolved only after, never to hide them. Nothing
merges over a red check or an unaddressed comment.

Squash-merge when everything answers green, then: delete the remote
branch, delete the local branch, remove the worktree if one was used,
`git fetch --prune`, and return to an updated default branch. Anything
that escaped to a reviewer becomes a `.procoder/github/LESSONS.md` entry
with the adaptation that closes its class, in the same PR.

## 10. Close the sprint

`procoder backlog close story <id>` refuses without checked criteria,
recorded evidence, and a clean gate. `sprint close` refuses while a
committed story is neither done nor carried back with a reason — and
scaffolds the retro whose answers are the price of the next `sprint
open`.

## 11. Release

`procoder release [<version>]` verifies the version across
`[release] files`, the changelog entry, a clean tree, the gate, and the
suite — every failure listed at once. On success it prints the `git tag`
command for a human to run. It tags nothing itself.
