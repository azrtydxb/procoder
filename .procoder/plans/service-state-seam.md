# service-state-seam — implementation plan

Status: complete
Spec: .procoder/specs/service-state-seam.md

## Goal

Put every `.procoder/` read and write behind one typed package that locks,
writes atomically, and knows which repository it is serving.

## Architecture

A new package `internal/store` owns three primitives — a lockfile, an
atomic write, and a repository identity — and exposes one typed load/save
pair per `.procoder/` owner on top of them. The twenty-five packages that
read and write `.procoder/` today keep their exported signatures and lose
their direct filesystem calls; the only new user-visible surface is one
line in `procoder config` and one optional key in `.procoder/config.toml`.
Nothing starts a server and no file format changes.

## Constraints

Every task inherits these.

- **Zero dependencies.** The module file go.mod has no require block. No
  task may add one. Locking is stdlib only, which is why it is a lockfile
  and not flock.
- **P-CONTROL.** The store grants no write the binary could not already
  make. It is plumbing, not permission.
- **No behaviour change.** The same inputs produce the same bytes on
  stdout and the same bytes on disk. Task 7 asserts this.
- **A hook must never wedge a session.** No lock wait is unbounded.
- **Windows, macOS, Linux.** Paths compared and sorted as repo-relative
  slash-separated strings, never as OS paths.
- **Literal values, fixed here, used verbatim by every task:**
  - lock directory: `.procoder/state/locks/`
  - lock file name: first 16 hex characters of the SHA-256 of the
    repo-relative slash-separated path, plus `.lock`
  - lock file contents: two lines — the pid in decimal, then the unix
    time in seconds in decimal
  - stale threshold: 30 seconds
  - lock acquire timeout: 5 seconds
  - lock retry interval: 10 milliseconds
  - temp file prefix for atomic writes: `.procoder-tmp-`
  - identity string shape: `host/path`, host lower-cased, path case
    preserved, no scheme, no `user@`, no trailing `/`, no `.git` suffix

## Task 1: the lockfile

Files:

- `internal/store/lock.go` — acquire, release, stale detection, ordering
- `internal/store/lock_test.go` — its tests

Interfaces produced, consumed by tasks 5 and 6:

```go
// Lock takes an exclusive lock on each repo-relative path, in sorted
// order. The returned func releases every lock it took, and broken names
// any stale lock this call had to break, for the caller to report. On
// failure nothing is held and the error says which path and why.
//
// broken is returned rather than read from a package-level accessor:
// concurrent Lock calls would race on shared state, and this package
// exists precisely because that concurrency is real.
func Lock(root string, relPaths ...string) (release func(), broken []string, err error)
```

- [ ] Write `TestLockIsExclusive` in `internal/store/lock_test.go`:

```go
    func TestLockIsExclusive(t *testing.T) {
        root := t.TempDir()
        rel, _, err := Lock(root, ".procoder/state/dispatch.json")
        if err != nil { t.Fatalf("first lock: %v", err) }
        defer rel()
        if _, _, err := Lock(root, ".procoder/state/dispatch.json"); err == nil {
            t.Fatal("second lock succeeded while the first was held")
        }
    }
```

          Run `go test ./internal/store/` — expect FAIL with
          `undefined: Lock`.

- [ ] Implement `Lock`: for each path, sort the repo-relative slash paths
      byte-wise, then for each, `os.MkdirAll(root + "/.procoder/state/locks", 0o755)`
      and `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)`.
      On success write `fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Unix())`
      and close. On `os.IsExist`, retry every 10ms until 5 seconds have
      passed, then return an error reading
      `procoder: could not lock <rel> within 5s — the write was NOT made`.
      If any path in a multi-path call fails, release the locks already
      taken before returning.
- [ ] Run `go test ./internal/store/` — expect PASS.
- [ ] Write `TestStaleLockIsBrokenAndReported`:

```go
    func TestStaleLockIsBrokenAndReported(t *testing.T) {
        root := t.TempDir()
        plant(t, root, ".procoder/state/dispatch.json", os.Getpid(), time.Now().Add(-31*time.Second).Unix())
        rel, broken, err := Lock(root, ".procoder/state/dispatch.json")
        if err != nil { t.Fatalf("stale lock was not broken: %v", err) }
        defer rel()
        if len(broken) != 1 || !strings.Contains(broken[0], "dispatch.json") {
            t.Fatalf("breaking the lock was not reported: %v", broken)
        }
    }
```

          with a `plant` helper that writes the lock file for a repo-relative
          path with the given pid and unix time. Run — expect FAIL with
          `stale lock was not broken: procoder: could not lock ... within 5s`.

- [ ] Implement stale detection: before retrying, read the existing lock
      file. Break it — remove it and try again — when the file's own mtime
      is more than 30 seconds old, or when the contents parse and the
      recorded timestamp is more than 30 seconds in the past or in the
      future. Contents that do not parse on a file with a FRESH mtime are a
      lock being written this instant: O_EXCL creates it empty and the pid
      and timestamp arrive a moment later, so condemning on contents alone
      hands two callers the same lock.
      When a lock is broken, return its repo-relative path in `broken` so
      the caller can report it. Empty on the ordinary path.

- [ ] Run `go test ./internal/store/` — expect PASS.
- [ ] Write `TestUnreadableLockIsStale`: plant a lock whose contents are
      `not a lock`, and a second case whose timestamp is
      `time.Now().Add(time.Hour).Unix()`. Both must be broken and the
      lock taken. Run — expect FAIL for the future-timestamp case with
      `could not lock`, because only the age check exists so far.
- [ ] Extend the stale check with the unparsable and future-timestamp
      cases. Run `go test ./internal/store/` — expect PASS.
- [ ] Write `TestLiveLockRefusesRatherThanBlocks`: plant a lock with
      `os.Getpid()` and `time.Now().Unix()`, then

```go
    start := time.Now()
    _, _, err := Lock(root, ".procoder/state/dispatch.json")
    if err == nil { t.Fatal("write proceeded while a live lock was held") }
    if d := time.Since(start); d > 8*time.Second {
        t.Fatalf("Lock blocked for %v, past its 5s timeout", d)
    }
    if !strings.Contains(err.Error(), ".procoder/state/dispatch.json") {
        t.Fatalf("error does not name the file: %v", err)
    }
```

          Run — expect PASS if the timeout is already right; if it hangs, the
          retry loop has no deadline and the test is what says so.

- [ ] Write `TestLockOrderIsSortedPaths`:

```go
    func TestLockOrderIsSortedPaths(t *testing.T) {
        root := t.TempDir()
        a, b := ".procoder/backlog/stories/s1.md", ".procoder/backlog/sprints/001.md"
        done := make(chan error, 2)
        for _, pair := range [][2]string{{a, b}, {b, a}} {
            go func(p [2]string) {
                for i := 0; i < 50; i++ {
                    rel, _, err := Lock(root, p[0], p[1])
                    if err != nil { done <- err; return }
                    rel()
                }
                done <- nil
            }(pair)
        }
        for i := 0; i < 2; i++ {
            select {
            case err := <-done:
                if err != nil { t.Fatalf("lock failed: %v", err) }
            case <-time.After(30 * time.Second):
                t.Fatal("deadlock: locks were not taken in sorted order")
            }
        }
    }
```

          Run `go test ./internal/store/` — expect PASS.

- [ ] Run `procoder check` and commit.

## Task 2: the atomic write

Files:

- `internal/store/atomic.go` — write-temp-then-rename, temp sweeping
- `internal/store/atomic_test.go` — its tests

Interfaces produced, consumed by tasks 5 and 6:

```go
// WriteFile replaces the file at the repo-relative path with data, or
// leaves it exactly as it was. Creates parent directories as needed.
func WriteFile(root, relPath string, data []byte, perm os.FileMode) error

// ReadFile reads the file at the repo-relative path. An absent file
// returns an error satisfying os.IsNotExist, as os.ReadFile does.
func ReadFile(root, relPath string) ([]byte, error)
```

- [ ] Write `TestAtomicWriteLeavesOriginalOnRenameFailure` in
      `internal/store/atomic_test.go`. Make the rename fail by making the
      destination a directory that cannot be replaced:

```go
    func TestAtomicWriteLeavesOriginalOnRenameFailure(t *testing.T) {
        root := t.TempDir()
        rel := ".procoder/state/claims.json"
        if err := WriteFile(root, rel, []byte("original"), 0o644); err != nil {
            t.Fatalf("seed: %v", err)
        }
        // a directory where the temp file wants to go cannot be renamed over
        if err := os.MkdirAll(filepath.Join(root, ".procoder/state/claims.json.d"), 0o755); err != nil {
            t.Fatal(err)
        }
        forceRenameFailure = true
        t.Cleanup(func() { forceRenameFailure = false })
        if err := WriteFile(root, rel, []byte("replacement"), 0o644); err == nil {
            t.Fatal("WriteFile reported success though the rename failed")
        }
        got, _ := ReadFile(root, rel)
        if string(got) != "original" {
            t.Fatalf("target was modified: %q", got)
        }
    }
```

          Run `go test ./internal/store/` — expect FAIL with
          `undefined: WriteFile`.

- [ ] Implement `WriteFile`: `os.MkdirAll` the parent, create a temp file
      in the destination directory with `os.CreateTemp(dir, ".procoder-tmp-*")`,
      write, `Sync()`, `Close()`, `Chmod(perm)`, then `os.Rename` over the
      target. On any error remove the temp file and return the error with
      the target path in it. Add an unexported `forceRenameFailure bool`
      used only by the test — this is the seam the test needs and there is
      no portable way to make rename fail otherwise.
- [ ] Implement `ReadFile` as `os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))`.
- [ ] Run `go test ./internal/store/` — expect PASS.
- [ ] Write `TestReaderNeverSeesPartialFile`:

```go
    func TestReaderNeverSeesPartialFile(t *testing.T) {
        root := t.TempDir()
        rel := ".procoder/state/claims.json"
        old, new := strings.Repeat("a", 1<<16), strings.Repeat("b", 1<<16)
        if err := WriteFile(root, rel, []byte(old), 0o644); err != nil { t.Fatal(err) }
        stop := make(chan struct{})
        go func() {
            for {
                select {
                case <-stop: return
                default: _ = WriteFile(root, rel, []byte(new), 0o644)
                    _ = WriteFile(root, rel, []byte(old), 0o644)
                }
            }
        }()
        for i := 0; i < 2000; i++ {
            got, err := ReadFile(root, rel)
            if err != nil { continue }
            if s := string(got); s != old && s != new {
                close(stop)
                t.Fatalf("partial read of %d bytes", len(s))
            }
        }
        close(stop)
    }
```

          Run `go test ./internal/store/` — expect PASS. Replacing
          `WriteFile`'s body with a plain `os.WriteFile` makes it fail, which
          is what it is for.

- [ ] Write `TestTempFilesAreSwept`: leave a `.procoder-tmp-stale` file in
      `.procoder/state/`, call `WriteFile` into that directory, assert the
      stale temp file is gone and the written file is present. Run — expect
      FAIL with `stale temp file survived`.
- [ ] Implement the sweep: after a successful rename, remove entries in
      the destination directory whose name starts with `.procoder-tmp-`
      and whose modification time is more than 30 seconds old. Run
      `go test ./internal/store/` — expect PASS.
- [ ] Write `TestReadOnlyStateRefusesWrite`: `os.Chmod` the
      `.procoder/state` directory to `0o555`, call `WriteFile` into it,
      assert the error names the path, and assert the previously written
      file still holds its old contents. Skip the test when running as
      root, where the mode does not apply. Run `go test ./internal/store/`
      — expect PASS.
- [ ] Run `procoder check` and commit.

## Task 3: repository identity

Files:

- `internal/gitx/remotes.go` — exported remote lookup
- `internal/gitx/remotes_test.go` — its test
- `internal/store/identity.go` — the ladder and the normalisation
- `internal/store/identity_test.go` — its tests
- `internal/config/config.go` — parse `[service] repo`
- `internal/config/config_test.go` — its test

Interfaces produced, consumed by task 4:

```go
// Remotes maps remote name to URL, as git reports them.
func Remotes(root string) map[string]string          // internal/gitx

// Identity is the repository key and the rung that produced it. Rung is
// one of "config", "origin", "remote", "path".
type Identity struct{ Key, Rung, Detail string }     // internal/store

// IdentityFor resolves the ladder: cfgRepo, then origin, then the first
// remote in alphabetical order, then the resolved absolute root path.
func IdentityFor(root, cfgRepo string) Identity      // internal/store
```

`Config` gains one field, set from `[service] repo`:

```go
// ServiceRepo overrides the computed repository identity. Empty means
// compute it.
ServiceRepo string
```

- [ ] Write `TestRemotes` in `internal/gitx/remotes_test.go`: initialise a
      temp git repository, `git remote add origin git@example.com:o/r.git`,
      assert `Remotes(root)["origin"] == "git@example.com:o/r.git"`, and
      assert `Remotes(t.TempDir())` returns an empty map rather than
      panicking. Run `go test ./internal/gitx/` — expect FAIL with
      `undefined: Remotes`.
- [ ] Implement `Remotes` in `internal/gitx/remotes.go` using the existing
      unexported helper: `git(root, "config", "--get-regexp", `^remote\..*\.url`)`,
      splitting each line on the first space and taking the remote name
      from between the first and last dot of the key. A non-zero exit —
      no remotes, or not a repository — returns an empty map, never an
      error, because absence is the answer at that rung.
- [ ] Run `go test ./internal/gitx/` — expect PASS.
- [ ] Write `TestIdentityNormalisation` in
      `internal/store/identity_test.go`:

```go
    func TestIdentityNormalisation(t *testing.T) {
        for _, url := range []string{
            "git@host:o/r.git", "https://host/o/r.git",
            "ssh://git@host/o/r", "https://HOST/o/r/", "https://host/o/r",
        } {
            if got := normalise(url); got != "host/o/r" {
                t.Errorf("normalise(%q) = %q, want host/o/r", url, got)
            }
        }
    }
```

          Run `go test ./internal/store/` — expect FAIL with
          `undefined: normalise`.

- [ ] Implement `normalise`: strip a leading `scheme://`; strip everything
      up to and including a `@`; replace the first `:` after the host with
      `/` (the scp-like form); split host from path at the first `/`;
      lower-case the host; trim trailing `/`; trim a `.git` suffix; return
      `host + "/" + path`. A string matching none of these shapes is
      returned trimmed, unchanged.
- [ ] Run `go test ./internal/store/` — expect PASS.
- [ ] Write `TestIdentityLadder`, four cases against temp git repositories:
      `cfgRepo` set returns `Rung == "config"`; `origin` plus `fork`
      returns origin's normalised URL and `Rung == "origin"`; `fork` plus
      `upstream` and no origin returns fork's, `Rung == "remote"`,
      `Detail == "fork"`; no remotes returns the resolved absolute path
      and `Rung == "path"`. Run — expect FAIL with
      `undefined: IdentityFor`.
- [ ] Implement `IdentityFor` down the ladder. For the path rung use
      `filepath.EvalSymlinks` and fall back to the unresolved absolute
      path when that errors, so a path that cannot be resolved is still an
      answer.
- [ ] Run `go test ./internal/store/` — expect PASS.
- [ ] Write `TestIdentityBlankConfigKeyIgnored`: `IdentityFor(root, "   ")`
      against a repository with an origin must return the origin rung, not
      an empty key. Run — expect FAIL if the implementation tests only for
      the empty string; make it `strings.TrimSpace`.
- [ ] Write `TestIdentityWithoutGit`: `IdentityFor(t.TempDir(), "")` must
      return the resolved temp directory and `Rung == "path"`. Run — expect
      PASS.
- [ ] Add `ServiceRepo` to `Config` in `internal/config/config.go`, read
      from the `[service]` section's `repo` key, alongside the existing
      keys. Add `TestServiceRepoKey` in `internal/config/config_test.go`
      asserting that `[service]\nrepo = "acme/widgets"\n` loads as
      `ServiceRepo == "acme/widgets"`. Run `go test ./internal/config/` —
      expect FAIL with `cfg.ServiceRepo undefined`, then PASS.
- [ ] Run `procoder check` and commit.

## Task 4: procoder config prints the identity

Files:

- `internal/config/report.go` — one added line
- `internal/config/report_test.go` — its test

Interfaces consumed: `store.IdentityFor(root, cfgRepo string) store.Identity`
from Task 3.

- [ ] Write `TestConfigPrintsIdentityRung` in
      `internal/config/report_test.go`, one case per rung, asserting the
      output contains a line beginning `repo identity` that carries both
      the key and the rung wording:

| Rung   | wording in the line                       |
| ------ | ----------------------------------------- |
| config | `[service] repo in .procoder/config.toml` |
| origin | `origin remote`                           |
| remote | `first remote alphabetically: <name>`     |
| path   | `no remote — repository root path`        |

          Run `go test ./internal/config/` — expect FAIL with
          `output has no "repo identity" line`.

- [ ] In `Report`, after the settings table and before the problems block,
      print one blank line and then

```go
    id := store.IdentityFor(root, cfg.ServiceRepo)
    fmt.Fprintf(stdout, "repo identity  %s  (%s)\n", id.Key, id.Source())
```

          where `Source()` renders the four wordings above. Adding this to
          `Report` and not to `cfg.Settings` is deliberate: identity is not a
          setting with a default that can be relaxed, and putting it in the
          table would make it one.

- [ ] Run `go test ./internal/config/` — expect PASS.
- [ ] Run `procoder check` and commit.

## Task 5: typed pairs for the five state owners

Files:

- `internal/store/state.go` — the typed pairs listed below
- `internal/store/state_test.go` — its tests
- `internal/dispatch/dispatch.go` — `Load`/`Save` bodies call the store
- `internal/claims/claims.go` — same
- `internal/envsync/envsync.go` — same
- `internal/learn/learn.go` — `Append`/`Read` call the store
- `internal/hook/stop.go` — the handoff and digest files call the store

Interfaces produced:

```go
func LoadDispatch(root string) ([]byte, error)
func SaveDispatch(root string, data []byte) error
func LoadClaims(root string) ([]byte, error)
func SaveClaims(root string, data []byte) error
func LoadEnvState(root string) ([]byte, error)
func SaveEnvState(root string, data []byte) error
func AppendLearn(root string, line []byte) error
func LoadLearn(root string) ([]byte, error)
func LoadHandoff(root string) ([]byte, error)
func SaveHandoff(root string, data []byte) error
func LoadMarker(root, name string) ([]byte, error)
func SaveMarker(root, name string, data []byte) error
```

`LoadMarker`/`SaveMarker` cover `last-decisions-digest` and
`last-unasked-decision`, which are one-line files with no structure of
their own; `name` is the file name, not a path.

Interfaces consumed: `Lock` (Task 1); `ReadFile`, `WriteFile` (Task 2).

- [ ] Write `TestSaveDispatchLocksAndWritesAtomically` in
      `internal/store/state_test.go`: assert that while a lock on
      `.procoder/state/dispatch.json` is held by the test,
      `SaveDispatch` returns an error naming that path rather than
      writing. Run `go test ./internal/store/` — expect FAIL with
      `undefined: SaveDispatch`.
- [ ] Implement every pair above. Each save is `Lock` the one path,
      `WriteFile`, release. Each load is `ReadFile` with no lock — readers
      do not serialise, per the spec's Out of scope. `AppendLearn` locks
      `.procoder/state/learn.jsonl`, reads, appends the line, writes, and
      releases, so two appends cannot lose one another.
- [ ] Run `go test ./internal/store/` — expect PASS.
- [ ] Write `TestConcurrentAppendsBothSurvive` in
      `internal/store/state_test.go`:

```go
    func TestConcurrentAppendsBothSurvive(t *testing.T) {
        root := t.TempDir()
        var wg sync.WaitGroup
        for i := 0; i < 20; i++ {
            wg.Add(1)
            go func(i int) {
                defer wg.Done()
                if err := AppendLearn(root, []byte(fmt.Sprintf("{\"n\":%d}\n", i))); err != nil {
                    t.Errorf("append %d: %v", i, err)
                }
            }(i)
        }
        wg.Wait()
        got, err := LoadLearn(root)
        if err != nil { t.Fatal(err) }
        if n := bytes.Count(got, []byte("\n")); n != 20 {
            t.Fatalf("%d lines survived, want 20 — an append was lost", n)
        }
    }
```

          Run `go test ./internal/store/` — expect PASS. Removing the `Lock`
          call from `AppendLearn` makes it fail, which is the race this whole
          spec exists for.

- [ ] Replace the direct filesystem calls in `internal/dispatch`,
      `internal/claims`, `internal/envsync`, `internal/learn` and
      `internal/hook/stop.go` with the pairs above. Exported signatures do
      not change. Where `Broken()` reports a broken lock, append the
      sentence `procoder: broke a stale lock on <path>` to the command's
      existing output rather than inventing a new channel for it.
- [ ] Run `go test ./...` — expect PASS with no changes to any existing
      test.
- [ ] Run `procoder check` and commit.

## Task 6: typed pairs for the twenty content owners

Files:

- `internal/store/content.go` — the typed pairs
- `internal/store/content_test.go` — its tests
- the twenty owning packages, each losing its direct filesystem calls:
  `internal/adr`, `internal/analysis`, `internal/answers`,
  `internal/backlog`, `internal/bench`, `internal/codeindex`,
  `internal/config`, `internal/docs`, `internal/glossary`,
  `internal/gitcmd`, `internal/lessons`, `internal/lint`, `internal/plan`,
  `internal/principles`, `internal/review`, `internal/security`,
  `internal/spec`, `internal/templates`, `internal/todo`,
  `internal/wizard`

Interfaces produced. Directory-backed owners get a triple, single-file
owners get a pair:

```go
// Directory owners: adr, analysis, backlog (epics, stories, sprints,
// milestones), bench, plans, review lenses, review perspectives, specs,
// templates, todo, wizards, index.
func ListDir(root, relDir string) ([]string, error)
func LoadIn(root, relDir, name string) ([]byte, error)
func SaveIn(root, relDir, name string, data []byte) error

// Single-file owners: config.toml, context.md, PRINCIPLES.md,
// ask/decisions.md, ask/answers.md, ask/QA.md, docs/RULES.md,
// docs/mermaid.json, lint/RULES.md, security/RULES.md, and the six
// files under github/.
func LoadDoc(root, relPath string) ([]byte, error)
func SaveDoc(root, relPath string, data []byte) error
```

Interfaces consumed: `Lock` (Task 1); `ReadFile`, `WriteFile` (Task 2).

- [ ] Write `TestSaveInLocksItsOwnFile` in
      `internal/store/content_test.go`: hold a lock on
      `.procoder/backlog/stories/s1.md`, assert `SaveIn(root,
".procoder/backlog/stories", "s1.md", ...)` returns an error naming
      that path, and assert a save to `s2.md` in the same directory
      succeeds — per-file locking, not per-directory. Run
      `go test ./internal/store/` — expect FAIL with `undefined: SaveIn`.
- [ ] Implement `ListDir`, `LoadIn`, `SaveIn`, `LoadDoc`, `SaveDoc`. Every
      save locks exactly the one repo-relative path it writes. `ListDir`
      returns names sorted, and an absent directory returns an empty slice
      and a nil error — the same shape the callers already treat as "none".
- [ ] Run `go test ./internal/store/` — expect PASS.
- [ ] Write `TestMultiFileSaveTakesSortedLocks` in
      `internal/store/content_test.go`, driving a story-and-sprint save
      from two goroutines in opposite argument order fifty times each,
      failing after a 30 second timeout with `deadlock`. Run — expect PASS,
      because Task 1's `Lock` already sorts; this test is what keeps a
      later change from bypassing it.
- [ ] Replace the direct filesystem calls in each of the twenty packages
      listed under Files. Exported signatures do not change.
- [ ] Run `go test ./...` — expect PASS with no changes to any existing
      test.
- [ ] Run `procoder check` and commit.

## Task 7: the guard tests

Files:

- `internal/store/coverage_test.go` — the two structural guards
- `internal/store/deps_test.go` — the dependency guard
- `internal/store/golden_test.go` — the parity harness
- `internal/store/testdata/golden/` — the committed expected outputs

Interfaces consumed: everything produced by tasks 1 to 6. This task adds
no production code — it is the evidence that the previous six did what
they claimed.

- [ ] Write `TestStoreCoversEveryPathConstant` in
      `internal/store/coverage_test.go`: walk `internal/` and `cmd/` with
      `go/parser`, collect every string literal beginning `.procoder/`
      outside `internal/store` and outside `_test.go` files, and assert
      each appears in a `storePaths` slice declared in
      `internal/store/coverage_test.go` that lists the paths the store
      knows. Fail with `path constant %q has no store pair`. Run
      `go test ./internal/store/` — expect PASS once the slice is filled;
      adding a new constant elsewhere without a pair makes it fail.
- [ ] Write `TestNoDirectProcoderFileIO` in the same file: walk the same
      trees, and for each call to `os.ReadFile`, `os.WriteFile`,
      `os.Create`, `os.OpenFile` or `os.Remove` whose first argument
      mentions a `.procoder` literal or a variable named `path`, `p` or
      `dir` in a function that also mentions `.procoder`, fail with
      `%s:%d writes under .procoder/ without the store`. Exempt
      `internal/store` itself. Run `go test ./internal/store/` — expect
      PASS.
- [ ] Write `TestNoModuleDependencies` in `internal/store/deps_test.go`:
      read `go.mod`, fail if it contains the substring `require`. Run
      `go test ./internal/store/` — expect PASS.
- [ ] Build `TestMigrationOutputUnchanged` in
      `internal/store/golden_test.go`: a
      fixture repository written into `t.TempDir()` carrying a spec, a
      story, a sprint, a todo, an ADR, a `config.toml`, one Go file and one
      Markdown file, with a deterministic git history created by
      `git init`, `git add`, `git -c user.email=t@t -c user.name=t commit`
      with `GIT_AUTHOR_DATE` and `GIT_COMMITTER_DATE` fixed to
      `2026-01-01T00:00:00Z`. Run `procoder status`, `procoder check`,
      `procoder config` and each of the four hook entrypoints against it,
      capture stdout, and compare against files in
      `internal/store/testdata/golden/`.
- [ ] Generate the golden files from the commit BEFORE task 1 — check that
      commit out into a worktree, run the harness with `-update`, and
      commit the results. This ordering is the whole point: goldens taken
      after the migration would prove nothing.
- [ ] Run `go test ./internal/store/` — expect PASS. A byte of drift in
      any of the seven outputs fails with a diff.
- [ ] Run `procoder check` and commit.
