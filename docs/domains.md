# The nine domains

procoder organises senior-developer work into nine domains. Each follows
the same architecture: the binary computes findings, the write hook hands
them to the agent in the same turn, the gate carries them at commit time,
a skill packages the workflow, and every rule is repo-overridable.

## 1. Security — blocking where it must be

gitleaks scans every written file the moment it lands and the changed set
at the gate: a secret always blocks, the finding names rule and location
(never the value) and orders a rotation. `procoder security --deep` adds
semgrep with the community rulesets (ERROR blocks) and osv-scanner over
explicitly named manifests (CVSS ≥ 7.0 blocks). Missing scanners read as
blocking NOT-checked — a security check that silently didn't run is worse
than a red one.

## 2. Best practices — the canonical linter per ecosystem

golangci-lint, ruff check, shellcheck, and eslint, each under the
project's own configuration. Configless plain JavaScript gets a baseline
of eslint's built-in rules, labeled as procoder's voice. Report by
default — lint is judgment where formatting was not.

## 3. Maintainability — judgment, informed

Dead-code candidates from the index's precise tier (exported API marked —
a public surface is legitimately unreferenced from inside), complexity
and function length at repo-tunable thresholds. Nothing blocks; deletion
is the recommended refactor.

## 4. Performance — measure first

`/procoder:perf` encodes the discipline: baseline before touching,
profile before guessing, re-measure after, report the delta with the
command that produced it. A fix without a benchmark is a hope.

## 5. Documentation — correct, presentable, delivered

Broken relative references and non-compiling Mermaid diagrams block; doc
drift, API doc comments, badges, README structure, and command coverage
report. Version-tracked pages must carry the current version — a release
without a reviewed page blocks. External links verified by lychee in CI;
this site is built and deployed by the harness's own docs job.

## 6. Clean code — the formatter's answer, the agent's hands

Every write is checked against the ecosystem's canonical formatter; when
unformatted, the agent receives the formatted result in-turn and writes
it itself — the file is never touched behind its back. Unchecked is a
verdict, never silence.

## 7. CI/CD/CT — pipeline discipline

Actions pinned to commit SHAs (a tag can be silently repointed), per-job
timeouts, concurrency cancellation, and the existence of tests — plus run
health via `gh`.

## 8. DevOps/IaaS/CaaS — each tool where its files exist

hadolint, terraform fmt/validate/tflint, kubeconform, helm lint —
inventory-driven, so a repo without infrastructure pays nothing. A
failing `terraform validate` blocks; an uninitialised directory says NOT
validated instead of failing on providers.

## 9. GitOps/GitHub — the finishing discipline

Conflict markers, junk and caches, oversized files, and AI-attribution
lines block; commit subjects and default-branch work report. actionlint
runs on every workflow write. Templates standardize PRs and commits;
feature work happens in worktrees; merges wait for every check and every
review (bots included), then clean up local and remote.

## Beneath them: the code index

Two tiers — universal-ctags for breadth, SCIP for precision — with eleven
queries from `find` to the call `graph`, kept current by the hook and the
gate, consumed by the agent and the domains alike.
