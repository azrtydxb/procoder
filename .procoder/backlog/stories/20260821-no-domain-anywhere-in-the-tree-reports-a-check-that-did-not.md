# No domain anywhere in the tree reports a check that did not run as merely informational — asserted by an audit over the source, so a domain that does not exist yet is covered too.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

The question was whether this covered all the gates. It did not: it covered the two that had been noticed. Done means every domain, and an audit that covers domains not written yet.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] No domain anywhere in the tree reports a check that did not run as merely informational — asserted by an audit over the source, so a domain that does not exist yet is covered too.

## Evidence

`go test ./internal/audit/ -run TestNoDomainReportsAnUnrunCheckAsMerelyInformational`: PASS, 13 NOT-checked findings across the tree, 0 non-blocking. The sweep found three more — tflint missing left Terraform unlinted, helm lint failing without findings left a chart unchecked, and a missing gh left GitHub Pages unverified. Proved by mutation: un-blocking the tflint branch makes the audit name internal/infra/infra.go:258. It reads source rather than behaviour because the failure worth preventing is a domain nobody has written yet.
