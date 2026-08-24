# e2e-campaign — report

The running record of the campaign. One section per pass, findings
attributed to procoder or to the fixture before they are fixed, and what
each round did NOT cover alongside what it did.

## Sprint 014 — the fixture, and a clean pass over it

### The fixture

`scripts/build-e2e-fixture.sh` builds a repository from `git init`: one
file for every extension in procoder's tool table — all thirty-three
across thirteen rows, not the twelve languages the spec's prose named —
plus tests in three runners, a CI workflow, docs linking both ways, a
README with badges and a quick start, a changelog, a licence, and
manifests for Go, Python, Rust, Ruby, PHP, npm, Maven, Gradle, Dart and
NuGet.

Reproducible to the commit hash: identity and clock are fixed, so two
builds produce the same tree and the same commit. Verified by building
twice and comparing.

### Findings — procoder

| #    | Severity | What                                                                       | Status |
| ---- | -------- | -------------------------------------------------------------------------- | ------ |
| F-1  | high     | A flag no command implements was read as a filename; the gate exited 0     | fixed  |
| F-2  | high     | `pyproject.toml` aborted osv-scanner, silencing every other manifest       | fixed  |
| F-3  | medium   | A failed tool's reason was the first line of stderr, which is progress     | fixed  |
| F-4  | medium   | `lint` and `security` printed "0 finding(s)" over an empty change set      | fixed  |
| F-5  | medium   | procoder writes three artifacts into a repository and advises ignoring one | fixed  |
| F-6  | medium   | The README version check read two hardcoded manifests, unconfigurable      | fixed  |
| F-7  | low      | A usage error printed all 159 lines of usage, for any of 45 sites          | fixed  |
| F-8  | low      | `sprint status` exited 1 with no active sprint; `todo list` exits 0        | fixed  |
| F-9  | medium   | `brew install rubocop` — no such formula, and no fallthrough to the gem    | fixed  |
| F-10 | medium   | A tool installed exactly where procoder said to put it stayed unresolvable | fixed  |
| F-11 | medium   | `dependencies = []` was read as a dependency set — a gap about nothing     | fixed  |
| F-12 | high     | `procoder security` was weaker than the gate it previews                   | fixed  |
| F-13 | medium   | `config`, `review` and `analyze` shipped absent from `procoder help`       | fixed  |
| F-14 | low      | `docs --ack` — what the gate tells agents to run — was undocumented        | fixed  |
| F-15 | low      | `principles --hook` missing from the command reference                     | fixed  |
| F-16 | medium   | `ci --runs` was in a heading and explained nowhere, exit code included     | fixed  |
| F-17 | high     | The release controller pronounced an already-tagged version ready          | fixed  |

**F-12 is the one the broken pass exists for.** A hardcoded AWS key in a
changed file: the commit gate blocked it, and `procoder security` — the
command a person runs to check their work before committing — reported
"0 finding(s) (0 blocking)". The gate runs the secret scanner AND the SAST
leg over changed files; the command ran only the first. Reachable rather
than theoretical, because the two genuinely differ: gitleaks does not fire
on a bare `const K = "AKIA…"` in Go, and semgrep does.

**F-17 came from cutting a real release** and then asking for the same
version again: "ready — tag it: `git tag -a v0.2.0`", for a tag that
existed and a release that was published. The printed command answers
"fatal: tag 'v0.2.0' already exists". No local fixture could have produced
it — it needs a tag that exists, which needs a release that happened.

**F-9 and F-10 are the same shape as F-17, twice more.** `brew install
rubocop` has never been a formula; rubocop ships as a gem, and brew being
on PATH meant the gem candidate was never reached. `composer global
require phpstan/phpstan`, which procoder also prints, installs into
`~/.composer/vendor/bin` and puts nothing on PATH — so following procoder's
advice exactly left procoder still reporting the tool missing. Three
findings in one family: **an instruction procoder gives that cannot work.**

**F-1.** `procoder check --staged` exited 0. The arm hands `args[1:]` to
the gate as paths, no formatter covers a file called `--staged`, so it
was counted out of scope, nothing else was looked at, and the gate said
clean. Every arm read its flags positionally, so `ci --run`, `security
--deeep` and `format --write` did the same. `procoder version` alone
refused unknown flags, from the start; one guard at the dispatch is that
rule applied to the other seventy-seven.

This is also what surfaced `procoder ci --emit` — specified, backlogged,
and not yet built — which had been exiting 0 while quietly running plain
`ci`.

**F-2.** osv-scanner has no extractor for `pyproject.toml`: it declares
ranges, not pinned versions. Handed one, osv exits 127 and emits no JSON
at all, so one unscannable manifest took every other manifest in the
same invocation with it. A repository with `golang.org/x/text v0.3.0`
in its `go.mod` — seven known vulnerabilities, max severity 7.5 — and a
`pyproject.toml` beside it was told only that output was unreadable.
Verified before and after against exactly that pair.

**F-3.** The reason quoted was `firstLine(stderr)`, and a scanner's first
line is its progress log: "dependencies were NOT checked: Starting
filesystem walk for root: /". Alarming, and not the reason. The exit
status is the fallback, not the answer, or every failure reads "exit
status 127".

**F-9 and F-10 are the same shape twice: an instruction that cannot
work.** `brew install rubocop` was the first thing procoder printed to
anyone missing rubocop, and there has never been such a formula — it
ships as a gem — while brew being on PATH meant the gem candidate was
never reached. Then `composer global require phpstan/phpstan`, which
procoder also prints, installs into `~/.composer/vendor/bin` and puts
nothing on PATH, so following the advice exactly left procoder still
reporting the tool missing. Every other package name in the table was
checked the same way and is real; that check is now part of the clean
pass so it is asked again rather than remembered.

**F-11 was found by the fixture in a fix made earlier the same day.** The
python dependency gap added for F-2 matched the word `dependencies`
anywhere in a `pyproject.toml`, so `dependencies = []` — a project
declaring none — was told its dependencies had not been checked.

### Findings — fixture

| What                                                           | Status |
| -------------------------------------------------------------- | ------ |
| `node --test web/` does not glob a directory on node 26        | fixed  |
| C++ sources written in Google's style, not procoder's baseline | fixed  |
| `set -u` tripped by two exports sharing one statement          | fixed  |

### The clean pass

Fifty-four invocations against the healthy fixture: **43 pass, 3 finding,
7 NOT RUN**, and no false alarm among them.

Every remaining finding is correct behaviour: `doctor` exits 1 because
three tools are absent from this machine, `ci --emit` exits 2 because the
flag is not implemented yet, and `index impls` on a function reports that
functions have no implementors.

Over the whole tree: **45 clean, 0 unformatted, 1 unchecked**. The one is
`cs/Greet.cs`, which needs a dotnet SDK this machine does not have.

### A finding in the campaign itself

The brew-formula check read `tools.go` by a path relative to the working
directory, and the script had already `cd`'d into the fixture. grep found
nothing, the loop ran zero times, and it reported every formula valid.
The campaign committed the exact failure it exists to find, and an empty
list is now a NOT RUN rather than a pass.

Its classifier had the same shape earlier: it claimed NOT RUN for any
output containing "missing", which read a finding about an absent PR
template as a check that never happened.

### What this pass did NOT cover

- **One of the thirteen formatter rows never ran.** Six did at first;
  installing the missing tools closed five of them, and csharpier is the
  last — it needs a dotnet SDK this machine does not have, so C# is
  reported UNCHECKED rather than checked. Java's precise index tier is
  also absent (scip-java needs coursier), and `mvn` is missing, so the
  fixture's Java tests report NOT run.
- **Nothing was planted.** Every "clean" verdict above is only as
  meaningful as the broken pass that follows it, which is sprint 015.
- **Nothing that needs GitHub.** `ci --runs`, `copilot-leak`, `docs
--external` and the release path are sprint 017.
- **The hooks were not fed payloads.** Sprint 016.
- **The docs were not compared against the binary.** Sprint 016.

## Sprint 015 — the broken pass

Twenty-two defects planted, each alone in a freshly built fixture and
removed before the next: two at once cannot tell you which was found.
**21 caught, 0 missed, 1 NOT RUN** (C# formatting needs a dotnet SDK this
machine does not have).

Every catch is matched against the owning command's verdict text, not the
planted file's name — `unformatted  <file>`, `merge conflict marker left
in the file`, `over the 5 MB limit`, `[no-trigger]`, `broken reference`,
`SC2034`, `subprocess-shell-true`, `AWS Access Key ID Value detected`,
`golang.org/x/text`.

The security domain is proved in the directions the product offers. A
planted secret, a SAST finding and a vulnerable dependency each block; an
absent scanner reports "NOT checked — semgrep is not installed" rather
than clean; and the flagged credential's VALUE appears in no finding, no
`QA.md` entry and no hook payload, asserted by searching each for it.

One criterion was rewritten rather than quietly dropped. "Each stops
blocking when the documented configuration relaxes it" was written on a
premise measurement disproved: procoder documents one security knob,
`sast_blocks_at`, and all three of its values only make MORE findings
block. Secrets and vulnerable dependencies have no relaxation at all. The
criterion was unsatisfiable rather than unmet.

The `WARNING`/`ERROR` boundary is recorded **UNPROVED**, not passed:
semgrep's `--config auto` produces no WARNING-severity finding this fixture
can carry, so both settings block the same one.

## Sprint 016 — the hooks and the docs

**20 hook assertions, 0 failures**, at the process boundary the suite
cannot reach. PreToolUse returns `permissionDecision: "deny"` with a reason
naming the file and the fix, and lets a non-commit Bash call through with
no decision at all. PostToolUse returns `additionalContext` carrying
gofmt's output and the sentence saying the file was NOT modified — with
the file, read back after, unmodified. Empty stdin, a truncated payload
and a payload naming a missing file crash nothing.

**53 docs assertions, 0 failures.** Every documented flag is accepted,
every implemented flag is documented, the ADR 0003 exit codes hold, and 28
read-only invocations leave the tree byte-identical.

## Sprint 017 — the GitHub half

`ci --runs` read four real states: in progress, success, failure with the
failing job named, and "HEAD is not pushed — CI cannot have seen it".
`copilot-leak` answered live. `docs --external` reported Pages as not
enabled and blocked on a real 404 with URL and line. A release was cut end
to end and verified by download: the published checksum matched the
downloaded binary, and it ran.

**Not covered:** the Pages health check's "enabled but stale" branch.
Enabling Pages on a throwaway repository to reach one branch was judged
out of proportion, and it is written down rather than left to read as
tested.

## Findings in the campaign itself

Seven, and they matter because each one made a broken check look like a
working one:

1. The clean-pass classifier claimed NOT RUN for any output containing
   "missing", reading a finding about an absent PR template as a check
   that never happened.
2. The brew-formula check read `tools.go` by a path relative to a
   directory the script had already left; grep matched nothing, the loop
   ran zero times, and it reported every formula valid.
3. The broken-pass catch test matched the planted file's name, so
   `UNCHECKED cs/Sloppy.cs — csharpier is not installed` counted as a
   catch.
4. Correcting (3) over-reached and called Dart a NOT RUN, because procoder
   separately reports "NOT linted — Dart: procoder has no linter for it
   yet" about a file whose formatter had caught the defect.
5. Under `set -o pipefail`, `procoder security | grep -q X` fails whenever
   procoder exits 1 — which is what procoder does when it finds something
   — so two checks that matched perfectly read as checks that failed.
6. The docs pass was edited while bash was executing it; bash reads
   incrementally from a byte offset, so its P-CONTROL block ran twice and
   it reported 73 passes instead of 50. A count that is too HIGH reads as
   better news.
7. The P-CONTROL loop ran `procoder format` only over already-clean files,
   so the branch that prints a rewritten file never executed — and a
   mutation making that branch write to disk passed the check untouched.

Five of the seven are one shape: **a grep that finds nothing is
indistinguishable from a grep that found nothing wrong.** Two are the
other: **a check that never reaches the dangerous branch reports success
from the safe one.** Both are recorded in `.procoder/github/LESSONS.md`.

Three times the interesting-looking finding turned out to be the
instrument rather than the subject — lodash 4.18.1 was a real version,
`ci --emit` was planned work rather than a broken promise, and a `.invalid`
dead link is excluded by lychee under RFC 2606. Each check cost about a
minute.

## Sprint 018 — the loop, and teardown

Round two, against a fixture rebuilt from `git init` with eighteen fixes
layered in, is identical to round one in every phase: 42/3/8 clean, 20
hooks, 7 knobs, 53 docs, 21 caught and 0 missed. No finding appeared that
was not already recorded and fixed, and nothing the fixes broke.

The P-CONTROL loop then grew to 55 assertions, covering `bench` and
`release` — the only two commands that can legitimately write. Both write
nothing without their flag. That gap was found by a false alarm: a stale
`baseline.txt` looked like a violation, and following it after it turned
out to be stale is what showed the loop had never covered the two commands
where a violation could actually happen.

Teardown: **6 pass, 0 fail.** The fixture directory gone, the build script
proved to still produce commit `00ea6ca` after the removal, that rebuild
removed too, the script and this report surviving, and the repository no
longer answering — confirmed independently by `gh api` returning 404.

The repository was deleted by its owner; this session's token had no
`delete_repo` scope. The teardown decides its verdict on whether the thing
still answers rather than on a delete command's exit code, so its report is
identical to the one a successful self-delete would have produced.

Before removal the pushed repository was cloned fresh and scanned: **0
findings.** Every planted defect lived in the local fixture and was never
pushed, so the removal was tidiness rather than remediation.

## The campaign, in one place

**17 procoder defects, all fixed, all with a regression test.** 31
mutations applied to source, built, and watched to fail. Three mutations
did NOT fail their test, and each exposed a hollow assertion rather than a
working one.

Three of the seventeen are one family — **an instruction procoder gives
that cannot work**: `brew install rubocop` (no such formula, ever),
`composer global require phpstan/phpstan` (installs where procoder then
cannot look), and `git tag -a v0.2.0` (printed for a tag that exists).

Two are the campaign's headline shape — **a check that reports clean
because it never looked**: `procoder check --staged` exiting 0 having
assessed a filename somebody mistyped, and `procoder security` reporting
0 findings on a hardcoded AWS key the gate blocks.

**7 defects in the campaign's own harness.** Five are one shape: a grep
that finds nothing is indistinguishable from a grep that found nothing
wrong. Two are the other: a check that never reaches the dangerous branch
reports success from the safe one. Both are in
`.procoder/github/LESSONS.md`.

**Four times an alarming finding turned out to be the instrument** — a
version number that was real, a feature that was backlogged rather than
broken, a dead link excluded by RFC 2606, and a stale benchmark baseline.
Each check cost about a minute; each would otherwise have been a false
report filed against the project's own record.

### What this campaign did not cover

- C# formatting — csharpier needs a dotnet SDK this machine lacks.
- The Java precise index tier — scip-java needs coursier.
- Maven tests — `mvn` is absent.
- The `sast_blocks_at` WARNING/ERROR boundary — semgrep's `--config auto`
  produces no WARNING-severity finding this fixture can carry, so both
  settings block the same one.
- The Pages health check's "enabled but stale" branch — enabling Pages on
  a throwaway repository to reach one branch was out of proportion.
- Windows and Linux — CI covers all three platforms; this campaign ran on
  darwin only.
