---
applyTo: "**/*.go"
---

# Go in procoder

Procoder is a Go CLI plus an agent layer. `cmd/procoder/main.go` is the
dispatch and every domain lives under `internal/<domain>/`. The rules
below are not style preferences — each one is here because breaking it
produced a defect that shipped.

## No silent green

The rule the whole tool is built on: **a check that could not run must
never read as one that passed.**

- A tool that is absent, a file that could not be read, a scanner that
  crashed — each is reported as NOT checked, naming the reason, and it
  fails the gate exactly as a real finding would.
- Never return `nil` findings on an error path. `if err != nil { return
nil }` is the shape that hides a whole domain.
- An empty result is not a clean result. If a check scanned zero files,
  say so; do not print "0 finding(s)", which is what a clean scan prints.
- When a tool fails, quote its **last** line of stderr, not its first.
  Scanners log progress first and the reason they gave up last, so the
  first line produced "dependencies were NOT checked: Starting filesystem
  walk for root: /" — alarming, and not the reason. Fall back to the exit
  status only when stderr is empty.

## P-CONTROL: the binary prints, the agent writes

No command may modify a file in the repository it is checking.
`procoder format` prints the formatted result for review. The one
exception is procoder's own state under `.procoder/`, written by
controllers that say so, and `--save`/`--yes` flags where writing is the
whole point of the flag.

## Paths and arguments

- A command taking paths must honour them. `procoder security <dir>`
  scanned the change set and ignored its argument, so a person who named
  a directory was told "0 finding(s)" about files nobody opened.
- Expand a directory argument to its files. Checks that work file by file
  silently do nothing with an unexpanded directory.
- Resolve relative paths against the repository root, not the process
  directory — procoder can be run from a subdirectory, and
  `gitx.ChangedFiles` hands out absolute paths, so mixing the two makes a
  check scan nothing.
- A flag a command does not implement is a usage error (exit 2), never a
  path. Every flag lives in `knownFlags` in `cmd/procoder/flags.go`, and a
  test holds that table against the literals main.go compares.

## Exit codes are the public interface

ADR 0003 pins them: **0 clean, 1 findings or a refusal, 2 usage.** A
command that exits 0 today must not start exiting 1 for the same input
without a major version. Asking a question whose answer is "nothing here"
is 0 — `sprint status` with no sprint open, `todo list` with no tasks.
Refusing to do something is 1.

Adding a new blocking check is a major change too: it can fail a build
that passed yesterday.

## Findings

Domains return `[]gitx.Finding`. A finding names the file, the line where
there is one, and what to do about it — specifically enough that a reader
who does not already know what is wrong can act. `Blocking` is for things
that are objectively wrong; judgment calls report.

Never echo a secret's value into a finding, a question, `QA.md`, or a hook
payload.

## Instructions procoder prints must work

If a message tells somebody to run a command, that command has to succeed.
Three shipped that could not: `brew install rubocop` (no such formula —
it is a gem), `composer global require phpstan/phpstan` (installs where
procoder then could not look), and `git tag -a v0.2.0` printed for a tag
that already existed. Verify the package name and the resulting path
before writing the instruction.

## Comments

Comments explain **why**, and the best ones name the defect that made the
line necessary. Write them for the person who will wonder whether the code
can be simplified — tell them what happened last time it was.
