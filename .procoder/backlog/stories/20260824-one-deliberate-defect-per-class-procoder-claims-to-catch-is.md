# The broken pass — one planted defect per class, each must be caught

Status: done 2026-08-24
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 015-one-planted-defect-per-class-and-every-one-of-them-caught

## Description

This is the story the epic exists for. A gate that finds nothing on
healthy code and also nothing on a planted defect is not clean, it is
silent, and silence is exactly the shape of every bug found in the
session that prompted this work.

The fixture is seeded with one deliberate defect per class procoder
claims to catch: unformatted source in each language, a lint finding, a
hardcoded secret, a SAST finding, a manifest pinning a known-vulnerable
version, a conflict marker, an oversized file, AI attribution in a
commit message, a debt marker with no revisit trigger, agent rules
drifted from the principles, and a doc reference pointing at a file that
does not exist.

Each must be caught, and caught by the command that owns it, and named
in the output specifically enough that somebody could fix it without
already knowing what was planted. A defect caught by the wrong command,
or reported so vaguely the reader cannot locate it, is recorded as a
finding too.

## Acceptance criteria

- [x] One deliberate defect per class procoder claims to catch is
      planted, and each is caught and named by the command that owns it;
      any that is not caught is reported with the command that missed it.
- [x] The output for each caught defect names the file, and where the
      class has a location, the line — enough to act on without knowing
      what was planted.
- [x] The unformatted-source defect is planted in every one of the
      twelve languages separately, since a formatter table has twelve
      independent rows.

## Evidence

- `scripts/e2e-broken-pass.sh`: **21 caught, 0 missed, 1 NOT RUN.** Each
  defect is planted alone in a freshly built fixture and removed before
  the next, because two at once cannot tell you which was found.
- The one NOT RUN is C# formatting: csharpier needs a dotnet SDK this
  machine does not have, so procoder reports UNCHECKED and the pass counts
  it with neither the catches nor the misses.
- Twelve languages planted separately, one unformatted file each, twelve
  caught. The thirteenth row (prettier's web extensions) is covered by
  `web/sloppy.js`.
- Every catch is matched against the owning command's verdict text, not
  the planted file's name — `unformatted  <file>`, `merge conflict marker
left in the file`, `over the 5 MB limit`, `[no-trigger]`, `broken
reference: "docs/nowhere.md"`, `SC2034`, `subprocess-shell-true`,
  `AWS Access Key ID Value detected`, `golang.org/x/text`.
- **One real miss, now fixed:** `procoder security` reported 0 findings on
  a hardcoded AWS key that the commit gate blocks. The gate runs the
  secret scanner AND the SAST leg over changed files; the command ran only
  the first, and gitleaks does not fire on a bare `const K = "AKIA…"` in
  Go where semgrep does. Held by `TestSecurityAsksWhatTheGateAsks`.
- **Two findings in the pass itself.** The catch test first matched the
  planted file's name, so `UNCHECKED cs/Sloppy.cs — csharpier is not
installed` counted as a catch. The correction over-reached and called
  Dart a NOT RUN, because procoder separately reports "NOT linted — Dart:
  procoder has no linter for it yet" about a file whose formatter had
  caught the defect. Both were found by replaying the classifier over logs
  already on disk rather than trusting the next version.
- The secret plant originally used AWS's documented example access key (the `AKIA…EXAMPLE` one), AWS's own
  documented example key, which every scanner allowlists deliberately — it
  tested the allowlist, not the scanner. It also carried a `// nolint`
  line, which measurement showed made no difference. The replacement is
  derived at run time from a fixed string.
