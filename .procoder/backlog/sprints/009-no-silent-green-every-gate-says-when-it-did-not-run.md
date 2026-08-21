# No silent green: every gate says when it did not run

Status: closed 2026-08-21
Created: 2026-08-21

## Goal

A green gate means the code was checked. Today it can also mean the
machine was empty: a missing linter reports as info, a formatter without a
project config reports the file out of scope, and both let `procoder
check` exit 0 over code nothing read.

This sprint closes every route to that verdict — in every domain, not the
two that happened to be noticed — ships a working default wherever the
project brought none, and leaves an audit over the source so a domain
written next month cannot reopen it.

What it must not do is turn the gate into noise: a file type procoder does
not claim stays out of scope, stays silent, stays green.

## Stories

<!-- pulled below -->

## Retro

**What slowed us down.** Asked whether the rule covered all the gates,
the honest answer was that it covered the two that had been noticed. The
other three — tflint, helm, GitHub Pages — were found only by reading
every Finding literal in the tree. A policy is not a diff, and reviewing
the diff is how three domains kept their exemption through a change whose
entire purpose was to remove it.

Worse, two of the blocking refusals had remedies that did not work.
`brew install llvm` is keg-only, so init would install clang-tidy and the
resurvey would still report it missing; the PHP plugin install candidate
was global while resolution walks the project. Both would have printed a
command, run it successfully, and left the file blocked — a wall with no
door, which is worse than the silence this sprint replaced. Neither was
caught by writing them; both were caught by review.

And one new test could only ever skip: it looked for the real
typescript-eslint inside its own temp directory, where nothing had put
it. It would have sat green in CI forever looking like coverage.

**What we change.** A refusal is not finished until its remedy has been
run and the check re-verified afterwards — installing is a claim, the
tool answering is the fact, and that is the rule `init` already applies
to itself in its resurvey. And a test that skips is written with its skip
deliberately exercised: run it once with the dependency present, once
without, and know which of the two CI will get.

**Worth keeping.** The source-level audit. `TestNoDomainReports...` reads
every Finding literal in internal/ and fails naming file and line. No
behavioural test could have covered the domains that were missing,
because the failure was code nobody had thought to look at — and it is
the only form of test that will cover the domain somebody writes next
month. For a rule that is supposed to hold everywhere, assert it
everywhere.

## Result

committed: 11
done: 11 (20260821-a-c-file-in-a-repository-with-no-clang-format-is-formatted, 20260821-a-c-file-reaches-clang-tidy-and-a-real-finding-is-reported, 20260821-a-file-type-procoder-does-not-claim-is-still-out-of-scope, 20260821-a-language-procoder-formats-but-cannot-lint-reports-not, 20260821-a-php-file-with-no-prettier-plugin-is-unchecked-with-the, 20260821-a-repository-carrying-its-own-clang-format-is-formatted-by, 20260821-a-ts-file-in-a-repository-with-no-eslint-config-is-linted, 20260821-mts-and-cts-files-reach-the-same-linter-as-ts-and-pyi, 20260821-no-domain-anywhere-in-the-tree-reports-a-check-that-did-not, 20260821-procoder-doctor-lists-every-new-default-tool-with-its, 20260821-with-no-linter-installed-procoder-check-over-a-file-of-that)
carried: 0

