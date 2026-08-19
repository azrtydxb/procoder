# Daily practices: bugs+retro, release, adr, deps, bench (0.28.0)

Status: closed 2026-08-19
Created: 2026-08-19

## Goal

<!-- What this sprint commits to deliver, in the reader's terms — the
     outcome the stories add up to, not a list of them. -->

## Result

committed: 24
done: 24 (20260819-a-missing-optional-tool-cargo-outdated-yields-not-checked-naming-the-tool--verified-by-test-with-a-stub-path, 20260819-a-repo-with-no-benchmarks-answers-the-no-benchmarks-line-exit-0, 20260819-a-repo-with-no-manifests-answers-with-the-no-manifests-line-and-exit-0, 20260819-adr-check-refuses-on-an-empty-decision-section-an-unknown-status-a-dangling-supersede-reference-and-a-duplicated-number--each-named--and-passes-on-a-valid-set, 20260819-adr-list-orders-proposed-before-accepted-and-shows-superseded-by-targets, 20260819-adr-new-prints-0001-for-an-empty-repo-and-0003-when-0002-exists-the-printed-file-carries-all-three-sections-and-todays-date-nothing-is-written-by-the-binary, 20260819-after-fixing-all-three-it-prints-the-tag-command-and-exits-0-having-tagged-nothing, 20260819-all-existing-backlog-and-sprint-tests-pass-unmodified, 20260819-baseline-parsing-and-delta-math-have-unit-tests-over-recorded-go-test--bench-output-including-the-renamed-benchmark-case, 20260819-on-a-fixture-npm-project-with-a-pinned-old-dependency-the-js-section-names-it-with-current-and-latest-versions, 20260819-on-a-fixture-with-one-benchmark-bench---save-writes-the-baseline-with-the-header-a-second-run-reports-0-delta-and-exits-0, 20260819-on-a-fixture-with-two-listed-files-one-stale-procoder-release-123-lists-exactly-the-stale-file-the-missing-changelog-heading-and-the-dirty-tree--all-in-one-output--and-exits-1, 20260819-on-procoders-own-repo-procoder-deps-prints-a-go-section-with-real-freshness-rows-or-an-explicit-up-to-date-line-and-a-licenses-line-that-is-either-a-go-licenses-report-or-an-honest-not-checked-with-the-install-hint, 20260819-parse-tests-cover-npm-outdated-json-and-go-list--u--m-output-shapes-including-the-everything-current-case, 20260819-procoder-audit-on-a-repo-with-a-broken-adr-includes-the-finding, 20260819-procoder-backlog-bug-login-500s---severity-s1-prints-a-story-file-with-type-severity-the-repro-prompting-description-and-the-pre-seeded-regression-test-criterion-writing-and-closing-it-without-a-severity-header-is-refused, 20260819-procoder-release-no-argument-reads-the-newest-changelog-version-and-reports-the-checklist-for-it, 20260819-procoders-own-config-lists-its-version-files-and-procoder-release-current-passes-on-a-clean-tree, 20260819-slowing-the-benchmarked-code-fixture-with-a-tunable-loop-makes-bench-mark-the-regression-and-exit-1-at-the-default-threshold, 20260819-sprint-close-writes-a-retro-scaffold-sprint-open-then-refuses-until-the-retro-has-real-content-and-proceeds-after-one-sentence-is-added--verified-by-a-test-walking-the-sequence, 20260819-sprint-retro--off-disables-the-retro-gate-verified-by-test, 20260819-the-board-marks-an-open-bug-with-its-severity-and-the-summary-counts-open-bugs-list-shows-kind-bug, 20260819-the-perf-skill-instructs-measuring-via-procoder-bench-and-its-opencode-twin-matches, 20260819-with-release-files-unset-the-output-states-version-sync-verified-nothing)
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
