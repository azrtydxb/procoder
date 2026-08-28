# service-state-seam

Status: complete

## Problem

Every read and write of `.procoder/` today is an `os.ReadFile` or an
`os.WriteFile` issued from whichever package happens to own that path —
twenty-five of them, each with its own path constant and its own idea of
what a failed write means. That was correct while procoder was a
short-lived process: one hook ran, touched a file, and exited before the
next one started.

Issue #117 ends that. A daemon serving several sessions and several repos
has to answer three questions this code cannot answer today.

**Who is writing?** There is no locking anywhere in `internal/` — the only
mutex in the tree is in a test. `dispatch.json`, `claims.json` and the ask
ledger are read-modify-write: load the whole file, change one field, write
the whole file back. Two hooks racing on the same wave lose one of the two
updates, silently, and today that is rare only because each hook is its own
short-lived process. The daemon makes it ordinary.

**Which repository is this?** The only answer procoder has is the
filesystem path from `tools.RepoRoot`. One daemon serving ten checkouts
needs a key that means the same thing on two machines, and a path does not.

**What did a partial write leave behind?** Every write is a plain
`os.WriteFile`, which truncates first. A process killed mid-write leaves a
truncated `claims.json` — valid JSON is not guaranteed, and the next reader
reports a corrupt ledger for a crash that had nothing to do with it.

None of this is visible from outside procoder today, which is why it has
survived. It becomes the daemon's first three bugs.

## Users

- **An agent in a session** — needs a hook that writes state to either
  write it or say it did not. A lost dispatch record or a half-written
  claims ledger is worse than a refusal, because it looks like success.
- **A person running two Claude sessions in one repo** — needs the second
  session's write not to erase the first's. Today it does.
- **Whoever builds the daemon (#117 phase 1)** — needs one place where
  state access happens, so the daemon can serialise it, and one stable
  repo key, so it can tell its ten checkouts apart.
- **Whoever reviews this change** — needs to see that behaviour did not
  change, which is why byte-identical output is an acceptance criterion
  and not a hope.

## In scope

- [S-1] A new `internal/store` package: one typed load/save pair per
  `.procoder/` owner, covering every path constant in the tree. Typed, not
  a byte store — each operation names what it reads, so the daemon can
  reason about it later.
- [S-2] Migrate all twenty-five owners onto it in one change: adr,
  analysis, answers, backlog, bench, claims, codeindex, config, dispatch,
  docs, envsync, glossary, gitcmd, learn, lessons, lint, plan, principles,
  review, security, spec, `internal/templates`, todo, wizard, and the
  handoff files in `internal/hook/stop.go`. No behaviour change: the same inputs produce the same
  bytes on stdout and the same bytes on disk.
- [S-3] Every write through the store is atomic: write a temp file in the
  destination directory, fsync, rename over the target. A reader never sees
  a partial file and a crash never truncates one.
- [S-4] Per-file advisory locking around read-modify-write, using an
  an O_EXCL lockfile with stale detection. No new dependency: the module
  file go.mod has no require block and this change does not add one.
- [S-5] Multi-file operations acquire their locks in sorted path order,
  always. One mechanical rule, so a lock cycle cannot be introduced by
  accident.
- [S-6] A stable repository identity, resolved down a fixed ladder:
  `[service] repo` in `.procoder/config.toml`, then the normalised URL of
  the `origin` remote, then the normalised URL of the first remote in
  alphabetical order, then the resolved absolute path of the repository
  root. Each rung says which one answered.
- [S-7] `procoder config` prints the identity and the rung that produced
  it, so it is observable without a daemon and testable without one.

## Out of scope

- **The daemon.** No socket, no server, no `procoder serve`. This change
  adds the seam the daemon needs and nothing that uses it. #117 phase 1.
- **Team mode.** No network, no remote store, no second implementation of
  the store interface. #248, parked.
- **Changing any file format.** Every file keeps its current shape, so a
  repository that upgrades sees no diff in `.procoder/` and a repository
  that downgrades keeps working.
- **Changing what any domain decides.** The gate reaches the same verdict,
  the hooks emit the same text. Only the plumbing underneath moves.
- **Read locking.** Writers serialise against writers. A reader may observe
  a file that a writer is about to replace; the atomic rename in S-3 means
  what it observes is always a whole file, which is the property that
  matters.
- **Distinguishing two clones of the same repository on one machine.** The
  identity in S-6 is deliberately the same for both — see Edge cases.

## Constraints

- **Zero dependencies.** The module file go.mod has no require block at all. Locking
  must be pure stdlib, which rules out `golang.org/x/sys` and therefore the
  portable `flock`/`LockFileEx` pair. Hence the lockfile in S-4.
- **P-CONTROL.** The store does not acquire the right to write anything the
  binary could not write before. It is plumbing, not permission.
- **The gate runs on every commit.** Locking and the atomic rename sit on
  the hot path. The added cost must not be measurable against the existing
  gate, whose legs are seconds long.
- **A hook must never wedge a session.** A lock that cannot be taken fails
  the operation with a message; it never blocks indefinitely.
- **Windows, macOS and Linux.** The lockfile, the rename and the identity
  normalisation all behave the same on all three. `os.Rename` over an
  existing file is the one that historically differs, and is called out in
  Failure modes.
- **No behaviour change is a testable claim, not a promise.** See the
  acceptance criteria.

## Interfaces

**`internal/store`** — a Go package, internal API only, no CLI surface. One
typed pair per owner, for example:

```go
func LoadWaves(root string) ([]dispatch.Wave, error)
func SaveWaves(root string, ws []dispatch.Wave) error
```

Callers keep their current signatures. What changes inside each package is
that the stdlib read/write pair becomes a store call.

**`procoder config`** gains one line, printed with the existing settings:

```
repo identity  git@github.com:azrtydxb/procoder.git  ([service] repo in .procoder/config.toml)
repo identity  git@github.com:azrtydxb/procoder.git  (origin remote)
repo identity  git@github.com:azrtydxb/procoder.git  (first remote alphabetically: upstream)
repo identity  /Users/x/src/thing                    (no remote — repository root path)
```

**`.procoder/config.toml`** gains one optional key:

```toml
[service]
repo = "acme/widgets"     # overrides the computed identity
```

## Data

**Lock files** live under `.procoder/state/locks/`, which is already
gitignored, never beside the file they protect — a lock file next to
`.procoder/specs/foo.md` would show up in `git status` and in review. One
lock per protected path, named by a hash of the repo-relative path so the
name is filesystem-safe on every platform. Contents: the pid and the unix
timestamp at which the lock was taken, one per line, for the stale check.

**Temp files** for the atomic write live in the destination directory —
`os.Rename` is only atomic within a filesystem — under a fixed prefix
(`.procoder-tmp-`), removed on successful rename and swept on the next
successful write into the same directory.

**The identity** is computed on demand and cached for the lifetime of the
process. It is not written to disk: a cached file would be one more thing
to invalidate when somebody adds a remote.

**Nothing else moves.** Every `.procoder/` file keeps its current path,
name and format.

## Edge cases

- **No `.git` at all.** `tools.RepoRoot` returns the directory it was given.
  Identity falls back to that resolved absolute path, and says so.
- **`.git` is a file, not a directory** — a worktree or a submodule.
  `os.Stat` succeeds for both today, so root resolution already works; the
  remote lookup must run in that directory and not in the parent.
- **No remotes.** Falls to the path rung.
- **`origin` exists alongside others.** `origin` wins, regardless of
  alphabetical order — this is the case that made the pure alphabetical
  rule wrong: a colleague who adds a remote named `fork` would otherwise
  key the same repository differently from everybody else.
- **`[service] repo` set to an empty or whitespace string.** Treated as
  unset and the ladder continues, rather than an empty identity.
- **Remote URL forms.** `git@host:o/r.git`, `https://host/o/r.git`,
  `ssh://git@host/o/r`, a trailing slash, an uppercase host, a `.git`
  suffix present or absent — all normalise to one key.
- **Two clones of the same repository on one machine** produce the same
  identity, deliberately. Locking is per file path and is unaffected;
  anything that must tell the two apart uses the repository root, which is
  what the path rung already provides.
- **Stale lock from a killed process.** A lock older than the stale
  threshold is broken and taken, and the fact that it was broken is
  reported, not swallowed.
- **A lock file whose contents do not parse.** Treated as stale — a lock
  nobody can interpret cannot be proven live.
- **Lock timestamp in the future** (clock skew, a restored backup). Treated
  as stale, for the same reason.
- **`.procoder/` does not exist.** No state to protect; the store creates
  what it needs on first write and reads report absence as absence, exactly
  as they do today.
- **Read-only filesystem, or `.procoder/state/` not writable.** Covered in
  Failure modes.
- **Two processes racing for the same lock.** `O_EXCL` gives exactly one
  winner; the loser retries until its timeout.
- **A multi-file operation whose paths sort differently on two platforms.**
  The sort is over repo-relative slash-separated paths, byte-wise, so it is
  identical everywhere.

## Failure modes

- **The lock cannot be taken within the timeout.** The operation reports
  that it did NOT write, and why. It never proceeds unlocked and never
  blocks past the timeout — a hook that hung would take the session with
  it. The timeout is short relative to the gate's own deadline.
- **The lock directory cannot be created or written** (read-only tree,
  permissions). Same answer: the write is refused with the reason. A store
  that fell back to writing unlocked would reintroduce the race it exists
  to remove, and would do it silently.
- **`os.Rename` fails.** The temp file is removed and the operation
  reports failure. The target is untouched — the previous contents survive,
  which is the whole point of writing this way.
- **The temp file cannot be created.** Reported. Nothing is truncated,
  because nothing was opened for writing over the target.
- **`git` is missing, or the directory is not a repository.** Identity falls
  to the path rung and says which rung answered. It never reports a
  computed identity it did not compute.
- **A remote URL that does not parse as any known form.** Used verbatim
  after trimming, and the rung reported names the remote, so a surprising
  key is traceable to its source rather than mysterious.
- **A crash between temp-write and rename.** A `.procoder-tmp-` file is
  left behind; the next successful write into that directory sweeps it. The
  target file was never touched.
- **A crash while holding a lock.** The lock file is left behind and is
  broken by the stale check on the next attempt.

## Acceptance criteria

- [ ] [S-1] `TestStoreCoversEveryPathConstant` enumerates every
      `.procoder/` path constant in the tree and asserts each has a typed
      store pair. Fails if a constant is added without one.
- [ ] [S-2] `TestNoDirectProcoderFileIO` greps the tree for stdlib file
      reads and writes against a `.procoder/` path outside
      `internal/store`. Fails if any package reintroduces one.
- [ ] [S-2] `TestMigrationOutputUnchanged` runs `procoder check`,
      `procoder status` and the four hook entrypoints over a fixture
      repository and compares stdout against committed golden files. Fails
      if any byte of any output moves.
- [ ] [S-3] `TestAtomicWriteLeavesOriginalOnRenameFailure` makes the
      rename fail after the temp file is written. Fails if the target file
      differs by a single byte from its previous contents.
- [ ] [S-3] `TestReaderNeverSeesPartialFile` reads a file in a loop while a
      writer replaces it repeatedly. Fails if any read returns anything
      other than the complete old or complete new contents.
- [ ] [S-4] `TestConcurrentAppendsBothSurvive` runs two processes each
      appending a wave to `.procoder/state/dispatch.json`. Fails if either
      wave is missing — and fails today, without the lock, which is what
      makes it worth writing.
- [ ] [S-4] `TestStaleLockIsBrokenAndReported` plants a lock older than the
      stale threshold. Fails if the write is refused, or if it succeeds
      without reporting that the lock was broken.
- [ ] [S-4] `TestUnreadableLockIsStale` plants a lock whose contents do not
      parse and one whose timestamp is in the future. Fails if either is
      treated as live.
- [ ] [S-4] `TestLiveLockRefusesRatherThanBlocks` holds a lock past the
      timeout from a live process. Fails if the write proceeds unlocked, or
      if the call has not returned a message naming the file by the time
      the timeout has passed.
- [ ] [S-4] `TestNoModuleDependencies` reads go.mod. Fails if a require
      block has appeared.
- [ ] [S-5] `TestLockOrderIsSortedPaths` drives two operations over the
      same two files in opposite request order. Fails if either deadlocks,
      or if locks are taken in anything other than sorted repo-relative
      path order.
- [ ] [S-6] `TestIdentityLadder` asserts the order: a `[service] repo`
      value beats the `origin` remote, `origin` beats an alphabetically
      earlier remote named `fork`, and a repository with no remote uses its
      resolved absolute root path. Fails if any rung overtakes the one
      above it.
- [ ] [S-6] `TestIdentityNormalisation` feeds `git@host:o/r.git`,
      `https://host/o/r.git`, `ssh://git@host/o/r`, `https://HOST/o/r/` and
      `https://host/o/r`. Fails if they do not all produce one identity.
- [ ] [S-6] `TestIdentityBlankConfigKeyIgnored` sets `[service] repo` to
      whitespace. Fails if the identity is empty rather than falling to the
      next rung.
- [ ] [S-6] `TestIdentityWithoutGit` runs in a directory with no `.git`.
      Fails if the identity is anything but the resolved absolute path, or
      if the reported rung does not say so.
- [ ] [S-7] `TestConfigPrintsIdentityRung` runs `procoder config` against
      a fixture per rung. Fails if the output omits the identity, or names
      a rung other than the one that answered.
- [ ] [S-3] [S-4] `TestReadOnlyStateRefusesWrite` makes
      `.procoder/state/` unwritable. Fails if the write succeeds, if the
      refusal does not name the reason, or if any file under `.procoder/`
      has changed afterwards.

## Open questions
