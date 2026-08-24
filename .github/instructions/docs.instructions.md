---
applyTo: "*.md,docs/**/*.md,.procoder/specs/**/*.md"
---

# Prose in procoder

<!-- Scoped to the prose a person writes: the root documents, the docs
     site, and the specs. Deliberately NOT every markdown file — most of
     this repository's .md files are generated agent rule files or
     .procoder/ backlog state, and changelog guidance fired at a backlog
     story is noise that teaches a reader to ignore the file. -->

Documentation is checked, not merely written. `procoder docs` blocks on
broken relative references and non-compiling Mermaid; `procoder docs
--external` adds link checking and Pages health in CI.

## What the docs must keep in step with

- Every command in the dispatch appears in `procoder help` AND in
  `docs/commands.md`. Three shipped absent from both — `config`, `review`
  and `analyze` — because the test that existed pinned the usage text
  against a canonical list, and a command absent from both agrees with
  itself. `TestEveryShippedCommandIsDiscoverable` reads the dispatch now.
- Every flag the binary implements is documented, and every documented
  flag is accepted. `docs --ack` — the command the gate's own blocking
  message tells an agent to run — was documented nowhere.
- A command's entry describes what its flags do, not just that they
  exist, and says what it exits when that is surprising.

## The changelog

`CHANGELOG.md` carries its rules at the top and the suite enforces them on
the newest entry, because the release job publishes that entry verbatim.
In short: an italic one-sentence summary; every headline opens with Added,
Fixed, Changed, Removed or Security; every entry links the PR that shipped
it; every outside contributor is named AND linked, including somebody who
only filed the issue. Verify a handle with `gh issue view` before writing
it — a misattributed credit is worse than none, and `procoder release`
checks each one against GitHub.

## Specs

Every bullet under `## In scope` carries an `[S-n]` id, and every
acceptance criterion cites the id it covers. `spec check` refuses
otherwise, and `backlog seed` refuses an incomplete spec. This exists
because a spec once closed at fourteen of fourteen with two features never
built.

A criterion written on a false premise is **wrong**, not unmet. Rewrite it
in the open, with the measurement that disproved the premise, rather than
quietly dropping it.

## Tone

Write for a reader who is tired. Say what happened and what it cost, not
how hard it was to fix. State what a check does NOT cover beside what it
does — a shrinking finding count must not read as shrinking risk.
