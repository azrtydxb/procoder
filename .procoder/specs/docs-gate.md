# docs-gate

Status: complete

## Problem

Documentation rot is caught late or never. The one completeness check
procoder ships refuses to run outside its own source tree — it is gated
on `go.mod` declaring `module procoder` and then greps for the literal
string naming each command — so it documents exactly one repository and
verifies only that a name appears somewhere. The universal check that
does exist, doc drift, is advisory: it fires on every write and is
trivially ignored, which is exactly what happened while building the
last four releases — four command families shipped with their docs
updated in a single page and every other page left stale, and the gate
stayed green. Nothing ever forces the question "does this change
invalidate a document?" at the moment it can still be answered.

## Users

- Any repository using procoder: doc rot is universal, and the check
  must not be about procoder's own command names.
- The agent finishing a change: it needs to be asked, at the boundary,
  rather than trusted to remember.
- The next reader — human or agent — who acts on a stale document with
  full confidence.

## In scope

- [S-1] A change-driven documentation obligation, computed from the diff and
  the index, universal to any repository:
  - the changed files include a public-surface change (an exported
    symbol added, removed, or renamed per the index; a CLI subcommand
    or flag string added or removed; a configuration key added or
    removed), OR a documentation file names one of the changed files;
  - AND no documentation file changed in the same diff;
  - THEN the gate raises a documentation obligation naming the trigger.
- [S-2] `[docs] policy = "block" | "report"` in config.toml (D-OVERRIDE,
  default report — procoder never blocks a repository by surprise);
  this repository opts into block.
- [S-3] The obligation clears two ways, both explicit: change a documentation
  file, or record the decision — a `docs: none — <reason>` line in the
  commit message, or `procoder docs --ack "<reason>"` which prints that
  line for the agent to use. Silence never clears it.
- [S-4] The command-coverage check is replaced by universal public-surface
  coverage: the repository's own exported surface (from the index's
  entrypoints and exported symbols, capped and ranked) that no
  documentation file mentions at all, reported not blocking, with the
  identity gate and the hardcoded command list deleted.
- [S-5] The documentation corpus grows to include `AGENTS.md` and root-level
  Markdown files — what every non-Claude host reads and the current
  corpus ignores.
- [S-6] The docs backfill this rot created: AGENTS.md (19 commands absent),
  docs/configuration.md (four config keys absent), docs/domains.md (no
  testing domain; bench and deps unplaced), docs/workflow.md (no
  backlog, sprint, test, or release), docs/index.md, README.md, and the
  mkdocs navigation.

## Out of scope

- Judging whether prose is correct — no model call, no semantic check.
  The gate asks the question; the agent answers it.
- Requiring documentation for every change: an internal refactor with
  no public-surface change and no doc naming its files raises nothing.
- Generating documentation, or editing it.
- Enforcing a documentation structure, a style guide, or coverage
  percentages.
- Cross-repository or published-site verification beyond the existing
  external link and Pages checks.

## Constraints

- Pure Go stdlib; the work lives in internal/docs, consuming the index
  the same way `maintain` already does.
- P-CONTROL: the gate reports and refuses; nothing is written.
- Universal by construction: no string may be procoder-specific, and no
  check may be gated on the repository's identity.
- The honesty rule: a repository with no index cannot have its public
  surface computed — say that in the finding rather than passing
  silently, and fall back to the file-mention trigger, which needs no
  index.
- Default report keeps every existing adopter's gate verdict unchanged
  on upgrade; only this repository's config changes behaviour.
- Acknowledgment lines are read from the commit message being prepared
  or the most recent commit, never from a state file — the record lives
  in history where a reviewer sees it.

## Interfaces

- `procoder check` gains the documentation obligation among its
  findings, blocking only under the block policy.
- `procoder docs` reports the same obligation for the current change,
  plus the universal public-surface coverage section.
- `procoder docs --ack "<reason>"` prints the acknowledgment line to
  place in the commit message; exit 0.
- config: `[docs] policy = "block" | "report"` (default report).
- The docs skill and the pr and merge skills state the obligation and
  how it clears.

## Data

- No new state. The trigger is computed from the diff, the index, and
  the commit message.

## Edge cases

- A documentation-only change → no obligation (documentation changed).
- A change that renames an exported symbol AND updates a doc → cleared,
  even if the doc updated is unrelated; the gate asks the question, the
  agent owns the answer.
- A repository with no documentation at all → the file-mention trigger
  cannot fire and the public-surface trigger reports that nothing
  documents the surface; under report this is information, not a wall.
- No index built → public-surface detection says so and only the
  file-mention trigger applies.
- A generated documentation directory (site output) → excluded the same
  way the existing markdown survey excludes it, so regenerating a site
  never clears an obligation on its own.
- Vendored or dependency directories → excluded from both triggers.
- An acknowledgment line with an empty reason → does not clear; the
  reason is the point.
- A merge commit touching many files → the obligation is computed from
  the changed files exactly as the gate already scopes itself.

## Failure modes

- Index unreadable or stale → public-surface detection reports NOT
  computed with the reason; the file-mention trigger still runs.
- git unavailable → the changed-file set cannot be computed, which the
  gate already treats as a failure to judge; the obligation reports NOT
  computed rather than clean.
- A documentation file that cannot be read → named as unreadable, and
  it does not count as a documentation change (unknown is never done).
- Commit message unavailable at check time → the acknowledgment path is
  reported as unavailable, and the obligation stands until a doc
  changes or the check runs where the message exists.

## Acceptance criteria

- [x] [S-1] In a fixture repository with no procoder identity, renaming an
      exported symbol with no documentation change raises the
      obligation naming the symbol; the same change with any doc edited
      raises nothing.
- [x] [S-1] A change to a file that a documentation page names, with no doc
      edited, raises the obligation naming the page.
- [x] [S-1] An internal change touching neither public surface nor
      doc-mentioned files raises nothing.
- [x] [S-2] The block policy makes the obligation block the gate; the default
      report leaves the gate's verdict unchanged — both verified by
      test.
- [x] [S-3] A `docs: none — internal refactor` line in the commit message
      clears the obligation; an empty reason does not.
- [x] [S-4] The public-surface coverage check runs in a fixture repository
      whose go.mod names another module, and reports exported surface
      that no document mentions.
- [x] [S-5] AGENTS.md and root-level Markdown are part of the documentation
      corpus — verified by a test where the only mention of a symbol
      lives in AGENTS.md.
- [x] [S-6] Every command shipped in 0.29.0 appears in AGENTS.md,
      docs/configuration.md carries all config keys the binary reads,
      and docs/domains.md, workflow.md, index.md, README.md, and the
      mkdocs navigation describe the backlog, test, adr, deps, bench,
      and release capabilities.
- [x] [S-1] [S-4] A repository with no index gets the file-mention trigger and an
      explicit NOT-computed line for public surface.

## Open questions

<!-- none — decisions recorded above -->
