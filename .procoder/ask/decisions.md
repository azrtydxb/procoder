# Decisions waiting on a human

## Close #206, whose premise does not hold for procoder?

#206 asks for ReDoS defence — bounded, sandboxed regex evaluation — because
procoder "evaluates regex in several places that ultimately read
repo-controlled or config-controlled text".

Checked rather than taken on trust, the way #201's premise was:

- there is no `regexp.Compile` anywhere in the tree;
- the only two `MustCompile` calls with a non-literal argument build their
  pattern from constants in source (`gitx.aiIdentities`,
  `security.pyDeps`);
- `.procoder/lint/RULES.md`'s `checks` list is passed to clang-tidy as its
  `--checks` value. It never reaches a Go regex engine.

So no repository- or config-controlled text becomes a pattern procoder
compiles. The exposure the issue describes is not there.

- close it, with the evidence recorded, and reopen if a runtime-compiled
  pattern is ever introduced.
- keep it open as a standing constraint on future work — a reminder not to
  add one.

## Do #210 now, before #193?

`docs/influences.md` credits superpowers, ponytail and serena. BMad appears
in it zero times, while `internal/planning/bmad.go` has shipped since 2.0.0
and `[planning] method = "bmad"` reads BMad's own artifacts. Verified: the
integration is real and the doc gap is real.

Small, factual, and the trademark constraint is already understood — BMad
is named to describe interoperation, never as a procoder feature name.

- do it now: ten minutes, and it closes a verified gap in a provenance
  record that is currently wrong by omission.
- after #193: #193 was already chosen as the next piece of work.

## Label #209 and #211 roadmap alongside #194?

All three are documentation-positioning work: an evidence bibliography, a
comparable-projects doc, and the docs-hardening issue already labelled
roadmap. They are one cluster and none is small.

- label both roadmap: the cluster reads as direction rather than queued
  work, which is what the label was created for.
- leave them unlabelled: they stay in the ordinary queue.

## Remove the cached 3.1.0 plugin too, or keep it as the rollback?

**Decided: keep 3.1.0.** The rollback is worth 45 MB; prune's
active-plus-one-previous policy stands as written.

`procoder prune --apply` removed 3.0.0 and reclaimed 45 MB. It kept 3.1.0
deliberately: the policy is the active version plus one previous, so a
release that misbehaves has somewhere to fall back to. Removing 3.1.0 is
outside what prune will do — it would be a manual `rm -rf` of
`~/.claude/plugins/cache/procoder/procoder/3.1.0`.

The trade is a fixed ~45 MB against the one-step rollback. Note that 3.1.0
is now three releases behind and predates `release.maintainers`, so as a
rollback target it would reintroduce the false positive that blocked the
3.3.0 release commit.

- keep 3.1.0: prune's own policy, and a rollback that costs 45 MB.
- remove 3.1.0: reclaims the space; rollback then means reinstalling from
  the marketplace rather than a local directory.

## `procoder wizard` (#192): what shape may it take?

CORRECTED after reading the code. An earlier pass here claimed #192
contradicts AGENTS.md and that #200 shipped a fingerprinted approval to
gate it on. Both were wrong, and both came from grepping a summary line
instead of reading what shipped.

The rule reads "never executed AUTOMATICALLY", and it names `procoder run`
as the shape to copy: prints the candidates, executes only under `--exec`,
refuses when more than one candidate exists. `procoder run --exec` already
executes a command the repository declared. So a human typing
`procoder wizard run <name>` is the sanctioned shape, not a violation.

What #200 actually shipped is ten lines in internal/runcmd/runcmd.go that
print which binary `--exec` resolved to on PATH. There is no fingerprint
store and no approval record. Anything needing one builds it first.

**Decided: the `procoder run` shape.** Print by default, execute under an
explicit flag, refuse rather than guess. No new boundary, no new mechanism.

- build it in the `procoder run` shape: print by default, execute under an
  explicit flag, refuse rather than guess. No new boundary needed.
- build the fingerprinted approval first, then the wizard on top of it.
- close #192: the manual procedures it targets (#66-#73) are one-offs.

## What is next after v3.3.0?

**Decided: all of them.** Working the six roadmap issues in dependency
order: #194, then #211, then #209 (the docs cluster, which cross-reference
each other), then #192, then #189, then #190 last as the largest.

The ordinary queue is empty; all six open issues are roadmap-labelled.

- #189 SKILL.md hardening: smallest, and its own research note calls it the
  single highest-leverage change. Three of its four criteria still apply.
- #194 + #211 docs cluster: pitfalls tables, honest-limits, positioning,
  comparable projects. Medium, low risk, aimed at a skeptical adopter.
- #209 research bibliography: needs real external citations, verified. The
  one item where the failure mode is fabricating a source in a document
  whose entire purpose is evidentiary honesty.
- #190 `procoder learn`: largest by far, and the issue itself says it needs
  a spec before any implementation.

## #211 asks for a claim the issue record contradicts

**Decided: write it accurately.** Sources are named as sources; the
convergence argument is made only where it holds.

#211's acceptance criteria require a "framing paragraph [that] makes the
'independent convergence as evidence' argument explicit" about unlazy,
addyosmani/agent-skills and mattpocock/skills.

Checked against this repo's own issue record, that framing is false for
those three. #199 (leases), #200 (approval binding), #201 (never-execute),
#202 (dispatch waves) and #207 (depth scaling) each open with
"## Source — Leonxlnx/unlazy's ..." and were all filed on 2026-08-26 in one
research sweep. #189's anti-rationalization table cites addyosmani's repo
the same way. Procoder did not converge on these independently; it read
them and took them, deliberately and recently.

Verified against each project's own repo today: unlazy does ship the Depth
Tree, lease coordination, a Stop hook, and pre-execution PATH disclosure;
addyosmani/agent-skills does ship per-skill rationalization tables, a
docs/comparison.md and a docs/skill-anatomy.md.

A convergence claim is still available, but only for what procoder had
BEFORE the sweep — the gate, the quality controllers, evidence-gated
closes — which unlazy arrived at separately. That is a narrower and
checkable claim.

- write it accurately: name unlazy and agent-skills as SOURCES in
  influences.md (a fifth relationship: borrowed from, recently), and make
  the convergence argument only for the pre-sweep features where it holds.
- follow the criteria as written: make the broad convergence argument.
  Publishing a claim this repo's own issues contradict, in a document whose
  purpose is credibility.
- split: comparable-projects.md lists the projects factually with no
  convergence framing; the argument moves to #209's research page or is
  dropped.

## The gitleaks batching premise was wrong — batch, or run concurrently?

CORRECTION. The decision to "batch above a threshold" was taken on my
figure that one whole-tree gitleaks call costs about a second, from a CI
probe. Measured locally it costs 27.8s, and the CI step carried `|| true`,
so whether it scanned anything at all is unknown.

The real local numbers: per-file 0.05s × 787 = 39.4s, whole tree 27.8s. A
30% saving, not the ~290s I implied. And the whole-tree scan reads `.git`,
`dist` and ignored directories the per-file scan never looks at — it finds
115 leaks there, all outside the tracked set.

Which of the ~240s unaccounted for in CI is gitleaks remains unproven. I
attributed it to per-file startup; that was inference, not measurement.

- run the per-file scans concurrently: attacks process startup directly,
  scans exactly what it scans today, helps whether the leg is 41s or 240s,
  and is the pattern already used for the formatter pass.
- batch above a threshold as originally agreed: fewer processes, but 30%
  locally rather than the number the decision was made on, and it changes
  WHAT is scanned.
- measure gitleaks in CI properly first, then decide: one more probe cycle
  before any code changes.

## The tracked-tree gate is 787 gitleaks process startups — batch it?

`Secrets` calls `scanOne` once per path, and `scanOne` starts a gitleaks
process. Over 787 tracked files that is 787 startups. A probe scanned the
whole tree with ONE invocation in about a second, which accounts for the
~290s of the 341s CI step that four other explanations did not.

`SastChanged` in the same file already solves this shape: scan the tree
once, then filter the FINDINGS to the paths asked about, rather than naming
targets. Its comment says why — naming targets made semgrep scan files its
own default selection skips.

- batch above a threshold: one scan when the path set is large, per-file
  when it is small. Keeps a three-file commit from scanning a monorepo.
- always scan the tree and filter, like SastChanged: one rule, no
  threshold, no heuristic to tune — and a large repository pays it on
  every commit.
- leave it: the local cost is 41s and only CI sees the full tree.

## PR #241 is a probe that must not merge

It adds timing steps to the gate job and exists only to answer where the
441s went. It has answered.

- revert the probe steps and close #241 unmerged.
- keep the runner-cores line, drop the rest: the cores line is a permanent
  diagnostic; the PROBE steps are not.

## Issue #107 is not reachable — close it, or ship the comment too?

Probing the real host inverts the issue. OpenCode and Kilo are Bun binaries;
Bun auto-detects module format and ignores `type`. A probe project with a
`type`-less `.opencode/package.json` loaded BOTH an `import`-using plugin and
one mixing `require()` with `export default` — which Node rejects under every
setting. The reported failure cannot occur in a host.

Worse, the discovery glob in both loaders is `{plugin,plugins}/*.{ts,js}`:
`.mjs` is NOT matched, so renaming would silently stop Kilo loading procoder.
The `.js` extension is load-bearing. And `.kilo/package.json` is Kilo-generated
and gitignored (`.kilo/.gitignore:2`), so the proposed fix cannot ship at all;
a root-level `"type"` works only until Kilo writes that file.

The only real casualty is procoder's own Node harness
(`TestEveryHostAdapterLoadsWithACallableDefault`), which leans on Node's
syntax-detection fallback for the Kilo entry alone.

- close #107 with the evidence, and add a comment to the plugin header saying
  why the extension is `.js` and must stay `.js` — the rename risk is the only
  live one, and nothing in the repo records it.
- close #107 with the evidence and change no code: a rename already fails
  loudly (`portability_test.go:307` t.Fatals, `adapter_test.go` names the
  missing path), so the comment is belt on braces.
- comment the findings on #107 but leave it open: keep it as the record until
  someone with a real Kilo install confirms the fork matches OpenCode here.

## Issue #60: who sends the awesome-ai-plugins listing PR, and how far do we go?

The blocking work is one README line in `hashgraph-online/awesome-ai-plugins`,
under `## Community Plugins` → `### Development & Workflow`, between `Praxis`
and `Project Autopilot`. Three hard gates: case-insensitive alphabetical order,
the entry regex, and a reachable public repo — the last already passes.

Everything else they describe is optional and declared so in their own
CONTRIBUTING.md: scanner CI in procoder's own workflows (absence costs 10% of a
trust score, blocks nothing), the local `plugin-scanner` preflight, the HOL
registry ownership claim (a read-only GitHub OAuth grant), and the trust/Guard
badges.

- I fork and open the listing PR now, listing only, no scanner CI and no
  registry claim — the smallest thing that satisfies what was already agreed
  on the issue.
- I prepare the branch and the exact diff, and you send it: the PR is outward
  facing under your name in someone else's repository.
- listing PR plus the pinned-SHA scanner action in procoder's CI: takes the
  full trust score, at the cost of a third-party action in the pipeline.

## Issue #117 (procoder as a service): the perf case is inverted — what now?

Measured, 3.4.0 binary, 25 runs after warmup: bare spawn floor 2.9ms;
`procoder hook pre-tool-use` direct 9.2ms; `curl` to a local HTTP service —
the proposal's own hook command — 11.1ms; via `launcher.sh` 25.8ms;
`principles --hook` 194ms direct, 235ms via launcher.

A hook command is spawned either way, so the service pays a spawn PLUS a TCP
round trip and comes out slower than the binary it replaces. The proposal's
"~10-50ms to ~2-5ms" is wrong at both ends.

"Stateless" conflates process state with system state: `.procoder/` already
persists adr, ask, backlog, plans, specs, state, todo, index, security, bench.
The real gap is that nothing is user-global — no `~/.procoder`, no
`UserConfigDir` anywhere in `internal/`. That is a question of where the store
lives, not whether a daemon fronts it.

Three items survive: cross-repo memory (small, no daemon), a shared code-index
cache (a cache-location problem), and inter-repo coordination (the only one
that genuinely wants a long-lived process, and the least specified).

- close #117 and open one narrow issue for the user-global store: take the
  gap that is real, drop the architecture that was proposed to reach it.
- comment the measurements on #117 and rescope it in place to "cross-repo
  state without a daemon", keeping the discussion in one thread.
- comment the measurements and leave #117 open as written: the inter-repo
  coordination case is unproven either way and may still want a service.

## How do the 34 command files reach pi?

`package.json` declares `pi.skills: ["./commands"]`. Measured, that yields one
skill named `commands` — pi falls back to the parent directory name when a
skill has no `name:` frontmatter, so all 34 files collide and it keeps the
first (`adr.md`) and warns about the rest. The other 33 are invisible.

They are also not skills: each is a user-typed command that expands `$ARGUMENTS`
into a shell line. pi has exactly that shape, as prompt templates. But 33 of the
34 invoke `"${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"`, and pi sets no plugin-root
variable at all (grep for `PLUGIN_ROOT` in pi's dist: zero hits), so the line
expands to `/hooks/launcher.sh` and fails.

- Register them at load as extension commands, reading `commands/*.md` and
  substituting the launcher path the extension resolves from its own module
  URL: one source of truth, ~40 lines of glue, no second copy to drift.
- Emit a pi-specific `.pi/prompts/*.md` at release time and pin it with the
  portability drift guard, as the rule-file copies already are: 34 more files,
  and the drift guard grows to cover generated command text.
- Reword the canonical `commands/*.md` to name `procoder check` rather than the
  launcher path, and make each host adapter responsible for putting a correct
  `procoder` in front of it: cleanest text, but it changes what Claude Code runs.

## Where does the pi adapter get installed from?

Every rule-file host gets a committed copy under its own path; every plugin-tier
host gets a manifest and an install step. pi can take either shape, and the
choice decides whether a governed repository carries pi files at all.

- `pi install git:github.com/azrtydxb/procoder@vX` (user scope): one install per
  machine, works in every repository, nothing committed, and the version is
  pinned by hand rather than by the repo.
- `pi install -l` writing a committed `.pi/settings.json`: the repo declares its
  own procoder version, matching how `[release] files` pins versions; needs
  project trust, and every adopter runs the install after cloning.
- A committed `.pi/extensions/procoder.ts` copy, byte-identical to
  `pi-extension/index.mjs`, pinned by the drift guard like the other rule files:
  zero install step, and a copy to regenerate in every governed repository.

## Does the pi adapter match the Claude hook set, or use what pi offers?

pi can do two things Claude Code's hook set cannot. `tool_result` lets the
post-write findings land inside the write's own tool result instead of as a
side-channel `additionalContext`, and `agent_settled` fires per turn rather than
only at session end, so the handoff note and the unasked-decision block can run
every turn.

- Hook parity first: the four hooks, nothing more, so one comparison across
  hosts stays honest and the pi row in docs/portability.md means what the
  Claude row means.
- Go further now, and record in docs/portability.md that pi gets strictly more
  than Claude does — accepting that "supported on host X" stops being a
  comparable claim across the host table.

## `procoder format` prints content in one of four verdicts — fix the command, or the habit?

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

## Which of the pi integration's claims are backed by a measurement?

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

## The ask queue reports 13 open questions; four of them look answered already. Fix the queue or answer the list?

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

## The principles hook delivers 2KB of a 10KB document — what do we do about it?

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

## What lands in the first `feat/command-api` commit?

`.procoder/specs/command-api.md` is written and passes its controller
(COMPLETE — sections answered, no open questions, criteria testable), the
gate is clean, and it sits uncommitted on `feat/command-api`. It carries 11
scope items and 13 acceptance criteria.

The spec's own controller names the next step: seed one task per criterion
group from the 13 criteria. Doing that before the commit makes the branch
sprint-ready in one change; doing it after keeps the spec reviewable on its
own.

- commit the spec alone, so the widening of #117 D5 is reviewed before any
  work is decomposed against it.
- seed the backlog from the 13 criteria first, and commit spec and stories
  together as one sprint-ready change.
- leave it uncommitted and change the spec first — the scope that widens
  #117, or the three deferrals (structured output, interactive commands
  over the socket, the team server).

## Which part of `command-api` changes before it is committed?

The spec is complete and uncommitted; the call was to revise it rather than
commit or decompose it. Four candidates, and they are independent — more
than one may apply.

- the #117 relationship: the spec currently supersedes D5 outright, so all
  47 commands are served. It could instead stay hooks-only, or keep full
  parity but be filed as its own issue rather than a rewrite of #117's
  scope.
- structured output: currently out of scope, so the response carries the
  same prose bytes the CLI prints today. Bringing it in gives callers typed
  results and makes the byte-identical parity test unwritable as specified.
- the six asking commands: currently out of scope, so they take their
  non-interactive path over the socket. A confirmation field in the
  envelope would make them reachable.
- the team server (#248): currently out of scope. In scope would mean a
  network transport, auth and multi-user, which is a different spec.
