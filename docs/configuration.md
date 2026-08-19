# Configuration reference

Everything procoder reads from a repository lives under `.procoder/` —
plain files, made to be edited, and always winning over built-in defaults
(the D-OVERRIDE rule).

## `.procoder/config.toml`

```toml
[git]
# Working directly on the default branch: "report" (default) or "block".
default_branch_policy = "report"
# Oversized-file threshold for the gate, in MB. Default 5.
max_file_mb = 5

[lint]
# Lint findings in the gate: "report" (default) or "block".
policy = "report"

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

## `.procoder/github/REVIEW.md` and `LESSONS.md`

The catch-first-and-learn pair. `REVIEW.md` is the pre-PR self-review
rubric a fresh-context reviewer reads against the branch diff before any
PR is opened. `LESSONS.md` is the ledger of findings that escaped anyway:
each entry names the layer that should have caught it and the adaptation
that now does (`procoder lessons` flags unlearned entries). Defaults from
`procoder templates`; both are the repo's to grow.

## `.procoder/PRINCIPLES.md`

The engineering principles injected at session start (see
`procoder principles`). Absent, procoder's default build philosophy
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

### `.procoder/security/RULES.md`

Security rules the agent follows: what blocks (secrets always; SAST ERROR;
CVSS ≥ 7.0), how to review from the index's entry points, and what never
happens (echoing a secret, silencing a scanner).

### `.procoder/github/WORKFLOW.md`

The team workflow the pr/merge skills follow: worktree-first feature work,
the merge-watcher protocol (calibrate, poll per job, fail fast, report on
change), and post-merge cleanup.

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

## Derived state

- `.procoder/index/` — the code index (gitignored: derived, per-machine).
  The write hook keeps it current; the gate rebuilds it when stale.
