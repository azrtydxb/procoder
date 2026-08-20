# the gate blocks documentation that teaches merge conflicts

Status: closed 2026-08-20
Created: 2026-08-20

## Description

`procoder check` reports a blocking finding for every merge-conflict
marker in a changed file, with no way to exempt one. That is right for
source, and wrong for a document whose subject IS conflict markers: the
tutorial written for this site could not show a learner what a conflict
looks like without failing the repository's own gate.

Found while writing `docs/getting-started.md`. The lesson was reshaped
around a staged junk file instead, which works but teaches the less
recognisable example. Any repository documenting merge resolution, git
tooling, or CI output hits the same wall, and the available workarounds
are all bad: mangle the example so a reader who copies it gets broken
text, drop the topic, or turn the check off entirely.

Whatever the mechanism, it must be explicit and require a reason —
silently skipping fenced code blocks is not acceptable, because a real
conflict landing inside a fence in a Markdown file would then go
unreported, and a check that quietly misses things is the failure this
whole product argues against.

Done means a documented, greppable way to exempt a specific marker with
a stated reason, and no reduction in what the check catches everywhere
else.

## Acceptance criteria

- [x] A file containing conflict markers with the documented exemption
      and a reason produces no blocking finding.
- [x] The same file WITHOUT the exemption still produces one blocking
      finding per marker.
- [x] An exemption with no reason given does not suppress the finding.
- [x] Markers inside a fenced code block with no exemption still block —
      proving the fix did not widen into a silent skip.
- [x] Each behaviour is covered by a test whose mutation was proved.
- [x] `docs/configuration.md` documents the exemption alongside the
      security domain's `gitleaks:allow`.

## Evidence

- Exemption with a reason: `TestAnExplicitAllowWithAReasonExemptsTheFile`
  — RED before the change, showing the two markers reported from a file
  carrying the allow line; green after.
- Without the exemption: `TestMarkersInAFenceStillBlockWithoutAnAllow`
  keeps both findings for markers inside a fenced block, which pins that
  the fix did not widen into a silent skip of fences.
- No reason, no exemption: `TestAnAllowWithoutAReasonDoesNotExempt`
  covers `<!-- token -->`, `# token`, and `<!-- token    -->`.
- Reason as free prose: `TestAReasonMayStartWithAnyCharacter` pins that a
  reason beginning `-`, `->` or `(` still counts — added after the first
  implementation sniffed the first character, which would have rejected
  them.
- Mutations proved, all killed: the exemption ignored entirely; the
  comment terminator left unstripped so a bare token exempts; the reason
  requirement dropped; the captured reason never read.
- Documented in `docs/configuration.md` under "In-file exemptions",
  beside `gitleaks:allow`, including that the exemption is file-scoped
  and that fences alone are not exempt.
- End to end on this repository: `docs/getting-started.md` now shows a
  real merge conflict and `procoder check` reports
  `16 clean, 0 unformatted, 0 unchecked, 0 blocking`.
