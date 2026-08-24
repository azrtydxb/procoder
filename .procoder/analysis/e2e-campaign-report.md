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

| #   | Severity | What                                                                       | Status |
| --- | -------- | -------------------------------------------------------------------------- | ------ |
| F-1 | high     | A flag no command implements was read as a filename; the gate exited 0     | fixed  |
| F-2 | high     | `pyproject.toml` aborted osv-scanner, silencing every other manifest       | fixed  |
| F-3 | medium   | A failed tool's reason was the first line of stderr, which is progress     | fixed  |
| F-4 | medium   | `lint` and `security` printed "0 finding(s)" over an empty change set      | fixed  |
| F-5 | medium   | procoder writes three artifacts into a repository and advises ignoring one | fixed  |
| F-6 | medium   | The README version check read two hardcoded manifests, unconfigurable      | fixed  |
| F-7 | low      | A usage error printed all 159 lines of usage, for any of 45 sites          | fixed  |
| F-8 | open     | `sprint status` exits 1 with no active sprint; `todo list` exits 0         | asked  |

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

**F-8** is a question rather than a finding, because ADR 0003 pins exit
codes as the public interface and the answer is a judgement about intent,
not a fact about the code.

### Findings — fixture

| What                                                           | Status |
| -------------------------------------------------------------- | ------ |
| `node --test web/` does not glob a directory on node 26        | fixed  |
| C++ sources written in Google's style, not procoder's baseline | fixed  |
| `set -u` tripped by two exports sharing one statement          | fixed  |

### The clean pass

Fifty-three invocations against the healthy fixture: **40 pass, 4
finding, 9 NOT RUN.**

Every remaining finding is correct behaviour: `doctor` exits 1 because
eleven tools are absent from this machine, `ci --emit` exits 2 because
the flag is not implemented yet, `index impls` on a function reports that
functions have no implementors, and `sprint status` is F-8.

### What this pass did NOT cover

- **Six of the thirteen formatter rows never ran.** rubocop, ktfmt,
  csharpier, dart, google-java-format and the prettier PHP plugin are
  not installed here, so Ruby, Kotlin, C#, Dart, Java and PHP were
  reported UNCHECKED rather than checked. That is the honest verdict and
  it is also a hole in the campaign: six languages procoder claims are
  untested by it until those tools exist on the machine.
- **Nothing was planted.** Every "clean" verdict above is only as
  meaningful as the broken pass that follows it, which is sprint 015.
- **Nothing that needs GitHub.** `ci --runs`, `copilot-leak`, `docs
--external` and the release path are sprint 017.
- **The hooks were not fed payloads.** Sprint 016.
- **The docs were not compared against the binary.** Sprint 016.
