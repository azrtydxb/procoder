# repoint the pi install at a released tag

Status: closed 2026-09-01
Created: 2026-08-31

## Description

The user-scope pi install in `~/.pi/agent/settings.json` names this working
tree (`packages: ["/Users/pascal/Development/procoder"]`), so every pi session
on this machine loads the adapter from whatever branch the tree happens to be
on. That is deliberate for now — the user said "repoint later, but keep it in
the backlog" — but it means the machine's enforcement points move with an
`git checkout`, and a rebase or branch deletion can leave a session loading a
tree that no longer exists.

Done means the install points at a released tag of the procoder repository,
the local path entry is gone, and a fresh session still shows the gate
holding.

## Acceptance criteria

- [x] `~/.pi/agent/settings.json` names
      `git:github.com/azrtydxb/procoder@v<release>` (or the equivalent install
      form pi documents) instead of a local path, and no local path entry
      remains.
- [x] The repoint happens only AFTER the release it names is cut; pointing at a
      tag that does not exist is the failure this record exists to prevent.
- [x] A fresh `pi` session loads the extension and the commit gate still
      blocks an unclean commit (one probe, `git log` shows nothing landed).

## Evidence

2026-09-01, after v3.5.0 was published (release job 33537811388, green, the
five platform binaries and SHA256SUMS under the GitHub release):

- `pi remove ../../Development/procoder` then
  `pi install git:github.com/azrtydxb/procoder@v3.5.0`; the settings file now
  carries `"packages": ["git:github.com/azrtydxb/procoder@v3.5.0"]` and the
  local path entry is gone. The clone sits at
  `~/.pi/agent/git/github.com/azrtydxb/procoder`, and its
  `.claude-plugin/plugin.json` says `"version": "3.5.0"`.
- Probe: a scratch repo with a staged, un-parseable `bad.go` and a
  `.procoder/` config; `pi --no-session` asked to run `git commit -m probe`
  reported the gate's refusal — `bad.go` UNCHECKED (gofmt found no package
  clause) with a blocking finding — and `git log` on that repo shows no
  commits: `your current branch 'main' does not have any commits yet`.
- The install's own launcher closed the loop the record was written to
  prevent: `hooks/launcher.sh` read `3.5.0` from the installed manifest,
  fetched the release binary, verified it against the published SHA256SUMS,
  and `procoder version --check` answered `procoder 3.5.0 is the latest
release — nothing to do`. Nothing on this machine moves with a
  `git checkout` any more.

