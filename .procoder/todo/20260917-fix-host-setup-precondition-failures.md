# Fix host setup precondition failures

Status: closed 2026-09-17
Created: 2026-09-17

## Description

Complete the PR #289 defect pass. Missing or unreadable AGENTS.md must stop
setup before printing generated content or changing ignores. Existing host
copies without a declaration must not silently pass when the master is removed.
Preserve the unanswered release-date decision unchanged.

## Acceptance criteria

- [x] Missing and unreadable masters stop CLI/API init before writes or generated output.
- [x] Existing integrations without a declaration block if their master is missing.
- [x] Focused tests, full tests, review lenses and the gate pass over the final feature content.

## Evidence

Both new regressions failed before the implementation fix: init printed a
declaration and wrote .gitignore after its missing-master error, and Check
returned no findings for an existing Kilo copy without its master.

`go test ./cmd/procoder ./internal/host ./internal/initcmd ./internal/portability`
passed after the fix, including TestSetupRequiresReadableMasterBeforeOutputOrWrites
and TestExistingHostWithoutDeclarationStillRequiresMaster.

`procoder test` passed all 57 Go packages. Standalone JavaScript tests were
explicitly NOT run because package.json has no test script. `procoder check`
reported 6 clean files and zero blocking findings. `git diff --check` passed.
Adversarial and edge-case review covered selection precedence, additive
declarations, installed copies, missing masters and the init early return.
No new debt markers. The release-date decision remains unanswered and unchanged.
