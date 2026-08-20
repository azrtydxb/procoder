# Update principles hook with Q&A behavior instructions

Status: open 2026-08-20
Created: 2026-08-20
Epic: interactive-qa
Sprint: -

## Description

Add a new "Asking the user" section to the principles text that instructs AI coders on how to behave when presented with procoder questions.

## Acceptance criteria

- [ ] `principles.go` Default text gains a new "## Asking the user" section
- [ ] Section is placed after "Communicating: ADHD/ASD-friendly formatting" and before "Output preferences"
- [ ] Section instructs: do NOT guess when asked by procoder
- [ ] Section instructs: stop processing and ask the human the exact question text
- [ ] Section instructs: use `procoder ask --file` or write answers to `.procoder/ask/answers.md`
- [ ] Section instructs: re-run `procoder check` after answering
- [ ] Section is under 15 lines (principles are injected at every session start)
- [ ] Both `Run` and `RunHook` include the new section
- [ ] Section text uses the same tone and formatting as the existing principles

## Evidence
