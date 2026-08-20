# Interactive Q&A — procoder asks the human

Status: open 2026-08-20
Created: 2026-08-20

## Goal

When procoder finds something that needs a human decision (spec OPEN: questions, docs obligations, security flags, lint judgments), it actually asks the user instead of letting the AI coder guess. Questions are collected, presented interactively, written to `.procoder/ask/`, and re-injected as ground truth into the AI coder's context.
