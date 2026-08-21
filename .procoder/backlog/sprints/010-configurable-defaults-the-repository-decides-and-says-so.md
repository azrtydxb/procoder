# Configurable defaults: the repository decides, and says so

Status: active
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

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->
