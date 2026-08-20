# Write QA.md and answers.md file formats

Status: open 2026-08-20
Created: 2026-08-20
Epic: interactive-qa
Sprint: -

## Description

Define and implement the `.procoder/ask/QA.md` and `.procoder/ask/answers.md` file formats. These files persist between procoder runs so the AI coder can forward questions to the human and the human can submit answers.

## Acceptance criteria

- [ ] `.procoder/ask/QA.md` format: numbered questions with Source, Text, and HTML comment for Answer
- [ ] `.procoder/ask/answers.md` format: numbered sections with Question text and Answer text
- [ ] `parseQA(root string) map[string]string` reads answers.md, returns ID -> answer mapping
- [ ] `writeQA(qs Questions, root string) error` creates directory and writes QA.md
- [ ] `writeAnswers(answers map[string]string, root string) error` creates directory and writes answers.md
- [ ] File formats are human-readable and parseable by AI coders
- [ ] `parseQA` handles missing file (returns empty map, not error)
- [ ] `parseQA` handles malformed file (returns partial map for valid sections, logs error for bad ones)
- [ ] All write/read operations create `.procoder/ask/` directory if missing

## Evidence
