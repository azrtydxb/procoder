# Honest limits

Procoder's reports are worth what their claims are worth, so this page
states where the rigor stops paying and where a check is quieter than it
sounds. Everything here is a property of the current build, checked against
the code rather than remembered.

The rule the rest of this page follows: **a check that could not run is
never reported as one that passed.** That honesty has a price, and the
price is what this page is about.

## When the gate costs more than it returns

**A first `procoder audit` on a large unformatted repository.** `audit` runs
every domain over the whole tree rather than a commit's diff. On a codebase
that has never been formatted, the result is a finding count in the
thousands, and none of it is about the change you came to make. Onboard in
scope order — format one directory, commit, move on — rather than reading a
report you cannot act on in one sitting.

**A repository where the toolchain is not installed.** Domain checks resolve
a real binary: gitleaks for secrets, semgrep for SAST, osv-scanner for
dependency vulnerabilities. A missing tool produces `NOT checked`, and for
secrets that finding is **blocking**. A scan that did not run must not read
as a scan that found nothing — but the honest answer is a red gate on a
machine that is not set up, not a green one. `procoder doctor` names the
gaps and `procoder init` closes them.

**A language procoder has no formatter for.** The summary says
`N file(s) not formatting-checked` rather than counting them clean. That
number is not a defect count and not a pass; it is the size of what nobody
looked at.

**Very large commits.** The gate reads the diff, and the checks that narrow
to changed lines do that work per line. A thousand-file commit is slow
because it is a thousand files, not because the gate is inefficient — and
splitting it is better for the reviewer anyway.

## Where a check is narrower than its name

**`procoder check` answers about the change, not the tree.** Two tiers, on
purpose: the gate answers "is this commit clean", CI answers "is the
repository clean". A green gate is not a claim about code you did not
touch.

**Adoption scope changes what content checks see.** In universal scope — a
repository that has not adopted procoder — content checks narrow to the
lines the commit actually wrote, so procoder does not lecture a third party
about code it did not come to change. Checks about a file's _existence_ do
not narrow, because absence has no line number.

**`backlog scope` is silent when there is nothing to compare.** No plan, or
plans that declare no files, reports `scope NOT checked`. That is not a
finding of surgical work; it is the absence of a declaration to check
against.

**`bench` is Go-only in this version.** Other ecosystems get no benchmark
comparison at all, and the absence is not reported per-language.

**Coverage is reported, never enforced.** `procoder test --coverage` prints
a number. No policy consumes it.

## Where honesty is the friction

**Secrets block on a missing scanner.** Covered above, and it is the single
most common surprise: people expect a skip and get a block.

**The attribution check reads unpushed commits.** One old trailer in an
unpushed commit blocks every later commit, and the message reads as though
it concerned the change in hand. Correct, and confusing the first time.

**An out-of-date binary reports valid configuration as unrecognised.** A
key added in a later release is unknown to an older build. The finding is a
real block on a file that is not wrong, which is why it names the running
build and both routes — a typo is yours to fix, an old build is not.

## What has never been measured

No benchmark numbers appear in procoder's documentation because none have
been run. Whether the gate's overhead is repaid in defects caught is an
open empirical question, not a settled one, and
[#190](https://github.com/azrtydxb/procoder/issues/190) exists to make it
measurable rather than assumed. Any future number will carry its method
alongside it.

Claims about _behaviour_ — what each check does and does not look at — are
checkable today by reading the code, and this page is held to that standard.
Claims about _value_ are not yet, and this page does not make them.
