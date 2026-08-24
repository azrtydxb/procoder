# `procoder review --lens edge-case` prints exactly that lens and exits 0; an unrecognised name reports the name, prints no lens at all, and exits 2 — a usage error, not a finding.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: -

## Description

A full review is the default, but a reader who already knows what they
are worried about should be able to ask one question rather than five.

Done means `--lens` selects, exits 0 when it resolved, and treats an
unrecognised name as a usage error — reported by name, nothing printed,
exit 2 — rather than silently running the other four and leaving the
reader believing they got the lens they asked for.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `procoder review --lens edge-case` prints exactly that lens and exits 0; an unrecognised name reports the name, prints no lens at all, and exits 2 — a usage error, not a finding.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
