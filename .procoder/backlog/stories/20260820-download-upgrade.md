# Create internal/releases/upgrade.go — Download & install

Status: open 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Implement the download-and-install flow that fetches the latest procoder binary from GitHub releases and replaces the running binary.

## Acceptance criteria

- [ ] `internal/releases/upgrade.go` created
- [ ] `DownloadAndInstall(latest string, out func(string)) int` downloads and installs
- [ ] Platform detection: use `GOOS/GOARCH` or `uname -s/-m` to determine dist directory (matches launcher.sh logic)
- [ ] Asset URL constructed from release: find matching `procoder-<os>-<arch>` in release assets
- [ ] Download to temp file via `os.CreateTemp`
- [ ] Temp file is made executable: `os.Chmod +x`
- [ ] Binary resolution: find the procoder binary from `os.Args[0]` or from the current working directory
- [ ] Atomic replace: rename temp file over the original binary (or copy then rename)
- [ ] Refuse to downgrade: compare current version before installing
- [ ] If download fails: leave current binary untouched, clean up temp file
- [ ] Interactive prompt: "Update procoder X.Y.Z → A.B.C? (y/N)"
- [ ] No TTY: print warning, do not hang
- [ ] Tests: mock HTTP server serves fake binary; download succeeds; temp file is cleaned up on failure

## Evidence
