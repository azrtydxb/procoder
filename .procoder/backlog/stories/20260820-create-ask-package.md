# Create internal/ask/ask.go — Question struct and collect framework

Status: open 2026-08-20
Created: 2026-08-20
Epic: interactive-qa
Sprint: -

## Description

Create the `internal/ask/ask` package that defines the Question struct and the Collect framework. This is the foundation that all other ask features build on.

## Acceptance criteria

- [ ] `internal/ask/ask.go` created with Question struct: Source, ID, SpecName, Text, Default, Answered
- [ ] Questions type defined as []Question
- [ ] Collect(root string) Questions function calls each domain's collector
- [ ] TTY() bool function checks stdin using syscall.IsTerminal (or existing copilot.Prompt pattern)
- [ ] WriteFile(qs Questions, root string) error writes questions to `.procoder/ask/QA.md`
- [ ] Question.Source distinguishes: spec, docs, security, lint
- [ ] Question.ID is unique (e.g., "spec:my-spec:1")
- [ ] When a domain has no questions, its collector returns zero Questions (no panics)
- [ ] Package compiles and all tests pass

## Evidence
