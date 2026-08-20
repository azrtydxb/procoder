# Configuration reference

Everything Procoder reads from a repository lives under `.procoder/` —
plain files, made to be edited, and always winning over built-in defaults
(the D-OVERRIDE rule).

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

[lint]
# Lint findings in the gate: "report" (default) or "block".
policy = "report"

[test]
# The suite verdict in the close controllers: "report" (default) or
# "block" — under "block", `todo close` and `backlog close story` refuse
# while `procoder test` is red or cannot be verified at all.
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
# Unset, the version-sync leg honestly reports that it verified nothing.
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
```

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
