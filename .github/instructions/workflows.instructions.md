---
applyTo: ".github/workflows/**"
---

# CI in procoder

Two tiers, kept apart on purpose: **the gate answers about the change, CI
answers about the tree.** Do not move a whole-tree check into the gate to
make a commit safer, and do not repeat the gate's answer in CI to make the
pipeline look thorough.

## Pins

Every tool whose answer the gate reports is pinned in
`.github/tool-versions.env`, for the reason that file states in its own
opening paragraph: the gate's verdict must not change because somebody
upstream released something overnight.

**Installing a tool from apt is not a pin.** gitleaks was installed with
`apt-get install gitleaks`, one line above a comment explaining that the
versions are pinned, so CI ran 8.16.0 while a developer's machine ran
8.30.1 — and the two disagreed about whether the tree contained a secret.
The gate gave two different verdicts for the same files.

A tool added to the download step must also be added to the loop that
installs those binaries onto PATH, and to the cache key. `procoder doctor`
runs afterwards and fails the job when one did not arrive, which is what
that step is for.

## Actions

Every action is pinned by commit SHA with its tag in a trailing comment.
Every job carries `timeout-minutes`. Concurrency cancels in progress.
`procoder ci` checks all three and will tell you which is missing.

## Honesty in a job

A step that could not run must fail the job or say so in its output.
"Nothing to do" and "did not run" are different results and CI is the last
place that distinction survives — nobody reads a green pipeline's log.

Prefer a downloaded release binary over `go install …@latest`: compiling
on every run cost 92 seconds for one tool, and `@latest` cannot be cached
or pinned.
