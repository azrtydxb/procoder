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
- [x] Setup command examples preserve arguments in both initial and repeat invocations across host twins.
- [x] New and modified setup tests carry observed mutation-failure evidence.

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

Copilot follow-up: both setup commands now forward $ARGUMENTS in initial and
repeat invocations, including OpenCode and Kilo twins. The new
TestSetupCommandExamplesForwardArguments failed for all six command files
before the fix and passes afterward; command parity tests also pass.

Mutation experiments were applied to production code and run with targeted
`go test` selections and `-count=1`, then restored using explicit patches:

- Setup returning all without validation failed TestSetupSelection and
  TestSetupCLIAPIAndCallerIsolation on accepted invalid/unknown requests.
- Removing PROCODER_HOST from envKeys failed
  TestSetupContextTravelsInProcessEnvironment on lost caller context.
- IgnoreHosts returning success without work failed all five gitignore tests
  and TestInitUsesHostSelectionWithoutWritingRules: missing entries and accepted
  nonregular/symlink inputs were observed, not inferred.
- Selecting all copies failed the print-only selection test, declared/existing
  copy test, Check single-host test, and both modified drift tests.
- Returning no declaration without reading failed the missing-master and
  dangling-declaration tests with empty findings.
- Assigning outer in shell.env failed both Node adapter-context subtests.

The earlier two precondition failures are recorded above. Every new setup test
and the modified single-host tests now name its observed mutation in a
`proved by:` comment. `git diff --exit-code` over the mutated production files
proved restoration. The complete focused packages passed uncached afterward;
`procoder test` again passed 57 Go packages and `procoder check` reported 14
clean files with zero blocking findings. Adversarial and edge-case review
checked argument retention, twin parity and mutation restoration.
