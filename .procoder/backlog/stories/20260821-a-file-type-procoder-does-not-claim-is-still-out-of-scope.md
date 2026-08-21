# A file type procoder does not claim is still out of scope and still passes, asserted so the change cannot swallow every unknown file.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

The rule must not grow into noise. A text file, an image, a CSV is genuinely out of scope and must stay silent, or the gate gets switched off.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A file type procoder does not claim is still out of scope and still passes, asserted so the change cannot swallow every unknown file.

## Evidence

`go test ./internal/lint/ -run TestAFileTypeProcoderDoesNotClaimStaysSilent`: PASS for notes.txt, logo.png, data.csv. Proved by mutation: making lintUnlinted report on any unrecognised extension makes a .txt file block the gate, and the test names it. Run end to end — `procoder check notes.txt` exits 0 with the file counted under out of scope.
