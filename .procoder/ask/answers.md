# What a human decided

Written 2026-08-31 18:31 UTC. procoder reads this
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

## [spec] marketplace-strategy

Key: de6d0d01265d
Question: Which sections of this spec's Criteria table are release blockers and which are follow-ups? [C-07] cannot be met on our own schedule — it depends on other people's review queues.

Answer: Keep open, deliberately. The spec is not gating this release, so the

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
