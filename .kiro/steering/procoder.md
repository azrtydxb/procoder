---
inclusion: always
---

# procoder

You are working in a repository governed by procoder — a harness that
gives AI coders the tools and discipline of a senior developer. The
`procoder` binary computes; you act. It never modifies code behind your
back, and a file it could not check is never reported as clean.

## The contract

- Before calling any work finished, run `procoder check` — the commit
  gate. Blocking findings (unformatted files, conflict markers, junk,
  secrets, attribution lines) must be fixed, not argued with.
- `procoder format <file>` prints the formatted result; you review and
  write it. The binary never touches the file.
- Never add AI-attribution lines (Co-Authored-By, "generated with") to
  commits or PRs — `procoder scrub` verifies.
- Deliberate corner-cuts carry a `debt:` comment naming the ceiling and
  the revisit condition; `procoder debt` harvests the ledger.
- Specs live in `.procoder/specs/`, plans in `.procoder/plans/`, tasks
  in `.procoder/todo/` — each has a quality controller (`spec check`,
  `plan check`, `todo close`) that blocks until the work is actually
  complete. Do not game the checkboxes; the controllers ask for evidence.

## Build principles

Climb this ladder and stop at the first rung that holds: does it need to
exist at all → does this codebase already have it → stdlib → platform →
an installed dependency → one line → only then the minimum code that
works. The ladder runs AFTER you understand the problem — read every
file the change touches first. Bug fix = root cause: find every caller
before editing. Never simplify away input validation, error handling
that prevents data loss, security, or accessibility. Non-trivial logic
leaves one runnable check behind. A repo overrides these wholesale with
`.procoder/PRINCIPLES.md` (`procoder principles` prints the effective
text).

## The toolbox

- `procoder doctor` / `procoder init` — which tools this repo needs and
  how to install the gaps.
- `procoder index <sub>` — the code map: find, search, refs, outline,
  callers, impact, unused, entrypoints. Reach for it before grepping.
- `procoder lint` / `security [--deep]` / `ci` / `infra` / `docs` /
  `maintain` — the domain reports; blocking beats advisory, honesty
  beats convenience.
- `procoder audit` — the whole-tree onboarding sweep for a repo procoder
  has not governed before.
- `procoder git` and `procoder templates` — pre-finish status and the
  repo's template files under `.procoder/`.

Install: the binary ships per platform in `dist/` of the procoder repo
(github.com/azrtydxb/procoder); put the one for your platform on PATH,
or use the Claude Code plugin which wires everything automatically.
