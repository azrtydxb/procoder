# Configuration reference

**Reference.** Everything Procoder reads from a repository lives under
`.procoder/` — plain files, made to be edited, and always winning over
built-in defaults (the D-OVERRIDE rule).

## `.procoder/config.toml`

```toml
[git]
# Working directly on the default branch: "report" (default) or "block".
default_branch_policy = "report"
# Oversized-file threshold for the gate, in MB. Default 5.
max_file_mb = 5
# The commit interception: "block" (default) stops a commit whose gate
# has blocking findings, "report" prints them and lets it through,
# "off" skips the check.
commit_gate = "block"

[gate]
# How much of Procoder this repository is subject to: "adopted" or
# "universal". Normally omitted — Procoder decides from the repository
# itself, and this only forces the answer.
#
# "adopted"   everything runs.
# "universal" only the checks that are true in any repository: secrets,
#             oversized files, conflict markers, junk files, and AI
#             attribution in the commit message. Procoder's own
#             conventions — formatting, linting, the agent layer, the
#             planning chain, templates, docs, debt, the suite — do not
#             run, and the content checks see only the lines the commit
#             wrote.
#
# Setting this to "universal" in a repository that HAS adopted Procoder
# really does reduce the gate; it is not just a label.
scope = "adopted"

[lint]
# Lint findings in the gate: "report" (default) or "block".
policy = "report"

[test]
# The suite verdict: "report" (default) or "block". Under "block",
# `todo close`, `backlog close story` and the commit gate refuse while
# the suite is red. At the gate the run is narrowed to the packages the
# commit touches and to the ecosystems it is written in — the whole
# suite cold is a minute on a repository this size, one package a second.
#
# A suite that could NOT run blocks under either setting: this governs
# whether a failing test stops a commit, and "no answer" is not a
# verdict it has an opinion about.
policy = "report"

[maintain]
# Complexity findings in the gate: "report" (default) or "block".
# Off by default — they are judgement calls, and a threshold that blocks
# by surprise stops people committing to the very file that needs the
# refactor.
policy = "report"

[docs]
# The documentation obligation in the gate: "report" (default) or
# "block". Off by default — procoder never blocks a repo by surprise.
policy = "report"

[sprint]
# The retro gate: unset, `sprint open` refuses while the last closed
# sprint's retro is empty. Set to "off" to opt out.
retro = "off"

[release]
# The version-bearing files `procoder release` verifies stay in sync.
# Unset, the version-sync leg reports that it verified nothing.
files = ["README.md", "docs/index.md"]

[bench]
# Benchmark regressions beyond this percentage are marked and exit 1.
# Default 10.
threshold = 10

[ci]
# Actions pinned to mutable refs: "report" (default) or "block".
pin_actions_policy = "report"

[maintain]
# Complexity/length thresholds for `procoder maintain`. Defaults shown.
gocyclo = 15
funlen_lines = 80
funlen_statements = 50

[debt]
# Comment marker `procoder debt` harvests. Default shown.
marker = "debt:"

[ask]
# Pending questions: "report" (default) lists them and leaves the gate's
# verdict alone; "block" refuses the commit until a human has answered.
policy = "report"

[version]
# "warn" (default) reports a newer release at session start; "off" silences
# it for CI and scripted runs. There is deliberately no third value: a
# setting that upgraded without asking would remove the consent the
# upgrade is built on.
check = "warn"
```

## Adopted and universal repositories

Procoder runs two gates, and which one you get is decided from the
repository in front of it — never from the machine, because a contributor's
laptop looks identical in their own repository and in somebody else's.

A repository has **adopted** Procoder if it has a `.procoder/` directory,
or an `AGENTS.md` that names Procoder. Everything runs, exactly as it
always has.

A repository with neither is somebody else's, and gets the **universal**
gate: a credential, an oversized blob, a conflict marker, a junk file, and
an AI-attribution trailer nobody wrote. Those are wrong in any repository
whatever its house style. Procoder's own conventions do not run, because
that project never asked for them — its formatter may be Biome, its
`AGENTS.md` is its own, and its test command is not Procoder's business.

In the universal gate the checks that read file **content** see only the
lines the commit added or changed. A secret four thousand lines from your
diff is not yours to answer for. Checks about a file's **existence** —
oversized, junk — do not narrow, because a file the commit introduces is
the commit's, all of it.

Every run says which mode it was in. A quieter gate that does not announce
itself is indistinguishable from a clean one:

```
gate scope: universal (no .procoder/ and no AGENTS.md naming procoder)
  procoder's own conventions are NOT checked here — this repository has not adopted it.
  For the full gate: PROCODER_GATE_SCOPE=adopted, or adopt procoder in this repository.
```

`PROCODER_GATE_SCOPE` takes the same two values as `[gate] scope`, for a
fork that cannot carry configuration without that itself being a change the
contributor does not want to make. The config file wins over the
environment variable: the file is the repository's deliberate choice, the
variable is whichever shell this happens to be.

The reasoning is in ADR 0005.

## `.procoder/github/REVIEW.md` and `.procoder/github/LESSONS.md`

The catch-first-and-learn pair. `REVIEW.md` is the pre-PR self-review
rubric a fresh-context reviewer reads against the branch diff before any
PR is opened. `LESSONS.md` is the ledger of findings that escaped anyway:
each entry names the layer that should have caught it and the adaptation
that now does (`procoder lessons` flags unlearned entries). Defaults from
`procoder templates`; both are the repo's to grow.

## `.procoder/PRINCIPLES.md`

The engineering principles injected at session start (see
`procoder principles`). Absent, Procoder's default build philosophy
applies; present, the repo's file replaces it wholesale — the override is
total, not merged.

## Rules files (prose + machine-read lists)

### `.procoder/docs/RULES.md`

Documentation rules. Machine-read sections (one `- item` per line):

- `## Required docs` — files that must exist (default: README.md,
  CHANGELOG.md)
- `## Required badges` — keywords that must appear inside a badge image on
  the README's first screen (default: ci, license)
- `## README first screen` — required first-screen elements (default:
  usp, badges, quick start)
- `## Version-tracked docs` — pages whose first screen must carry the
  current version; a release that skips one blocks the gate (default:
  README.md, docs/index.md)
- `## README must mention` — the feature families the README's narrative
  must carry (empty by default; when filled, a family the front page
  stops telling blocks the gate — the mechanical floor against README
  rot)

Everything under `## Guidance` is prose the agent follows when it writes,
not something the binary parses. The shipped default carries the
[Divio documentation system](https://docs.divio.com/documentation-system/) —
four kinds of document (tutorial, how-to guide, reference, explanation),
never mixed, the kind decided before the first line — plus the writing
rules that follow from it: answer first, examples over prose about
examples, real names rather than `foo`, short sentences, scannable
structure, and an explicit "common pitfalls" list wherever a feature has a
known misuse. Replace it with your own house style; the repo's copy wins.

## In-file exemptions

Two checks can be waived from inside the file itself, and both demand a
reason in the same line:

- `gitleaks:allow` — this secret finding is a false positive (a pinned
  SHA, a fixture). `.gitleaksignore` does the same at repository scope.
- `procoder:allow-conflict-markers <reason>` — this file shows merge
  conflict markers on purpose. It exempts the **whole file**, so keep it
  to documents whose subject is conflicts; a real conflict in an exempt
  file goes unreported.

The reason is not decoration. A bare
`<!-- procoder:allow-conflict-markers -->` exempts nothing, because a
token with no reason is someone silencing a check rather than
documenting an exception.

Markers inside a fenced code block are **not** exempt on their own. A
real conflict lands inside a fence often enough that skipping fences
would be a silent miss.

### `.procoder/security/RULES.md`

Security rules the agent follows: what blocks (secrets always; SAST ERROR;
CVSS ≥ 7.0), how to review from the index's entry points, and what never
happens (echoing a secret, silencing a scanner).

### `.procoder/github/WORKFLOW.md`

The team workflow the pr/merge skills follow: worktree-first feature work
(a git practice the skills describe — Procoder itself creates and removes
nothing), the merge-watcher protocol (calibrate, poll per job, fail fast,
report on change), and post-merge cleanup.

### `.procoder/docs/mermaid.json`

The shared Mermaid theme applied when compiling diagrams.

## Templates

- `.procoder/github/PULL_REQUEST_TEMPLATE.md` — the master the pr skill
  fills; mirrored to `.github/PULL_REQUEST_TEMPLATE.md` (GitHub reads only
  that path), and drift between the two blocks the gate.
- `.procoder/github/COMMIT_TEMPLATE.md` — registered with
  `git config commit.template`.

`procoder templates` prints the default for anything missing; the agent
writes it — the binary creates no files itself.

## Work state

Committed, reviewable Markdown — the record of what is being built, kept
where the code is:

- `.procoder/specs/`, `.procoder/plans/`, `.procoder/todo/` — the design
  documents, the implementation plans, and the standalone task list.
- `.procoder/backlog/` — `milestones/`, `epics/`, `stories/`, and
  `sprints/`, the project layer `procoder backlog` and `procoder sprint`
  read and write through their controllers.
- `.procoder/adr/` — the numbered architecture decision records; a
  changed mind supersedes a record, it never rewrites one.

## Derived state

- `.procoder/index/` — the code index (gitignored: derived, per-machine).
  The write hook keeps it current; the gate rebuilds it when stale.
- `.procoder/bench/baseline.txt` — the benchmark baseline that
  `procoder bench` compares against. Committed rather than derived: it
  is a deliberate decision, written only by `bench --save`, and a
  baseline recorded on another GOOS/GOARCH compares with a warning.

Three of these are written by procoder into the repository it governs and
belong in `.gitignore`. `procoder git` names any that are missing:

```gitignore
.procoder/index/
.procoder/state/
.lycheecache
```

The last is lychee's link cache, left behind by `procoder docs
--external`. The rest of `.procoder/` is committed on purpose — it is the
repository's own rules, specs and backlog, and a teammate who clones
without them gets a different gate.

## `[security]`

| key              | values                     | default | meaning                                         |
| ---------------- | -------------------------- | ------- | ----------------------------------------------- |
| `sast_blocks_at` | `INFO`, `WARNING`, `ERROR` | `ERROR` | the lowest semgrep severity that stops a commit |

`ERROR` is the level semgrep reserves for findings it is confident about.
Lowering the bar to `WARNING` makes more findings block — a strengthening,
and silent. Raising it makes fewer block, which is a relaxation and prints
on every gate run.

## Seeing the effective configuration

`procoder config` prints every setting, its value, and where that value
came from — `default`, or the config file and line. A setting weaker than
its default is marked, so a reader of an unfamiliar repository can tell at
a glance which of Procoder's defaults are still in force.

A setting Procoder cannot apply — a mistyped key, a value of the wrong
kind — is **not** silently ignored. It is reported with its line number
and it blocks, because a config that quietly reverts to defaults lets a
team believe a setting is in force when it never was.

## `[tools]`

Choose among the tools Procoder ships, by language:

```toml
[tools]
js = "biome"   # instead of the default, prettier
```

One key per language, covering every extension that language owns — `js`
covers `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.mts` and `.cts`.

A repository names a tool; it does not name a binary and an argv.
Procoder owns the invocation, and that is what keeps the print-don't-write
contract a guarantee: a formatter is only on the menu if it can emit the
formatted source on stdout and leave the file alone. Tools that can only
write in place — Laravel Pint, phpcbf, php-cs-fixer — are absent for that
reason and not for any other.

Naming a tool Procoder does not ship is reported with the list of what it
does ship, and blocks. The file is still formatted by the default: a
mistyped tool name is a reason to tell somebody, never a reason to stop
reading their code.

## `.procoder/templates/`

The nine templates that drive the quality chain — spec, plan, ADR, todo,
milestone, epic, story, sprint, bug — plus a changelog template, are the
repository's to replace. Put a file at `.procoder/templates/<name>.md` and
it wins; leave it out and Procoder's own is used.

An **empty** template file is an error, not a fallback. It blocks, and
Procoder uses its own template for that run while saying so. The reason is
specific: `procoder format` prints a single header line for a file that is
already formatted and nothing after it, so a pipeline that strips the
header and writes the rest empties the file on the success path. Falling
back quietly would mean a team discovers their customised template is gone
when their next story comes out in Procoder's shape instead of theirs.

## `.procoder/lint/RULES.md`

The lint domain reads rules the same way `docs` and `security` do: prose
for the agent, with list sections a machine reads. A section that is
present replaces the default; a section that is absent keeps it.

```markdown
## checks

- `readability-*`
- `bugprone-*`
```

That list replaces Procoder's curated clang-tidy families. Replace means
replace — a family left out does not survive. A project `.clang-tidy`
still wins over both: that is the tool's own configuration.

## `[planning]`

| Key      | Values             | Default    | Effect                           |
| -------- | ------------------ | ---------- | -------------------------------- |
| `method` | `procoder`, `bmad` | `procoder` | Who owns the planning artifacts. |

`procoder` is the chain under `.procoder/` — specs, plans, backlog,
sprints.

`bmad` means a separately installed [BMad
Method](https://github.com/bmad-code-org/bmad-method) owns them, and
Procoder reads its artifacts instead: `sprint-status.yaml` for sprint
state, and the installation's own `output_folder` setting for where to
look. `procoder status` reports that sprint; `procoder doctor` names the
installed version. Procoder never writes into those directories — BMad
owns what it wrote.

**The setting moves planning and nothing else.** The gate, the suite,
formatting, the release controller, the debt ledger, security and docs
run identically either way and reach the same verdict about the same
code. A test asserts that every finding the gate makes about the code is
identical across both settings, so the seam cannot drift.

Setting `bmad` with no BMad installed is a blocking finding naming both,
rather than a silent fall back to Procoder's own chain: a repository that
chose one methodology must not be governed by the other without being
told.
