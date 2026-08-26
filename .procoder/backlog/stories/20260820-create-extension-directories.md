# Create client-specific extension directories with hooks

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

Create the formal extension directory structure under reverse-domain namespaces. Each AI coding tool gets its own hook configuration with proper lifecycle events (SessionStart, PreToolUse, PostToolUse, PreCompact, Stop). This moves hooks from ad-hoc locations into the spec-compliant structure.

## Acceptance criteria

- [ ] `com.anthropic.claude-code/hooks/claude-hooks.json` created with lifecycle hooks
- [ ] Claude hooks include: SessionStart, PreToolUse (Bash gate), PostToolUse (Write/Edit format), PreCompact, Stop
- [ ] `com.kiro/hooks/kiro-hooks.json` created with Kiro-specific lifecycle events
- [ ] Kiro hooks include: PreToolUse (gate before edits), PostToolUse (format check)
- [ ] All hook files define proper matcher regex, timeouts, and statusMessages
- [ ] Commands use `${CLAUDE_PLUGIN_ROOT}` or `${PLUGIN_ROOT}` variables
- [ ] All JSON files parse without errors

## Evidence
