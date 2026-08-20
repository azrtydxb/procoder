# gate-enforcement

Status: complete

## Problem

The README promises "a commit gate it cannot talk its way past", but
nothing enforces it. There are two hooks — principles at session start
and format/lint/secrets on write — and neither intercepts a commit.
`procoder check` is an instruction in AGENTS.md that a model can skip
silently, and across a long session it is skipped exactly when it
matters most: at the end, under time pressure, when the agent believes
it is done. The product's headline claim is currently an honour system.

## Users

- The agent, which should be stopped at the commit boundary rather than
  trusted to remember the gate.
- Pascal, who wants the promise to be true on every host and in a plain
  terminal, not only where a Claude hook happens to be installed.
- Any repository adopting procoder: the gate is the reason to adopt it.

## In scope

- A PreToolUse hook on Bash that inspects the command about to run and,
  when it is a `git commit`, runs the gate first: blocking findings stop
  the commit and are handed back to the agent as the reason; a clean
  gate lets it through untouched.
- `procoder hook pre-tool-use` — the binary side of that hook, reading
  the host's PreToolUse payload on stdin and answering in the host's
  JSON shape (Claude Code today; the same envelope work `principles
--hook` already does per host).
- Escape hatches, both visible: `git commit --no-verify` passes through
  with a loud note that the gate was bypassed, and
  `[git] commit_gate = "report" | "off"` in config.toml (D-OVERRIDE)
  downgrades or disables the interception. Default is `block`.
- `procoder hook install-git` — prints a `.git/hooks/pre-commit` script
  (and the one command that installs it) so the gate also holds for
  commits made outside any agent: a human in a terminal, an IDE button,
  another tool. Printed, never written: `.git/hooks` is the user's.
- Host coverage stated honestly in docs/portability.md: which hosts
  support command interception, and which fall back to the git hook.

## Out of scope

- Intercepting anything other than `git commit` (no push, merge, or
  rebase interception).
- Running the test suite at the commit boundary; the gate's own scope
  is unchanged, and `[test] policy = "block"` already governs closes.
- Rewriting or amending the commit, staging files, or generating the
  message.
- Installing the git hook automatically, or touching `.git/` at all.
- Interception on hosts that expose no pre-command hook — they get the
  git-hook path and a documented tier, not a silent gap.

## Constraints

- Pure Go stdlib; the hook payload parsing joins internal/hook.
- P-CONTROL holds: the hook computes a verdict and hands it back; it
  never edits files, never stages, never commits.
- The honesty rule: a gate that could not run (unreadable payload,
  broken tooling) must NOT read as clean — it reports that it could not
  judge, and under `block` that stops the commit.
- Latency: the interception runs the same `gate.Run` the agent would
  run; the hook declares a 120-second timeout, and a timeout is
  reported, never treated as clean.
- Detection of a commit command must survive real shells: leading `env`
  or directory changes, `&&` chains, quoted paths, `-C <dir>`. False
  positives are worse than false negatives here, so the matcher looks
  for a `git` invocation whose first non-flag argument is `commit`.

## Interfaces

- `procoder hook pre-tool-use` — stdin: the host's PreToolUse JSON;
  stdout: the host's decision envelope (deny with the gate's findings,
  or allow). Exit 0 always; the decision lives in the payload.
- `procoder hook install-git` — stdout: the pre-commit script content
  plus the install command; exit 0.
- config: `[git] commit_gate = "block" | "report" | "off"` (default
  block).
- hooks/claude-hooks.json gains the PreToolUse registration with
  matcher `Bash`; hooks/copilot-hooks.json gains its equivalent where
  the host supports it.

## Data

- No new state. The hook reads the payload, runs the existing gate over
  the changed files, and answers.

## Edge cases

- `git commit --no-verify` → allowed, with a line saying the gate was
  bypassed deliberately (visible in the transcript, never silent).
- A compound command (`go build && git commit -m x`) → the commit is
  detected and the whole command is judged; blocking findings stop it.
- `git commit` inside a subdirectory or with `-C <dir>` → the gate runs
  against the repository root that git itself would use.
- A commit with nothing staged → the gate has no changed files and
  passes; git's own "nothing to commit" answer follows.
- A message merely containing the word commit (`echo "commit"`) → not
  a commit; the matcher requires a git invocation.
- `gh pr merge`, `git merge --continue`, and other commit-creating
  commands are out of scope and pass through (stated, not silent).
- Repository with no `.git` → nothing to intercept; allow.
- `commit_gate = "report"` → findings are printed, the commit proceeds.

## Failure modes

- Malformed or truncated payload on stdin → allow the command and print
  that the gate could not judge (a broken hook must never wedge the
  user's session — the precedent in internal/hook).
- Gate tooling missing (a formatter absent) → the gate's existing
  "unchecked is never clean" rule already makes those findings
  blocking, and the commit is stopped with the reason.
- Timeout → reported as NOT judged; under `block` the commit is stopped
  and the message says to run `procoder check` directly.
- Host that ignores the decision envelope → the git hook remains the
  backstop; the portability table says which hosts need it.

## Acceptance criteria

- [ ] With the hook installed, a `git commit` on a tree with a blocking
      finding is stopped and the refusal names the finding.
- [ ] The same commit succeeds once the finding is fixed, with no extra
      ceremony.
- [ ] `git commit --no-verify` proceeds and the output says the gate was
      bypassed.
- [ ] A compound `... && git commit -m x` is detected; `echo "commit"`
      and `gh pr merge` are not.
- [ ] `[git] commit_gate = "report"` prints findings and allows the
      commit; `"off"` skips the check entirely.
- [ ] A malformed payload allows the command and says the gate could
      not judge — verified by a test feeding broken stdin.
- [ ] `procoder hook install-git` prints a working pre-commit script
      that runs the gate and returns non-zero on blocking findings, and
      writes nothing itself.
- [ ] docs/portability.md states, per host, whether command
      interception is supported or the git hook is the fallback.

## Open questions

<!-- none — decisions recorded above -->
