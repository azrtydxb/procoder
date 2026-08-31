# repoint the pi install at a released tag

Status: open
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

- [ ] `~/.pi/agent/settings.json` names
      `git:github.com/azrtydxb/procoder@v<release>` (or the equivalent install
      form pi documents) instead of a local path, and no local path entry
      remains.
- [ ] The repoint happens only AFTER the release it names is cut; pointing at a
      tag that does not exist is the failure this record exists to prevent.
- [ ] A fresh `pi` session loads the extension and the commit gate still
      blocks an unclean commit (one probe, `git log` shows nothing landed).

## Evidence
