# Batch close: verify once, not per story (0.32.0)

Status: closed 2026-08-19
Created: 2026-08-19

## Goal

<!-- What this sprint commits to deliver, in the reader's terms — the
     outcome the stories add up to, not a list of them. -->

## Result

committed: 1
done: 1 (20260819-backlog-close-story-takes-several-ids-and-verifies-once)
carried: 0

## Retro

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->

What slowed us: nothing — a single story with a measured problem, a
failing test written first, and a fix that is twenty lines of
memoisation. The sprint took minutes because the previous sprint's
retro had already done the thinking.

What we change: keep filing measured annoyances as stories instead of
working around them in the moment. This one was spotted mid-run,
written down with real acceptance criteria, and closed with a number
attached (729s → 27s on this repository).

Adaptation worth keeping: the fix went in as memoisation of the two
verdict closures rather than a new batch code path, so the single-id
form is provably unchanged — a test asserts identical output between
the two forms. Prefer a change that cannot alter the old behaviour over
one that merely intends not to.
