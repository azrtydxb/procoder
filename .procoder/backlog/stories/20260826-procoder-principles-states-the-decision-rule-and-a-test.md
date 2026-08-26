# `procoder principles` states the decision rule, and a test pins that it does — the rule is the deliverable, so its absence must fail rather than be noticed later.

Status: done
Created: 2026-08-26
Epic: decisions-reach-the-user
Sprint: 022-a-decision-the-agent-cannot-make-reaches-the-user

## Description

The rule is the deliverable. The queue makes a decision durable and
visible, but nothing in it makes an agent ask — only the principle does
that, and a principle that quietly disappears fails silently.

This exists because of a concrete failure on 2026-08-26: two decisions
were put to the user as prose at the end of a long status report while
the work continued underneath them. The user said they had not been asked
properly. They were right, and nothing in the principles covered the
case — only questions arising from findings.

Done when the rule ships and its absence fails a test.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder principles` states the decision rule, and a test pins that it does — the rule is the deliverable, so its absence must fail rather than be noticed later.

## Evidence

`internal/principles/principles.go` gains four bullets: a decision is not
the agent's to make; STOP means asking before continuing rather than
mentioning at the end; use the host's structured question tool where one
exists; do not batch a decision with a status report.

`TestThePrinciplesCarryTheDecisionRule` pins three phrases rather than
the paragraph — pinning prose would make every wording change a failure
and the rule would get weakened to keep the suite green. It reads
`Default`, not `Effective`, so a repository override cannot mask the
shipped text having lost the rule. Killed by editing the bullet out.
