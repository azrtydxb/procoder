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
