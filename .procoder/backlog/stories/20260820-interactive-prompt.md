# Implement interactive prompt flow

Status: open 2026-08-20
Created: 2026-08-20
Epic: interactive-qa
Sprint: -

## Description

Implement the interactive terminal prompt flow that presents each question one at a time and collects the human's answer using free-text input.

## Acceptance criteria

- [ ] `promptQuestion(q Question) string` prints the question and reads from stdin
- [ ] `runInteractive(qs Questions, out func(string)) int` iterates all questions, collects answers
- [ ] Terminal check uses syscall.IsTerminal(int(os.Stdin.Fd())) (same pattern as copilot.Prompt)
- [ ] Empty input is recorded as "skip" (not as a valid answer)
- [ ] "skip" answers are recorded but marked as skipped, not resolved
- [ ] Questions with free-text answers accept user typing
- [ ] After all questions, prints summary: "answered: N / N questions (M skipped)"
- [ ] Function returns exit 0 always (never breaks the host)
- [ ] Interactive path only triggers when TTY() is true

## Evidence
