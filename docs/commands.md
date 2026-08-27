# Command reference

**Reference.** Two ways to reach the same engine, and one of them is the
way in.

## Run the `/procoder:` commands

**This is the interface.** Each one is a skill: it runs the binary, reads
the output, and knows what to do with it — which checks to re-run, what
counts as evidence, when to refuse. Typing the binary yourself gets you
the numbers without the judgment that surrounds them.

| Command              | What it does                                                                                                                                                                 |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/procoder:adr`      | Architecture decision records: durable decisions with their date, context, and consequences — supersede, never rewrite.                                                      |
| `/procoder:agents`   | Keep the universal agent layer in sync: per-host rule files derived from AGENTS.md, with drift blocking the gate.                                                            |
| `/procoder:audit`    | Onboard an existing codebase: every domain's checks over the whole tree, then a triaged plan to bring it in line.                                                            |
| `/procoder:backlog`  | The project layer: milestones, epics, and user stories with refusing controllers — spec-seeded, sprint-ready.                                                                |
| `/procoder:check`    | Run the formatting gate over the changed files, as a commit or CI would.                                                                                                     |
| `/procoder:ci`       | CI/CD hygiene and run health: pinned actions, timeouts, concurrency, tests — plus the latest runs via gh.                                                                    |
| `/procoder:debug`    | Systematic debugging: root cause before any fix, one hypothesis at a time, and a three-strikes rule that questions the architecture instead of stacking patches.             |
| `/procoder:deps`     | The dependency freshness report: what is behind, by how much, per ecosystem — judgment stays yours.                                                                          |
| `/procoder:docs`     | The documentation report: references, diagrams, drift, API docs, badges, README structure, links, Pages.                                                                     |
| `/procoder:doctor`   | Report which formatters this repository needs, which are installed, and how to install the rest.                                                                             |
| `/procoder:env`      | What changed in the project's environment since you last synced: dependencies, migrations, new env keys.                                                                     |
| `/procoder:format`   | Show the formatted result for files, so you can review and write it.                                                                                                         |
| `/procoder:git`      | The pre-finish status: branch, hygiene findings, message checks, workflow lint, template state.                                                                              |
| `/procoder:index`    | The code index: build it, then find, search, refs, outline, and impact instead of grepping blind.                                                                            |
| `/procoder:infra`    | DevOps hygiene where the files exist: Dockerfiles, Terraform, Kubernetes manifests, Helm charts.                                                                             |
| `/procoder:init`     | Install the formatters this repository needs, with every command visible before it runs.                                                                                     |
| `/procoder:lint`     | The canonical linter per ecosystem over your changes: findings are diagnoses you judge, fix, or explain.                                                                     |
| `/procoder:maintain` | The maintainability report: dead-code candidates, complexity, function length — judgment calls you decide on.                                                                |
| `/procoder:merge`    | Finish a PR properly: every check green, every review addressed — human and bot — then merge and clean up.                                                                   |
| `/procoder:perf`     | The performance discipline: measure before touching, benchmark what matters, prove regressions and fixes with numbers.                                                       |
| `/procoder:plan`     | Turn an approved spec into an implementation plan an engineer with zero context could execute — with a quality controller that blocks placeholders and hollow tasks.         |
| `/procoder:pr`       | Prepare and open a pull request the senior way: gate, template, scrubbed, everything visible.                                                                                |
| `/procoder:release`  | The pre-tag controller: version sync, changelog, clean tree, gate, and suite — every failure listed, the tag printed, never run.                                             |
| `/procoder:run`      | How to run this project: the launch commands it declares, with the file that declared each.                                                                                  |
| `/procoder:security` | The security pass: secrets (blocking), SAST, dependency vulns — plus the index's entry points to review from.                                                                |
| `/procoder:simplify` | The over-engineering review: five tags (delete, stdlib, native, yagni, shrink), a mandatory replacement per finding, and a real null result when there is nothing to cut.    |
| `/procoder:spec`     | Spec-first design: a gap-closing interview that produces a complete spec, with a quality controller that blocks until every section is answered and every question resolved. |
| `/procoder:sprint`   | Scope-boxed sprints over the backlog: one active sprint, explicit carry-over, a close that refuses to hide unfinished work.                                                  |
| `/procoder:status`   | The state of play, computed fresh: branch, dirty files, the active sprint, open work, index freshness.                                                                       |
| `/procoder:tdd`      | Test-driven development with tests that actually catch breaks: red before green, name the break each test catches, and the mutation check before done.                       |
| `/procoder:test`     | Run the repository's actual test suite: every ecosystem's canonical runner — NOT run is never green.                                                                         |
| `/procoder:todo`     | The quality-gated task list: tasks with real descriptions, testable acceptance criteria, and evidence — a task only closes when the controller agrees it is done.            |
| `/procoder:update`   | Update the procoder plugin from the marketplace and verify the new version end to end.                                                                                       |

## The binary underneath

Everything below is what those skills call. Reach for it directly only
when you have to:

- your agent is not Claude Code — see
  [Install without the plugin](how-to-install-manually.md)
- you are scripting CI, where `procoder check` is the whole point
- you are debugging Procoder itself

It is the same binary and the same rules either way; what you give up is
the skill's judgment about what to do with the answer. **Prefer the
slash command.**

The binary only ever computes and reports — the agent (or you) acts on
the results (P-CONTROL).

### Onboarding

#### `procoder audit`

The onboarding sweep for a repository Procoder has not governed before:
every domain's checks over the WHOLE tracked tree — formatting verdicts,
hygiene, secrets, lint — aggregated into one scorecard with a triage
order. Exit 1 while the repository would fail the gate; the
`/procoder:audit` skill drives the fixing.

### Everyday commands

#### `procoder check [paths...]`

The commit gate. Over the changed files (or the given paths): formatting
(unformatted and unchecked both fail), git hygiene (conflict markers, junk
and caches, oversized files, AI-attribution lines — all blocking), workflow
lint, CI hygiene, infrastructure hygiene, lint findings, secrets (blocking),
docs checks, and the change's blast radius from the index. Exit 1 on any
blocking finding.

That full list is what a repository that has **adopted** Procoder gets. In
somebody else's repository — no `.procoder/`, no `AGENTS.md` naming
Procoder — only the checks that are true anywhere run, and the content
checks see only the lines the commit wrote. The run says which mode it was
in; see [Adopted and universal repositories](configuration.md#adopted-and-universal-repositories).

**Where review sits.** Finishing a piece of work is four passes, each
asking something different: implement what was scoped with nothing quietly
deferred; reread the diff as a reviewer who did not write it; hunt defects
deliberately; then the cheap polish and stop.

The same four at the eleventh story as at the first. A task split three
levels deep gets them at every leaf, not a share of them — "I am nine
stories in, I can go faster now" is the feeling that precedes the bug that
took longest to find, and `backlog close story` says so where the
temptation arrives.

`procoder review` is the third pass, and its `adversarial` and `edge-case`
lenses are pointed at exactly that. Nothing checks that four passes
happened — nothing on disk could — so this is a discipline, not a gate.
What it buys is that thoroughness comes from asking four different
questions rather than asking the same one harder.

#### `procoder review [--lens <name>[,<name>]] [paths...]`

The judgment half of the gate. Every other check Procoder runs is
mechanical and has one right answer; this one asks the questions that do
not — is this the right shape, what breaks at the edges, would a test
catch this regressing.

It prints five lenses over the changed files (or the given paths), each a
distinct stance rather than a checklist: **adversarial** (assume it is
wrong and find where), **edge-case** (enumerate paths, report only the
unhandled ones), **verification-gap** (would verification actually fail if
this broke?), **structure** (how it is organised), and **prose** (how it
is expressed). Overlap between two lenses is signal, not duplication.

The binary judges nothing — it cannot, it is not a language model. It
prints the lens and the scope; the agent judges. Same contract as
`procoder format`: the binary prints, the agent writes, and nothing on
disk changes.

`--lens` narrows to the named ones. A name that is not a lens is a usage
error (exit 2), never a silent full review.

`--perspectives` reads with a different set: **who** is reading, where a
lens is **how**. Analyst (is this the right problem), architect (what
does this commit the system to), implementer (what will it be like to
live in), reviewer (what is a reader owed). Meant for a spec or a plan —
the architectural question is cheapest to answer before there is a diff
to answer it against. Deliberately stances without names: Procoder has
no voice by design, so it takes the multi-angle read and leaves the
cast.

Any lens is replaceable from `.procoder/review/lenses/<name>.md`
(D-OVERRIDE). An empty or unreadable override **blocks and prints
nothing** — unlike a template, which falls back to Procoder's version and
says so. A lens shapes a judgment, and a review under your lens's name
running Procoder's words is worse than no review, so nothing is printed
for an agent to act on by mistake. Exit 1.

#### `procoder analyze <sub>`

The phase before the spec. `spec check` judges whether a document is
complete, never whether the idea in it is good — it will pass a
thoroughly filled-in specification for the wrong feature. This is where
an idea becomes something worth checking.

`brief <name>` prints an analysis document (Question, What we know, What
we do not know, Options, Recommendation) for the agent to write under
`.procoder/analysis/`; `list` shows what exists; `check [name|all]`
refuses one whose sections are still template comments, to the same
standard a spec is held to.

`where` names every entry point in the chain — analysis, spec, plan,
backlog, todo, build — with what each is for and what to run, and says
which one this repository has already used. **Nothing requires you to
start above the point you need:** no gate finding asks for a spec, and a
change that begins at build is gated, tested, formatted and released
like any other. What the chain refuses is a story closing without
evidence, and that applies wherever you began.

Never required. Nothing demands an analysis document exist — it is the
answer to "I do not know what I am building yet", not a new tollgate.
`procoder spec check` names the analysis a spec came from when one
shares its name, and says nothing when none does.

#### `procoder git`

The pre-finish status: branch vs default, changed-file count, template
presence and registration, and every hygiene finding — the same rules as
`check`, shared through one code path so they can never disagree.

#### `procoder format <files...>`

Prints each file's formatted result (gofmt, ruff, prettier, rustfmt,
clang-format, shfmt — the project's config always wins) so it can be
reviewed and written. Never touches the file.

#### `procoder lint [--types] [paths...]`

The canonical linter per ecosystem: golangci-lint (Go), ruff check
(Python), shellcheck (shell), eslint (JS/TS — configless plain JavaScript
gets the built-in-rules Procoder baseline; configless TypeScript is out of
scope), cargo clippy (Rust), ktlint (Kotlin), swiftlint (Swift), rubocop
(Ruby), checkstyle (Java, google_checks baseline), clang-tidy (C/C++),
and for PHP whichever of phpstan and phpcs the repository configured —
falling back to Procoder's own phpstan baseline when it configured
neither, so an unconfigured PHP project is still linted rather than
merely parsed. A linter that could not run is BLOCKING: a green gate has
to mean the code was checked, not that the machine was empty. Go repositories without a golangci config get Procoder's curated
baseline (standard set plus gosec, gocritic, errorlint, unparam,
copyloopvar, nilerr) — the repo's own golangci config always wins,
whichever of `.golangci.yml`/`.yaml`/`.toml`/`.json` it uses. Report
by default; `[lint] policy = "block"` makes findings block.

`--types` adds the type-checker where the canonical linter does not
compile the code: `tsc --noEmit` for TypeScript (grouped under each
file's nearest tsconfig — without one the file is declared out of scope,
never silently skipped) and pyright for Python. Go and Rust need no
flag: golangci-lint and clippy already compile what they lint.

#### `procoder backlog <sub>`

The project layer under `.procoder/backlog/`: **milestones → epics →
user stories**, with the story as the execution unit of spec-based work
(the todo list stays standalone for everything else).

- `milestone <title>` / `epic <title> [--milestone <id>]` /
  `story <title> --epic <id>` — print each file for the agent to
  review and write; slug collisions refuse rather than overwrite.
- `bug <title> [--epic <id>] [--severity s1|s2|s3|s4]` — a defect is a
  story with `Type: bug` and a severity (default s3): the description
  prompts for reproduction steps, the criteria are pre-seeded with the
  non-negotiable regression test, and closing without a severity is
  refused. The board marks open bugs with their severity.
- `seed <spec> [--milestone <id>]` — decompose a COMPLETE spec into an
  epic plus one story per acceptance criterion. The epic records the
  spec name and a fingerprint of its acceptance criteria — the contract,
  so rewrapping prose never reads as drift; an incomplete spec is refused
  with the spec checker's gaps replayed.
- `list` / `board` — the flat listing, and the tree with statuses,
  sprint tags, spec-drift flags (`⚠ spec drift` / `⚠ spec missing` /
  `⚠ spec not seeded`), orphans, and a summary line. An open story
  missing a section the close controller reads is flagged there rather
  than at close. The backlog is versioned like the code, so the board
  answers about the current checkout: its last line names the branch it
  read and counts the open stories the default branch holds that this
  one cannot see. The two are never merged — whose status wins when
  both branches carry a story is a decision, not a default.
- `close story <id>...` — refuses until the description is real, every
  acceptance criterion is checked, evidence is recorded, and the gate
  is clean — todo-close rigor, applied to stories. Several ids share
  ONE gate and suite run: the tree is what they judge, so asking per
  story only repeats the answer. An incomplete story is refused by name
  while the rest still close.
- `close epic <id>` / `close milestone <id>` — refuse while any child
  is open; epic close warns on spec drift (never blocks on it).

**`backlog stalled`** — which carried-over items are carried _and_
untouched.

A story edited nine times across three sprints, criteria still unchecked
and evidence still empty, looks busy in every other report. `sprint status`
says "carried over"; nothing said "carried over and never advanced".

The state is hashed over what carries status — the `Status:` line, which
criteria are checked, whether evidence says anything — and everything else
is deliberately excluded. A timestamp, a reworded paragraph, a reordered
list: the file changed and the work did not, and a hash that moved on those
would report every story as progressing. Evidence counts as
present-or-absent rather than by content, because its text is rewritten as
it is gathered and what matters is that it went from nothing to something.
The template's own comment is not evidence.

A detection aid, never a verdict. An item can legitimately wait; what this
reports is that it waited while looking otherwise. A file git cannot answer
for is reported as NOT checked, never as unstalled.

#### `procoder sprint <sub>`

Scope-boxed sprints over the backlog — a goal plus the stories pulled
into it, no story points, no calendar enforcement.

- `open <goal>` — refuses while another sprint is active (one active
  sprint is the WIP limit); prints the sprint file.
- `pull <story-id>...` — commits stories to the active sprint; done,
  missing, or already-committed stories are refused individually while
  the rest still pull.
- `carry <story-id> <reason>` — returns an unfinished story to the
  backlog with the reason recorded in the story file; no reason, no
  carry.
- `status` — goal, committed stories, done/total and carried counts.
- `close` — refuses while a committed story is neither done nor
  carried; on success the sprint file gains a Result section with
  committed/done/carried counts, plus a Retro scaffold (what slowed
  us, what we change, one adaptation worth keeping).
- The retro is the price of the next sprint: `open` refuses while the
  last closed sprint's Retro is empty. A repo opts out with
  `[sprint] retro = "off"` in config.toml.

#### `procoder release [<version>]`

The pre-tag controller: verifies in one pass that every file in
`[release] files` (config.toml) carries the version, CHANGELOG.md has
the `## <version>` entry, the working tree is clean (untracked
included), the gate is clean, the suite is green under
`[test] policy`, the version is not already tagged, and **every
contributor the entry credits actually opened one of the issues or pull
requests that paragraph cites**.

The shipped binaries are no longer among the things it checks. They are
not committed any more: CI builds them at the tag and the launcher fetches
what it needs (ADR 0004), so there is nothing local for this controller to
find stale.

**Credits are computed, not proofread.** Two questions, and the second is
the one that used to be nobody's job:

- a handle credited in a paragraph must have opened something that
  paragraph cites, or the credit is wrong;
- everybody a paragraph OWES a credit must have one, or somebody's work is
  going unacknowledged.

The rule for who is owed is mechanical. A cited **issue** owes its author a
credit. A cited **pull request** owes its author a credit. When one person
did both, that is one credit and not two. When the reporter and the fixer
are different people, **both** are owed — crediting only the pull request
quietly erases whoever found the problem. Whoever is cutting the release is
excluded; thanking yourself in your own notes is noise, and a rule that
demanded it would be ignored within one release.

The report names the handle and hands over the line to paste, because a
check that says "this is wrong" and leaves you to work out the right answer
is one you satisfy by deleting the credit.

Who counts as "yours" comes from `[release] maintainers` in
`.procoder/config.toml`, not from who is running the command. `gh api user`
answers only where a person is logged in — in CI the token is an app
installation token with no user behind it and returns 403, which made the
check unrunnable in the one place it most needed to run. Falling back to
whoever triggered the workflow would be worse: on a contributor's pull
request that is the contributor, who would then be excluded from the credit
they are owed. With nothing configured it asks `gh` locally, so a
repository that has set nothing still gets the rule by hand.

**The skill contract.** `skills/procoder/SKILL.md` is what agents are told
to do; its body is generated from `AGENTS.md` and its frontmatter carries a
`contract` version. When the body changes since the last tag and that
version does not, `procoder release` says so — an adopter upgrading would
otherwise be governed by different rules with nothing to tell them.

Reported, not refused. A wording fix changes the body too, and blocking
every typo behind a version bump would make the bump meaningless within a
month. The version is bumped in `internal/portability/portability.go`,
which makes it a deliberate act rather than a field somebody edits in
passing, and `procoder agents` regenerates the file.

`procoder release --credits` runs those two checks and nothing else, and CI
runs it on every commit with a token. That is the difference between a rule
and a habit: until it ran in CI it ran only when whoever cut the release
remembered to type the command — and "remember to check" is exactly what
kept failing, which is why the rule exists. The rest of the controller asks
about the working tree, the tag and the version files, none of which mean
anything on a pull request.

That last pair asks GitHub, which the test suite deliberately cannot —
the suite runs offline on every commit. This controller can, because the
tag it prepares is published by a job that talks to the same API, so the
network was already a precondition. A misattributed credit hands one
person's thanks to another, permanently, in notes GitHub publishes
verbatim. GitHub not answering is reported as NOT verified and blocks,
never as a pass. Every failure is listed together; on success the
`git tag` command is printed for the agent to run — the binary tags
nothing. Without `[release] files` the version-sync leg says
it verified nothing. Bare `procoder release` reads the newest changelog
version and checks that.

**Nothing procoder runs automatically comes from a file an agent wrote.**
It reads plenty of agent-written state — `.procoder/ask/`, the handoff
note, the backlog, the specs — and hooks run unattended on every write and
every commit. `procoder run` is the only surface that executes a command
the repository declared, and it prints by default, executes only under
`--exec`, and refuses even then when more than one candidate exists rather
than guessing. A test in `internal/hook` reads the hook sources and fails
if any of them reaches that path.

Under `--exec` it also names the binary it resolved:

```
--exec resolving npm -> /opt/homebrew/bin/npm
```

The command comes from the repository and the binary comes from `PATH`, so
the same declared `npm start` is a different program depending on which npm
is first. Naming it is the difference between consenting to a command and
consenting to a string.

#### `procoder evidence record <command>`

Runs a command and prints a fingerprint of what it produced — sha256, byte
count, exit code — for pasting under a story's `## Evidence` heading.

**The output itself is never printed and never written.** Evidence gets
committed, and a suite's output can carry a token, a customer name or a
path identifying somebody's machine. The fingerprint proves a specific
command produced a specific result, which is more falsifiable than prose,
without putting the result on disk. A failing command is recorded too: a
ledger that only holds successes is a highlight reel.

The `Command:` line does echo what you typed, because evidence that does
not say what ran proves nothing — so a secret passed as an _argument_ is
printed. What this protects is the output, which is the part you cannot
control.

`todo close` and `backlog close story` say which kind of evidence a story
carries — a measurement or a manual claim. **Both are accepted.** Most
evidence is prose, and a sentence explaining why a check was not needed is
exactly right; what was missing is a reader being able to tell which they
are looking at, since a section that is somebody's word for it and one that
can be re-checked looked identical to every check procoder made.

#### `procoder context <sub>`

The project's shared vocabulary in `.procoder/context.md` — what the team
calls things, which is not always what the code calls them.

- `add <term> <definition>` — prints the entry to append. The binary does
  not write the file.
- `list` — the vocabulary, alphabetically.
- `check` — every term has a definition, and no term is defined twice under
  two spellings.

`procoder spec check` reads it and notes when a spec appears to be
describing something already named, which costs a reader the work of
noticing they are the same thing.

**The checks report; nothing blocks the gate over vocabulary.** `list` and
`check` never fail a build, and `spec check`'s cross-reference is a note. A
glossary that stopped work over a wording disagreement would be worse than
not having one.

`add` is the exception and refuses two things: an entry with no definition,
and a term already defined under another spelling. Neither is a wording
judgement — one is an incomplete entry, the other is the duplication a
glossary exists to prevent — and both exit 2 so a script notices. It is
also not an ADR — an ADR is a decision with reasoning; this is vocabulary —
and not generated from code, because the value is what a team agreed to
call something, which is not always its identifier.

#### `procoder adr <sub>`

Architecture decision records under `.procoder/adr/`, numbered and
immutable — a changed mind writes a new record and supersedes the old.
`new <title>` prints the next-numbered record (Context / Decision /
Consequences); `list` shows proposed first; `check` refuses hollow
records, unknown statuses, dangling supersede references, and
duplicated numbers. The audit sweep includes these findings.

#### `procoder deps`

The freshness report: outdated dependencies per ecosystem via each
one's native tool — `go list -u -m`, `npm outdated`, cargo-outdated and
pip where available — capped, summarized, report-only. Licenses report
where a tool exists (go-licenses for Go) and answer NOT checked
everywhere else. A missing optional tool is information; a
tool that errored is a failure.

#### `procoder bench [--save]`

The Go benchmarks (`go test -bench . -benchmem`), compared against the
committed baseline in `.procoder/bench/baseline.txt`: per-benchmark
ns/op and B/op deltas, regressions beyond `[bench] threshold` (default
10%) marked and exiting 1, new and vanished benchmarks listed. `--save`
records a new baseline — a decision, taken deliberately. Results are
single-run and machine-local; a baseline from another GOOS/GOARCH
compares with a warning. Go only in this version, said out loud.

#### `procoder test [--coverage] [--name <pattern>] [paths...]`

The repository's actual test suite, run by every detected ecosystem's
canonical runner: `go test ./...`, `cargo test`, the package.json test
script (via the lockfile's package manager), pytest, and gradle/maven.
Verdicts are PASS with counts where the output allows, FAIL
with the failing tests named, and NOT run when a runner or test script
is absent, which is never reported as green. `--coverage` adds the
percentage where the runner measures it natively (Go; pytest with
pytest-cov); a number is reported, never enforced. With
`--name <pattern>` narrows the run to matching tests — `-run` for Go,
`-k` for pytest, `--tests` for gradle, `-Dtest=` for maven, the pattern
after `--` for a JS test script, and a positional for cargo. A runner
that cannot express the pattern reports NOT filtered rather than
silently running everything, and zero matches is a pass saying
so. With `[test] policy = "block"` in config.toml, `todo close` and
`backlog close story` run the suite and refuse while it is red — or
while it cannot be verified at all, because unknown is never done.

#### `procoder status`

The state of play, computed fresh: current branch against the default,
dirty file count, the active sprint with its done/total and open
stories, open todo tasks, unlearned lessons, and index freshness. Every
line is a computed fact; anything that cannot be read says so with the
reason rather than defaulting to something comfortable. The same block
is injected at session start, inside a hard three-second budget that
never runs the gate, the suite, or any network tool.

It also names the suites the commit gate will **not** run here:

```
gate defers to CI: rust, js suite(s) — the gate runs go, python
```

The gate narrows to the runners that accept a target list, so every other
suite is CI's. Without this line that trade is invisible — a JavaScript
commit passes a green gate having never run its suite. Nothing deferred,
nothing said; and a `package.json` with no test script is no suite rather
than a deferred one, so it is not named.

#### `procoder run [--exec]`

How to run this project: the launch command(s) it declares — package.json
scripts, Makefile targets, a Go main, a Cargo bin, manage.py, docker
compose, a Procfile — each with the file and line that declared it,
most specific first. Procoder does not manage processes: a server is
long-running, and backgrounding and log capture belong to the shell that
owns it. `--exec` runs a single one-shot candidate (120s, stdin closed)
and refuses when there is a choice to make or the command looks like a
server. A repository with nothing to run says so and exits 0.

#### `procoder env [--sync]`

What changed in the project's environment since you last synced:
lockfile digests per ecosystem with the install command to run,
migrations added or removed, and keys declared in an `.env.example` that
the local `.env` lacks — key **names** only, never a value from either
file. `--sync` records the current tree as the new baseline, which is a
statement that you have installed and migrated. Report-only: drift is
judgment, never a block. Files git ignores are never surveyed.

#### `procoder security [--deep]`

Secrets over the changed files with gitleaks — always blocking, values
never echoed, rotation ordered. `--deep` adds semgrep SAST (ERROR blocks)
and osv-scanner dependency vulnerabilities (CVSS ≥ 7.0 blocks) over the
repository.

#### `procoder maintain`

`maintain` is report-only about its findings: complexity and dead code are
judgement calls and nothing blocks on them. A check that could not run is
not a finding, though — a missing golangci-lint exits non-zero, because a
report that never read the code must not look like a clean one. CI runs
this over the whole tree.

Dead-code candidates from the index's precise tier, cyclomatic complexity
and function length from isolated linter runs. Nothing blocks; thresholds
are the repo's to set (`[maintain]` in config.toml).

#### `procoder docs [--external] [--ack <reason>]`

Broken relative references and non-compiling Mermaid diagrams block; doc
drift, missing API doc comments, required docs, badges, README structure,
version-tracked pages, and command coverage report. `--external` adds
lychee link checking and GitHub Pages health.

`--ack <reason>` prints the one line that clears the documentation
obligation for a change that genuinely needs no documentation. When a
commit adds an exported symbol and touches no documentation file, the gate
blocks and names this command; the agent puts the printed line in the
commit message, where a reviewer sees the decision and its reason instead
of a silent skip. The binary prints the line — it never edits the message.

#### `procoder ci [--runs]`

Workflow hygiene: actions pinned to mutable refs (report by default,
`[ci] pin_actions_policy = "block"` to block), missing per-job
`timeout-minutes`, missing concurrency cancellation, and pipelines without
tests.

`--runs` asks GitHub about this branch instead: the newest run of each
workflow, its conclusion, its age, and — when it failed — which jobs
failed. It then answers the question the report exists for, which is
whether that verdict is about the commit in your working tree: a run older
than HEAD is named stale, an unpushed HEAD is named as one CI cannot have
seen, and a branch with no runs at all is called an absence of evidence
rather than a green verdict.

It always exits 0, including when the run failed. `--runs` reports what a
remote system said; it is not a verdict on your tree, and the two tiers are
kept apart deliberately — the gate answers about the change, CI answers
about the tree. Read the text, not the exit code: `procoder ci --runs |
grep failure` is the shape a script wants, and nothing here will ever
answer "green" by staying quiet.

#### `procoder infra`

Where the files exist: hadolint over Dockerfiles, `terraform fmt` /
`validate` / tflint over Terraform (a failing validate blocks),
kubeconform over Kubernetes manifests, `helm lint` over charts.

### Specs, plans, and tasks

The chain: a spec says what and why, a plan says how exactly, todos track
gated execution — and each link has its own quality controller.

#### `procoder todo <sub>`

The quality-gated task list under `.procoder/todo/` — one Markdown file
per task, with a real description, testable acceptance criteria, and an
evidence section.

- `add <title>` — prints the task file and its path; the agent writes it
  and replaces the placeholders before starting work.
- `list` / `show <id>` — every task (open first) / one task in full.
- `close <id>` — the quality controller. It refuses to close until every
  acceptance criterion is checked, the evidence section records what was
  run and what it proved, and the commit gate is clean — and it names
  exactly what is missing. Only a passing close moves `Status:` to closed.

#### `procoder spec <sub>`

Spec-first design under `.procoder/specs/` — the `/procoder:spec` skill
interviews the gaps closed; the binary judges completeness.

- `template <name>` — prints the spec shape (Problem, Users, In/Out of
  scope, Constraints, Interfaces, Data, Edge cases, Failure modes,
  Acceptance criteria, Open questions) for the agent to write.
- `list` — every spec in the repo.
- `check [name|all]` — the quality controller: blocks while any required
  section is missing or empty, while any question in Open questions is
  still unanswered — whatever it is called, `OPEN:` or otherwise; an
  answer recorded through `procoder ask` counts as the decision —
  and while acceptance criteria are not testable checkboxes. A complete
  spec whose `Status:` line still says `draft` earns a note to advance it
  to `complete` — a note, never a gap. A complete spec seeds the todo
  list — one task per criterion group.

**Claims are checked, not only counted.** Everything above is structural:
sections present, questions closed, ids covered. All of it can be satisfied
by a spec that asserts things the code does not do and promises criteria no
fixture can produce. Two further checks ask whether the document is TRUE.

A **citation** that does not resolve is refused. A backticked `pkg.Symbol`,
a repository path, or a `procoder <command>` named in In scope,
Constraints, Interfaces or Decisions must exist. Code fences are excluded,
and Edge cases and Failure modes are left alone — they describe what WOULD
happen and name things hypothetically. Nothing judges prose: a machine can
check that `gitx.Attribution` exists and cannot check that "the gate runs
formatting first" is true. Requiring the claim to name something turns the
second into the first for the claims that matter.

An **acceptance criterion that names no observable** is refused. Say the
command that runs it, the test that asserts it, or the file it inspects — a
criterion nobody can run is an agreement, not a test. A criterion whose
observable has a known prerequisite must name it: the documentation domain
needs a built index, and without one it reports `public surface NOT
computed` and never reaches a finding, so the criterion passes whatever the
code does.

A **promise that names a domain and cites nothing** is refused. If a bullet
in In scope says something about formatting, linting, the docs domain or
the suite, it must cite where that lives. The rule does not verify the
claim — nothing here judges prose. It puts the author in the file, which is
where the discovery happens: sprint 021's In-scope listed formatting among
nine domains, cited nothing, and nobody noticed the format loop ran before
the scope decision until the sprint was underway.

An **acceptance criterion that never says what would make it fail** is
refused. This is the mutation discipline `procoder test` already expects of
a test, applied to the criterion. `fails if`, `proved by`, `breaks if` and
several other phrasings count — and naming the test that asserts it counts
too, because a test carries its own `proved by:` and asking here as well
would be the same question twice. You cannot state the falsifier without
constructing the case that separates pass from fail — and when you cannot,
that is the answer. Two of sprint 021's deviations were criteria that could
not fail at all: one describing a narrowing that cannot happen, one on a
fixture where the two outcomes were indistinguishable.

Three more ways a criterion can look measured and not be, each refused:

- **a command whose output cannot differ** — `echo`, a `--version`, a
  `--help`. It prints the same thing on a working system and a broken one,
  so the criterion has no failing branch at all.
- **hedged vocabulary** — "mostly", "generally", "as appropriate". There is
  no observation that contradicts them, so the criterion passes whatever
  the code does.
- **a bar with no number** — "fast enough", "not too many". Two people
  reading the same result would disagree about whether it passed.

All seven refuse only while the spec is a `draft` — deliberately the moment
before the sprint opens, when a deviation is cheap. A spec already marked
`complete` gets notes instead, because retrofitting a rule onto an archive
nobody will rewrite is how a check gets switched off. `backlog close story`
reports the same criterion faults as notes, never as refusals, for the same
reason.

Of the four, the falsifier rule asks the most — but naming the test you
would write, which a good criterion does anyway, satisfies it. It costs
nothing where the work is already scoped, and bites exactly where nobody
has asked what would break the promise.

Known limitation, stated rather than discovered: only the top-level command
resolves. `procoder backlog check` passes because `backlog` exists, though
`check` is not one of its subcommands — and that exact citation was in the
spec for this feature.

**Scope coverage.** Every `## In scope` bullet carries an id — `- [S-1]
…` — and at least one acceptance criterion must cite it: `- [ ] [S-1]
…`. One criterion may cite several ids where it genuinely covers them.

Scope no criterion cites is a gap, and a gap makes the spec incomplete,
and `procoder backlog seed` refuses a spec that is not complete. That is
the loop: **work cannot be seeded from a spec that promises more than it
tests.** It exists because a spec here once put five things in scope,
wrote criteria for three, and passed — `seed` writes one story per
criterion, so the two untested promises became no stories, got no
sprint, and the epic closed at "all stories done" with the feature
half-built.

Coverage is **declared, never inferred**. Matching a bullet to a
criterion by keyword would fail open, and a bullet wrongly judged
covered is the exact silence this prevents. A spec whose bullets carry
no ids is reported NOT CHECKED rather than covered.

#### `procoder plan <sub>`

Implementation plans under `.procoder/plans/`, written from an approved
spec for an engineer with zero context.

- `template <name>` — prints the plan shape (Goal, Architecture,
  Constraints, `## Task N:` blocks with Files, Interfaces, checkbox
  steps).
- `list` — every plan in the repo.
- `check [name|all]` — the quality controller: blocks on placeholders
  ("TBD", "handle edge cases", "similar to task N" — a plan is written,
  not promised), on empty sections, and on tasks without `Files:` or
  checkbox steps.

#### `procoder debt`

Harvests deliberate-simplification markers into a ledger. Convention: a
corner cut on purpose carries a comment with the configured marker
(default `debt:`, `[debt] marker` in config.toml) naming the ceiling and
the condition to revisit. Markers with no revisit trigger are flagged —
those are the ones that silently rot — and the command **exits 1** when
the tree carries any, so the CI whole-tree step fails on rot instead of
printing it into a log nobody opens. Read-only: it reports, it never
edits.

The commit gate reports the same thing for the files a commit carries, so
a shortcut is named while the reason for it is still in the author's
head. Only the changed files: the whole ledger is a property of the tree,
and printing it on every commit is how a list becomes wallpaper. Reported
rather than blocking — a deliberate shortcut is the author's call to
make, but not to make silently.

#### `procoder ask [--file <path>]`

The questions no domain can answer for itself, put to a person: a spec's
undecided questions, a documentation gap that may be deliberate, a flag on
something that may be a test credential, a lint finding that may be a false
positive. It decides nothing — that is the point.

With a terminal it asks one at a time; an empty answer is a skip, not a
decision. Without one — a hook, a pipe, CI — it asks nothing, writes
`.procoder/ask/QA.md` with one section per question, and names the way back
in. The agent relays the questions to the user, writes their answers into
that file, and hands it over with `--file`, which records them against the
questions they belong to and refuses a file it cannot read rather than
recording half of it.

Answers persist in `.procoder/ask/answers.md`, keyed by a fingerprint of the
question text. An unchanged question is never asked twice; a question whose
wording changed is asked again, because the old answer belonged to a
different question. Exit 1 means something is still unanswered, 0 means
nothing is.

**Decisions.** The four sources above are computed from the repository —
each comes from a finding. A decision does not: commit or hold, merge now
or after, which of two approaches. It comes from the work, when the next
step forks and the fork is not the agent's to pick.

The agent writes those to `.procoder/ask/decisions.md`, and `procoder ask`
collects them with the rest. One `## ` heading is one decision; the lines
under it are its options:

```markdown
## Merge #187 before or after #181?

- before: the gate fix lands first, #181 rebases onto it
- after: one release, but #181 is written against the old gate
```

Procoder never writes that file — the agent does, and procoder reads it.

The Stop hook makes the rule bind rather than depend on remembering. A turn
ending with a decision put to the user in prose, and none recorded here,
does not end: the hook reads `last_assistant_message` from the host and
exits 2, which is the documented way a Stop hook continues the
conversation. `ask.PendingDecisions` is the cheap query behind it — the
whole collection runs git, the lint pass and the secret scan, which a hook
firing at the end of every turn under a ten-second timeout cannot afford.

It fires on an explicit ask, never on a question mark, and never twice on
the same message. That asymmetry is deliberate: a missed burial costs one
prose question, while a check that interrupts correct work at the end of
every turn is one people route around.
That is P-CONTROL, and it is why there is no `--raise` flag: it would read
better on the command line and break the rule the tool rests on.

A file that exists but cannot be read, or one with content and no `## `
heading, produces a note naming it. Silence there would leave decisions
sitting on disk that nobody is ever asked.

A flagged secret's value never appears in the question, the files, or the
terminal: what a human is asked is whether the flag is real, and answering
that does not need the credential.

`[ask] policy = "block"` in `.procoder/config.toml` makes pending questions
block the gate. The default, `report`, lists them and leaves the verdict
alone — a question is a request for judgement, not a defect, and failing a
commit on one stops work the person who could unblock it may not be awake
for.

An answered spec question no longer blocks `spec check`: the answer is the
decision, even while the section still lists it, and the verdict says where
the decisions live so nobody reads that section as finished. An unanswered
one blocks exactly as before.

#### `procoder agents`

The universal agent layer: per-host rule files (Cursor, Windsurf, Cline,
Kilo Code, Roo Code, Kiro, Antigravity, Qoder, Copilot editors, Codex)
derived from the canonical `AGENTS.md`. Prints the content for anything
missing or drifted so the agent can write it.

Drift blocks the gate — and until now it did not, though this page and
the command's own output both said so. A rule file that has drifted means
another host is reading rules this repository no longer holds, which is
the failure the agent layer exists to prevent, so it is blocking rather
than advisory. A repository with no `AGENTS.md` ships no agent layer and
is asked nothing.

See [Every agent](portability.md) for the full host matrix.

#### `procoder lessons`

The self-learning loop's ledger (`.procoder/github/LESSONS.md`): every
finding that escaped the repository's gates and was caught downstream becomes a
lesson, and every lesson must carry the adaptation that closes its class —
a linter enabled, a line added to the pre-PR review rubric
(`.procoder/github/REVIEW.md`), a controller tightened, a pinning test.
Entries with no adaptation are flagged UNLEARNED and exit 1 — recorded is
not learned. An unreadable ledger exits 2.

#### `procoder copilot-leak [--since <dur>] [--quiet] [--from-copilot]`

What GitHub Copilot's auto-review caught that this repository's gates did
not, from both places it says it. **Issues**: those authored by
`copilot[bot]` or `copilot-preview[bot]`, those carrying the
`auto-copilot` label, and those whose body quotes a review with
`> **Copilot**`. And **pull request review comments**, which is where the
inline review actually lands — matched on a Bot account whose login begins
with `copilot`, because the review path posts as `Copilot` with no `[bot]`
suffix at all.

Both sources must answer. If either query fails the command reports NOT
checked rather than a count, because "0 findings" from one source and
silence from the other is indistinguishable from a clean review.

Findings are sanitised before anything is shown or sent — fenced and
indented code stripped, secrets redacted, absolute paths made relative — so
what leaves the machine is metadata about a failure, never the source that
failed. Nothing is published without an explicit yes on a terminal; with no
terminal to ask, it asks nothing and captures nothing.

A captured finding becomes a GitHub issue and an entry in
`.procoder/github/COPILOT-LEAKS.md`, deliberately not `LESSONS.md`: a raw
finding is not yet a lesson until a human names its class and its adaptation.
`--from-copilot` reads that ledger back, listing each entry as learned or
UNLEARNED, and exits 1 while any remain unclassified. `--quiet` reports the
count without asking. Without `gh`, unauthenticated, or given output it
cannot parse, it reports NOT checked and exits 2 rather than reporting zero.
A repository with no GitHub remote is a different case: there are no
auto-reviews to ask about, so the empty answer is real and the exit is 0.

#### `procoder principles [--hook]`

Prints the engineering principles each session starts with (`--hook` is
how the SessionStart hook asks for them, wrapped in the envelope its host
reads; without it the same content goes to the terminal).

`--hook` reads the SessionStart payload on stdin and answers differently
depending on why the session started. A **resume** or a **compact** gets
one line saying the rules are in force and where to read them, because the
full text is already in that conversation and re-sending it pays roughly
7KB to repeat what the model can already see. Every other start gets the
whole thing — including one whose payload could not be read, could not be
parsed, or names a source this version does not recognise. Saying too much
costs tokens; saying too little leaves a session governed by rules nobody
sent.

The read gives up after two seconds. A SessionStart hook runs before the
session can begin, so a host that opens the pipe and sends nothing must not
hold it open — and the fallback there, as everywhere else, is the full
text.

The principles themselves: build-ladder first — reuse, stdlib, platform, then the
minimum code that works — the delegation discipline (independent work
fans out to parallel subagents under a clear contract, nothing started
that somebody else is already on, watched as it
lands, nothing merged unjudged), and ADHD/ASD-friendly formatting for
complex answers: a title and one-line summary, type-labeled problem
cards, decisions in their own numbered list, noise filtered, and short
single-topic answers left plain. A repo replaces them wholesale with
`.procoder/PRINCIPLES.md`.

### The code index

#### `procoder index <sub>`

- `build` — both tiers: universal-ctags (broad) + SCIP (precise) into
  `.procoder/index/` (gitignored), stamped with the commit.
- `find <symbol>` / `search <text>` / `outline <file>` — definitions,
  fuzzy lookup, a file's symbols in order.
- `refs <symbol>` — every reference, labeled precise (SCIP) or textual.
- `impls <symbol>` — what implements an interface or its methods.
  Precise tier only: implementation relationships exist nowhere else,
  so without SCIP the answer is "not built", never a textual guess.
- `callers <symbol>` / `graph` — the call graph and its JSON edge list.
- `unused` — dead-code candidates, exported API marked.
- `entrypoints` — mains and the exported surface.
- `impact` — the blast radius of the working-tree change.
- `stats` — what's indexed and staleness, said out loud.
- `rename <symbol> <new> [--at path:line]` — the cross-file rename as a
  reviewable unified diff, computed by the language's own engine (Go via
  gopls). Per P-CONTROL nothing is written: the agent reviews and applies
  the diff itself. Languages without an engine answer with the
  reference worksheet (`refs`) instead of a half-right rewrite; `--at`
  picks one definition when the name is defined more than once.

### Setup and plumbing

#### `procoder config`

Every setting Procoder is actually running under: its value, and where
that value came from — a line in `.procoder/config.toml`, with the line
number, or Procoder's own default. A policy set weaker than the default
is marked as relaxed, so a repository that quietly turned a gate down
says so out loud rather than looking like a repository that never had one.

#### `procoder doctor`

Which tools this repository needs (by its file inventory), which are
installed, versions, and the install command for each gap.

#### `procoder init [--yes]`

Prints one install command per missing tool for this machine's package
managers; `--yes` executes them and re-surveys — an installer exiting 0 is
a claim, the tool resolving is the fact.

#### `procoder templates`

Prints the default content for any missing repo file Procoder reads:
the PR/commit/workflow templates under `.procoder/github/`, the docs and
security rules, the Mermaid theme.

#### `procoder scrub <file|->`

Checks text for AI-attribution lines; exit 1 when any are found. When the
line came from the host rather than from you, removing it is only half
the fix — [the trailer your host
adds](portability.md#the-trailer-your-host-adds) has the setting that
stops it recurring.

#### `procoder hook post-tool-use`

The write hook's entry point — reads a PostToolUse payload on stdin and
answers with findings for the file just written. Wired by the plugin;
rarely invoked by hand.

#### `procoder prune [--apply]`

The cached plugin versions that can go. `claude plugin update` writes each
new version into its own directory and removes none of the previous ones;
`claude plugin prune` does not cover it, so they accumulate. On the
maintainer's machine that reached 55 versions and 1.11 GB, one of them in
use.

Bare `prune` reports and removes nothing:

```
  would remove  2.0.1
  ...
prune: 53 version(s) can go, reclaiming 1.03 GB
  keeping 3.1.0, 3.0.0 (3.1.0 is active)
  run `procoder prune --apply` to remove them
```

`--apply` removes them and reports what actually went, summing the
reclaimed figure from the directories that were removed rather than from
the ones that were planned. Deletion is the one thing here that cannot be
undone, so it is the one thing that is not the default: typing `procoder
prune` to find out what it does must not cost you a gigabyte.

The version in use is protected twice, independently — it is named in
`installed_plugins.json`, and it is the directory the running binary
executes from. Either check alone leaves a way to delete what is in use.
The retention window keeps the active version and one previous, because
repointing `installed_plugins.json` at the directory below is the only
cheap rollback there is and a window of one leaves none.

procoder refuses rather than guesses. A registry that is absent,
unparseable, or does not list procoder means the active version is unknown,
and unknown is never a licence to delete: exit 2, nothing removed. A
directory whose name is not a version — a partial download, an editor's
backup — cannot be ranked, so it is kept and named rather than guessed at.
An active version that is not on disk stops the sweep entirely. A cache
directory that does not exist is not an error at all: procoder may be
installed from a release binary rather than the marketplace.

Nothing calls this from a hook, and a test asserts that by reading the hook
sources — a delete of this size is a deliberate action a person takes, not
something that happens to them while they are typing.

#### `procoder version [--check]` and `procoder self-upgrade [--force]`

Bare `version` prints one line and asks nobody anything.

`--check` asks GitHub for the newest release, compares it against this
build, and says which is which. Every newer release earns the warning —
patch, minor and major alike, because a major is exactly the upgrade whose
behaviour changes. A check that could not run reports NOT known and exits
2: an unanswered check never means "you are current". A build with no
version stamped has nothing to compare and says so. `[version] check =
"off"` in `.procoder/config.toml` silences the session-start check for CI
and scripted runs; there is no third value, because a setting that
upgraded without asking would remove the consent this is built on.

`self-upgrade` installs the newest release over the running binary, and
only after an explicit yes on a terminal — no terminal is a question nobody
answered, not a yes. It refuses to move backwards, so a maintainer on an
unreleased branch is never told to install an older tag. The download lands
beside the binary and the rename is the last step, so a failed download
leaves the working binary exactly where it was.

Every download is verified before it is installed. The asset is fetched
from GitHub over https and from nowhere else — the URL comes out of the
release payload, and a redirect that leaves those hosts is refused — and
its sha256 is compared against the `SHA256SUMS` file the release publishes
beside the binaries. A mismatch refuses the install, deletes the download
and leaves the working binary untouched. A release that publishes no
`SHA256SUMS` at all is refused too, rather than warned about: unknown is
never the same as verified, and treating a missing file as permission would
make deleting one small file the whole attack.

`scripts/build-dist.sh` is what writes both the `dist/` binaries and that
`SHA256SUMS`. It builds every platform with `CGO_ENABLED=0 -trimpath
-buildvcs=false`, stamping the version it reads from
`.claude-plugin/plugin.json` — the same manifest CI compares the binaries
against. `-buildvcs=false` is what makes the output reproducible at all:
without it Go records the commit hash, the commit time and whether the
tree was dirty inside every binary, so the same source at two commits
produces different bytes. The Go toolchain is part of the input too, and
`go.mod` pins one. With both fixed, two builds of one tree produce
identical digests, and the test suite
checks the recorded digests against the committed binaries — a rebuild that
skipped the script goes red there rather than after a tag is cut.

Where the binary belongs to a package manager — a Homebrew cellar, a snap,
a nix store path, `/usr/bin`, a scoop shims directory — the upgrade refuses
and prints that manager's own upgrade command instead, because overwriting
a file a package database believes it owns is a change the manager will
silently revert. The detection is a path heuristic and errs toward
refusing; `--force` is the way past it when the install really is yours.
