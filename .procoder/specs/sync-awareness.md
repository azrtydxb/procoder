# sync-awareness

Status: complete

## Problem

The repository moves under you and nothing says so. A pull lands a new
lockfile, three migrations, and two env vars; the agent resumes editing
against stale `node_modules`, an unmigrated database, and a `.env` that
is missing keys the code now reads — and the first symptom is a
confusing runtime error twenty minutes later. The same blindness exists
one layer out: a branch is pushed, CI judges it, and nobody looks, so
the failure is discovered by a reviewer instead of by the author.
procoder already reports dependency freshness and workflow hygiene, but
neither answers the question that actually bites: what changed since I
last synced, and has CI seen this commit.

## Users

- Pascal returning to a branch after a pull or a switch: one command
  that names the setup steps the repo now expects of him.
- The agent starting a session: a report it can act on before it edits,
  instead of debugging a stale environment it created.
- Either of them before asking for review: the CI verdict for this
  branch, and whether that verdict is even about the current commit.

## In scope

- [S-1] `procoder env` — what changed in the project's environment since the
  last recorded sync, three checks against `.procoder/state/env.json`:
  - lockfile hashes — package-lock.json, pnpm-lock.yaml, yarn.lock,
    bun.lock, bun.lockb, go.sum, Cargo.lock, poetry.lock, uv.lock,
    requirements.txt, Gemfile.lock, composer.lock, gradle.lockfile. A
    changed hash prints "dependencies changed since your last sync —
    run <the ecosystem's install command>", where the command is the
    ecosystem's frozen-lockfile install: npm ci, pnpm install
    (frozen), yarn install (immutable), bun install (frozen), go mod
    download, cargo fetch, poetry install, uv sync, pip install from
    requirements.txt, bundle install, composer install, gradlew
    dependencies.
  - migration directories — migrations/, db/migrate/,
    alembic/versions/, prisma/migrations/, supabase/migrations/ — that
    gained entries since the last sync print "N migration(s) added
    since your last sync" with the added names.
  - `.env.example` / `.env.sample` / `.env.template` keys absent from
    the local `.env` print "new env var(s) declared: X, Y" — key names
    only, never a value from either file.
- [S-2] `procoder env --sync` records the current tree as the new baseline —
  the explicit act of saying "I have installed and migrated" — and
  prints what it recorded (counts, not contents).
- [S-3] Bare `procoder env` reports only: exit 0 with findings, exit 1 only
  when a check could not run (an unreadable lockfile, migration
  directory, or state file); every check that did not run prints a NOT
  checked line naming the reason.
- [S-4] The first run with no baseline says so plainly — "no sync baseline
  recorded — run `procoder env --sync` once your setup is done" —
  lists the files it would track, and exits 0.
- [S-5] `procoder ci --runs` — the CI verdict for the current branch via
  `gh`: per workflow, the newest run's status, conclusion, and age,
  plus the failing job names when it failed.
- [S-6] The staleness verdict: when the newest run's head commit is not
  HEAD's, and HEAD is pushed, print "the newest run predates your
  latest push — CI has not judged this commit yet" — the forgotten
  step this exists to catch.
- [S-7] `.procoder/state/` joins the gitignore guidance: per-machine derived
  state, tracked no more than `.procoder/index/` is.

## Out of scope

- Running any install, migration, or `.env` edit. procoder names the
  step; the agent or the human runs it (P-CONTROL).
- Blocking anything. Both halves are judgment: the gate is untouched
  and neither command ever emits a blocking finding.
- Guessing what `--sync` means. A baseline is only ever written by an
  explicit `--sync`; no command records one as a side effect.
- Reading `.env` for anything but key names, or diffing values between
  `.env` and its example.
- Watching, polling, re-running, or waiting on CI — one snapshot per
  invocation, no `gh run watch`.
- Non-GitHub CI providers (GitLab, CircleCI, Jenkins): out of v1, and
  said out loud rather than silently unhandled.
- Changing existing `procoder ci` behaviour — workflow hygiene stays
  the default with no flag, unchanged.

## Constraints

- Pure Go stdlib. New package internal/envsync for Part A; Part B is a
  new entry point in internal/ciops beside the hygiene rules.
- P-CONTROL: the only file either half ever writes is
  `.procoder/state/env.json`, and only under `--sync`. Nothing else in
  the tree is touched by any code path, including error paths.
- Security, hard: a value from `.env`, `.env.example`, or any sibling
  is NEVER emitted, logged, hashed into visible output, or stored in
  the state file. Key names only. The state file records only the key
  names seen in the example file; the local `.env` is read, compared,
  and dropped.
- Honesty rule: gh missing, no GitHub remote, gh unauthenticated,
  detached HEAD, or a hung tool all yield NOT checked with the reason.
  A CI report with no runs listed is never printed as if it were green.
- Every path in output uses forward slashes on every platform — the
  Windows CI leg asserts it.
- 60s timeout on the gh invocation, answered with the hung-tool
  message: "gh gave no answer in 60s — the process was killed; CI runs
  were NOT checked".
- Hashing is SHA-256 of the file bytes; the state file stores hex
  digests of lockfiles, never their contents.

## Interfaces

- `procoder env` — report; exit 0 (including with findings), 1 when a
  check could not run, 2 on a bad flag.
- `procoder env --sync` — record the baseline; exit 0 on write, 1 when
  the state file cannot be written.
- `procoder ci` — unchanged workflow hygiene (the default).
- `procoder ci --runs` — the branch's run report instead of the
  hygiene report; exit 0 with findings, 1 when the run check could not
  run.
- `gh run list` for the branch, limited to 20, asked in JSON for
  workflowName, status, conclusion, createdAt, headSha and databaseId;
  then `gh run view` in JSON for the jobs of the newest failed run per
  workflow, to name the failing jobs.
- Usage text, docs.Commands, docs site, commands/env.md skill +
  OpenCode twin; commands/ci.md gains the `--runs` section.
- gitx.IgnoreCoverage gains the `.procoder/state/` entry, keyed on the
  presence of `.procoder/`.

## Data

- `.procoder/state/env.json` — procoder-owned, gitignored, written
  only by `procoder env --sync`. Same precedent as `.procoder/index/`
  and the bench baseline: derived, per-machine, never committed.
- Shape: a `version` integer (1), a `synced_at` RFC3339 stamp, a
  `lockfiles` object mapping each tracked path to its hex digest, a
  `migrations` object mapping each directory to its entry count and
  sorted entry names, and an `env_keys` object mapping each example
  file to the sorted key names it declared.
- Keys are repo-root-relative forward-slash paths. Entry lists are
  sorted so the file is stable and its diffs are readable.
- No value from any `.env`-family file is ever stored — the `env_keys`
  arrays hold key names and nothing else.
- Nothing else is stored; the CI half keeps no state at all and asks
  gh fresh every time.

## Edge cases

- No baseline yet → the no-baseline line, the tracked-file list, and
  exit 0 — not an error and not a finding.
- `.procoder/state/env.json` present but corrupt or of an unknown
  `version` → exit 1 naming the file and telling the reader to re-run
  `--sync`; never silently treated as an empty baseline.
- A tracked lockfile deleted since the baseline → "package-lock.json
  is gone since your last sync" rather than a hash mismatch.
- A lockfile that appeared after the baseline → reported as new with
  its install command, the same as a changed one.
- Migrations removed (a squash, a rebase) → the count is reported as
  changed, not as "N added"; a negative delta never prints.
- `.env` missing entirely while an example exists → every example key
  is new, and the report says the local `.env` does not exist.
- Multiple example files present → each is reported separately under
  its own name.
- Example keys that are commented out (`# FOO=`) or blank lines → not
  keys, not reported.
- Detached HEAD → the run check is NOT checked ("no current branch").
- A branch with no runs at all → "no CI runs for this branch" —
  distinct from a green verdict, and said as such.
- HEAD not pushed → the staleness verdict is skipped with "HEAD is not
  pushed — CI cannot have seen it".
- A workflow whose newest run is still in progress → status reported
  as in-progress with its age, and no conclusion claimed.
- A repo with no `.procoder/` directory → `procoder env` still runs
  and reports no baseline.

## Failure modes

- `.env.example` unreadable → NOT checked for that file, exit 1; the
  reason is the OS error, and no partial key list is printed.
- A lockfile unreadable (permissions, a broken symlink) → NOT checked
  for that lockfile with the reason, exit 1; other lockfiles still
  report.
- A migration directory unreadable → NOT checked for that directory,
  exit 1.
- `.procoder/state/` not creatable under `--sync` → exit 1 with the
  write error; no partial file is left behind (write to a temp file in
  the same directory, then rename).
- Any error path that would otherwise interpolate file content into a
  message prints the key name or the path only — an `.env` value never
  reaches output, not even inside an error string.
- gh not installed → "CI runs NOT checked — gh is not installed
  (https://cli.github.com)".
- No GitHub remote → "CI runs NOT checked — this repository has no
  GitHub remote".
- gh unauthenticated or an API error → NOT checked with gh's first
  error line ("gh auth login" included when gh says so).
- gh timeout at 60s → the hung-tool message; the hygiene half of
  `procoder ci` is unaffected because it never shells out.
- gh JSON unparseable or of an unexpected shape → NOT checked naming
  the parse failure, never an empty run table read as green.

## Acceptance criteria

- [x] [S-1] On a fixture with a recorded baseline and a mutated
      package-lock.json, `procoder env` names package-lock.json and
      prints `npm ci`, exiting 0.
- [x] [S-1] On a fixture whose `db/migrate` gained two files since the
      baseline, the output reads "2 migration(s) added since your last
      sync" and lists both names.
- [x] [S-1] On a fixture whose `.env.example` declares DATABASE_URL and
      REDIS_URL with only DATABASE_URL in `.env`, the output names
      REDIS_URL and no value from either file appears anywhere in the
      output — asserted by a test that plants a distinctive secret
      string as both values and greps the whole output for it.
- [x] [S-2] [S-4] With no `.procoder/state/env.json`, `procoder env` prints the
      no-baseline line naming `--sync` and exits 0; after a
      `--sync` run on the same tree, a second bare run reports no
      changes and exits 0.
- [x] [S-2] `procoder env --sync` writes exactly one file
      (`.procoder/state/env.json`) — a test snapshots the tree before
      and after and asserts a single added path — and the written JSON
      contains no `.env` value.
- [x] [S-3] A lockfile made unreadable yields a NOT-checked line naming it
      and exit 1, while the other lockfiles in the same fixture still
      report their verdicts.
- [x] [S-5] With gh absent from a stub PATH, `procoder ci --runs` prints the
      NOT-checked line naming gh and exits 1, and the same fixture run
      as bare `procoder ci` prints the unchanged hygiene findings.
- [x] [S-5] [S-6] Parse tests over recorded `gh run list --json` output cover a
      failing run (with failing job names), an in-progress run, an
      empty run list, and a newest run whose headSha differs from a
      pushed HEAD — the last producing the "newest run predates your
      latest push" line.
- [x] [S-1] Every path printed by either half uses forward slashes, asserted
      by a test that builds a nested fixture path and rejects any
      backslash in the output.
- [x] [S-7] `.procoder/state/` appears in the gitignore guidance the docs
      domain checks, alongside the state files `docs/commands.md` already
      names.

## Open questions

<!-- none — decisions recorded above -->
