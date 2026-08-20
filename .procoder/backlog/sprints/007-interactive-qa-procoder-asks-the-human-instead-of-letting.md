# interactive-qa: procoder asks the human instead of letting the coder guess

Status: active
Created: 2026-08-20

## Goal

Every domain here already knows what it cannot decide — a spec's open
question, a documentation gap nobody cleared, a flagged secret, a lint
finding that needs judgement. Today those reach the AI coder as prose, the
coder answers them itself, and the answer reads like a resolution rather
than the guess it is. The human never sees the question.

By the end of this sprint `procoder ask` collects those questions, puts
them to a person — on a terminal, or through a file when there is no
terminal to ask — and records the answers where both the coder and the
gate can read them. An answer outlives the session, a changed question is
asked again, and a flagged secret's value never appears in any of it.

Done means a question reaches a person instead of being invented, and the
decision survives the session that made it.
