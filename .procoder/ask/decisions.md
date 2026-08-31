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
