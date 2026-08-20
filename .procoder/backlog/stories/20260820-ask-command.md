# Implement `ask` command at top level

Status: open 2026-08-20
Created: 2026-08-20
Epic: interactive-qa
Sprint: -

## Description

Add the `procoder ask` command to the top-level CLI dispatcher with `--file <path>` flag support.

## Acceptance criteria

- [ ] `procoder ask` command registered in `cmd/procoder/main.go` switch statement
- [ ] `askCmd` function calls `ask.Collect(root)` to get questions
- [ ] If no questions: prints "no questions to ask" and exits 0
- [ ] If TTY: runs interactive prompt, collects answers, exits 0
- [ ] If no TTY: writes `.procoder/ask/QA.md`, exits 1 with instruction
- [ ] `--file <path>` flag: loads existing answers from file, writes to `answers.md`, exits 0
- [ ] Usage text includes `ask [--file <path>]`
- [ ] Command resolves root correctly from working directory
- [ ] Exit codes: 0 = questions answered or no questions; 1 = questions unanswered (no TTY)

## Evidence
