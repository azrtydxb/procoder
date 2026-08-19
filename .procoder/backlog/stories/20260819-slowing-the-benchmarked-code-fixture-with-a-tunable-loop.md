# Slowing the benchmarked code (fixture with a tunable loop) makes `bench` mark the regression and exit 1 at the default threshold.

Status: done 2026-08-19
Created: 2026-08-19
Epic: perf-bench
Sprint: 002-daily-practices-bugsretro-release-adr-deps-bench-0280

## Description

Regressions are marked and exit 1.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Slowing the benchmarked code (fixture with a tunable loop) makes `bench` mark the regression and exit 1 at the default threshold.

## Evidence

- TestCompareMarksRegressionBeyondThreshold marks REGRESSION beyond the threshold and drives exit 1; threshold default (0→10) and negative→exit 2 pinned by TestThresholdZeroMeansTen / TestNegativeThresholdIsUsageError.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
