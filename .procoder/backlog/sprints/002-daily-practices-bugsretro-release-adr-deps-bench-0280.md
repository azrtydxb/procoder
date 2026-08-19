# Daily practices: bugs+retro, release, adr, deps, bench (0.28.0)

Status: closed 2026-08-19
Created: 2026-08-19

## Goal

<!-- What this sprint commits to deliver, in the reader's terms — the
     outcome the stories add up to, not a list of them. -->

## Result

committed: 24
done: 24 (20260819-a-missing-optional-tool-cargo-outdated-yields-not-checked, 20260819-a-repo-with-no-benchmarks-answers-the-no-benchmarks-line, 20260819-a-repo-with-no-manifests-answers-with-the-no-manifests-line, 20260819-adr-check-refuses-on-an-empty-decision-section-an-unknown, 20260819-adr-list-orders-proposed-before-accepted-and-shows, 20260819-adr-new-prints-0001-for-an-empty-repo-and-0003-when-0002, 20260819-after-fixing-all-three-it-prints-the-tag-command-and-exits, 20260819-all-existing-backlog-and-sprint-tests-pass-unmodified, 20260819-baseline-parsing-and-delta-math-have-unit-tests-over, 20260819-on-a-fixture-npm-project-with-a-pinned-old-dependency-the, 20260819-on-a-fixture-with-one-benchmark-bench---save-writes-the, 20260819-on-a-fixture-with-two-listed-files-one-stale-procoder, 20260819-on-procoders-own-repo-procoder-deps-prints-a-go-section, 20260819-parse-tests-cover-npm-outdated-json-and-go-list--u--m, 20260819-procoder-audit-on-a-repo-with-a-broken-adr-includes-the, 20260819-procoder-backlog-bug-login-500s---severity-s1-prints-a, 20260819-procoder-release-no-argument-reads-the-newest-changelog, 20260819-procoders-own-config-lists-its-version-files-and-procoder, 20260819-slowing-the-benchmarked-code-fixture-with-a-tunable-loop, 20260819-sprint-close-writes-a-retro-scaffold-sprint-open-then, 20260819-sprint-retro--off-disables-the-retro-gate-verified-by-test, 20260819-the-board-marks-an-open-bug-with-its-severity-and-the, 20260819-the-perf-skill-instructs-measuring-via-procoder-bench-and, 20260819-with-release-files-unset-the-output-states-version-sync)
carried: 0

## Retro

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->

What slowed us: the dev binary rebuilt without release flags crossed the
gate's 5 MB limit and blocked every story close until rebuilt stripped;
and a Windows CI checkout flake cost one rerun on the parallel PR.

What we change: dev rebuilds use the release ldflags by habit (the
Makefile-less loop copies the release command), and CI flakes get one
rerun before any diagnosis time is spent.

Adaptation worth keeping: five parallel agents on five disjoint
packages with the integrator owning main.go/docs produced zero file
conflicts for the second sprint running — keep that decomposition rule.
