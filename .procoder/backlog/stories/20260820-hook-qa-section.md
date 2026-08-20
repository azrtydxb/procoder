# Update PostToolUse hook to inject Q&A section

Status: open 2026-08-20
Created: 2026-08-20
Epic: interactive-qa
Sprint: -

## Description

Modify the PostToolUse hook (`internal/hook/hook.go`) to inject a Q&A section into `additionalContext` when there are pending questions from the just-written file.

## Acceptance criteria

- [ ] Hook calls `ask.Collect(root)` alongside existing format/docs/lint/security checks
- [ ] If questions exist, appends a `== q&a` section to the hook output
- [ ] Q&A section text uses clear instructions: "Do NOT guess — stop and ask the user"
- [ ] Q&A section lists each question with source and ID
- [ ] Q&A section includes instruction: "Answer via: \`procoder ask --file .procoder/ask/answers.md\`"
- [ ] When no TTY, Q&A section mentions `.procoder/ask/QA.md` and instructs to answer there
- [ ] When no questions, no Q&A section is added (keeps output clean)
- [ ] Q&A section is under the maxInlineBytes threshold (48KB)
- [ ] Hook still handles errors gracefully (failed Q&A collection does not break the hook)

## Evidence
