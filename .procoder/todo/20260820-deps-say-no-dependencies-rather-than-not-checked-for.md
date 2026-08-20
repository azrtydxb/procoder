# deps: say 'no dependencies' rather than 'NOT checked' for licenses when an ecosystem has none

Status: closed 2026-08-20
Created: 2026-08-20

## Description

`procoder deps` reports `licenses (go): NOT checked — go-licenses is not
installed` even in a repository whose `go.mod` has no `require` block at
all. The honesty rule is right in general — an unchecked file must never
read as clean — but it applies to work that exists and was not done. With
zero dependencies there is no license surface, so "NOT checked" points the
reader at a gap that is not there, and a reader who learns to skim that
line will skim it in a repo where it does matter.

Done means: an ecosystem with no third-party dependencies reports that
fact plainly, and an ecosystem that HAS dependencies but no license tool
still reports "NOT checked" exactly as it does today. The distinction is
between nothing to check and something unchecked; the report must not
blur them.

## Acceptance criteria

- [x] In a fixture repo with a `go.mod` carrying no `require` directives,
      `procoder deps` prints a "no dependencies" line for go licenses and
      does not print "NOT checked" for them.
- [x] In a fixture repo with at least one third-party require and no
      `go-licenses` on PATH, `procoder deps` still prints
      `licenses (go): NOT checked` with the install hint.
- [x] The same distinction holds for js: an empty or absent
      `dependencies`/`devDependencies` reports "no dependencies"; a
      populated one keeps the honest NOT-checked line.
- [x] Each behaviour is covered by a test whose mutation was proved:
      inverting the has-dependencies predicate makes it fail.

## Evidence

- Dependency-free go.mod: `go test ./internal/deps/ -run
TestGoWithNoRequiresHasNoLicenseSurface` — RED before the change
  (output showed `licenses (go): NOT checked — go-licenses is not
installed`), green after; the fixture is `module x` with no requires.
- Requires present, no tool: `TestGoWithRequiresAndNoToolStaysUnchecked`
  passes; mutating `hasGoDeps` to answer `(false, true)` fails it.
- js both ways: `TestJSDevDependenciesAreALicenseSurface` (dev-only deps
  still report NOT checked) and `TestJSUnreadableManifestStaysUnchecked`
  (a malformed manifest is no evidence of anything). Python gained three
  tests covering the pyproject array, metadata-only, and the styles
  procoder declines to read.
- Mutations proved, all killed: hasGoDeps final return inverted; the
  single-line `require` branch disabled; the `dependencies` array key
  ignored; the non-empty-array test disabled; the other-manifest guard
  in hasPythonDeps weakened to `(false, true)`; `[project]` metadata
  counted as dependencies; the outdated-rows override dropped. Each one
  fails the package suite; two earlier mutations that survived exposed
  an uncovered branch, which is what the three Python tests close.
- Whole suite green (`go test ./... -count=1`), deps coverage 79.9%,
  gate `14 clean, 0 unformatted, 0 unchecked, 0 blocking`.
- On this repository the report now reads `licenses (go): no
dependencies to report` / `licenses (js): no dependencies to report`.
