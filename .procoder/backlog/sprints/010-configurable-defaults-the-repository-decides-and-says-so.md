# Configurable defaults: the repository decides, and says so

Status: closed 2026-08-21
Created: 2026-08-21

## Goal

A team can drive Procoder with the tools and thresholds they already
chose, instead of running a second set beside them or abandoning the
domain. Which tool answers for a language, what severity blocks, what a
template says, what a baseline checks — all of it moves from being
Procoder's decision to being the repository's, through the mechanisms
`.procoder/` already has rather than a new one.

Two things hold it honest. A repository can only select tools Procoder
knows how to invoke, so the print-don't-write contract stays a guarantee
rather than a hope. And a setting that weakens a default prints a line
naming what was relaxed, because a green gate must never be able to mean
"the config was loosened" without saying so.

The sprint is finished when a repository that changes nothing behaves
exactly as it did, and a repository that changes everything can still be
read by a stranger — `procoder config` names every effective setting and
where it came from.

## Stories

<!-- pulled below -->

## Retro

**What slowed us down.** An acceptance criterion in our own spec was
impossible, and we found out by building it. D-1 said a repository may
select any tool procoder ships a definition for and used `php = "pint"`
as the example; pint writes in place, so offering it would have broken
the contract D-1 exists to protect. The criterion had been reviewed,
merged and seeded into a story before anybody ran the tool.

That change mid-sprint then produced spec drift — the epic carried the
fingerprint of a spec that no longer existed — which is the same flag
that opened this whole line of work. The controller refused to re-seed
over the epic, correctly, and made it a manual decision.

**What we change.** A criterion that names a TOOL is not written until
the tool has been run and seen to do the thing. The evidence for "can it
print" took one command per candidate and would have moved the example
before it reached a spec, a review, a merge and a story. And when a
criterion changes mid-sprint, the epic's fingerprint and the superseded
story are updated in the same commit as the spec, rather than discovered
by the board two hours later.

**Worth keeping.** Closing the superseded story instead of deleting it. A
criterion proven impossible is a result: somebody will ask why pint is
not on the menu, and the answer — with the commands that established it —
is in a file with their question as its title. Deleting it would have
left the spec's D-6 asserting a conclusion whose working was gone.

## Result

committed: 12
done: 12 (20260821-a-checks-list-in-a-domain-s-rules-md-replaces-that-domain-s, 20260821-a-config-toml-that-cannot-be-parsed-reports-the-failure, 20260821-a-repository-lowering-any-default-gets-a-line-in-the-gate, 20260821-a-repository-setting-tools-php-pint-is-formatted-by-pint, 20260821-a-repository-upgrading-with-no-config-changes-produces-byte, 20260821-a-tools-entry-naming-a-tool-procoder-does-not-ship-is, 20260821-an-empty-template-file-blocks-rather-than-falling-back-to, 20260821-procoder-config-prints-every-effective-setting-with-its, 20260821-procoder-templates-story-md-replaces-the-embedded-story, 20260821-raising-a-default-prints-no-such-line-strengthening-is-not, 20260821-security-sast-blocks-at-warning-makes-a-warning-finding, 20260821-setting-a-severity-procoder-does-not-recognise-names-it-and)
carried: 0
