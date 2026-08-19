# The loop: single test, run, status, handoff, env and CI awareness (0.31.0)

Status: closed 2026-08-19
Created: 2026-08-19

## Goal

<!-- What this sprint commits to deliver, in the reader's terms — the
     outcome the stories add up to, not a list of them. -->

## Result

committed: 27
done: 27 (20260819-a-lockfile-made-unreadable-yields-a-not-checked-line-naming, 20260819-agent-authored-notes-survive-a-second-hook-stop--verified, 20260819-an-unwritable-state-directory-leaves-hook-stop-exiting-0, 20260819-every-path-in-both-commands-output-uses-forward-slashes, 20260819-every-path-printed-by-either-half-uses-forward-slashes, 20260819-name-combined-with-paths-and---coverage-in-one-invocation, 20260819-on-a-fixture-whose-dbmigrate-gained-two-files-since-the, 20260819-on-a-fixture-whose-ecosystem-cannot-express-filtering-the, 20260819-on-a-fixture-whose-envexample-declares-database-url-and, 20260819-on-a-fixture-with-a-recorded-baseline-and-a-mutated-package, 20260819-parse-tests-over-recorded-gh-run-list---json-output-cover-a, 20260819-principles---hook-output-contains-the-status-block-after, 20260819-procoder-env---sync-writes-exactly-one-file, 20260819-procoder-hook-stop-writes-procoderstatehandoffmd-containing, 20260819-procoder-run---exec-on-a-single-candidate-naming-a-server, 20260819-procoder-run---exec-with-two-or-more-candidates-refuses-and, 20260819-procoder-run-in-a-repository-with-no-launch-declaration, 20260819-procoder-run-on-a-fixture-with-packagejson-dev-and-start, 20260819-procoder-status-in-a-non-git-temporary-directory-reports, 20260819-procoder-status-on-this-repository-prints-branch-dirty, 20260819-procoder-test---name-pattern-on-a-go-fixture-with-two-test, 20260819-procoderstate-appears-in-the-gitignore-guidance-the-docs, 20260819-the-js-path-appends-the-pattern-after----and-the-output, 20260819-the-sessionstart-path-completes-within-the-3-second-budget, 20260819-usage-lists-run-docscommands-the-docs-site-commandsrunmd, 20260819-with-gh-absent-from-a-stub-path-procoder-ci---runs-prints, 20260819-with-no-procoderstateenvjson-procoder-env-prints-the-no)
carried: 0

## Retro

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->

What slowed us: closing 27 stories in a serial shell loop ran the full
gate AND the full suite 27 times — about thirteen minutes of waiting
that looked like a hang from the outside. Each close was correct; the
loop around them was the waste. Live verification also caught `env`
surveying 48 lockfiles inside gitignored agent worktrees, because the
new package hand-rolled a skip list instead of asking git.

What we change: batch closes need one verification, not N — filed as
the first story of the next milestone (`backlog close story <id>...`
taking several ids and running the gate and suite once). And any new
file-discovery walk asks git what it ignores rather than keeping its
own list; the codeindex precedent was there to copy.

Adaptation worth keeping: the live-verification pass after the agents
report earned its place twice this sprint — the gitignore defect and
the story-list flooding at session start were both invisible in green
tests and obvious the moment the command ran on this repository.
