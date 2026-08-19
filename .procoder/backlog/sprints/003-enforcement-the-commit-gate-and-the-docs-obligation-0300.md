# Enforcement: the commit gate and the docs obligation (0.30.0)

Status: closed 2026-08-19
Created: 2026-08-19

## Goal

<!-- What this sprint commits to deliver, in the reader's terms — the
     outcome the stories add up to, not a list of them. -->

## Result

committed: 17
done: 17 (20260819-a-change-to-a-file-that-a-documentation-page-names-with-no, 20260819-a-compound---git-commit--m-x-is-detected-echo-commit-and-gh, 20260819-a-docs-none--internal-refactor-line-in-the-commit-message, 20260819-a-malformed-payload-allows-the-command-and-says-the-gate, 20260819-a-repository-with-no-index-gets-the-file-mention-trigger, 20260819-agentsmd-and-root-level-markdown-are-part-of-the, 20260819-an-internal-change-touching-neither-public-surface-nor-doc, 20260819-docsportabilitymd-states-per-host-whether-command, 20260819-every-command-shipped-in-0290-appears-in-agentsmd, 20260819-git-commit---no-verify-proceeds-and-the-output-says-the, 20260819-git-commit-gate--report-prints-findings-and-allows-the, 20260819-in-a-fixture-repository-with-no-procoder-identity-renaming, 20260819-procoder-hook-install-git-prints-a-working-pre-commit, 20260819-the-block-policy-makes-the-obligation-block-the-gate-the, 20260819-the-public-surface-coverage-check-runs-in-a-fixture, 20260819-the-same-commit-succeeds-once-the-finding-is-fixed-with-no, 20260819-with-the-hook-installed-a-git-commit-on-a-tree-with-a)
carried: 0

## Retro

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->

What slowed us: a careless shell pipeline emptied two spec files
(`procoder format` prints only a header when a file is already
formatted, so `tail -n +2` produced nothing) — the spec controller
caught it in seconds, but it cost a rewrite; and shell escaping of
`&&` inside a test harness produced a false "the matcher is broken"
scare that took a second look to clear.

What we change: never pipe `procoder format` output without checking
it has more than one line — the same guard now used in every script
here; and reproduce a suspected product bug through a real payload
before believing a hand-rolled shell test.

Adaptation worth keeping: the docs obligation was threaded all the way
to the commit message (hook -m parsing → gate.RunWith →
gitcmd.CollectFor), so the acknowledgment clears at the moment the
decision is actually made rather than being a promise the tool could
not check. Wire escape hatches to the point of decision, not near it.
