# consolidate the duplicated text helpers

Status: closed 2026-08-19
Created: 2026-08-19

## Description

The simplify sweep found four text helpers copied across packages:
`slugify` (3 byte-identical copies), `section` and `stripComments` (5
each), and `firstLine` (12 copies in 7 variants). The slug cap for the
Windows path limit had to be hand-patched into all three copies last
night, which is how the fourth copy gets missed. Consolidate the
genuinely identical ones into one package; leave the variants that
differ on purpose alone rather than smuggling a behaviour change into
a cleanup. Also delete two exported docs wrappers with zero callers.

<!-- What this task is, why it exists, and what "done" looks like in the
     reader's terms. A title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] slugify, section and stripComments have one definition each, and
      the five domains that used them call it.
- [x] The seven firstLine copies with identical semantics call one
      helper; the five that differ deliberately keep their own.
- [x] docs.CollectOffline and docs.Run, which had zero callers
      anywhere, are gone.
- [x] The full suite passes with no behaviour change.

## Evidence

- `go test ./...` green throughout; no output-shaping code changed
  behaviour, because only byte-identical or semantically identical
  copies were merged.
- Measured: 371 lines deleted and 81 added across existing files (net
  -290); the new internal/textutil is 99 lines plus 53 lines of tests
  it did not have before. True net: -191 implementation lines.
- Duplicate counts after: slugify 0, section 0, stripComments 0,
  firstLine 5 (the deliberately distinct ones: a []byte/200-cap
  variant, one taking an error, a byte-scanner, and two no-trim forms).
- The docs obligation fired on the two deleted exported symbols and
  blocked the gate once [docs] policy = "block" was set — answered with
  a recorded `docs: none` line rather than silence.
- My own review overstated firstLine as 12 duplicates; only 7 share
  semantics. Corrected before acting on it.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the task open. -->
