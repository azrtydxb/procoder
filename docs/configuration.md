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
# The handles the changelog's credit rule does not ask about: whose
# release notes these are. Thanking yourself in your own notes is noise.
#
# Configured rather than discovered. `gh api user` answers only where a
# person is logged in; in CI the token is an app installation token with
# no user behind it and returns 403, which made the check unrunnable in
# the one place it most needed to run. "Whoever triggered the workflow"
# is worse — on a contributor's pull request that is the contributor,
# who would then be excluded from the credit they are owed.
maintainers = ["your-handle"]
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

# A question's identity is its words, not its line breaks. Recording an
# answer under a section that a formatter later rewrapped keeps the answer
# valid — the question was not reworded. Rewording it asks it again, and an
# answer recorded by an older build (whose key hashed the text as written)
# still reads as an answer.

[version]
# "warn" (default) reports a newer release at session start; "off" silences
# it for CI and scripted runs. There is deliberately no third value: a
# setting that upgraded without asking would remove the consent the
# upgrade is built on.
check = "warn"

[service]
# Overrides the repository identity procoder computes. Unset by default.
repo = "acme/widgets"
```

## A setting procoder does not know

An unrecognised key blocks. A key that does nothing while its writer
believes it is in force is the failure this whole feature would otherwise
introduce, so silence is not an option.

The finding names both reasons a key can be unrecognised, because only one
of them is yours to fix. A **typo** is: correct the spelling. A key **added
in a later release** is not — you spelled it correctly, this build is
simply older, and no edit to the file will help. The finding says which
build is doing the not-knowing and points at `procoder self-upgrade`.

That distinction is not cosmetic. An instruction nobody can follow is how
`--no-verify` becomes muscle memory, which is the failure behind both #172
and #185 — and it happened here, with a key added in one commit reported
unknown by the plugin binary from the release before it.

## `.procoder/context.md`

The project's shared vocabulary: a `## <term>` heading per entry with a
one-paragraph definition beneath. Written by hand or by an agent, never
generated — the value is what the team agreed to call something.

`procoder context list` and `check` read it, and `procoder spec check` notes
when a spec seems to be describing a term already defined. None of those
blocks anything — vocabulary is not grounds for failing a build.

`procoder context add` does refuse an entry with no definition, or a term
already defined under another spelling. Neither is a wording judgement, and
both exit 2.

A glossary that exists and cannot be read is reported as such, never as an
absent one.

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
that an empty file is indistinguishable from an emptied one — written out
by an editor, truncated by a bad merge, or replaced by a pipeline that
printed a header and nothing else. (`procoder format` did exactly that on
its already-formatted path until the contract changed: stdout now carries
the bytes that belong in the file for every verdict, and the verdict line
went to stderr where it cannot overwrite anything.) Either way, falling
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

### `[learn]`

| Key           | Default | What it does                                                                                                                                    |
| ------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `record`      | `false` | Append one timing record per command run to `.procoder/state/learn.jsonl`. Off until asked: no repository starts measuring because it upgraded. |
| `min_samples` | `20`    | How many recorded runs `procoder learn propose` wants before it proposes anything.                                                              |

The records are gitignored state, not repository content, and hold a
command name, a duration and an exit code — nothing about a file's
contents and nothing identifying a person.

### `[service]`

| Key    | Default    | What it does                                                         |
| ------ | ---------- | -------------------------------------------------------------------- |
| `repo` | _computed_ | The repository's stable name, overriding the one procoder works out. |
| `mode` | `off`      | `local` runs commands through the daemon when one is listening.      |
| `exec` | `false`    | Opens the second socket, which serves the commands that run things.  |

#### `mode`

`off` is the default and stays the default: no repository changes
behaviour because it upgraded. `local` means a command tries the socket
first, and falls back to running in-process on any failure — no daemon, a
daemon from another build, a socket that went away mid-request. A degraded
transport costs you the daemon's speed and never your answer.

The CLI is unaffected either way. Every command still runs in-process with
no daemon and no setup, in CI and on a fresh clone. `procoder init` asks
which this machine should be rather than choosing for you, and a value
that is neither `off` nor `local` leaves the machine where it was rather
than in a state nobody chose.

`procoder serve` is the daemon itself — see
[Commands](commands.md#procoder-serve---socket-path---exec---idle-duration).

#### `exec`

Four commands run what a repository — or a prior agent session — declared:
`run --exec`, `evidence record`, `init --yes` and `self-upgrade`. They are
never served on the ordinary socket, whatever this key says.

`exec = true` opens a **second** socket for them alone, at
`~/.procoder/run/procoder-exec.sock`, whose address the hooks are never
told. That separation is the point rather than a detail: the socket's 0600
mode authenticates the **user**, not the process, so every process running
as you can open it — including an agent session's own shell. A path that
runs an agent-written command must not be reachable by something running
unattended.

Leave it `false` unless you specifically want a caller to be able to run
those four. The binary always can, and that is the door meant for them.

Procoder needs a name for this repository that means the same thing on
somebody else's machine. A filesystem path does not: the same repository
lives at a different path for every person who clones it.

So it works one out, down a ladder, and `procoder config` prints both the
answer and the rung that produced it:

```
repo identity  host/owner/repo   (origin remote)
```

| Rung             | When it answers                                                                    |
| ---------------- | ---------------------------------------------------------------------------------- |
| `[service] repo` | You set it. Nothing below is consulted.                                            |
| origin           | There is an `origin` remote.                                                       |
| first remote     | There is no `origin`; the alphabetically first remote wins, and the line names it. |
| root path        | There are no remotes at all. The resolved absolute path.                           |

Remote URLs are normalised to `host/owner/repo`, so
`git@host:o/r.git`, `https://host/o/r.git` and `ssh://git@host/o/r` are
one identity rather than three.

**`origin` beats an alphabetically earlier remote deliberately.** Pure
alphabetical order is simpler and wrong: a colleague who adds a personal
remote named `fork` would key the same repository differently from
everybody else, which defeats the one thing an identity is for.

Set `repo` when the computed answer is wrong for you — a monorepo serving
several products, a mirror whose remote is not the name anybody uses, or
a checkout with no remote at all that you would rather not identify by
path.
