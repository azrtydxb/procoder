---
description: "The over-engineering review: five tags (delete, stdlib, native, yagni, shrink), a mandatory replacement per finding, and a real null result when there is nothing to cut."
---

The user invoked /procoder:simplify with arguments:

The command below is the `procoder` binary on PATH.

This review hunts one thing only: code that should not exist. Correctness
bugs, security, performance — out of scope here; those belong to
`procoder check`, `security`, and /procoder:debug.

Scope: with no arguments, review the working-tree diff (`git diff` plus
staged). With `repo` (or a path) in the arguments, sweep that whole scope
— use `procoder index unused` and `procoder maintain` as starting
evidence, then read.

The two scopes are different jobs on different clocks. The DIFF pass runs
before every pull request, inside /procoder:pr's pre-PR review, where it
answers about the code you just wrote. The REPO sweep runs before a tag,
inside /procoder:release: it reports on code the current change never
touched, so attaching it to a pull request buries that diff's own
findings under a backlog of pre-existing ones — and a report nobody can
act on today is a report nobody reads tomorrow.

Every finding is one line, with a mandatory replacement — a finding
without a replacement is hedging, not reviewing:

    <file>:L<line>: <tag> <what>. <replacement>.

The five tags:

- `delete:` dead code, unused flexibility, speculative feature.
  Replacement: nothing.
- `stdlib:` hand-rolled code the standard library ships. Name the
  function.
- `native:` a dependency or code doing what the platform already does
  (CSS over JS, a DB constraint over app code). Name the feature.
- `yagni:` an abstraction with one implementation, config nobody sets, a
  layer with one caller.
- `shrink:` same logic, fewer lines. Show the shorter form.

End with the score line: `net: -<N> lines possible.` — and when there is
genuinely nothing to cut, say exactly `Lean already. Ship.` and stop;
never invent findings to look thorough.

Rules:

- Never flag a lone smoke test or assert-based self-check as bloat — the
  minimum check is part of the code, not decoration (see /procoder:tdd).
- A deliberate simplification marked with the debt marker (see
  `procoder debt`) is a recorded decision, not a finding — unless its
  revisit trigger has arrived.
- List the cuts; do not apply them. The user (or the task at hand)
  decides — P-CONTROL applies to reviews too.
