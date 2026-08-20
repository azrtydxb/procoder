# Update AGENTS.md and SKILL.md with Q&A workflow

Status: open 2026-08-20
Created: 2026-08-20
Epic: interactive-qa
Sprint: -

## Description

Update the AI coder-facing documentation (AGENTS.md and skills/procoder/SKILL.md) to document the Q&A workflow — when to stop and ask, how to submit answers, and where answers live.

## Acceptance criteria

- [ ] `AGENTS.md` gains a reference to the Q&A workflow (either new section or integration into existing sections)
- [ ] `skills/procoder/SKILL.md` gains documentation about `procoder ask` behavior
- [ ] Both files reference `.procoder/ask/` directory as the canonical Q&A location
- [ ] Both files instruct AI coders: "When you see a question from procoder, do NOT guess — stop and ask the user"
- [ ] Both files reference `procoder ask --file` as the submission mechanism
- [ ] AGENTS.md references the skill file as source of truth
- [ ] No duplication between AGENTS.md and SKILL.md (they should cross-reference)
- [ ] Changes are minimal — no rewrite of existing content

## Evidence
