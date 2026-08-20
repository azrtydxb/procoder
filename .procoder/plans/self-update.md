# Plan: Self-Update — Version Check & Upgrade

## Context

Procoder's binary version is stamped by release builds via `-ldflags`. There is no mechanism to check for updates, warn users, or install newer versions. The `procoder version` command just prints the version string.

This plan introduces `procoder version --check` (version check + upgrade prompt) and `procoder self-upgrade` (upgrade command), both using only Go stdlib.

## Architecture Decision

- Use `net/http` (stdlib-only) to query GitHub's releases API: `https://api.github.com/repos/azrtydxb/procoder/releases`
- Parse the first (most recent) release tag and compare semver
- Download the correct binary for the current OS/arch from the release assets
- Use a temp file + atomic rename for the upgrade (same pattern as launcher.sh resolver)
- Run the version check in a goroutine with a hard 1-second timeout — never blocks the caller
- The version check output goes to stderr when in a session hook (does not mix with hook output)

## Tasks

### 1. Create `internal/releases/releases.go` — GitHub release query

**File:** `internal/releases/releases.go` (new)
**Steps:**
1.1 Define `Release struct { TagName string; Assets []ReleaseAsset }` and `ReleaseAsset struct { URL string; Name string }`
1.2 Define `Latest(root string) (string, error)` — fetches `api.github.com/repos/azrtydxb/procoder/releases`, returns the first release's tag name
1.3 Use `fmt.Sprintf("https://api.github.com/repos/%s/releases", repoPath)` (repoPath="azrtydxb/procoder")
1.4 The function must accept a timeout parameter — callers pass a `time.Duration`
1.5 Returns error if network fails, if the API returns non-200, or if no tag found (e.g., private repo or rate limit)
1.6 Use `http.Client{Timeout: timeout}` to enforce hard deadline

**Evidence:** `Latest(ctx, 1*time.Second)` fetches the latest tag. Timeout is enforced.

### 2. Create `internal/releases/compare.go` — Semver comparison

**File:** `internal/releases/compare.go` (new)
**Steps:**
2.1 Define `Compare(current, latest string) int` — returns -1 (current newer), 0 (equal), 1 (latest newer)
2.2 Parse version strings as `Semver{Major, Minor, Patch int}` using `regexp.MustCompile` or manual parse
2.3 Compare major → minor → patch in order (semver precedence)
2.4 Handle edge cases: `dev` tag (unknown version → return 0), `v` prefix (strip it), empty strings (treat as equal)
2.5 Define `ShouldWarn(current, latest string) bool` — returns true only if latest is strictly newer AND the version is a release (not dev)
2.6 Tests for: identical versions, patch bump, minor bump, major bump, invalid strings, dev version

**Evidence:** `Compare("0.28.0", "0.28.1") == 1`, `Compare("0.28.1", "0.28.1") == 0`, `Compare("0.29.0", "0.28.1") == -1`.

### 3. Create `internal/releases/upgrade.go` — Download & install

**File:** `internal/releases/upgrade.go` (new)
**Steps:**
3.1 Define `DownloadAndInstall(latest string, out func(string)) int`
3.2 Determine the platform-specific asset name from the tag: construct from `github.com/azrtydxb/procoder/releases/latest` — use `uname -s` + `uname -m` to build asset name (e.g., `procoder-darwin-arm64`, `procoder-linux-amd64`)
3.3 Find the matching asset URL from the release's assets list
3.4 Download to a temp file in `/var/folders/` (or `os.TempDir()`)
3.5 Make the temp file executable (`os.Chmod +x`)
3.6 Prompt the user: "Update procoder %s → %s? (y/N)"
3.7 If yes: replace the current binary with the temp file (atomic rename)
3.8 If no / timeout / no TTY: delete temp file, exit 0
3.9 Defend: `cmd/procoder/main.go` knows the binary path. If invoked via `$PATH`, find the containing directory and replace the binary in-place. If invoked as a path (e.g., `./dist/procoder`), replace that specific file.
3.10 Defend: refuse to downgrade if already known (current version > latest)

**Evidence:** After upgrade, running `procoder version` prints the new version. Downgrade is refused.

### 4. Add `--check` flag to `version` command

**File:** `cmd/procoder/main.go` (edit)
**Steps:**
4.1 Modify the `"version"` case to optionally accept `--check`
4.2 `versionCmd` function:

- If no flags: print version string and exit 0 (existing behavior unchanged)
- If `--check`: call `releases.Latest(root, 1*time.Second)`
- If `latest != ""`, compare with current version
- If `latest` is newer: print warning to stderr, check TTY
- If TTY: prompt for upgrade (yes/no)
- If yes: call `upgrade.DownloadAndInstall(latest, out)`
- If no TTY / upgrade skipped: exit 0
- If no newer version: print "up to date" (or silent) and exit 0
- On network error: print warning to stderr but do not block ("version check failed — network unavailable")
- Must exit 0 always (version check failure is not a gate failure)
  4.3 Update usage text: `version [--check] [flags]`

**Evidence:** `procoder version` prints one line. `procoder version --check` may print warning + prompt.

### 5. Add `self-upgrade` command

**File:** `cmd/procoder/main.go` (edit)
**Steps:**
5.1 Add `"self-upgrade"` case to the command switch
5.2 `selfUpgradeCmd` function:

- Call `upgrade.DownloadAndInstall(latest, out)` directly (no version comparison needed — this command implies upgrade)
- Add to usage text: `self-upgrade        download and install the latest procoder version`
  5.3 Usage text location: after `version` in the existing usage block

**Evidence:** `procoder self-upgrade` downloads and installs the latest version on the platform.

### 6. Integrate version check into SessionStart hook

**File:** `internal/principles/principles.go` (edit)
**Steps:**
6.1 In `RunHook`, after printing the principles + status, check for version updates in a non-blocking way
6.2 The version check runs in a goroutine inside the hook: it prints to stderr (not stdout, so it does not corrupt the hook output)
6.3 If a newer version is available, print a short note to stderr (visible in the AI coder's output logs)
6.4 The note: `== procoder: version X.Y.Z is available (you have A.B.C)`
6.5 Include upgrade instruction: `Run \`procoder self-upgrade\` to update.`

**Evidence:** SessionStart hook still completes within the budget — version check runs in background and prints to stderr.

### 7. Update AGENTS.md with upgrade instruction

**File:** `AGENTS.md` (edit) — add to the principles or a new "Upgrading procoder" section
**Steps:**
7.1 When procoder says a newer version is available, the coder should not ignore it
7.2 Document: "If procoder prints a version warning, ask the user if they want to upgrade. The user can answer 'yes' and you should run `procoder self-upgrade`."

**Evidence:** The AGENTS.md (or skills) file documents the upgrade flow.

### 8. Test and validate

**Steps:**
8.1 Test `releases.Latest()` with mock HTTP server — verify it returns correct tag, handles 404, handles timeout
8.2 Test semver comparison — all edge cases covered
8.3 Test `procoder version --check` path — verify correct output
8.4 Test no-TTY path — verify no hang, warning still printed
8.5 Test `procoder version` still works without `--check`
8.6 Run `procoder check` — must pass
8.7 Run `procoder test` — must pass
8.8 Run `procoder lint` — must pass

**Evidence:** `go test ./internal/releases/...` passes. All validation steps succeed.

## Dependencies

- Task 1 creates the GitHub release query that Task 2 and Task 3 depend on
- Task 2 is independent (semver parsing)
- Task 3 depends on Task 1 (needs Latest to find asset URL)
- Task 4 depends on Task 1 and Task 2 (needs Latest and Compare)
- Task 5 depends on Task 1 and Task 3 (needs Latest and Download)
- Task 6 is independent (calls releases in goroutine)
- Task 7 is independent
- Task 8 is last (all pieces in place)

## Risk Assessment

| Risk                                                  | Impact                                           | Mitigation                                                                      |
| ----------------------------------------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------- |
| GitHub API rate limiting                              | Blocks version check for unauthenticated calls   | Accept graceful failure — don't block the hook if network fails                 |
| Binary not found on PATH (hooks use launcher.sh path) | Blocks Task 3/5 — can't find the file to replace | Resolve the binary path from the launcher.sh location; fall back to PATH search |
| Download fails mid-flight                             | Corrupts binary                                  | Use temp file + atomic rename; only replace on success                          |
| Network timeout freezes the hook                      | Blocks Task 6 — session stalls                   | Hard timeout of 1s; run in background goroutine; never blocks stdout            |
| Downgrade protection missed                           | Bad UX — installs older version                  | Compare before installing; refuse if current > latest                           |
