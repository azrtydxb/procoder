# Changelog

Every release, in words a user can read. Newest first.

## 0.32.10 — 2026-08-20

**The command reference's table was not a table.** The skills list
shipped in 0.32.9 with data rows and no header row, so Markdown never
made it a table — it rendered as a wall of text with literal pipes in
it, and the formatter reflowed it into a paragraph, which made it worse.
Fixed, with the header row it always needed and a command column that no
longer breaks `/procoder:agents` across two lines.

**The page filters now.** The reference is long by design — 33 skills
plus every binary subcommand — and the site search takes you to a
different page rather than narrowing the one you are on. A filter box
sits under the title: type `secrets` or `rename` or `sprint close` and
the page narrows to what matches, across the skills table and the
binary sections at once. Section headings whose contents all filtered
away hide too, so you never get an empty "Everyday commands". Escape
clears it, and with JavaScript off the page is simply the full list.

Two things found while testing it in a browser rather than by reading
the code: the `hidden` attribute loses to Material's own `display` rule
on tables, so the emptied table still painted its header; and a link
into `architecture.md` had an anchor that no heading generated.

## 0.32.9 — 2026-08-20

Documentation, after reading it as a user rather than as its author.

- **Serena joins the provenance map.** [Influences](https://azrtydxb.github.io/procoder/influences/)
  covered superpowers and ponytail but not serena, whose navigation half
  Procoder took into the binary — `index find` / `refs` / `outline` /
  `callers` / `impls` / `rename`, plus `lint --types` and `.procoder/` as
  the memory that survives a lost context. Serena's symbol-level **write**
  tools are listed under what was deliberately not adopted: the binary
  computes the rename and hands over the diff. The README now says
  outright that all three plugins can be uninstalled, because running
  them alongside Procoder puts two sets of instructions in front of one
  agent.
- **The tutorial installs once.** It read as though the plugin install
  were step one of seven, with a clone and a `PATH` export after it. For
  Claude Code the marketplace install is the whole thing, and the tools
  step (`/procoder:init`) was missing entirely. The manual path — the
  binary on `PATH`, the agent contract, the git hook — is now its own
  how-to for the agents that need it.
- **The command reference leads with the commands you run.** All 33
  `/procoder:` skills in one table, then the binary underneath for
  scripting CI, other hosts, and debugging Procoder itself. The
  instructional pages call the skills; `procoder templates` is gone from
  the onboarding guide, since `/procoder:audit` writes those files.
- **Tone.** The word "honest" is out of the documentation. The behaviour
  it described stays exactly as it is — a file that could not be checked
  is never called clean — but a product that keeps calling itself honest
  is telling the reader what to think of it.

## 0.32.8 — 2026-08-20

**Documentation now has a shape, not just a checklist.** The docs domain
enforced that documentation exists — required files, badges, a README
first screen, no broken links. It said nothing about what a page should
be, so every page invented its own shape and drifted toward serving
three readers at once.

The shipped `.procoder/docs/RULES.md` now carries the
[Divio documentation system](https://docs.divio.com/documentation-system/):
four kinds of document, never mixed, the kind decided before the first
line.

| Kind         | Serves                        | Answers                 |
| ------------ | ----------------------------- | ----------------------- |
| Tutorial     | a newcomer learning           | "teach me"              |
| How-to guide | a competent user working      | "how do I X?"           |
| Reference    | someone looking a thing up    | "what are the options?" |
| Explanation  | someone wanting to understand | "why is it like this?"  |

Each has a characteristic failure, and the rules name them: a tutorial
that stops to explain trade-offs loses the learner; a how-to that
teaches from scratch wastes a reader who already knows; reference that
argues cannot be trusted to describe; explanation carrying steps rots,
because the steps then live in two places.

Alongside it, the writing rules that follow: answer first, examples over
prose about examples, real names rather than `foo`, sentences under
fifteen words, scannable structure, the searchable synonym included, and
an explicit "common pitfalls" list wherever a feature has a known
misuse. `/procoder:docs` now applies all of it when it WRITES, where
before it only checked what already existed.

Every word of this is repo-overridable (D-OVERRIDE) — replace it with
your own house style and your copy wins. None of it blocks a commit; it
is guidance the agent follows, and the reasoning is recorded in
ADR 0002 along with the mechanical enforcement that was considered and
rejected.

**The site was then rebuilt to follow it.** The nav is grouped by kind,
and every page says which kind it is in its opening sentence.

- `getting-started.md` is a tutorial: install, watch the gate refuse a
  commit over a merge conflict, fix it, watch it pass. Every shell
  command in it was executed in order in a clean repository and the
  output blocks are that run, verbatim.
- `workflow.md` became **How to ship a change** — eleven numbered steps
  and a common-pitfalls list, with no paragraph arguing for the design.
- `how-to-onboard.md` is new: the audit path for a repository Procoder
  has never governed, which used to be a step inside the tutorial.
- `quality-chain.md` became explanation only, and now admits what
  refusal costs when a controller is wrong — the section it was missing.
- `commands.md`, `configuration.md`, `domains.md` and `portability.md`
  are reference; `architecture.md` and `influences.md` are explanation.

**The diagrams were rebuilt to the brand.** The site had exactly one
diagram and three ASCII-art blocks, and the shared Mermaid theme was
still the old teal. There are now five, all Mermaid, all on the brand
palette in both light and dark: the write-hook loop on the overview, the
quality chain with its refusal loops, the three-layer architecture, the
ship-a-change sequence, and the onboarding triage order.

Colour comes from the theme and meaning comes from shape — rounded for a
start or end, rectangle for a step, diamond for something that decides.
Hard-coded fills were tried first and rejected: they cannot follow a
reader who switches to dark mode. The rules file now says so, along with
the rest of what makes a diagram worth having.

**A gate gap surfaced while writing the tutorial.** `procoder check`
blocked on the tutorial's own conflict-marker example, and every
workaround was bad: mangle the sample so a reader who copies it gets
broken text, drop the topic, or turn the check off. A document whose
subject is merge conflicts now declares it:

```
<!-- procoder:allow-conflict-markers this tutorial shows what a conflict looks like -->
```

The reason is required — a bare token exempts nothing, because that is
silencing a check rather than documenting an exception. The exemption is
file-scoped and explicit rather than "skip fenced code blocks", since a
real conflict lands inside a fence often enough that skipping fences
would be a silent miss.

## 0.32.7 — 2026-08-20

**The product is spelled Procoder**, taken from the wordmark. The logo
reads _Procoder_, so every word of text around it does too: the README,
the documentation site and its header, the brand guide, the rules every
agent reads, and the engineering principles injected at session start.
The artwork is the authority — a name is a picture people recognise
before it is a string they parse, and text is the cheaper thing to
change.

Everywhere a machine reads the name it stays `procoder`, unchanged and
unchangeable: the binary, the package, the plugin id, `.procoder/`,
every command, and every URL. That distinction is the whole rename;
nothing executable moved.

## 0.32.6 — 2026-08-20

**`procoder maintain` was silently dropping every function-length
finding.** golangci-lint keeps only the first issue per line by
default — and a long function is usually a branchy one, so funlen and
gocyclo land on the same line and funlen loses. On this repository that
meant 31 complexity findings, 0 length findings, over a dispatch
function 343 lines long. The generated config now sets
`uniq-by-line: false`, and the two linters say their different things
about the same function. Seven length findings appeared here the moment
it was fixed.

A report that quietly drops half of what it found is worse than one
that says NOT checked, because nothing tells the reader anything is
missing. Same family as the honesty rule, opposite direction.

Then the report was taken at its word:

- **One findings printer instead of four.** `ci`, `infra`, `security`
  and `lint` each carried their own copy of the same render loop —
  mark, location, message, count line, exit code — which is how three
  of them drift while the fourth gets fixed. They now share
  `printFindings`, which has tests of its own.
- **`procoder lint` and `procoder security` now print repository-relative
  paths**, as `ci` and `infra` already did. Paths outside the repository
  are still printed as given rather than as a climb of `../..`.
- **`adr`, `lint` and `test` moved out of the dispatch switch** into
  their own functions, following `indexCmd` and `backlogCmd`. `run` drops
  from cyclomatic complexity 113 to 73 and from 269 statements to 159.
- The `func(s string) { fmt.Println(s) }` closure, written out 23 times,
  is now `printLine`.

## 0.32.5 — 2026-08-20

**`procoder deps` no longer reports an empty shelf as an unread one.**
The honesty rule — a thing that was not checked is never reported as
clean — is what makes the tool worth reading, and it was firing in a
case it was never meant for. A repository with no third-party
dependencies has no license surface at all, so
`licenses (go): NOT checked — go-licenses is not installed` pointed the
reader at a gap that did not exist. A reader who learns to skim that
line will skim it in a repository where it means something.

The report now separates the two:

- **Nothing to check** — the manifest declares no third-party
  dependencies — reads `licenses (<eco>): no dependencies to report`.
- **Something unchecked** — dependencies exist and no tool read them —
  keeps the NOT-checked line and its install hint, exactly as before.

Where procoder cannot tell, it says NOT checked. Go reads `require`
directives (block and single-line, indirect included); js reads
`dependencies`, `devDependencies`, `peerDependencies` and
`optionalDependencies`; Rust reads the dependency tables in
`Cargo.toml`. Python answers only for a pyproject-only repository — a
`requirements.txt`, a `Pipfile`, or a `setup.py` computing its
`install_requires` at runtime cannot be read off the text, so procoder
declines to answer rather than guess "none". A manifest it cannot parse
answers the same way. And a dependency the native tool reports as
behind is a dependency whatever procoder made of the manifest text, so
the report can no longer contradict itself in the same breath.

## 0.32.4 — 2026-08-20

A test sweep across the codebase, driven by mutation rather than by
coverage: every test written here was proved by breaking the code it
covers and watching it fail. Total statement coverage 64.2% → 71.9%,
with the weakest packages carrying the change.

| package  | before | after  |
| -------- | ------ | ------ |
| host     | 0.0%   | 100.0% |
| doctor   | 0.0%   | 93.7%  |
| audit    | 8.8%   | 95.6%  |
| tools    | 20.8%  | 81.9%  |
| gitcmd   | 27.4%  | 83.7%  |
| deps     | 38.2%  | 77.5%  |
| security | 39.8%  | 89.2%  |
| config   | 47.1%  | 98.0%  |

- **`procoder debt` no longer calls sound debt rot.** A marker's revisit
  condition routinely lands on a continuation line, because the marker
  line is already full of what the ceiling is. The harvester judged the
  first line alone and flagged every such marker as having no trigger —
  including two written the same day, following the principles exactly.
  The whole comment block counts now; the ledger still shows the first
  line, which is the summary.
- **A fix from 0.32.3 was incomplete.** `maintain`'s file survey still
  swallowed an unreadable root, so the walk error it recorded was always
  nil and the ecosystem was silently skipped. The test written for that
  behaviour is what found it.
- Two branches that cannot be tested without adding a seam — a failed
  rename beside a successful write, and a failed Close — are recorded
  as debt with the condition that would make them testable, rather than
  covered by a test that proves nothing.

## 0.32.3 — 2026-08-20

The audit run on procoder itself, and the four honesty gaps it found in
procoder's own code.

- **The sweep no longer asks diff-scoped questions.** `audit` passes
  every tracked file, so doc-drift and the documentation obligation —
  both of which ask about a CHANGE — answered about everything at once:
  129 of the hygiene section's 138 findings were that noise. The
  diff-independent half is `docs.CollectSweep` now, and the hygiene
  section on this repository went from 138 findings to 9. The diff path
  is untouched.
- **A swallowed walk error made "could not look" read as "nothing
  there."** infra, docs and maintain skipped an unreadable directory and
  kept going, which is right, but they swallowed an unreadable ROOT the
  same way — producing an empty inventory a caller would take for a
  clean answer. The root is now distinguished and reported; `maintain`'s
  file predicate errs toward running the check rather than silently
  skipping an ecosystem.
- **A failed index rename left a stale index and litter.** The atomic
  swap's error is checked; the temporary file is removed so a failed
  write does not accumulate one beside the index on every attempt.
- **A rule the codebase already knew, applied consistently.** The review
  rubric says a failed Close after a write IS a failed write; `lintGo`
  honoured it and `lintJS` and `maintain` did not. All three do now.
  Read-handle closes are deliberately left: closing a file you only read
  cannot lose anything.

## 0.32.2 — 2026-08-20

Two defects found by using the tool, plus the first benchmarks.

- `procoder bench` reported a successful run with no benchmarks as
  `NOT run … exit 1`. The detector is a `git grep` for `func Benchmark`,
  and a grep cannot tell a benchmark from those words inside a fixture
  string — which internal/bench's own test file contains. The run is the
  authority now: zero rows from a successful run means there are no
  benchmarks, said plainly, exit 0.
- The documentation obligation could fire unclearably. procoder's own
  store under `.procoder/` was excluded from CLEARING an obligation but
  not from RAISING one, so a bug story naming the file it fixes demanded
  documentation that no edit to that story could ever supply. The
  exclusion is symmetric now.
- The first benchmarks, over the two paths that run on every write and
  scale with the repository rather than the change: `docs.Drift` across
  a 200-page corpus and `codeindex.Refresh` over a 1500-entry index.
  Both measure ~10ms; the baseline is committed so a future change that
  makes either ten times worse is caught rather than felt.

## 0.32.1 — 2026-08-20

- Internal cleanup, no behaviour change: four text helpers that had been
  copied across packages now have one definition each in
  `internal/textutil` — `slugify` (three byte-identical copies),
  `section` and `stripComments` (five each), and the seven `firstLine`
  copies whose semantics matched. The five `firstLine` variants that
  differ on purpose keep their own, because moving those would change
  output under the cover of a cleanup. Net: 371 lines deleted, 81 added,
  and the shared helpers have tests the copies never had.
- `docs.CollectOffline` and `docs.Run` are gone: both had zero callers
  after the gate moved to the config-aware variants.
- This repository now sets `[docs] policy = "block"`, which the 0.30.0
  notes claimed but the config never carried.

## 0.32.0 — 2026-08-19

- `procoder backlog close story <id>...` takes several ids and verifies
  ONCE. The gate and the test suite judge the tree, not a story, so
  asking them per story re-ran the same answer N times: closing a
  27-story sprint on this repository cost about 729 seconds of identical
  re-verification, and now costs 27. Each story is still judged on its
  own criteria and evidence, and an incomplete one is refused by name
  without costing the others their close. The single-id form is
  unchanged, asserted by a test comparing both forms' output;
  `close epic` and `close milestone` still take exactly one id.

## 0.31.0 — 2026-08-19

The loop: the daily-workflow gaps the analysis found.

- `procoder test --name <pattern>` narrows the run to matching tests
  (`-run`, `-k`, `--tests`, `-Dtest=`, cargo's positional, the pattern
  after `--` for a JS script). A runner that cannot express the pattern
  reports NOT filtered instead of silently running everything, and zero
  matches is an honest pass that says so.
- `procoder run [--exec]` answers "how do I run this project": every
  launch command the repository declares, with the file and line that
  declared it, most specific first. procoder does not manage processes
  — a server belongs to the shell that owns it — so `--exec` runs only
  a single one-shot candidate and refuses when there is a choice or the
  command looks like a server.
- `procoder status` — the state of play, computed fresh: branch, dirty
  files, the active sprint and its open stories, open tasks, unlearned
  lessons, index freshness. The same block is injected at session start
  inside a hard three-second budget (measured: 77ms on this repository),
  so a resumed session opens knowing where the project stands instead of
  re-deriving it.
- Stop and PreCompact hooks write `.procoder/state/handoff.md`: the same
  facts plus HEAD and a timestamp, with a Notes section the agent owns
  and the writer never touches. Facts only — the note never guesses at
  intent.
- `procoder env [--sync]` — what moved under you since the last sync:
  lockfile digests per ecosystem with the install command, migrations
  added or removed, and new keys declared in an `.env.example`. Key
  names only; no value from either file is ever printed. Files git
  ignores are never surveyed.
- `procoder ci --runs` — this branch's newest run per workflow via gh,
  with the failing job names, and the line that matters most: the newest
  run predates your latest push, so CI has not judged this commit.

## 0.30.0 — 2026-08-19

Enforcement: the two promises procoder made and did not keep.

- **The commit gate is no longer voluntary.** A PreToolUse hook
  intercepts `git commit` and stops it while the gate has blocking
  findings, handing the agent the exact work list. `git commit
--no-verify` still passes — loudly, never silently — and
  `[git] commit_gate = "report" | "off"` downgrades or disables the
  interception (default block). `procoder hook install-git` prints a
  `.git/hooks/pre-commit` script so the gate also holds for commits
  made outside any agent. A clean gate deliberately emits no decision
  at all, so your own permission prompt is never bypassed.
- **The documentation gate is universal.** The old command-coverage
  check only ran inside procoder's own source tree and only grepped for
  its own command names; it is replaced by a change-driven obligation
  that works in any repository: a public-surface change (exported
  symbol, CLI flag, config key) or a change to a file documentation
  names, with no documentation changed in the same diff, raises an
  obligation. It clears by editing a doc or by recording the decision —
  `docs: none — <reason>` in the commit message, which the hook reads
  at the moment of the commit (`procoder docs --ack "<reason>"` prints
  the line). Silence never clears it. `[docs] policy = "block"` opts a
  repository in; the default stays report.
- `SurfaceCoverage` replaces the identity-gated check: exported surface
  no document mentions, in any repository, reported in `procoder docs`
  and deliberately kept out of the gate so the gate stays readable.
  AGENTS.md and root-level Markdown now count as documentation.
- **The docs backfill this rot created**: AGENTS.md gained the 19
  commands it never mentioned (and the ten derived host rule files were
  regenerated), configuration.md gained six missing config keys,
  domains.md became ten domains with Testing added and bench, deps and
  adr placed, workflow.md now describes the real sequence, and
  index.md, getting-started.md, README.md and the nav caught up. Four
  false claims were removed, including "start in a worktree", which no
  code ever implemented.

## 0.29.0 — 2026-08-19

- The engineering principles gain two sections: **ADHD/ASD-friendly
  formatting** and **output preferences**. Complex responses (multiple
  issues, decisions to make, long context to synthesize, mixed item
  types) get a title and one-line summary, type-labeled problem cards
  (Enhancement, Defect, Question, Blocker), a small related-context
  block, a numbered "decisions needed" list marking independent ones,
  and noise filtered out — with short single-topic answers skipping all
  of it, and "plain version" / "just the answer" turning it off for a
  response. Output preferences: shorter than you think, 2-4 sentence
  paragraphs, TL;DR on long documents, prose for formal content, two
  explicit versions when two audiences need one document, full code
  blocks, tables for comparisons only. As ever, a repo replaces the
  principles wholesale via .procoder/PRINCIPLES.md.

## 0.28.0 — 2026-08-19

Daily practices, complete: the six remaining gaps from the
what-a-real-dev-does review, shipped as sprint 002 of the Daily
Practices milestone (32 stories, all closed with evidence, milestone
closed).

- `procoder backlog bug <title> [--epic] [--severity s1..s4]` — a
  defect is a story with a severity and a pre-seeded regression-test
  criterion; closing without a severity is refused; the board marks
  open bugs and counts them.
- Sprint retrospectives: `sprint close` scaffolds a Retro (what slowed
  us, what we change, one adaptation), and `sprint open` refuses while
  the last retro is empty — the retro is the price of the next sprint
  (`[sprint] retro = "off"` opts out).
- `procoder release [<version>]` — the pre-tag controller: version
  sync across `[release] files`, the changelog entry, a clean tree,
  the gate, and the suite under [test] policy — every failure listed,
  the tag command printed, never run. This repo lists its nine
  version files.
- `procoder adr new|list|check` — architecture decision records under
  .procoder/adr/: numbered, immutable, superseded rather than edited;
  check refuses hollow records, bad statuses, and dangling supersede
  references; the audit sweep includes them. ADR 0001 records the
  stories-vs-todo decision.
- `procoder deps` — the freshness report per ecosystem via native
  tools (go list -u, npm outdated, cargo-outdated and pip where
  available), licenses where a tool exists — honest NOT-checked lines
  everywhere else, report-only.
- `procoder bench [--save]` — Go benchmarks against a committed
  baseline: ns/op and B/op deltas, regressions beyond [bench]
  threshold marked and exit 1, cross-platform baselines compared with
  a warning. The perf skill now drives it.

## 0.27.0 — 2026-08-19

The test domain: "done" finally runs the tests.

- `procoder test [--coverage] [paths...]` — every detected ecosystem's
  canonical runner (go test, cargo test, the package.json test script
  via the lockfile's manager, pytest, gradle/maven), each reported
  honestly: PASS with counts, FAIL with the failing tests named, and
  NOT run when a runner is absent — which is never the same as green.
- `--coverage` reports the percentage where the runner measures it
  natively (Go; pytest with pytest-cov). A number, never a threshold.
- `[test] policy = "block"` wires the suite into the close controllers:
  `todo close` and `backlog close story` refuse while the suite is red
  — or unverifiable, because unknown is never done. Without the policy,
  closes behave exactly as before. procoder's own repository adopts the
  policy.
- Built and tracked through procoder's own backlog: sprint 001 of the
  Daily Practices milestone, seeded from the test-domain spec.

## 0.26.0 — 2026-08-19

The project layer: lean/agile backlogs with sprints, on the quality
chain. Built spec-first with procoder's own spec and plan controllers.

- `procoder backlog` — milestones → epics → user stories under
  `.procoder/backlog/`, the home of a spec-first project. The story is
  the execution unit and carries todo-task rigor; the todo list itself
  stays untouched as the standalone list for work not born from a spec.
- `procoder backlog seed <spec>` decomposes a COMPLETE spec into an
  epic plus one story per acceptance criterion — everything printed for
  the agent to review and write. The epic records the spec and a
  fingerprint; the board flags `⚠ spec drift` / `⚠ spec missing` when
  traceability breaks.
- Refusing controllers all the way up: story close refuses without
  checked criteria, evidence, and a clean gate; epic close refuses
  while a story is open (and warns on drift); milestone close refuses
  while an epic is open. Unreadable files block conservatively —
  unknown is never done.
- `procoder sprint` — scope-boxed sprints: one active sprint at a time
  (the WIP limit), `pull` commits stories, `carry <id> <reason>`
  returns unfinished work to the backlog with the reason recorded, and
  `close` refuses while a committed story is neither done nor carried,
  then writes committed/done/carried counts into the sprint file. No
  story points, no calendar enforcement — stories are counted, the
  goal is the commitment.
- `procoder backlog board` — the tree with statuses, sprint tags,
  orphans, drift flags, and a one-line summary.
- The plan checker's placeholder rule is now case-sensitive for TODO:
  lowercase "todo" legitimately names procoder's own task domain, and a
  plan touching internal/todo must be writable.

## 0.25.0 — 2026-08-19

- `procoder index impls <symbol>` — what implements an interface or its
  methods, from SCIP implementation relationships. Precise tier only:
  the relationship exists nowhere else, so without SCIP the answer is
  "not built", never a textual guess.
- The precise tier goes polyglot: every SCIP indexer the repository's
  layout calls for runs (not just the first manifest match) and the
  results merge into one index. A missing or failing indexer costs only
  its own ecosystem, reported per indexer.
- CI now verifies the committed dist binaries match the manifest
  version on every platform leg — a version bump that forgot the
  rebuild fails the build instead of shipping stale binaries.
- `.procoderignore` deleted: the file was read by nothing; dead config
  that looks live is the rot the docs domain polices, applied to
  ourselves.

## 0.24.0 — 2026-08-19

- The engineering principles gain a delegation section — you are a
  lead, not a lone hand: independent work fans out to parallel
  subagents (launched together, not one by one), delegation goes where
  a fresh context does better under a clear contract (scope, owned
  files, output shape, definition of done; no shared file ownership),
  launched agents are watched and redirected early, and nothing an
  agent produced merges unjudged — verify claims against the code and
  run the gate over anything an agent wrote. As with the rest of the
  principles, a repo replaces them wholesale via
  .procoder/PRINCIPLES.md.

## 0.23.0 — 2026-08-19

Serena parity: the two capabilities that still needed the serena MCP
plugin now live in procoder, without giving up P-CONTROL.

- `procoder index rename <symbol> <new> [--at path:line]` — the
  cross-file rename as a reviewable unified diff, computed by the
  language's own engine (Go via gopls, which doctor now requires on Go
  repositories). Nothing is written: the agent reviews and applies the
  diff, then verifies with `index refs`. Languages without an engine
  answer honestly with the reference worksheet instead of a half-right
  rewrite; an ambiguous name lists every definition and asks for `--at`.
- `procoder lint --types` — the type-checker where the canonical linter
  does not compile the code: `tsc --noEmit` for TypeScript (grouped
  under each file's nearest tsconfig; without one the file is declared
  out of scope, never silently skipped) and pyright for Python. Go and
  Rust need no flag — golangci-lint and clippy already compile what
  they lint. Doctor requires tsc under a project tsconfig and pyright
  where Python is a real project (pyproject/requirements).
- The index skill now says when to reach past the index: a textual refs
  answer, same-named symbols, or interface implementations are the
  language server's job — use the host's native LSP tool and come back
  to the index for repo-wide sweeps.

## 0.22.1 — 2026-08-19

- The CLI help (`procoder` with no arguments) now lists every command
  alphabetically instead of grouped by workflow, so a command is found
  by name at a glance. Descriptions are unchanged.

## 0.22.0 — 2026-08-19

The language matrix: procoder now covers the popular languages end to end.

- Formatting adds Java (google-java-format), Kotlin (ktfmt), Swift
  (swiftformat, verified live), Ruby (rubocop autocorrect), Dart
  (dart format), and C# (csharpier) — joining Go, Python, the prettier
  family (JS/TS/JSON/CSS/HTML/Markdown/YAML), Rust, C/C++, and shell.
  Same contract everywhere: the project's config wins, the result is
  printed for the agent, unchecked is never clean.
- Lint adds cargo clippy (Rust, workspace-scoped, filtered to changed
  files), ktlint, swiftlint, rubocop, and checkstyle (google_checks
  baseline; a repo checkstyle.xml wins).
- The dependency scan enumerates the wider lockfile matrix: Maven and
  Gradle, .NET packages.lock.json, Swift Package.resolved, Elixir
  mix.lock, Dart pubspec.lock — one list shared with doctor, so the
  scanner requirement and the scan can never disagree. (Podfile.lock is
  deliberately excluded: osv-scanner has no extractor for it, verified.)
- The precise index tier adds rust-analyzer (Rust) and scip-java
  (Java/Kotlin/Scala builds); doctor recommends each only where the
  repository's files call for it.
- The broad index tier gains Swift and Dart: universal-ctags ships no
  parser for either, so procoder supplies regex-based definitions
  (top-level symbols, verified live) — every matrix language can now be
  found, searched, and outlined.
- Docs: an Influences page maps every idea absorbed from the superpowers
  and ponytail plugins to exactly where it lives in procoder; the
  quality-chain page now speaks its own name (spec-based development,
  design documents, quality gates) and carries a real verbatim
  spec-check refusal.
- Honesty note: Go/Python/JS/shell remain the continuously-tested paths
  (they gate this repo's own CI); Swift was verified against the live
  tool; the rest follow each tool's documented interface and fail
  honest — a wrong flag surfaces as NOT-checked, never as clean.

## 0.21.0 — 2026-08-19

The documentation overhaul, and the gates that keep it from rotting again.

- README rewritten from scratch: the whole product told value-first —
  the gate, the quality chain (spec → plan → todo), the self-learning
  loop, the nine domains, the code index, principles and debt, every
  agent — instead of a release-one story eleven versions stale.
- The docs site grew three pages (Getting started, The quality chain,
  Architecture) and a restructured landing; navigation now tells the
  product's story before its reference.
- The rot guards, because presence checks let this happen: a repo can
  declare its feature families (`## README must mention` in the docs
  rules) and a family the README stops telling blocks the gate;
  /procoder:pr gains the mandatory docs-impact question ("what does this
  change alter about what a reader must be told?") answered before any
  PR opens; and the review rubric carries the product-story line. The
  escape is recorded in the lessons ledger with all three adaptations.

## 0.20.0 — 2026-08-19

procoder now works with every AI coding agent, not just Claude Code.

- One canonical `AGENTS.md` carries the always-on contract; ten rule-file
  hosts (Cursor, Windsurf, Cline, Kilo Code, Roo Code, Kiro, Antigravity,
  Qoder, Copilot editors, Codex) get byte-pinned copies, and drift blocks
  the gate exactly like the PR-template mirror. `procoder agents` (and
  `/procoder:agents`) prints the content for anything missing or drifted.
- Plugin-tier adapters for Codex CLI (shares Claude's hooks file — the
  binary detects the host and answers in its JSON shape), GitHub Copilot
  CLI (own hook schema, bash+powershell), Gemini CLI/Antigravity
  (`contextFileName: AGENTS.md`), OpenCode (a thin JS shim plus generated
  command twins, parity pinned by test), Grok Build, Devin CLI, Qoder,
  pi, and Hermes. Adapter rule: adapters stay thin — logic lives in the
  binary, content in `AGENTS.md` and `commands/`.
- Host detection in the binary (`COPILOT_PLUGIN_DATA` → `PLUGIN_DATA` →
  `QODER_SESSION_ID` → Claude, with the VS Code Copilot path heuristic);
  `procoder principles --hook` emits each host's session-start shape.
- Manifest versions are pinned to the plugin version by the gate, and a
  root `hooks/hooks.json` is forbidden outright (Gemini would auto-load
  it with incompatible event names). Claude Code remains the tested
  reference host; the docs say so plainly.

## 0.19.0 — 2026-08-19

Catch first, learn on escape: downstream reviewers become the fallback,
not the net.

- The pre-PR self-review: `/procoder:pr` now dispatches a fresh-context
  reviewer over the branch diff against `.procoder/github/REVIEW.md`
  BEFORE the PR is opened; Critical/Important findings are fixed first.
  The default rubric is seeded from every class bot reviewers actually
  caught on this repo.
- The reflection loop: `/procoder:merge` treats an escaped finding as a
  bug in our gates — each one names the layer that should have caught it,
  that layer is adapted in the same PR, and the lesson lands in
  `.procoder/github/LESSONS.md`. `procoder lessons` flags entries with no
  adaptation as UNLEARNED (exit 1) — recorded is not learned. Our own
  ledger ships seeded with the eight real escapes to date: the PR #17/#18
  review findings, the CI mirror hang, and our own self-scan's fixture
  harvest.
- Go lint baseline: repositories without a golangci config get a curated
  default (standard set plus gosec, gocritic, errorlint, unparam,
  copyloopvar, nilerr) — the same pattern as the eslint baseline, and the
  repo's own config always wins.
- CI robustness: apt is repointed from the flaky Azure mirror to the
  canonical archive with fail-fast retries — a gate run once burned its
  whole timeout waiting on that mirror.
- Honesty fix from our own scanner: debt-marker test fixtures are now
  assembled at runtime so `procoder debt` on this repository reports a
  clean ledger instead of harvesting its own tests.

## 0.18.0 — 2026-08-19

Absorbed the best of the superpowers and ponytail plugins, so both can be
uninstalled.

- `procoder plan` and `/procoder:plan`: implementation plans under
  `.procoder/plans/` complete the spec → plan → todo chain. The quality
  controller blocks on placeholders ("TBD", "handle edge cases",
  "similar to task N"), empty sections, and tasks without `Files:` or
  checkbox steps — a plan is written, not promised.
- `procoder debt`: deliberate-simplification markers (`debt:` comments
  naming a ceiling and a revisit condition; marker configurable via
  `[debt]`) harvested into a ledger, with no-trigger markers flagged as
  rot.
- `procoder principles` plus a SessionStart hook: every session starts
  with the engineering principles (reuse → stdlib → platform → minimum
  code, root-cause bug fixing, deliberate corners marked as debt);
  `.procoder/PRINCIPLES.md` replaces them per repo.
- New skills: `/procoder:debug` (root cause before any fix, one
  hypothesis at a time, three strikes questions the architecture),
  `/procoder:tdd` (red before green, name the break each test catches,
  the mutation check), `/procoder:simplify` (the five-tag
  over-engineering review with an honest "Lean already. Ship." null
  result).
- Skill upgrades: `/procoder:spec` now classifies work as spike, bounded,
  or architectural with a one-way ratchet before interviewing;
  `/procoder:todo` defines what counts as evidence (fresh verification
  only, red-green proof for regression tests); `/procoder:merge` gains
  the review-receiving rules (verify claims before implementing, ask
  when unclear, facts instead of gratitude).

## 0.17.0 — 2026-08-19

Quality controllers for tasks and specs — done means verified.

- `procoder todo` and `/procoder:todo`: tasks live as Markdown files under
  `.procoder/todo/`, each with a real description, testable acceptance
  criteria, and an evidence section. `todo close` is the quality
  controller — it refuses to close a task until every criterion is
  checked, the evidence records what was run and what it proved, and the
  commit gate is clean, naming exactly what is missing.
- `procoder spec` and `/procoder:spec`: spec-first design under
  `.procoder/specs/`. The skill runs a gap-closing interview (problem,
  users, scope boundaries, constraints, interfaces, data, edge cases,
  failure modes, testable acceptance criteria, open questions);
  `spec check` blocks while sections are missing or empty, while any
  `OPEN:` question is unresolved, and while criteria are untestable.
  A complete spec seeds the todo list.
- The docs domain now requires CHANGELOG.md to carry an entry for the
  current version (blocking): a changelog that exists but skips the
  release being shipped is exactly how release notes go stale.

## 0.16.0 — 2026-08-19

The onboarding sweep, the comprehensive site, and a robustness batch.

- `procoder audit` and `/procoder:audit`: every domain's checks over the
  WHOLE tracked tree of a repository procoder has not governed before,
  aggregated into a triage-ordered scorecard. Its first run flagged our
  own pinned action SHAs as secrets — the false-positive flow
  (`gitleaks:allow` / `.gitleaksignore`, every allow a reviewed decision)
  is now part of the security rules.
- The docs site grew from one page to a real reference: the nine domains,
  every command, every configuration knob, and the workflow — and a new
  completeness check blocks a shipped command the documentation never
  mentions (usage text and the coverage list are pinned to each other by
  test).
- Robustness: CI runs once per change (push runs only on main), golangci
  caches are isolated per repository root (no more stale cross-worktree
  paths), the pr skill enforces ≤72-char titles, the merge skill deletes
  remote branches explicitly instead of trusting the flag's silent local
  failure, and the accepted Stdout.Write info line is excluded by config.

## 0.15.0 — 2026-08-19

Linters for all, without an asterisk — and the version tripwire now
guards every claims-bearing page.

- VersionSync generalizes from the README to a rules-driven list
  (## Version-tracked docs in .procoder/docs/RULES.md; default README.md
  and docs/index.md): the Pages site's index shipped eight releases
  stale because only the README was held to the version — the same
  prose-claims blind spot, now closed for every listed page. The site
  content itself is rewritten to the all-nine reality.

- Configless JavaScript now gets a procoder baseline: eslint's BUILT-IN
  core rules (no-undef, no-unused-vars, eqeqeq, no-var, …) via a
  generated temp flat config with common runtime globals — no npm
  packages installed, nothing written into the repo, and the project's
  own eslint config still always wins. Findings are labeled
  "(lint, procoder baseline)".
- TypeScript without a project config stays honestly out of scope: a TS
  parser is not built into eslint and installing one would be imposing.
- eslint v10 removed the unix formatter from core — both eslint paths now
  parse --format json, fixing config-carrying projects on v10 too.

## 0.14.1 — 2026-08-19

Morning review fixes, both dictated:

- hook.Run (complexity 25) and index Impact (25) refactored into named
  single-purpose helpers — both now under the threshold; the remaining
  switchboards (gate.Run 19 and friends) accepted as honest.
- Maintain thresholds are repo-overridable per D-OVERRIDE:
  `[maintain] gocyclo / funlen_lines / funlen_statements` in
  .procoder/config.toml, defaults 15/80/50.

## 0.14.0 — 2026-08-19

Domain 4, performance — and with it, all nine domains shipped.

- `/procoder:perf`: the measure-first discipline as a skill. Deterministic
  perf checks barely exist, so v1 teaches the real instruments (go test
  -bench/pprof/benchstat, cProfile/py-spy, node --cpu-prof) and the law:
  baseline, change, re-measure, report the delta with the command — a fix
  without a benchmark is a hope. Heavier tooling arrives when a real need
  shows.

## 0.13.0 — 2026-08-19

Domain 8, DevOps/IaaS/CaaS: each instrument only where its files exist.

- `procoder infra` and `/procoder:infra`: hadolint over Dockerfiles,
  `terraform fmt`/`validate`/tflint over *.tf directories (a failing
  validate BLOCKS — objectively broken; uninitialised dirs say NOT
  validated instead of failing on providers), kubeconform over Kubernetes
  manifests, `helm lint` over charts. Rides the gate and `procoder git`;
  a repo with no infrastructure pays nothing.
- doctor/init learn the five tools, each required only by inventory.

## 0.12.0 — 2026-08-19

Domain 7, CI/CD/CT: pipeline discipline as deterministic checks.

- `procoder ci` and `/procoder:ci`: mutable action refs (report by
  default, blocking via `[ci] pin_actions_policy = "block"`), missing
  per-job timeout-minutes, missing concurrency cancel-in-progress, and no
  tests anywhere. Rides `procoder git` and the gate too.
- Our own CI practices it: every action pinned to its commit SHA with the
  tag as a comment, and a concurrency group cancels stale runs.

## 0.11.0 — 2026-08-19

Domain 3, maintainability: a thin layer over the index and the linters.

- `procoder maintain` and `/procoder:maintain`: dead-code candidates from
  the precise index (exported API marked for judgment), cyclomatic
  complexity and function length from isolated linter runs with procoder's
  own thresholds (gocyclo 15, funlen 80/50, C901) — the repo's lint config
  is neither required nor touched. Everything reports; nothing blocks.

## 0.10.0 — 2026-08-19

Domain 1, security: the priority level, built on lint's rails and the index.

- Secrets (gitleaks): BLOCKING always — in the write hook the moment a
  secret lands in a file, in the gate over the changed set. The finding
  names rule and location, never the value, and orders a rotation.
- SAST (semgrep, community rules) and dependency vulnerabilities
  (osv-scanner): `procoder security --deep` and CI; ERROR severity and
  CVSS ≥ 7.0 block, the rest is judged.
- `/procoder:security` reviews from the index's entry points and call
  graph; rules live in .procoder/security/RULES.md.
- Missing scanners read as blocking NOT-checked — a security check that
  silently didn't run is worse than a red one.

## 0.9.0 — 2026-08-19

Domain 2, best practices: the canonical linter per ecosystem.

- `procoder lint` and `/procoder:lint`: golangci-lint (Go), ruff check
  (Python), shellcheck (shell), eslint (JS/TS, only where the project
  carries a config — procoder imposes no rules). The write hook lints the
  file just written in-turn; the gate lints the changed set.
- Report by default — lint is judgment, formatting was not; a repo opts
  into blocking with `[lint] policy = "block"` in .procoder/config.toml.
- Missing linters read as NOT checked, never clean; configless JS/TS is
  labeled out of scope.
- `/procoder:update`: update the plugin from the marketplace and verify
  the new version by direct invocation.

## 0.8.2 — 2026-08-19

- The README must carry the current version on its first screen — a
  blocking docs check, born from three releases shipping against a badge
  frozen at 0.7.0. Prose claims aren't file paths, so drift never fired;
  now a release without a reviewed README reds the gate. The README's
  domain list also caught up (documentation shipped, the index noted).
- The call graph dropped its noise: compiler-local temporaries and bare
  package descriptors are excluded from the edges (7,012 → 2,587 on this
  repo, all signal), `callers` shows each named call once with readable
  symbols (`io/ReadAll()`, not the SCIP provenance header).

## 0.8.1 — 2026-08-19

- The skills are back: command definitions moved from TOML with multiline
  strings — which the plugin loader silently drops — to Markdown with YAML
  frontmatter, the canonical format. Same nine commands, now actually
  registered.

## 0.8.0 — 2026-08-19

The code index (D-INDEX): the shared platform layer under the domains.

- `procoder index build|find|search|refs|outline|impact|stats` and the
  `/procoder:index` skill. Broad tier from universal-ctags (definitions,
  outlines, fuzzy search); precise tier from SCIP (scip-go and friends) for
  exact references — every answer says which tier produced it, and a stale
  index says so out loud.
- `index impact`: the blast radius of the working-tree change — which
  symbols it defines and which files reference them; the gate prints it
  and /procoder:pr makes the agent verify the named files.
- The security/maintainability surface, built now: `index callers` (the
  call graph from SCIP occurrences), `index unused` (dead-code
  candidates, exported API marked), `index entrypoints` (mains and the
  exported surface), and `index graph` (the machine-readable edge list
  future domains walk).
- The write hook keeps the broad tier current for each file written; the
  gate rebuilds a stale index at the finishing moment, covering editor
  edits and the precise tier the hook cannot reach.
- Tool resolution got honest: a probe rejects macOS's BSD ctags impostor,
  and `~/go/bin` / `~/.local/bin` count as installed.

## 0.7.1 — 2026-08-19

- The docs scan now asks git which Markdown belongs to the repository
  (tracked plus untracked-but-not-ignored) instead of walking every
  directory — gitignored scratch is no longer scanned.
- The PR-template mirror is enforced: drift between .github/ (the path
  GitHub reads) and the .procoder/github/ master now blocks the gate.
- The merge watcher got a protocol: calibrate against previous runs, poll
  per job in the foreground, report the first failure immediately with its
  log excerpt, poll dynamically — never a fire-and-forget monitor.
- Issue templates caught up with the reset: no more dropped "levels",
  Node-era fields, or renamed config paths.

## 0.7.0 — 2026-08-19

Domain 5, documentation: docs treated as a product.

- `procoder docs [--external]` and `/procoder:docs`: broken relative
  references and non-compiling Mermaid diagrams block; doc drift, missing API
  doc comments (Go/Python/TypeScript), required docs, badges, and README
  first-screen structure are reported; `--external` adds `lychee` link
  checking and GitHub Pages health.
- The write hook now checks Markdown references and diagrams in-turn, and
  reports which docs mention a code file the agent just changed.
- New repo-owned rules: `.procoder/docs/RULES.md` and the shared Mermaid
  theme `.procoder/docs/mermaid.json` (printed by `procoder templates`).
- Docs site: MkDocs Material built and deployed to GitHub Pages by CI.
- `doctor`/`init` learn `lychee`, `mmdc`, and `mkdocs`.

## 0.6.0 — 2026-08-18

Repo-overridable workflow rules (D-OVERRIDE begins here).

- `.procoder/github/WORKFLOW.md`: feature work in git worktrees, PR polling
  delegated to a watch-only background agent, full local+remote cleanup after
  a successful merge. The repo's file wins over the skills' defaults.
- Fixed `RepoRoot` to recognize a worktree's `.git` file — commands run
  inside a worktree no longer report against the parent checkout.

## 0.5.x — 2026-08-17

Domain 9, GitOps/GitHub, hardened by its own dogfood runs.

- The gate's git slice: conflict markers, junk files, oversized files
  (5 MB default), AI-attribution lines in commit messages (blocking — the
  work is the author's), commit subject shape, default-branch policy.
- `actionlint` on every workflow file written, findings in the same turn.
- PR and commit templates under `.procoder/github/`, `/procoder:pr` and
  `/procoder:merge` skills, `procoder scrub`.
- 0.5.1 fixed the gate exiting 0 over blocking findings; 0.5.2 fixed Windows
  test stubs and stopped prettier flagging the commit template's functional
  blank lines.

## 0.4.0 — 2026-08-17

- `procoder init [--yes]`: the binary computes the install plan per machine,
  the agent (or `--yes`) executes it, and the survey re-runs afterwards —
  an installer's exit 0 is a claim; the tool resolving is the fact.

## 0.1.0 – 0.3.0 — 2026-08-17

The Go rewrite and the plumbing proof (domain 6, formatting).

- One static binary per platform, committed in `dist/`, installed via the
  Claude Code marketplace; hooks and skills call a thin launcher.
- Formatting via each ecosystem's canonical tool (gofmt, ruff, prettier,
  rustfmt, clang-format, shfmt) with three honest verdicts: clean,
  unformatted (formatted bytes handed to the agent), unchecked (said out
  loud, never silent). The write hook hands the agent the formatted code and
  never touches the file (P-CONTROL).

Before 0.1.0 the project was a TypeScript analyzer engine; that history is in
git. The design reset that produced the current harness is recorded in the
design contract.
