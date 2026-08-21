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

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->

## Result

committed: 11
done: 11 (20260821-a-c-file-in-a-repository-with-no-clang-format-is-formatted, 20260821-a-c-file-reaches-clang-tidy-and-a-real-finding-is-reported, 20260821-a-file-type-procoder-does-not-claim-is-still-out-of-scope, 20260821-a-language-procoder-formats-but-cannot-lint-reports-not, 20260821-a-php-file-with-no-prettier-plugin-is-unchecked-with-the, 20260821-a-repository-carrying-its-own-clang-format-is-formatted-by, 20260821-a-ts-file-in-a-repository-with-no-eslint-config-is-linted, 20260821-mts-and-cts-files-reach-the-same-linter-as-ts-and-pyi, 20260821-no-domain-anywhere-in-the-tree-reports-a-check-that-did-not, 20260821-procoder-doctor-lists-every-new-default-tool-with-its, 20260821-with-no-linter-installed-procoder-check-over-a-file-of-that)
carried: 0

## Retro

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->
