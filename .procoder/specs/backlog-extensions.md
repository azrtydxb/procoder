# backlog-extensions

Status: complete

## Problem

Two daily practices have no home in the backlog. Production defects are
neither spec-born stories nor ad-hoc todos, so bugs land untyped, with
no severity and no forced regression test. And sprints close recording
only the what (committed/done/carried), never the why — the
retrospective, lean's actual learning loop, does not exist.

## Users

- Pascal and the agent triaging a defect: severity, reproduction, and a
  regression-test obligation, linked into the epic it broke.
- The sprint cadence: each close asks what to change, and the answer is
  the price of opening the next sprint.

## In scope

- [S-1] `procoder backlog bug <title> [--epic <id>] [--severity s1|s2|s3|s4]`
  — prints a story file (same directory, same lifecycle) with
  `Type: bug` and `Severity:` header lines, a Description section
  prompting for reproduction steps and observed-vs-expected, and the
  acceptance criteria pre-seeded with the non-negotiable first
  criterion: a regression test that fails before the fix and passes
  after. Severity defaults to s3; --epic is optional (a bug may predate
  any epic).
- [S-2] Story close on a bug additionally refuses while the Severity header
  is missing or not one of s1–s4.
- [S-3] Board and list mark bugs: list shows kind `bug`; the board line uses
  `[B]`-style marking plus the severity, so open s1/s2 bugs are visible
  at a glance. The board summary counts open bugs separately.
- [S-4] Sprint close writes a `## Retro` section (guidance comments: what
  slowed us, what we change, one adaptation) after the Result — content
  the agent/user fills by editing the file.
- [S-5] `procoder sprint open` refuses while the most recently closed sprint
  has an empty Retro section — the retro is the price of the next
  sprint. A repo can disable that with `[sprint] retro = "off"` in
  config.toml (D-OVERRIDE).

## Out of scope

- A separate bugs directory or lifecycle: a bug IS a story with a type.
- SLA clocks, aging alarms, or auto-escalation by severity.
- Retro templates beyond the three guidance questions; no retro
  ceremony tooling.
- Any change to milestone/epic semantics.

## Constraints

- Pure Go stdlib; extends internal/backlog only (plus config parsing).
- P-CONTROL: bug creation prints; only sprint close writes the Retro
  scaffold into the sprint file it already rewrites.
- Honesty: the retro gate names the sprint file it wants filled.
- Existing story files without a Type header keep working: absent Type
  means feature, absent Severity is only checked for bugs.

## Interfaces

- `procoder backlog bug <title> [--epic <id>] [--severity sN]`.
- Story header grows optional `Type:` and `Severity:` lines parsed into
  Item fields.
- `[sprint] retro = "off"` joins config.Load.
- Board/list rendering changes as scoped above; commands/backlog.md and
  commands/sprint.md skills, the docs site, and the changelog say so.

## Data

- Bug stories live in `.procoder/backlog/stories/` with `Type: bug` and
  `Severity: s1..s4` headers — no new directories.
- The sprint file gains a Retro section at close, between Result and
  end of file.

## Edge cases

- `bug` with an invalid severity flag → refused exit 2 naming the four
  valid values.
- A hand-edited story with `Type: bug` but no Severity → close refuses
  naming the missing header; board shows `s?`.
- Retro gate with zero closed sprints → open proceeds (nothing to
  retro).
- Retro gate when the last closed sprint predates this feature (no
  Retro section at all) → treated as empty, refusal explains and names
  the file — visible, and the repo can opt out via config.
- Retro containing only the guidance comments → still empty (comments
  are stripped before judging).

## Failure modes

- Unreadable last-closed sprint file at open time → refuse (an
  unreadable retro is unknown, and unknown is never done).
- Config unreadable → default behaviour (retro gate on), consistent
  with how other config errors degrade.

## Acceptance criteria

- [x] [S-1] [S-2] `procoder backlog bug "login 500s" --severity s1` prints a story
      file with Type, Severity, the repro-prompting description, and
      the pre-seeded regression-test criterion; writing and closing it
      without a Severity header is refused.
- [x] [S-3] The board marks an open bug with its severity and the summary
      counts open bugs; list shows kind bug.
- [x] [S-4] [S-5] Sprint close writes a Retro scaffold; `sprint open` then refuses
      until the Retro has real content, and proceeds after one
      sentence is added — verified by a test walking the sequence.
- [x] [S-5] `[sprint] retro = "off"` disables the retro gate, verified by
      test.
- [x] All existing backlog and sprint tests pass unmodified.

## Open questions

<!-- none — decisions recorded above -->
