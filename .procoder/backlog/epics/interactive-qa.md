# Interactive Q&A

Status: open 2026-08-20
Created: 2026-08-20
Milestone: interactive-qa
Spec: interactive-qa

## Description

This epic introduces `procoder ask` — a structured Q&A flow where procoder collects questions from all question-generating domains (spec, docs, security, lint), presents them to the human user, and re-injects the answers as ground truth. It adds an `internal/ask/ask` package with question collection, interactive prompting, and file-based Q&A. It updates the PostToolUse hook to inject Q&A sections into the AI coder's context, updates the principles hook with Q&A behavior instructions, and updates AGENTS.md and skills to instruct AI coders to stop and ask rather than guess.
