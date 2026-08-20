# Implement question collectors for spec, docs, security, lint

Status: open 2026-08-20
Created: 2026-08-20
Epic: interactive-qa
Sprint: -

## Description

Implement the per-domain question collectors in `internal/ask/ask.go`. Each collector reads its domain's state and returns Questions that need human answers.

## Acceptance criteria

- [ ] Spec collector: reads `.procoder/specs/*.md`, parses for `OPEN:` lines, returns one Question per OPEN: marker
- [ ] Spec collector reuses the existing `openRe` regex from `internal/spec/spec.go`
- [ ] Docs obligation collector: calls `docs.Obligation()` with block=false, returns one Question per finding not cleared by edit
- [ ] Security collector: calls `security.SecretsChangedFiles()` with block=false, returns one Question per flagged secret (real or test?)
- [ ] Lint collector: calls `lint.Files()` with block=false, returns one Question per non-trivial blocker
- [ ] Each collector respects the "conservative" policy: only collect truly ambiguous questions
- [ ] Collectors handle empty input and missing files gracefully
- [ ] Collectors are in `internal/ask/ask.go` as separate functions called by Collect()

## Evidence
