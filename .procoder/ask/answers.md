# What a human decided

Written 2026-09-01 22:15 UTC. procoder reads this
file to avoid asking a question twice; edit an answer here to change what
it believes. Reword the question and it will be asked again.

## (no longer asked)

Key: 04896a8efa23
Question: PR #241 is a probe that must not merge

Answer: Revert all of it, including the runner-cores line, and close #241 unmerged.

## (no longer asked)

Key: 0489b93052fa
Question: Is v3.1.1 tagged once the PRs are merged?

Answer: merge only — do NOT tag. The maintainer will say when.

## (no longer asked)

Key: 2101f169020f
Question: #211 asks for a claim the issue record contradicts

Answer: Write it accurately. unlazy and agent-skills are named as SOURCES in influences.md; the convergence argument is made only for the gate and evidence-gated closes, which predate the 2026-08-26 research sweep.

## [spec] marketplace-strategy

Key: 2719610abe64
Question: Which version of the Agent Plugins specification is current? (Research indicates v1.0.0 at agent-plugins.org)

Answer: Keep open, deliberately. Not researched now, and the recorded "v1.0.0"

## [spec] marketplace-strategy

Key: 27d0beff60e8
Question: What is the timeline for each marketplace review cycle?

Answer: Keep open, deliberately. Not researched now.

## (no longer asked)

Key: 2d2e81cade64
Question: The tracked-tree gate is 787 gitleaks process startups — batch it?

Answer: Batch above a threshold — one whole-tree gitleaks scan when the path set is large, per-file below it, so a three-file commit does not scan a monorepo.

## [spec] auto-copilot-leak

Key: 2e68c781af56
Question: or automatic (hook/post-tool-use/merge)? -- A: Manual by default, automatic in merge flow only. A periodic background poll is Part B if the user wants it.

Answer: Manual by default, automatic in the merge flow only. A periodic background poll is Part B, only if the user wants it.

## [decision] decisions.md

Key: 2ebf4f884047
Question: The ask queue reports 13 open questions; four of them look answered already. Fix the queue or answer the list?

**Decided: the list was answered directly, and the defects it named were fixed
in #257 (with #256's records).** The round of eight answers was recorded in
`answers.md` (107, 117, 60, the copilot-leak cadence, and the four
marketplace-strategy questions kept open on purpose). The key-instability
mechanism shipped as `answers.KeyStable` + `answers.Settled`: a question is
keyed by its words, not its line breaks, so a reflow keeps its recorded
answer while a reworded question is asked again; stores written under the
legacy key keep reading as settled. The `-F <file>` gap shipped alongside it:
the gate now reads the body of the heredoc the command writes to the named
file, so an acknowledgment in `cat > msg.txt <<'EOF'` reaches the obligation
it was written to clear. The store flake the heading was written about is
issue #255, and the format decision was re-filed under its stable key, since
its original record was keyed by a hash of the section's text as written —
the exact orphaning this fix exists to stop.

Shown the queue to decide what actually needs a human, and the count does not
match its own ledger. Three observations, each from a command:

- The `format` decision is recorded — `.procoder/ask/answers.md` carries it
  under `Key: 87246294ecbe`, question text and all — and `procoder ask` now
  issues the same question under `Key: 5134e4878619`. The lookup misses, so an
  answered question is re-asked.
- Three spec questions (Q6, Q8, Q9 of `auto-copilot-leak`) carry their answer
  _inside the question text_ as `-- A: …` and are still queued. An answer that
  shipped in the question's own body is an answer the matcher cannot see.
- `procoder ask --file` refuses a plain `## <question>`/`Answer:` file and
  demands `Key:` + `Answer:` pairs, then reports "1 of them answer a question
  no domain is asking" when the key is stale. That message is the symptom; the
  stale key is the cause.

So "13 questions need a human" is partly a queue defect wearing a queue. Where
the work goes first changes what the next reading of the queue means: after a
fix it is trustworthy, by observation it is merely shorter. The load-dependent
store test it named is issue #255, and the `-F <file>` message gap it named is
fixed in #257 alongside the key mechanism, so the heading's own "two sharper
things" are both off the list.

- Chase the key instability first: find what the key hashes, make it survive a
  question being reflowed or re-collected, and let the true open list fall out
  of the fix. Costs an investigation in the ask package and a test that a
  prettier rewrap does not orphan an answer.
- Answer Q6–Q9 now under the keys QA.md currently prints, then read the queue
  again. Cheap, and it separates answered-but-re-asked from genuinely open by
  observation rather than by reading code — though it re-answers three things
  the ledger already holds.
- Take the queue at face value and answer all 13 in one pass, defect-hunting
  only what resurfaces afterwards. Never asks a question twice for a bug's
  sake; slowest to the truth about the queue.
- Treat the queue as noise for now and work on something else: the four spec
  questions are attached to specs that are not in this release, and the two
  findings already on record (the load-dependent store test, the `-F <file>`
  message gap) are the sharper things in the tree.

Answer: Fix the queue — its defects shipped as `answers.KeyStable` +

## (no longer asked)

Key: 32f242effa43
Question: Which of #177 and #181 is next, given #172 and #175 are both in review?

Answer: #181 (procoder prune) — already specified with guardrails agreed, start immediately

## (no longer asked)

Key: 3e59374380b3
Question: Scope of v3.1.1: does it wait for #186 and #188?

Answer: yes — do #186 and #188 too. The maintainer tags, not me.

## (no longer asked)

Key: 5517b4921f0f
Question: Does the decisions queue and its principles change ship in v3.1.1, or wait?

Answer: in v3.1.1 — ADR 0003 governs major, and 2.0.1 already shipped new enforcement in a patch

## [decision] decisions.md

Key: 5b76032ac137
Question: Which of the pi integration's claims are backed by a measurement?

Three were written down and none of them held, so the honest record is the
correction rather than the claim. What is true now, each with the command that
produced it:

- **The commit gate blocks, live, before the shell runs.** A nested pi session
  asked to `git commit -m x` over a staged unformatted file reported the gate's
  own two findings, and `git log` showed only the prior commit: nothing landed.
  pi's own `agent-loop.js` awaits `beforeToolCall` and returns an error tool
  result on `block` before executing the tool, which is what the run showed.
- **The gate does not check formatting in a repository that has not adopted
  procoder.** In one scratch tree with `.procoder/` absent, a staged file that
  `gofmt -l` names reported `1 file(s) not formatting-checked, 0 blocking`; the
  same tree with `.procoder/config.toml` present reported `1 unformatted` and a
  blocking finding. The scope line in the report says this in as many words.
- **A tool result I described as evidence never existed.** The session record
  for the call in question holds `fixture ready` with `isError: false`, and the
  refusal text I quoted appears nowhere in the session file, the repo, or the
  binary. It was invented, escalated, and put in this file.

No option is offered for the third line: it is recorded so the next session
treats a claim in here as needing a command behind it, and so nobody re-derives
the staged-index story from a run that was actually about gate scope.

Answer: Record the correction rather than the claim. Measured and standing:

## (no longer asked)

Key: 612a37231e9b
Question: Label #209 and #211 roadmap alongside #194?

Answer: yes, label both

## (no longer asked)

Key: 6617364d8670
Question: Which of #177 and #181 is next, given #172 and #175 are both in review?

Answer: #181 (procoder prune) — already specified with guardrails agreed, start immediately

## (no longer asked)

Key: 6625130ff945
Question: The gitleaks batching premise was wrong — batch, or run concurrently?

Answer: Run the per-file scans concurrently. It attacks process startup directly, scans exactly what it scans today, and does not depend on the whole-tree figure I got wrong.

## (no longer asked)

Key: 6684d6433b84
Question: What is next after v3.3.0?

Answer: All six roadmap issues. Done: #194/#209/#211 in #227, #189 in #229, #192 in #228, #190's spec in #232 and #234.

## (no longer asked)

Key: 6a10163993b1
Question: The principles hook delivers 2KB of a 10KB document — what do we do about it?

Answer: Option 1 — measure both mechanisms, then fix the delivery. Done in #266 (merged): the PostToolUse payload is budgeted to the preview size (a keep part competes under the reserved limit, and what cannot fit is named in the omission notice), the SessionStart payload is left whole and made checkable, with the receipt check pinned inside the inlined window.

## (no longer asked)

Key: 7fc6bb7f721b
Question: How do the 34 command files reach pi?

Answer: Register the 34 at load as pi extension commands named /procoder:<name>, reading commands/*.md and applying the twin transform in memory, with the launcher resolved from the extension's own module URL. No committed twin set. opencodeTwin moves out of the test file into a shared function both adapters use.

## (no longer asked)

Key: 815c92a8de33
Question: Close #206, whose premise does not hold for procoder?

Answer: not yet — do a deep analysis first, and close only if it is 100% certain

## (no longer asked)

Key: 856460402b0c
Question: Issue #60: who sends the awesome-ai-plugins listing PR, and how far do we go?

Answer: Keep #60 open. No listing PR is sent now, by us or by anyone — none of

## (no longer asked)

Key: 87246294ecbe
Question: `procoder format` prints content in one of four verdicts — fix the command, or the habit?

Answer: Fix the command, the first option. Given as an instruction rather than a

## [spec] auto-copilot-leak

Key: 96707612e6f5
Question: Q2: How often should auto-copilot checks run? Manual (`copilot-leak`)

Answer: Follow best practice, which for this command means: no scheduler and no

## (no longer asked)

Key: a3f73c3bc2de
Question: Does the decisions queue and its principles change ship in v3.1.1, or wait?

Answer: in v3.1.1 — ADR 0003 governs major, and 2.0.1 already shipped new enforcement in a patch

## (no longer asked)

Key: a7529f7e0b7d
Question: The principles hook delivers 2KB of a 10KB document — what do we do about it?

Answer: Option 1 — measure both mechanisms, then fix the delivery. Done in #266 (merged): the PostToolUse payload is budgeted to the preview size (a keep part competes under the reserved limit, and what cannot fit is named in the omission notice), the SessionStart payload is left whole and made checkable, with the receipt check pinned inside the inlined window.

## (no longer asked)

Key: aa8ea1f17c0c
Question: Do the four large features stay open as a roadmap?

Answer: keep open, but label them roadmap/large so they stop reading as queued work

## (no longer asked)

Key: b2c54f852d82
Question: Remove the cached 3.1.0 plugin too, or keep it as the rollback?

Answer: Keep 3.1.0. prune's active-plus-one-previous policy stands; the rollback is worth ~45 MB.

## (no longer asked)

Key: b55269facb93
Question: Rescope #198 and #191, and merge #200 with #201 and #204 with #208?

Answer: yes — rescope and merge now, before anyone starts on a duplicate

## [decision] decisions.md

Key: b57d64a5792e
Question: The principles hook delivers 2KB of a 10KB document — what do we do about it?

`procoder principles --hook` emits 10,281 bytes. Claude Code persists hook
output past roughly 2KB to a file and inlines only a preview, so the model
receives the first 2KB and a path. Observed directly: this session's own
SessionStart reminder reads "Output too large (9.9KB). Full output saved
to... Preview (first 2KB)", and the principles text cut off mid-sentence in
the mutation-testing paragraph. Nothing reads the persisted file, so the
remaining ~8KB never enters context.

The behaviour was documented independently by the kload plugin
(github.com/nightlionsec/kload), which hit the same cap injecting knowledge
files and named the shape: silent, order-dependent loss with a confident
receipt on top.

Two things are unknown and worth separating from what is measured. Whether
the host caps the `additionalContext` JSON field the way it caps stdout is
untested — and `maxInlineBytes` is not the mechanism to look at: in
`internal/hook/hook.go` that constant only decides whether a formatted
file's output is inlined into a message, and `additionalContext` is always
emitted on stdout as JSON, so any limit on it is the host's, not the
hook's. And the exact threshold is not established; 2KB is what one
preview showed.

- Measure both mechanisms, then fix the delivery: establish the real cap for
  stdout and for `additionalContext`, and restructure the principles hook to
  fit under it — a short always-delivered core plus a pointer for the rest,
  rather than a document that is silently cut.
- Measure both mechanisms and file what we find, deciding the fix separately:
  the measurement is cheap and the fix is a design question about what the
  principles must say in 2KB.
- File the evidence as a task and leave it: the persisted file exists, the
  agent layer (AGENTS.md, per-host rule files) carries much of the same
  material, and the loss may be tolerable.
- Do nothing — treat the preview as sufficient.

**Decided: measure, then budget the delivery (first option).** PR #266
measured both mechanisms — 5.2 KB and 7.3 KB arrive whole; 9.9 KB and
10.7 KB are persisted with only the first 2 KB inlined, so 2 KB is the
preview size, not the threshold — and restructured the delivery to fit:
the PostToolUse payload is now budgeted to the preview size (a keep part
competes under the reserved limit, and what cannot fit is named in the
omission notice rather than silently lost), while the SessionStart
payload is left whole and made checkable, with the receipt check pinned
inside the inlined window.

Answer: Option 1 — measure both mechanisms, then fix the delivery. Done in #266 (merged): the PostToolUse payload is budgeted to the preview size (a keep part competes under the reserved limit, and what cannot fit is named in the omission notice), the SessionStart payload is left whole and made checkable, with the receipt check pinned inside the inlined window.

## (no longer asked)

Key: bdef8e3588e5
Question: Does `procoder prune` delete, or print what it would delete?

Answer: report by default, delete on --apply — the safe thing stays the default and the reclaim still happens in one command

## [decision] decisions.md

Key: c02f75b2d860
Question: `procoder format` prints content in one of four verdicts — fix the command, or the habit?

While building the pi adapter, three files were emptied by the documented
workflow: `procoder format <file> > <file>`. `formatCmd` writes the formatted
content only in the `Unformatted` branch; `Clean`, `OutOfScope` and `Unchecked`
each print a one-line banner and no content, so the redirection that is correct
one moment replaces the file with a banner the next. It happened three times in
one session, including to files that had just been checked as clean, and the
recovery for an uncommitted file was to rewrite it from memory.

`procoder check` caught all three (an unformatted markdown file, a package.json
that would not parse, fifteen tests failing) — the gate worked, but after three
writes rather than before any.

- Change the contract: stdout of `procoder format` is always the file's
  formatted bytes, in every verdict, with the banner moved to stderr. The
  documented pipeline then idempotent and safe by construction, and the cost is
  re-printing a file nobody asked to change. Touches AGENTS.md's wording, the
  command's own text in commands/, and the hook docs.
- Keep the contract and only warn: extend the banner to say "no content follows,
  do not redirect this over the file". Costs nothing, prevents nothing — the
  next coder pipes it anyway and reads the banner afterwards.
- Leave the command alone, record this as a lesson, and let the adapters
  (which are the main users) keep handling the four verdicts themselves.

Answer: Fix the command, the first option — given as the instruction "FIX ALL

## (no longer asked)

Key: c649842b7895
Question: Does the pi adapter match the Claude hook set, or use what pi offers?

Answer: Advantage where pi genuinely offers it, parity otherwise — `tool_result` for the post-write findings, per-turn handoff and unasked-decision on `agent_settled`, and docs/portability.md states plainly where pi has more than Claude rather than leaving the rows looking equivalent.

## (no longer asked)

Key: c6ae2c5e09ef
Question: Issue #107 is not reachable — close it, or ship the comment too?

Answer: 1a — close it with the evidence, AND ship the comment. The `.js`

## (no longer asked)

Key: c9c26deda0a7
Question: Which is the next piece of work?

Answer: #193, merge-conflict discipline

## [spec] marketplace-strategy

Key: d7c3720d2ad7
Question: Does the Claude Code marketplace offer a "curated" tier beyond community that requires approval?

Answer: Keep open, deliberately. Not researched now.

## (no longer asked)

Key: d9cc806ecc55
Question: `procoder wizard` (#192): what shape may it take?

Answer: Copy the `procoder run` shape — print by default, execute under an explicit flag, refuse rather than guess. Shipped in #228 as declarative markdown that executes nothing.

## [spec] auto-copilot-leak

Key: da2ab5da8026
Question: Q1: Should COPILOT-LEAKS.md be merged into LESSONS.md, or kept separate? Keeping it separate means two files to check; merging means the lessons check sees raw Copilot findings that need classification first. -- A: Keep separate. Raw Copilot notes need a human step to become a real lesson (classify the class: mechanical/judgment/taste, name the adaptation). Merging skips that step.

Answer: Keep separate. Raw Copilot notes need a human step to become a real lesson (classify the class: mechanical/judgment/taste, name the adaptation). Merging skips that step.

## [spec] marketplace-strategy

Key: de6d0d01265d
Question: Which sections of this spec's Criteria table are release blockers and which are follow-ups? [C-07] cannot be met on our own schedule — it depends on other people's review queues.

Answer: Keep open, deliberately. The spec is not gating this release, so the

## [spec] auto-copilot-leak

Key: e3a6495c7e4b
Question: Q3: The `copilot[bot]` — should we also check for `copilot[bot]` as issue author? Some instances use `copilot-preview[bot]`. -- A: Check for authors matching `copilot.*\[bot\]` regex. Cover both variants.

Answer: Check for authors matching the `copilot.*\[bot\]` regex, covering both `copilot[bot]` and `copilot-preview[bot]`.

## (no longer asked)

Key: e41c4108da55
Question: Where does the pi adapter get installed from?

Answer: Global — `pi install git:github.com/azrtydxb/procoder@vX` at user scope. Nothing committed into a governed repository; the adapter resolves its launcher from its own module URL, so it never depends on what is on PATH.

## (no longer asked)

Key: e721d4e97e3f
Question: Do #210 now, before #193?

Answer: no — #193 as planned

## (no longer asked)

Key: f13acf27a984
Question: Issue #117 (procoder as a service): the perf case is inverted — what now?

Answer: Verbatim: "i don't care about the speed." The measurements are not the

## (no longer asked)

Key: f4733ae6d75a
Question: How many cached plugin versions does `procoder prune` keep?

Answer: 2 — active + 1 previous
