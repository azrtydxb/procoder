# A repository carrying `.procoder/review/lenses/adversarial.md` gets that content in place of the shipped lens; without the file it gets the shipped one unchanged.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 012-review-with-judgment-not-just-tooling

## Description

D-OVERRIDE, applied to review like every other domain: a repository that
disagrees with a lens replaces it, without forking procoder or giving up
the other four.

Done means the override file's content appears in place of the shipped
lens, and a repository without the file gets the shipped one
unchanged.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A repository carrying `.procoder/review/lenses/adversarial.md` gets that content in place of the shipped lens; without the file it gets the shipped one unchanged.

## Evidence

- `go test ./internal/review/ -run TestAnOverrideReplacesTheShippedLens` — the override's content is what runs, the other four stay procoder's, and the printed lens names its source so a reader knows whose words they are reading. Verified end to end against a fixture carrying `.procoder/review/lenses/adversarial.md`. Mutation proven: Resolve ignoring the override directory returns procoder's shipped lens under the repository's name.
