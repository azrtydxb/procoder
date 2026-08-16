# Security Policy

## Reporting a vulnerability

Please use [GitHub's private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability)
on this repository, if it is enabled. **It needs enabling in repo settings**
(Settings → Security → Code security → Private vulnerability reporting) — this
document does not confirm it is on. If it is not available when you go to
report, open a regular issue asking for a private channel rather than
describing the vulnerability in public.

There is no published response-time commitment and no PGP key for this
project — do not assume either exists.

## What procoder actually does, so you can assess it

procoder runs inside your editor, on every file write, as a Claude Code hook.
Two things about that are worth knowing before you trust it with your code:

### It executes external linters it finds configured in your project

`hooks/checks/resolve.js` detects an already-configured linter (eslint, ruff,
golangci-lint, `cargo clippy`, …) and runs it with `execFileSync` — never
`exec`, so no shell is ever involved and no shell-injection vector exists
through argv. The command line (`tool.argv(absPath)`) is built from a static,
hardcoded table of known tools plus a filesystem path to the file being
checked. It is never built from file contents, and no shell interpolation
happens at any point. A `timeout` and `maxBuffer` bound the process; a linter
that hangs, crashes, or is missing degrades to procoder's own built-in checks
for that file rather than to silence.

In short: procoder will invoke binaries already on your `PATH` that your own
project's config already names. It does not download or install anything, and
it does not construct a command from anything an attacker could put in a file.

### It reads arbitrary repository files, and can phone home once a day

The hook reads whatever file it is asked to check — that is its job, and
that access is as broad as any editor plugin's.

Separately, when enabled (the default), procoder checks once a day whether a
newer version has been published: a single unauthenticated `GET` to a static
file on `raw.githubusercontent.com`. That request sends nothing about you, your
project, or your code — no identifiers, no telemetry, no code contents. Set
`PROCODER_NO_UPDATE_CHECK=1` to turn it off entirely: no request, no
background process, no notice. `PROCODER_NO_HOOK=1` disables every hook,
including this one.

## Supported versions

This project is pre-1.0 and does not maintain parallel supported branches —
security fixes land on the latest release. Update with `/procoder:update` or
`npm install` from the tag you track.
