# session-continuity

Status: draft

## Problem

Every session starts blind. The SessionStart hook injects the
principles and nothing else — not the active sprint, not the story in
flight, not the branch, not whether the tree is dirty or the gate last
passed. And no hook fires when a session ends or compacts, so nothing
is captured on the way out. The agent re-derives yesterday's context by
reading files at random, contradicts decisions made forty turns ago, or
asks the user what it was doing — the failure the whole backlog was
built to prevent, reappearing at the session boundary.

## Users

- The agent resuming work: it should open with a truthful picture of
  where the project stands, not a static essay.
- Pascal returning to a repo after a day: the same picture, on demand.
- Long sessions that compact: the handoff survives the summary.

## In scope

- `procoder status` — the state-of-play report, computed fresh, no
  arguments: current branch and how it compares to the default, dirty
  file count, the active sprint with its done/total story count, the
  stories in that sprint that are still open, open todo tasks, the
  unlearned lesson count, and whether an index exists and is stale.
  Every line is a fact the binary can compute; nothing is guessed.
- SessionStart injection grows: `principles --hook` output gains the
  `procoder status` block after the principles text, so a session opens
  knowing the state. Speed is a constraint, not a nicety (see below).
- A handoff note, written by the binary when a session ends or before a
  compaction: `.procoder/state/handoff.md` records the same computed
  facts plus the timestamp and the HEAD commit — facts only, never a
  guess at intent. The agent may append its own intent lines under a
  marked section, which survive the next rewrite of the facts block.
- `procoder hook stop` — the binary side, reading the host's Stop or
  PreCompact payload on stdin and writing the handoff note.
- hooks/claude-hooks.json gains Stop and PreCompact registrations; the
  portability table states which other hosts can carry them.
- The status report is included at the top of the handoff note, so the
  file reads as the answer to "where was I".

## Out of scope

- Guessing intent: the note records what is true, never "you were
  probably about to…". Intent lines are the agent's to write.
- Cross-session memory of conversation content, prompts, or model
  reasoning — the handoff is repository state, nothing else.
- Multi-user or multi-machine handoff: the note is local, per-clone,
  gitignored state like the index.
- Restoring anything automatically: reading the note is the next
  session's decision, not an action taken for it.
- Time tracking, session duration reporting, or activity analytics.

## Constraints

- Pure Go stdlib; new package internal/status; the hook payload work
  joins internal/hook.
- SessionStart must stay fast: the status block has a hard 3-second
  budget and NEVER runs the gate, the suite, or any network tool. Any
  check that cannot answer in budget is omitted with a one-line note
  rather than delaying the session.
- P-CONTROL: `status` prints; the handoff note is procoder-owned state
  under `.procoder/state/`, the same precedent as the index and the
  bench baseline. No other file is ever written.
- The honesty rule: a fact that could not be read (git absent, backlog
  unreadable) is reported as unknown, never as a comfortable default.
- `.procoder/state/` is per-machine derived state and belongs in the
  gitignore guidance.
- Agent-authored intent lines must survive: the writer replaces only
  the facts block, matched by its markers.

## Interfaces

- `procoder status` — stdout, one line per fact, exit 0 always (a
  report cannot fail); unknown values print as unknown with the reason.
- `procoder hook stop` — stdin: the host's Stop or PreCompact payload;
  writes `.procoder/state/handoff.md`; exit 0 always, silent on stdout
  so a session ending is never noisy.
- `principles --hook` gains the status block for every host envelope it
  already supports.

## Data

- `.procoder/state/handoff.md`: a facts block between
  `<!-- procoder:facts -->` and `<!-- /procoder:facts -->` (timestamp,
  HEAD, branch, dirty count, sprint, open stories, open tasks), then a
  free `## Notes` section the agent owns and the writer never touches.

## Edge cases

- No backlog, no todo, no index: status says so plainly and stays exit
  0 — a fresh repo has a valid, empty state.
- Not a git repository: branch and dirty count report unknown with the
  reason; the rest still computes.
- Handoff note absent at session start: nothing is injected about it;
  the status block stands alone.
- Handoff note present but its facts markers were hand-deleted: the
  writer rewrites the whole file, preserving any notes section it can
  still find.
- A session ends within seconds of starting: the note is rewritten with
  the same facts; harmless, and cheaper than deciding when to skip.
- Detached HEAD or mid-rebase: branch reports the raw git answer rather
  than pretending; the mid-rebase state is named when git reports it.
- Very large repos: the dirty count uses git's own porcelain output and
  is bounded by the 3-second budget like every other line.

## Failure modes

- git missing or failing → those lines report unknown with git's first
  error line; the rest of the report still prints.
- The state directory cannot be created or written → `hook stop` exits
  0 silently after printing nothing (a failed handoff must never break
  a session), and `procoder status` says the note could not be written
  when asked directly.
- Budget exceeded at SessionStart → the block prints what it has plus
  "some state omitted for speed"; the session never waits.
- Unreadable backlog or todo files → counted as unknown, and the line
  says which file could not be read (the honesty rule, not silence).

## Acceptance criteria

- [ ] `procoder status` on this repository prints branch, dirty count,
      active sprint (or none), open stories, open tasks, and index
      freshness — every line a computed fact, exit 0.
- [ ] `procoder status` in a non-git temporary directory reports the
      git-derived lines as unknown with a reason and still exits 0.
- [ ] `principles --hook` output contains the status block after the
      principles text, in each host envelope it already supports.
- [ ] The SessionStart path completes within the 3-second budget on a
      repository with a built index and an active sprint — verified by
      a timed test.
- [ ] `procoder hook stop` writes `.procoder/state/handoff.md`
      containing the facts block and today's HEAD.
- [ ] Agent-authored notes survive a second `hook stop` — verified by a
      test that writes notes, re-runs, and asserts they remain while
      the facts block updates.
- [ ] An unwritable state directory leaves `hook stop` exiting 0 with
      no output — verified by a test.
- [ ] `.procoder/state/` appears in the gitignore guidance the docs
      domain checks.

## Open questions

<!-- none — decisions recorded above -->
