# Changelog

Every release, in words a user can read. Newest first.

<!--
How an entry is written. This layout is the template — the release notes
GitHub publishes are this entry, extracted verbatim by the release job, so
what is written here is what a person downloading the binary reads.

    ## <version> — <YYYY-MM-DD>

    *One sentence: what this release is for, for someone deciding whether
    to upgrade.*

    **Fixed — the headline, as a statement.**
    ([#123](https://github.com/azrtydxb/procoder/pull/123)) Then the
    prose: what was wrong, what it cost the person using it, what is
    true now. Reported by
    [@handle](https://github.com/handle) — a bot is credited as
    [@github-actions[bot]](https://github.com/apps/github-actions), the
    `[bot]` suffix and all. Paragraphs, not bullet lists —
    a changelog is read, not parsed.

Rules that earn their place:

- The summary line comes first and is one sentence. It is the only part a
  reader skimming a release page is guaranteed to see.
- Every headline opens with its kind — Added, Fixed, Changed, Removed,
  Security — so a reader can tell a new feature from a broken thing made
  whole without reading the paragraph.
- Something that was broken for everyone leads. Order by what it costs the
  reader, not by what it cost to build.
- Every entry links the PR (or PRs) that shipped it —
  `[#123](https://github.com/azrtydxb/procoder/pull/123)` — not just the
  issue. An entry that spans several PRs' worth of change links all of
  them, the way 1.4.0's entries do. Link the tracking issue too when one
  exists and the reader would otherwise have no way to find it.
- Every outside contributor is named AND linked to their GitHub profile —
  `[@handle](https://github.com/handle)`, never the bare @handle text,
  which is not a link inside this file. This covers whoever reported it,
  diagnosed it, or wrote the fix; being the reason something got looked
  at is the bar, not having sent a PR. Filing the issue and nothing else
  still earns the credit — say so plainly ("Reported by ...") rather than
  folding a reporter into a sentence about the fix, which reads as if the
  fixer noticed it unprompted.
- Get the name right. A misattributed credit is worse than none — verify
  who actually opened the issue or PR being cited (`gh issue view <n>`,
  `gh pr view <n>`) before writing the handle, rather than recalling it
  from memory or another entry's context. `procoder release` checks this
  against GitHub and refuses to call a release ready when a credited
  handle opened none of what its paragraph cites.
-->

## Unreleased

_Every command can be called instead of spawned._

**Added — a second door: every command answers over a local socket.**
([#272](https://github.com/azrtydxb/procoder/pull/272)) `procoder serve`
runs a local daemon, and a caller that would rather make a call than spawn
a process may. A response carries the command's exact bytes on stdout and
stderr, its exit code, and — where the command has one — a typed result
beside them: findings with their file, line, domain and whether they
block, and named shapes for `config`, `todo list`, `version`, `status`,
`spec check` and the index lookups. Both, so that no caller has to parse
prose and none has to render it.

The first door does not move. Every command still runs in-process, with no
daemon, no socket and no setup, in CI and on a fresh clone; `[service]
mode` is `off` until a machine says otherwise, and `procoder init` asks
rather than choosing. A daemon that cannot be reached — absent, from
another build, gone mid-request — costs you its speed and never your
answer.

`TestParityAcrossEveryCommand` reads the command list out of the usage
text and runs all 48 twice, comparing stdout, stderr and the exit code
byte for byte. A command added without a parity case fails it.

The nine commands that run a whole toolchain over a whole tree — `test`,
`audit`, `release`, `bench`, `deps`, and `security --deep`,
`docs --external`, `ci --runs`, `index build` — answer with a job in
milliseconds and keep running behind it, so a suite no longer holds a
connection open for its duration.

**Added — the commands that run things have their own door, or none.**
([#272](https://github.com/azrtydxb/procoder/pull/272))
`run --exec`, `evidence record`, `init --yes` and `self-upgrade` run what
a repository, or a prior agent session, declared. They are refused on the
ordinary socket and served only on a second one, opt-in via `[service]
exec`, whose address the hooks are never told. The socket's 0600 mode
authenticates the user and not the process — every process running as you
can open it, an agent session's own shell included — so "look but don't
run" is held by the shape of the thing rather than by a rule.

**Fixed — one daemon no longer contends with itself over a repository's
own ledgers.**
([#272](https://github.com/azrtydxb/procoder/pull/272)) The state lock is a lockfile with a heartbeat and no
in-process registry, so two requests for one repository inside a single
daemon did not queue: the second waited out the five-second timeout and
reported that its write was not made. Requests are serialised per
repository now. Rare while every hook was its own process; ordinary the
moment one process serves several sessions.

**Fixed — the session-start hook answers in the calling host's shape, not
the daemon's.**
([#272](https://github.com/azrtydxb/procoder/pull/272)) Host detection read the process environment, so one daemon
started by a Claude session and later serving a Copilot or Qoder one
shaped its answer for the wrong host, with the payload identical and
nothing to see. The environment travels with the request.

## 3.5.0 — 2026-09-01

_Every host now holds the same turn end, and the record-keeping outlives the reflow._

**Fixed — the documented format pipeline no longer empties files.**
([#254](https://github.com/azrtydxb/procoder/pull/254)) Three files were
lost in one session to the workflow `procoder format <file> > <file>`:
the command wrote the formatted content only in one of its four verdicts,
and a file that was already clean — or out of scope, or unchecked —
replaced itself with a one-line banner. Stdout of `procoder format` is now
the file's formatted bytes in every verdict, the banner moved to stderr,
and a redirection that would overwrite the file being checked is refused
outright rather than raced.

**Fixed — the repository's gitleaks allowlist applies to the gate's
scans again.** ([#260](https://github.com/azrtydxb/procoder/issues/260),
commit `b16a000`) Per-file scans used gitleaks' hard-coded default config,
so a repository's `.gitleaks.toml` silently stopped applying the moment a
scan named a single file — which is exactly how the gate scans changed
files. Allowlists are read from the repository now.

**Fixed — a recorded answer outlives a reflow, and the gate sees a
commit's own heredoc.**
([#257](https://github.com/azrtydxb/procoder/pull/257)) The question key
hashed the text as written, so a formatter rewrapping a decision section
changed the key and the queue re-asked a settled question. Keys now hash
the words, not the line breaks, with a fallback that reads stores written
by older versions. Alongside it, an acknowledgment line sitting in the
command's own heredoc — `cat > msg.txt <<'EOF'` then `git commit -F
msg.txt` — now reaches the documentation obligation it was written to
clear, instead of being reported as "no commit message reached this
check".

**Added — pi is a first-class host.**
([#250](https://github.com/azrtydxb/procoder/pull/250)) The install line
in the README covers it: `pi install git:github.com/azrtydxb/procoder`.
pi gets the commit gate at the tool boundary, the write hook's findings
patched into the tool result that caused them, the turn-end handoff, the
command set registered as `/procoder:*`, and the contract injected only
when pi has not loaded it itself.

**Added — the rules reach every host, and every host has a turn end.**
([#259](https://github.com/azrtydxb/procoder/pull/259)) The parallel-
work policy — fan out where the work decomposes, fence writers in worktrees
when they touch the same feature, converge one branch one writer through
the chain — is in `AGENTS.md` and in every rule copy, drift-blocked by
the gate. OpenCode and Kilo run `hook stop` on `session.idle` and
`session.compacted`: where the host cannot refuse a turn, it records —
and the handoff now goes into the repository the session names, not where
the server process happens to sit.

**Changed — pinned tools moved.** golangci-lint 2.13.2,
([#251](https://github.com/azrtydxb/procoder/pull/251)), semgrep
1.175.0, ([#252](https://github.com/azrtydxb/procoder/pull/252)), ruff
0.16.5, ([#253](https://github.com/azrtydxb/procoder/pull/253)), contributed by [@github-actions[bot]](https://github.com/apps/github-actions).

## 3.4.0 — 2026-08-27

_Two checks that had quietly stopped doing their job, the six roadmap items
closed, and a gate that no longer waits on itself._

**Fixed — the linter was choosing which findings to show you, and choosing
differently each run.** ([#236](https://github.com/azrtydxb/procoder/issues/236))
golangci-lint caps its own output at fifty issues per linter and three with
the same text, and it lints packages concurrently — so which issues
survived depended on which package finished first. Two runs over an
unchanged tree reported forty-eight findings each and disagreed about their
members.

Worse than unstable: it was hiding work. `errcheck` emits near-identical
messages, so three survived and the rest never reached the report at all.
Both caps are off. The set is now complete and the same twice.

**Fixed — the check that stops a decision being buried had switched itself
off.** ([#242](https://github.com/azrtydxb/procoder/pull/242)) The rule that questions are the user's to answer is enforced at the
end of a turn. Its guard asked "did the agent record a decision", and
answered it by looking for any pending decision at all — so the first one
ever written to `.procoder/ask/decisions.md` disabled the check for every
turn after it, in that repository, permanently. An enforcement that goes
quiet exactly when decisions are piling up unanswered has it backwards. It
now compares the file across turns, so "recorded" means recorded _now_.

**Added — `procoder learn`: what procoder's own governance costs here.**
([#190](https://github.com/azrtydxb/procoder/issues/190)) Every other
domain measures your code. This one measures procoder, which is the claim
its documentation could not otherwise make: no benchmark of the gate's
overhead against defects caught had ever been run, and the honest position
was that nobody knew.

Recording is off until a repository asks for it, and holds a command name,
a duration and an exit code in gitignored state. `propose` prints
configuration changes and writes none of them; `verify` reports whether an
applied change reduced what it targeted, and prints the revert when it did
not. Every number says where it came from — `measured` for a total over
recorded runs, `manual claim` for anything projected from them. A proposal
that suggests relaxing a blocking check always says, in the same breath,
that the records hold what a check cost and nothing about what it
prevented.

**Added — `procoder wizard`: walking a human through what procoder cannot
do.** ([#192](https://github.com/azrtydxb/procoder/issues/192)) Creating an
account, generating a token, clicking submit in somebody else's dashboard.
Those are not shell commands, so this executes nothing: `show` prints the
steps and `run` advances through them one at a time, so none is skipped by
reading past it. A captured value is shape-checked and never echoed — not
in the message accepting it, not in the one rejecting it, and not in the
summary, which names `TOKEN` and never what it held.

**Added — the agent layer says what an agent talks itself into.**
([#189](https://github.com/azrtydxb/procoder/issues/189)) A
rationalization table whose every row is a sentence that has actually been
said, a routing table from where you are to the first command that fits,
and a five-item checklist with answers rather than prose to nod along to.
They live in `AGENTS.md` and reach all twelve host rule files. This changes
what the contract requires, so the skill's `contract` version is now `2`.

**Added — four documents a skeptical adopter can evaluate.**
([#194](https://github.com/azrtydxb/procoder/issues/194),
[#209](https://github.com/azrtydxb/procoder/issues/209),
[#211](https://github.com/azrtydxb/procoder/issues/211)) Known-pitfalls
sections on five commands, each verified against the code rather than
recalled. `docs/honest-limits.md` states where the rigor stops paying.
`docs/positioning.md` states the layer procoder occupies and what it is
not. `docs/research.md` separates premises with external evidence from
premises without, cites the sources it read, and leaves the multi-lens
review premise deliberately uncited because that literature disagrees with
itself.

`docs/comparable-projects.md` names the projects solving adjacent problems.
It does not make the independent-convergence argument this repository's own
issues contradict: five features shipped in 3.3.0 cite unlazy as their
source, so `docs/influences.md` gains a fifth relationship that says so.

**Fixed — a spec may name the command it introduces.**
([#230](https://github.com/azrtydxb/procoder/issues/230),
[#231](https://github.com/azrtydxb/procoder/issues/231)) The citation check
resolved a cited command against the registry, so a spec proposing a NEW
command could not name the thing it proposed. A command declared in
`## Interfaces` is now a forward reference while the spec is a draft, and
stops being excused once it is marked complete. Separately, a TOML value of
`= true` inside backticks no longer reads as the shell builtin.

**Changed — the gate stopped waiting on itself.**
([#237](https://github.com/azrtydxb/procoder/pull/237),
[#240](https://github.com/azrtydxb/procoder/pull/240),
[#243](https://github.com/azrtydxb/procoder/pull/243),
[#244](https://github.com/azrtydxb/procoder/pull/244),
[#235](https://github.com/azrtydxb/procoder/pull/235),
[#239](https://github.com/azrtydxb/procoder/pull/239)) Its independent passes
run at once and the per-file work is fanned out, all drawing on one
concurrency budget rather than three: 136s to 74s on a ten-core machine
over this repository's 787 files. Measured in CI it is unchanged, and the
CI job was fixed a different way — its budget had no headroom and a slow
run was being reported as a failure.

The gate job also scanned the tree twice. `procoder check` already runs the
whole-tree SAST and dependency passes, so the separate `security --deep`
step was repeating them; and the tree is now passed through
`check --paths-from` in one invocation rather than piped into `xargs`,
which splits a long list into batches and runs the command once per batch.

## 3.3.0 — 2026-08-27

_Fifteen ways an agent's work looked finished when it was not — a check
nobody ran, a fan-out that was secretly serial, a decision made on your
behalf — each made visible._

**Added — nothing procoder runs automatically comes from a file an agent
wrote.** ([#219](https://github.com/azrtydxb/procoder/pull/219),
[#201](https://github.com/azrtydxb/procoder/issues/201),
[#200](https://github.com/azrtydxb/procoder/issues/200)) Several procoder
commands read setup steps out of the repository. Those files are written by
agents, and a file an agent wrote is a file an attacker can influence
through anything that agent read. The boundary is now explicit: procoder
looks at those commands and prints them, and a human runs them. Where a
command genuinely must be re-run on trust, the approval is fingerprinted
against the environment it was granted in, so trust given on your laptop is
not trust taken in CI.

**Added — who is working on what, while several agents work at once.**
([#224](https://github.com/azrtydxb/procoder/pull/224),
[#199](https://github.com/azrtydxb/procoder/issues/199)) "Two agents never
own the same file" was a rule in prose, which means it held exactly as long
as everybody remembered it. `procoder claims add <glob> --by <agent>`
records the claim and reports an overlap. It reports and never prevents:
procoder does not own your editor, and a lease that cannot actually stop a
write should not pretend it can.

**Added — whether a fan-out was actually parallel.**
([#225](https://github.com/azrtydxb/procoder/pull/225),
[#202](https://github.com/azrtydxb/procoder/issues/202)) Launching five
agents and awaiting them one at a time looks identical, in every report, to
launching five agents at once. `procoder dispatch` opens a wave, takes a
start per task, and seals it — and a task that returns before the seal is
the signature of serial work wearing a parallel costume. Advisory: it makes
the claim checkable, not true.

**Added — evidence that can be re-checked, and saying so when it cannot.**
([#220](https://github.com/azrtydxb/procoder/pull/220),
[#204](https://github.com/azrtydxb/procoder/issues/204),
[#208](https://github.com/azrtydxb/procoder/issues/208)) `procoder evidence
record <command>` runs a check and stores a fingerprint of its result,
never its output — proof it ran, without a story's evidence section turning
into a log dump. And closing a story now distinguishes evidence that was
_measured_ from evidence that is a _claim_ someone typed. Both are allowed;
only one of them is checkable, and the difference used to be invisible.

**Added — did the change stay where the plan said it would.**
([#223](https://github.com/azrtydxb/procoder/pull/223),
[#197](https://github.com/azrtydxb/procoder/issues/197)) The failure is the
drive-by edit: a change that does what was asked and also touches four
files nowhere near it, each a small improvement nobody reviewed as such.
`procoder backlog scope` compares what changed against the files the plans
declare. When there is nothing to compare against it says `scope NOT
checked` rather than reporting a clean result, because a check that could
not run must never read as one that passed.

**Added — which carried-over items are carried _and_ untouched.**
([#222](https://github.com/azrtydxb/procoder/pull/222),
[#205](https://github.com/azrtydxb/procoder/issues/205)) A story edited
nine times across three sprints, criteria still unchecked and evidence
still empty, looks busy in every other report. `procoder backlog stalled`
hashes what a story _means_ — its status, criteria and evidence — and
ignores the metadata churn, so a long session cannot mistake motion for
progress.

**Added — the four passes before a piece of work is finished, and the same
rigor at every depth.** ([#221](https://github.com/azrtydxb/procoder/pull/221),
[#203](https://github.com/azrtydxb/procoder/issues/203),
[#207](https://github.com/azrtydxb/procoder/issues/207)) Implement, reread,
hunt defects, polish — closing after the first pass is how work ships that
compiles and is still wrong. And depth is where attention leaks: on a large
decomposed task the last leaves get a fraction of the care the first ones
got, though each is still somebody's afternoon spent reading what you
wrote.

**Added — a turn does not end with a decision buried in prose.**
([#215](https://github.com/azrtydxb/procoder/pull/215)) The rule that
questions are not the agent's to answer was already written down, and was
still being routed around — a decision would appear as a sentence in a
paragraph, and the session would move on as if it had been settled. It is
now enforced at the end of the turn, not merely documented. An invented
answer is indistinguishable from a decision once it is written down, and
the person who was never asked never finds out.

**Added — decisions that block other work appear on the board.**
([#224](https://github.com/azrtydxb/procoder/pull/224),
[#191](https://github.com/azrtydxb/procoder/issues/191)) An undecided
question holding up a story was visible only to whoever read the decisions
file. It is now a thing the backlog can show you, because a blocker nobody
can see is a blocker nobody unblocks.

**Added — a shared vocabulary, and three more oracles that refuse.**
([#217](https://github.com/azrtydxb/procoder/pull/217),
[#218](https://github.com/azrtydxb/procoder/pull/218),
[#195](https://github.com/azrtydxb/procoder/issues/195),
[#198](https://github.com/azrtydxb/procoder/issues/198)) `.procoder/context.md`
records what the team calls things, which is not always what the code calls
them, and spec and plan checks cross-reference it. Separately, acceptance
criteria that cannot fail are refused in three more shapes: a fixed-output
command, hedging vocabulary, and a threshold with nothing measuring it.

**Added — the skill contract carries a version, and the provenance map
names BMad.** ([#218](https://github.com/azrtydxb/procoder/pull/218),
[#217](https://github.com/azrtydxb/procoder/pull/217),
[#196](https://github.com/azrtydxb/procoder/issues/196),
[#210](https://github.com/azrtydxb/procoder/issues/210)) The skill file now
versions like an ADR, so a change to the contract is a thing you can point
at. And `docs/influences.md` records the BMad Method alongside procoder's
other influences — it is prior art worth naming, not a procoder feature.

**Added — merge-conflict discipline, written down.**
([#216](https://github.com/azrtydxb/procoder/pull/216),
[#193](https://github.com/azrtydxb/procoder/issues/193)) Resolving a
conflict by keeping one side is how a feature disappears between two green
test runs — it happened here, during 3.2.0, and survived because the lost
code had a comment rather than a test. The rule now: read the resolved file
before committing it, and confirm each side's contribution is still there.

## 3.2.1 — 2026-08-26

_The release controller works out who is owed a credit, and CI enforces it,
so getting attribution wrong stops depending on whoever remembers to look._

**Fixed — a contributor who should be credited and is not now blocks the
release.** ([#213](https://github.com/azrtydxb/procoder/pull/213))
`procoder release` already checked that a credited handle had opened
something its paragraph cites. That is the loud half — a wrong credit is
visible, and it caught one while 3.2.0 was being assembled. The quiet half
was nobody's job: whether somebody who _should_ be credited is missing. The
person a credit is taken from does not complain, and nothing else was going
to notice.

The rule is mechanical. A cited issue owes its author a credit. A cited
pull request owes its author a credit. One person who did both is owed one
credit, not two. A reporter and a different fixer are both owed, because
crediting only the pull request quietly erases whoever found the problem.
The finding hands over the line to paste rather than reporting a fault: a
check that says "this is wrong" and leaves you to work out the right answer
is one you satisfy by deleting the credit.

Whose release notes these are comes from `[release] maintainers` in
`.procoder/config.toml`, not from whoever is running the command. That is
not a preference. `gh api user` answers only where a person is logged in —
in CI the token is an app installation token with no user behind it and
returns 403, which made the check unrunnable in the one place it most
needed to run.

**Added — CI enforces the credit rule.**
([#213](https://github.com/azrtydxb/procoder/pull/213)) `procoder release
--credits` runs the two contributor checks and nothing else, and the gate
job runs it on every commit. Until now the rule ran only when somebody
typed `procoder release` on their own machine, which means it ran when they
remembered — and "remember to check" is exactly what the rule was written
to replace.

**Fixed — an unrecognised configuration key says which of the two causes it
is.** ([#213](https://github.com/azrtydxb/procoder/pull/213)) An unknown
key still blocks; a setting that does nothing while its writer believes it
is in force is the failure that check exists to prevent. But the finding
named one cause and the reader usually had the other. A typo is yours to
fix. A key added in a later release is not — you spelled it correctly, your
build is older, and no edit to the file will help. The finding now names
the running build and both routes, because an instruction nobody can follow
is how `--no-verify` becomes muscle memory.

## 3.2.0 — 2026-08-26

_procoder stops applying its own conventions to repositories that never
adopted it, and reclaims the gigabyte that updating in place left behind._

**Fixed — the attribution finding says which commit carries the trailer.**
([#187](https://github.com/azrtydxb/procoder/pull/187)) The gate refused
every commit with "commit message carries an AI-attribution line", against
a message that plainly did not carry one. The check runs over every
UNPUSHED commit, not over the message being written, so one trailer three
commits back blocked every commit after it — and the wording made it read
as being about the commit in your hand. It could not be argued with, and
`git commit --amend` amends the wrong commit, so `--no-verify` was the only
way through. The finding now names the commit, quotes its subject, and
gives the rebase that works.

**Added — specs are checked for truth, not only for completeness.**
([#187](https://github.com/azrtydxb/procoder/pull/187)) Every controller in
the chain was structural, and a spec could be complete and wrong: `spec
check` now refuses a draft that cites a symbol, path or command which does
not exist, and one whose acceptance criteria name no way to observe them.
A criterion whose observable has a prerequisite — the documentation domain
needs a built index — must name it, because without it the criterion passes
whatever the code does. A promise that names a domain must cite where that
domain lives, which does not verify the claim but does put the author in
the file. And every criterion must say what would make it fail — the
mutation discipline procoder already expects of a test, applied to the
promise: you cannot state the falsifier without constructing the case that
separates pass from fail, and when you cannot, that is the answer.

Measured against the spec whose deviations motivated it, all five are now
reported. Drafts are refused, completed specs get notes: the
point is to catch a deviation before the sprint opens, not to retrofit a
rule onto an archive nobody will rewrite.

**Fixed — a test whose verdict depended on reaching GitHub.**
([#187](https://github.com/azrtydxb/procoder/pull/187)) `kubeconform`
fetches its schemas over the network, so the Kubernetes manifest test could
fail under a parallel run because a download was rate-limited rather than
because anything was wrong. It is stubbed now, the way the gate tests
already stub their scanners, with the real-binary check kept as an opt-in
so a change in the tool's output is still something the suite can find out.

**Fixed — the gate no longer answers for code procoder was never given.**
([#187](https://github.com/azrtydxb/procoder/pull/187)) A contributor
cloned an upstream project, changed two files, and was handed nineteen
findings. Seventeen were procoder's own conventions applied to a project
that had never heard of it: its `AGENTS.md` was reported as "drifted" from
templates procoder never wrote it from, its missing pull-request template
was a finding, its formatter is Biome and procoder disagreed with it. The
eighteenth was a constant whose name ends in `_STORE_KEY`, on line 4,423 of
a file whose change sat 2,500 lines away. The project's own gate was green
throughout, and the only way to commit was `--no-verify`.

That is the real cost. The escape hatch is all-or-nothing, so a gate that
is wrong seventeen times out of nineteen teaches you to switch off the two
that were right.

procoder now decides, from the repository in front of it and never from
your machine, whether that repository has adopted it — a `.procoder/`
directory, or an `AGENTS.md` that names procoder. If it has, everything
runs exactly as before; nothing an adopting repository was told has
changed. If it has not, only the checks that are true anywhere run: a
credential, an oversized blob, a conflict marker, a junk file, and an
AI-attribution trailer nobody wrote.

In somebody else's repository the checks that read file content see only
the lines your commit wrote. A secret four thousand lines from your diff is
not yours to answer for. Checks about a file's existence — oversized,
junk — do not narrow, because a file your commit introduces is your
commit's, all of it.

Formatting is scoped too, which is the least obvious part of this. gofmt
looks universal and is not: the repository may run gofumpt, or Biome, or
nothing, and rewriting somebody's files to procoder's taste is exactly the
overreach that was reported.

Every run says which mode it was in, and the reduced gate reports how many
files went unchecked rather than "0 clean" over files nothing looked at. A
quieter gate that does not say it is quieter is indistinguishable from a
clean one. Force either mode with `[gate] scope` in
`.procoder/config.toml`, or `PROCODER_GATE_SCOPE` for a fork that cannot
carry configuration. The reasoning is in ADR 0005.

**Added — `procoder prune` reclaims the plugin cache.**
([#187](https://github.com/azrtydxb/procoder/pull/187)) `claude plugin
update` writes each new version into its own directory and removes none of
the previous ones, and `claude plugin prune` does not cover it. On the
maintainer's machine that had reached 55 versions and 1.11 GB, one of them
in use.

`procoder prune` reports what could go and removes nothing; `procoder prune
--apply` removes it. Typing the command to find out what it does must not
cost you a gigabyte. The version in use is protected twice and
independently — it is named in `installed_plugins.json`, and it is the
directory the running binary executes from — because either check alone
leaves a way to delete what you are running. The window keeps the active
version and one previous, so repointing `installed_plugins.json` at the
directory below is still a rollback.

procoder refuses rather than guesses: a record that is absent, unparseable
or does not list procoder means the version in use is unknown, and unknown
is never a licence to delete. Nothing calls this from a hook.

**Fixed — a resumed session gets a pointer, not the principles again.**
([#184](https://github.com/azrtydxb/procoder/pull/184)) The full 6,870-byte
principles payload re-fired on every resume and compaction. It is now 144
bytes on resume — a reminder that they are already active and how to read
them in full. A session whose origin cannot be determined still gets the
whole text, because an unknown origin is not a resumed one.

**Fixed — a workspace member is covered by its root lockfile.**
([#178](https://github.com/azrtydxb/procoder/pull/178)) In a pnpm, npm or
yarn workspace the lockfile lives at the root and resolves every member,
which is where the tooling puts it and the only place it exists. procoder
reported each member as unscannable and blocked the commit, telling you to
generate a lockfile the workspace deliberately does not have. Contributed
by [@ToberoCat](https://github.com/ToberoCat).

**Added — a decision the agent cannot make now reaches you, and
survives.** ([#187](https://github.com/azrtydxb/procoder/pull/187)) The
principles said questions are not the agent's to answer and `procoder ask`
collected them, but both were true only for questions arising from a
finding. A decision — commit or hold, merge now or after, which of two
approaches — came from the work instead, and nothing collected it,
recorded it, or noticed when it went unasked.

The agent now writes those to `.procoder/ask/decisions.md` and `procoder
ask` collects them alongside the rest, so an outstanding decision is
visible to the gate and an answered one survives the session that answered
it. procoder reads that file and never writes it. The principles gained the
rule that was missing: a decision is not yours, STOP means asking before
continuing rather than mentioning at the end, and use the host's structured
question tool where there is one.

## 3.1.0 — 2026-08-26

_The per-platform binaries are no longer committed. CI builds them at the
tag and the launcher fetches the one your machine needs, once._

**Changed — procoder's binaries are built by CI and fetched on first
use.** ([#176](https://github.com/azrtydxb/procoder/pull/176)) They used to
be committed: five binaries, 39MB, rewritten at every release and
permanent in history — which is most of why `.git` had reached 690MB. They
were also built by hand, and that failed twice in a single day: 3.0.0 was
tagged carrying 2.0.1's binaries with every manifest, the gate and the
suite green, and the corrected build then failed the reproducibility check
because it predated its own source.

Now the release job builds all five from the tagged source and publishes
them with checksums it generated, and `hooks/launcher.sh` fetches the one
binary for your platform on first use, verifies it against those
checksums, caches it beside the plugin and execs it. Every run after that
execs the cached binary directly, with no network — the path that fires on
every session start, every Bash call and every write is unchanged.

Nothing is built locally any more, by anyone.

**Changed — a first run now needs the network, and fails gracefully when
it cannot.** ([#176](https://github.com/azrtydxb/procoder/pull/176)) This
replaces a stated property: the launcher's own comment used to read
"marketplace install, no runtime, no network". A hook that cannot fetch
warns on stderr, writes nothing to stdout and exits 0, so the session
stays usable and you can see the gate is not running. Every other
invocation refuses and exits non-zero, because a launcher that exited 0
having run nothing would be a silent green underneath every check in the
tool.

Set `PROCODER_BIN` to an absolute path to use your own binary and skip all
of this — it is your file, and nothing about it is checked. `PROCODER_NO_FETCH`
disables the download entirely.

**Removed — the checks whose subject was a committed binary.**
([#176](https://github.com/azrtydxb/procoder/pull/176)) CI's
reproducibility job rebuilt the committed binaries and compared digests;
`procoder release` refused to tag when the shipped binary reported a
different version; a test compared the committed checksums to the
committed files. All three answered questions about a thing that no longer
exists, and all three are gone with their tests rather than left asserting
a world that has ended.

What that spends is written down in ADR 0004 rather than discovered later:
with nothing committed there is nothing to rebuild against, so nobody
outside CI can independently confirm the published bytes. That is the
trust already extended to every other CI-published artifact.

## 3.0.0 — 2026-08-24

_Procoder was run against a repository it had never seen, and against a
real one on GitHub, and twenty-four things it says or does turned out to
be wrong. Four of them were checks reporting clean because they never
looked, and one of those was the check that exists to catch the others._

**Fixed — `procoder check --staged` exited 0, having checked nothing.**
([#171](https://github.com/azrtydxb/procoder/pull/171)) The gate takes
paths, and every arm read its flags positionally, so a flag nobody
implemented was not refused — it was handed to the formatter as a
filename. No formatter covers a file called `--staged`, so it was counted
out of scope, nothing else was looked at, and the gate reported clean.
`ci --run`, `security --deeep` and `format --write` did the same. A flag
a command does not implement is now a usage error that names the flags it
does take, which is the rule `procoder version` alone had from the start.

**Fixed — `procoder security` was weaker than the gate it previews.**
([#171](https://github.com/azrtydxb/procoder/pull/171)) A hardcoded AWS
key in a changed file: the commit gate blocked it, and the command a
person runs to check their work first reported "0 finding(s) (0
blocking)". The gate runs the secret scanner and the SAST pass over the
changed files; the command ran only the secret scanner. The two genuinely
differ — gitleaks does not fire on a bare `const K = "AKIA…"` in Go, and
semgrep does — so this was reachable rather than theoretical. The command
now asks what the gate asks.

**Fixed — one unscannable manifest silenced every dependency scan.**
([#171](https://github.com/azrtydxb/procoder/pull/171)) `pyproject.toml`
declares version ranges rather than pinned versions, so osv-scanner has no
extractor for it: handed one, it exits 127 and emits no JSON at all. Every
other manifest in the same invocation went down with it, so a repository
with seven known vulnerabilities in its `go.mod` and a `pyproject.toml`
beside it was told only that the output was unreadable. Python now gets
the honest gap a bare `package.json` already got, and the scan reaches the
manifests osv can read.

**Fixed — three install instructions that could not work.**
([#171](https://github.com/azrtydxb/procoder/pull/171)) `brew install
rubocop` was the first thing procoder printed to anyone missing rubocop,
and there has never been such a formula — it ships as a gem, and brew
being on PATH meant the gem candidate was never reached. `composer global
require phpstan/phpstan` installs into `~/.composer/vendor/bin` and puts
nothing on PATH, so following procoder's advice exactly left procoder
still reporting the tool missing. And the release controller pronounced an
already-tagged, already-published version ready, printing a `git tag`
command that answers "fatal: tag already exists". Every other package name
in the tool table was checked against its registry; the rest are real.

**Fixed — three shipped commands `procoder help` never mentioned.**
([#171](https://github.com/azrtydxb/procoder/pull/171)) `config` prints
every setting procoder is running under and where it came from, `review`
is the judgment half of the gate, and `analyze` is the first stage of the
whole chain. All three worked; none appeared in the usage text. They
slipped through because the test that existed pinned the usage text
against a canonical list, and a command absent from both agrees with
itself. `docs --ack` — the command the gate's own blocking message tells
an agent to run — was documented nowhere either.

**Fixed — an empty change set was reported as a clean one.**
([#171](https://github.com/azrtydxb/procoder/pull/171)) `procoder lint`
and `procoder security` on an untouched tree printed "0 finding(s) (0
blocking)", the same sentence they print over a diff they actually read.
`procoder check` had always said "no changed files" instead. Also fixed:
a failed tool's reason was the first line of its stderr, which for a
scanner is a progress log ("dependencies were NOT checked: Starting
filesystem walk for root: /") rather than the reason; procoder writes
three artifacts into a repository it governs and advised ignoring one of
them; and a usage error printed all 159 lines of the usage text.

**Changed — the README version check is the repository's to configure.**
([#171](https://github.com/azrtydxb/procoder/pull/171)) It read
`.claude-plugin/plugin.json` then `package.json` and nothing else, so a Go
project carrying a `package.json` for its tooling was blocked because its
README did not mention the npm package's version. True, and
unconfigurable. `## Version source` in `.procoder/docs/RULES.md` now names
the file, and `none` switches the check off. The defaults do not move.

**Fixed — a project with no dependencies was blocked for having any list
at all.** ([#171](https://github.com/azrtydxb/procoder/pull/171)) The
pattern behind the new Python dependency gap matched any non-empty list
assignment, so a `pyproject.toml` with `dependencies = []` and
`keywords = ["cli"]` — no dependencies whatsoever — was told its
dependencies had not been checked, and blocked. A blocking false positive
about a file the reader has done nothing wrong with is worse than the gap
it was written to close. Found by GitHub Copilot's review of the pull
request that introduced it.

**Fixed — `procoder security` reported "no changed files" when it could
not find out.** ([#171](https://github.com/azrtydxb/procoder/pull/171))
The error from listing the changed files was discarded, so outside a git
repository, or whenever git could not run, the command answered as though
it had looked and exited 0. It now says NOT checked, names git's error,
and exits 1. Also found by Copilot's review, along with an install retry
line that credited every failure to the first package manager tried.

**Added — `procoder copilot-leak` reads the review, not only the issues.**
([#171](https://github.com/azrtydxb/procoder/pull/171)) The command exists
to collect what Copilot's auto-review caught that the gates did not, and
it queried GitHub issues. Copilot's auto-review does not open issues on
most repositories — it comments inline on the pull request — so it
reported "no findings" while four real defects sat in a review of the very
branch extending it. It now reads pull request review comments too, and
reports NOT checked when either source fails, because a count from one
place while the other went unread is the silence it was built to break.

**Added — the rules a change is held to, written where the change is
made.** ([#171](https://github.com/azrtydxb/procoder/pull/171))
`.github/instructions/` carries four path-scoped files — Go, tests,
workflows, prose — that Copilot reads in the editor and in its code
review, alongside the repository-wide instructions generated from
`AGENTS.md`. Every rule in them names the defect that produced it.

**Changed — exit codes move and blocking checks are added, which is what
makes this a major release.**
([#171](https://github.com/azrtydxb/procoder/pull/171)) Per ADR 0003 each
of these alone would be enough. A flag a command does not implement
now exits 2 rather than being silently ignored, and `procoder sprint
status` with no active sprint exits 0 rather than 1 — asking which sprint
is running is a question, and "none" is its answer, the way `todo list`
answers "no tasks". Pulling, carrying and closing with no sprint open are
refusals and keep their 1. Three checks can newly stop a commit or a
release: the SAST leg inside `procoder security`, a `pyproject.toml` with
dependencies and no lock file, and a release version that is already
tagged.

## 2.0.1 — 2026-08-24

_The changelog's own rules are checked now, including the one about
crediting the right person — which was written down after it was broken,
and left as prose until now._

**Fixed — four of the six changelog rules were written down and never
checked.** ([#168](https://github.com/azrtydxb/procoder/pull/168)) The
suite enforced the italic summary and the headline kind. It did not
enforce that an entry links the pull request that shipped it, or that a
contributor is named AND linked — both added in
[#157](https://github.com/azrtydxb/procoder/pull/157), after a merged
commit credited the wrong person, and both left as prose. Prose was what
had just failed.

Now checked on the newest entry, which is the one the release job
publishes verbatim: an unlinked claim is one a reader on the release
page cannot follow, and a bare handle inside a Markdown file renders as
text and leads nowhere, so a credit written that way is not a credit.

**Fixed — a credited contributor is verified against GitHub before a
release is called ready.**
([#169](https://github.com/azrtydxb/procoder/pull/169)) `procoder
release` now asks who actually opened each issue and pull request an
entry cites, and refuses when a credited handle opened none of what its
paragraph links. The message names who did open them, so the correction
is in the error rather than another lookup away.

This is the only check in the controller that reaches the network, and
it can, because the tag it prepares is published by a job that talks to
the same API. GitHub not answering is reported as NOT verified and
blocks — a credit nothing checked is exactly how the wrong name ships.

## 2.0.0 — 2026-08-24

_Procoder gains an opinion about a change, a phase before the spec, and a
way to hand planning to BMad Method entirely — and a spec can no longer
promise more than it tests._

**Changed — a spec that promises more than it tests is no longer
complete.** ([#166](https://github.com/azrtydxb/procoder/pull/166)) This
is the breaking change, and the reason for the major version: `procoder
spec check` now exits 1 on specs that passed yesterday, and `procoder
backlog seed` refuses them.

Every `## In scope` bullet carries an id, and at least one acceptance
criterion must cite it:

```
- [S-1] the thing being built
- [ ] [S-1] the observable behaviour that proves it
```

Scope no criterion cites is a gap, a gap makes the spec incomplete, and
seed refuses an incomplete spec. Work cannot be seeded from a spec that
promises more than it tests.

It exists because that is exactly what happened here. A spec put five
things in scope, wrote criteria for three, and passed every check
Procoder had. Seed writes one story per criterion, so the two untested
promises became no stories, got no sprint, and were never missed — the
epic closed at "fourteen of fourteen" with two of five features never
built. The verdict was true and useless: fourteen of fourteen
_criteria_, not fourteen of fourteen _scope_.

To upgrade, label the bullets in each spec's In scope section and cite
those ids from the criteria that cover them. A spec whose bullets carry
no ids is reported NOT CHECKED rather than assumed covered — coverage is
declared, never guessed, because matching prose to prose would fail open
and a bullet wrongly judged covered is the silence this prevents.

**Added — `procoder review`, five stances over a change.**
([#163](https://github.com/azrtydxb/procoder/pull/163)) Every other check
Procoder runs is mechanical and has one right answer; this one asks the
questions that do not. Adversarial (assume it is wrong and find where),
edge-case (enumerate paths, report only the unhandled), verification-gap
(would verification actually fail if this broke?), structure, and prose.
`--perspectives` reads with a different set — analyst, architect,
implementer, reviewer — meant for a spec or a plan, where the
architectural question is still cheap to answer.

The binary judges nothing: it prints the lens and the scope, the agent
judges, and nothing on disk changes. Any lens or perspective is
replaceable from `.procoder/review/`; one that cannot be read blocks and
prints nothing at all, because a review under your own name running
Procoder's words is worse than no review.

**Added — `procoder analyze`, the phase before the spec.**
([#165](https://github.com/azrtydxb/procoder/pull/165)) `spec check`
judges whether a document is complete, never whether the idea in it is
good — it will pass a thoroughly filled-in specification for the wrong
feature. An analysis document asks what is actually undecided, what is
known and how, what is not, what the options cost, and which one and why.
Never required. `analyze where` names every entry point in the chain and
says plainly that nothing makes you start above the one you need.

**Added — a repository can hand planning to BMad Method.**
([#165](https://github.com/azrtydxb/procoder/pull/165)) `[planning]
method = "bmad"` and Procoder reads that installation's artifacts —
sprint status, its own output folder, its version — instead of demanding
`.procoder/specs`, `plans` and `backlog`.

The setting moves planning and nothing else. The gate, the suite,
formatting, the release controller, the debt ledger, security and docs
reach the same verdict about the same code either way, asserted by a test
rather than promised. Setting it with no such installation present is a
blocking finding naming both, never a silent fall back to Procoder's own
chain.

**Fixed — three defects in the artifact reader, two of them silent.**
([#165](https://github.com/azrtydxb/procoder/pull/165)) An inline `#`
comment was read as part of the value in both TOML and YAML, so a
repository mid-sprint would have been told it had planned nothing, and a
finished story would have counted as open _and_ been reported as an
unknown status.

## 1.5.0 — 2026-08-24

_Two bugs that made procoder unusable for a whole language are fixed,
along with the two checks that only run when nobody typed them._

**Fixed — a language with no linter blocked every commit, regardless of
[lint] policy.** ([#152](https://github.com/azrtydxb/procoder/pull/152))
C# and Dart have no linter procoder ships yet, and that finding blocked
unconditionally — a repository writing either language could not land
any commit that touched it, whatever `[lint] policy` said. The only
escape a real repository found was turning off the commit gate entirely.
It now honors policy like every other lint finding, because there is no
`procoder init` that fixes "the linter does not exist." Reported by
[@codixio](https://github.com/codixio) in
[#150](https://github.com/azrtydxb/procoder/issues/150).

**Fixed — csharpier's own cache made a correctly formatted file fail the
gate.** ([#151](https://github.com/azrtydxb/procoder/pull/151))
csharpier 1.x prints nothing for a file it has already formatted once,
and procoder read that silence as the tool not having answered — so a
clean `.cs` file failed as UNCHECKED on every commit after the first
that touched it, because the cache fills on the very run `procoder
format` suggests. Runs with `--no-cache` now. Reported by
[@codixio](https://github.com/codixio) in
[#149](https://github.com/azrtydxb/procoder/issues/149).

**Fixed — OKF bundle links reported as broken when they were not.**
([#148](https://github.com/azrtydxb/procoder/pull/148)) An
OKF bundle resolves an absolute link
from the bundle's own root, not the repository's — `/log.md` inside
`.okf/` means `.okf/log.md`. procoder resolved every absolute link from
the repository root, so a conformant bundle reported dozens of false
"broken reference" findings. Contributed by
[@Acroaticum](https://github.com/Acroaticum).

**Fixed — two findings that should always block could report instead.**
([#159](https://github.com/azrtydxb/procoder/pull/159)) A missing
infrastructure tool and a GitHub Pages check that could not reach `gh`
both said "NOT checked" and reported it as information rather than
blocking it, because Go's slice-literal shape they were written in was
invisible to the audit meant to catch exactly this. Found by widening
that audit to see the shape it had been blind to — the same class of bug
this project's whole "no silent green" rule exists to prevent, this time
hiding from the check that enforces the rule itself.

**Fixed — the state-of-play report could run past its own three-second
budget.** ([#144](https://github.com/azrtydxb/procoder/pull/144),
[#158](https://github.com/azrtydxb/procoder/pull/158)) A slow or hung
git left the branch line waiting past the deadline the rest of the
report honors, with nothing to say why. Every git call the report makes
now shares its budget, and answers "unknown — \<reason\>" rather than
running past the wall.

**Added — a reference page for when each check runs.**
([#141](https://github.com/azrtydxb/procoder/pull/141)) One arrow, one
station per lifecycle event — session start, every file written, the
commit gate, CI — with every check placed where it actually fires,
published at
[procoder.azrty.com/lifecycle](https://procoder.azrty.com/lifecycle/).

**Changed — the pinned CI binaries download once per pin, not once per
run.** ([#147](https://github.com/azrtydxb/procoder/pull/147))

## 1.4.0 — 2026-08-22

_The checks that waited to be asked now run themselves — at the commit
for the change, in CI for the tree._

**Added — the commit gate answers the questions you used to have to ask
it.** ([#130](https://github.com/azrtydxb/procoder/pull/130),
[#133](https://github.com/azrtydxb/procoder/pull/133),
[#134](https://github.com/azrtydxb/procoder/pull/134),
[#135](https://github.com/azrtydxb/procoder/pull/135),
[#136](https://github.com/azrtydxb/procoder/pull/136),
[#138](https://github.com/azrtydxb/procoder/pull/138)) Static analysis
runs over the files a commit carries and blocks at the severity the
repository set. A function past the complexity limit is named as it is
written, not months later. A commit that touches a dependency manifest is
checked against known vulnerabilities. The test suite runs for the
packages the commit touches. Rule files that have drifted from AGENTS.md
block — which `docs/commands.md` had claimed for some time without it
being true. And a `debt:` marker with no revisit condition is called out
while the reason for the shortcut is still in your head.

These were all real commands before. They ran only when somebody
remembered to type them, which meant they protected the people who
already knew they existed.

**Added — CI answers about the tree.**
([#132](https://github.com/azrtydxb/procoder/pull/132)) `maintain`,
`debt` and `deps` run over the whole repository in CI and fail the job on
what they find. The gate answers about the change and CI answers about
the tree; asking either to do both makes it worse at the one it had.

**Fixed — `procoder debt` could not fail.**
([#138](https://github.com/azrtydxb/procoder/pull/138)) It printed the
count of markers that silently rot and exited 0 regardless, so any CI
step running it could only ever pass. It exits 1 on rot now.

**Fixed — `procoder maintain` reported clean when it had not run.**
([#132](https://github.com/azrtydxb/procoder/pull/132)) A missing tool
produced no findings, and no findings read as nothing wrong. It now
exits non-zero and says which check could not run.

**Added — the state of play names what the gate did not run.**
([#137](https://github.com/azrtydxb/procoder/pull/137)) The gate narrows
to the runners it can scope, so a Rust, PHP, Java or JavaScript suite is
CI's. That was a reasonable trade and an invisible one: a JavaScript
commit passed a green gate having never run its suite. `procoder status`
says so now, and says nothing when there is nothing to say.

**Changed — no budget on the heavy checks.** A slow scan finishes and
reports what it found rather than being cut off and reported anyway. A
verdict that depends on how fast your laptop is, is not a verdict about
your code. The ceiling that remains is a hung-process net: when it fires
it says the check was NOT run, and blocks.

**Fixed — closing a sprint no longer adds a second retro section.**
([#139](https://github.com/azrtydxb/procoder/pull/139)) The template
ships one and the closer appended another, so a retro you had already
written ended up above an empty scaffold — and the check that holds the
next sprint open until it is filled had two sections to choose between.

## 1.3.0 — 2026-08-21

_The decisions Procoder used to make on your behalf are yours to make,
and it says which ones you changed._

**Added — a repository chooses its own tools, thresholds and templates.**
([#123](https://github.com/azrtydxb/procoder/pull/123),
[#124](https://github.com/azrtydxb/procoder/pull/124),
[#125](https://github.com/azrtydxb/procoder/pull/125)) `[tools] js =
"biome"` picks the formatter for a language. `[security] sast_blocks_at`
sets the severity that stops a commit, where the code had `ERROR` as a
literal. `.procoder/templates/<name>.md` replaces any of the nine
templates that drive the quality chain — spec, plan, ADR, todo,
milestone, epic, story, sprint, bug — which were embedded constants with
no way in. And `.procoder/lint/RULES.md` gains the `## checks` list the
docs and security domains already had, replacing Procoder's curated
clang-tidy families.

A repository names a tool; it does not name a binary and an argv.
Procoder owns the invocation, which is what keeps the print-don't-write
contract a guarantee rather than a hope — a tool reaches the menu by
being able to emit formatted source on stdout, tested for each candidate
rather than assumed. Laravel Pint, phpcbf and php-cs-fixer are absent for
that reason and no other.

**Added — `procoder config` says what is in force and where it came
from.** ([#123](https://github.com/azrtydxb/procoder/pull/123)) Every
effective setting, its value, and its source — `default`, or the file and
line. A setting weaker than its default is marked. Configurability
without visibility is worse than none: a person reading an unfamiliar
repository has to be able to ask which of Procoder's defaults still
apply.

**Changed — a setting that weakens a default prints on every gate
run.** ([#123](https://github.com/azrtydxb/procoder/pull/123)) Naming
what was relaxed and what it costs. Strengthening prints nothing. This is
the rule the gate already lives by, applied to configuration: a green
verdict must not be able to mean "the config was loosened" without saying
so.

**Fixed — a setting Procoder cannot apply no longer passes in silence.**
([#123](https://github.com/azrtydxb/procoder/pull/123)) The loader fell
through for any key it did not recognise, so `polcy = "block"` was
accepted, did nothing, and said nothing — the writer believed their
policy was set. Unknown keys, malformed lines and values of the wrong
kind are each reported with their line number, and they block.

**Fixed — an emptied Markdown file blocks.**
([#120](https://github.com/azrtydxb/procoder/pull/120)) `procoder format`
prints one header line for a file that is already formatted, so a
pipeline that strips the header and writes the rest empties it on the
success path. That destroyed a 551-line documentation page in this
repository, and nothing noticed: the documentation obligation asks
whether a doc CHANGED, and emptying one is a change, so the destruction
satisfied the check meant to prevent it.

## 1.2.0 — 2026-08-21

_A green gate now means the code was checked rather than that the machine
was empty, and PHP is a language Procoder speaks._

**Changed — a check that could not run stops the commit.**
([#114](https://github.com/azrtydxb/procoder/pull/114)) A missing linter
used to print `NOT checked` as information and let the gate exit 0, so an
empty machine was indistinguishable from clean code. Domain 1 had always
blocked on a missing gitleaks; every domain does now — lint, formatting,
infrastructure, docs, workflows. A formatter that wants a config it cannot
find is unchecked rather than out of scope, because out of scope passes.

This will fail repositories that were passing on a toolchain that was
never there. That is the point, and every refusal names the tool and the
command that installs it: `procoder init` installs what `procoder doctor`
lists. A file type Procoder does not claim — a text file, an image — is
still out of scope, still silent, still green.

`[lint] policy` is unchanged: it governs whether a linter's findings block
a commit. Whether the linter ran at all was never a matter of policy.

**Added — PHP.**
([#112](https://github.com/azrtydxb/procoder/pull/112)) `.php` files are
formatted through prettier's PHP plugin, linted by whichever of phpstan
and phpcs the project configured, and phpunit is detected and run by
`procoder test` with its counts and `--name` filtering. A project that
configured no linter gets Procoder's phpstan baseline rather than a
syntax check; a project that configured one keeps it. Coverage reports
NOT measured rather than a number nobody measured.

**Added — Procoder brings a linter when the project brought none.**
([#113](https://github.com/azrtydxb/procoder/pull/113)) TypeScript with
no eslint config is linted against typescript-eslint's recommended set,
where it used to be declared out of scope because a parser would have had
to be installed — the most common TypeScript setup there is got no
linting and a green gate. C and C++ reach clang-tidy, and clang-format no
longer needs a project style file to format anything. Every baseline is
written to a temp file or named on the command line; nothing is written
into your repository, and a project config always wins.

**Fixed — an emptied documentation file no longer passes the gate.**
([#120](https://github.com/azrtydxb/procoder/pull/120)) `procoder format`
prints one header line for a file that is already formatted and nothing
after it, so a pipeline that strips the header and writes the rest empties
the file on the success path. That destroyed `docs/commands.md` during
this release's own development, and nothing noticed: the documentation
obligation asks whether a doc CHANGED, and emptying one is a change, so
the destruction satisfied the check meant to prevent it. An emptied
Markdown file is a blocking finding now, naming the command that restores
it. PHP also reached the language list in the README, where it was
missing.

**Fixed — five file types were formatted and never linted.**
([#114](https://github.com/azrtydxb/procoder/pull/114)) `.mts` and `.cts`
went unlinted while `.mjs` and `.cjs` were checked, and `.pyi` was
formatted and never read by ruff. C# and Dart have no linter yet and now
say so rather than passing quietly.

## 1.1.1 — 2026-08-21

_The pi adapter installs at all, generated ids stop welding words
together, and a spec's open questions reach a human instead of being
guessed._

**Fixed — pi could not install procoder at all.**
([#105](https://github.com/azrtydxb/procoder/issues/105)) The pi
extension was a CommonJS module and pi validates the export shape at
install time, so `omp plugin install` failed outright while the
portability docs listed pi as a supported host. Nothing here caught it:
Node hands a CommonJS export back as `default` on import, so every load
test passed and only pi's own validator could see the difference. The
adapter is an ES module now, and the check reads adapter source rather
than importing it. Reported and diagnosed by
[@striderZA](https://github.com/striderZA), who supplied the fix.

**Fixed — generated ids welded words together.**
([#103](https://github.com/azrtydxb/procoder/issues/103)) Dropping a dot
left `answers.md` filed as "answersmd", so the story about a file could
not be found by the name of the file, and `v1.2.3` and `v12.3` collapsed
onto one id — a collision that surfaced as `backlog story` refusing with
nothing on screen to say why. Punctuation separates now, accented
letters fold to their letters (`café` → `cafe`) instead of vanishing,
and two criteria that still land on the same id get two stories rather
than one written over the other. Names already on disk are untouched.

**Added — `procoder ask` puts the open questions to a human.**
([#104](https://github.com/azrtydxb/procoder/pull/104)) A spec's
undecided questions and the gate's own findings are collected into
`.procoder/ask/QA.md`, answered in `answers.md`, and an answered
question stops blocking the spec controller. A flagged secret's value
never travels — not into a question, not into the terminal, not into a
hook payload.

## 1.1.0 — 2026-08-21

**procoder now knows when it is out of date, and verifies what it
installs.** `procoder version --check` asks GitHub what the newest
release is; `procoder self-upgrade` installs it, but only after an
explicit yes on a terminal, only after the download matches the
`SHA256SUMS` the release publishes, and never over a binary a package
manager owns — that one is refused with the manager's own upgrade
command. A check that could not run reports NOT known and exits 2,
because an unanswered check has never meant "you are current". At a
session start the check runs alongside the payload, capped at one
second, so a slow network cannot hold a session open.

**Windows works.** Claude Code runs hooks through Git Bash, where
`uname -s` answers `MINGW64_NT-…`; the launcher recognised only Darwin
and Linux and exited 1, so every hook and every slash command failed on
a fresh install while `procoder.exe` sat there unreachable. Fixed by
this project's first outside contributor, and now exercised on a real
Windows runner in CI.

**`procoder copilot-leak`** collects what GitHub Copilot's auto-review
found, strips every trace of your code from it, and — only if you say
yes — files it as issues and records it as unlearned until somebody
writes the adaptation that closes the class.

**The backlog stops overstating itself.** The board names the branch it
read and counts the open stories the default branch holds that this
checkout cannot see. A spec's fingerprint tracks its acceptance
criteria rather than its prose, so rewrapping a paragraph no longer
reads as drift. A spec with undecided questions is no longer reported
COMPLETE whatever those questions are called, and the dead-code tier
separates surface referenced only by its own tests from live code.

**The gate is faster and no longer drifts.** Every tool it installs is
pinned in `.github/tool-versions.env` and cached, and the three that
publish binaries are downloaded rather than compiled: 5m47s to 3m06s.
Tool versions can no longer change the gate's verdict between two runs
of the same commit, and the Go toolchain is pinned for the same reason.

**`dist/` is reproducible.** `scripts/build-dist.sh` records the
procedure that used to live in a shell history, stamps the version from
the manifest, and builds with `-buildvcs=false` so two builds of one
tree produce identical bytes. CI rebuilds the committed binaries and
compares digests, so a stale `dist/` fails before a tag rather than
after one.

## 1.0.2 — 2026-08-20

**Kilo Code is a first-class host, and the gate goes with it.** Kilo's
CLI is an OpenCode fork and its plugin API is the same one, so the JS
shim that already served OpenCode now serves both, byte for byte. It
gained the piece that matters: the commit gate runs at the tool
boundary and refuses a commit with blocking findings — in Kilo that
covers the VS Code extension, not only a terminal, which makes it the
first editor where "done" has to survive the gate. OpenCode gets the
same enforcement in the same change. Every host reaches one
implementation, so there is one verdict wherever you work, and a
machine without the binary is told the gate did NOT run rather than
having its commits blocked.

Three more tiers came with it: Kilo's current rules path, a command
set, and `skills/procoder/SKILL.md` — the contract under the Agent
Skills envelope, read by Kilo and by any host that scans a skills
directory.

**A correct link stopped reading as a broken one.** The reference
check reproduced mkdocs' slugify everywhere, including repositories
whose Markdown is only ever read on github.com, which does not collapse
runs of hyphens. Every heading containing an ampersand, an em dash, or
a colon had a working anchor the gate called broken — blocking, with no
way out but rewording public headings. A heading is now credited with
the anchor each renderer generates. Underscores inside code spans
survive too, so a heading naming a snake_case symbol is reachable.

**The documentation acknowledgment works from wherever git reads the
message.** `docs: none — <reason>` only cleared its obligation when the
message arrived through `-m`. With `-F <file>`, a heredoc, or an editor
commit the line was invisible, so the finding told you to write exactly
the line it then ignored. All the forms the gate can see are read now,
and where no message can reach the check it says so instead of naming a
remedy that cannot work.

**The documentation obligation asks about your branch, not your
uncommitted slice.** Writing the doc in one commit and the code in the
next demanded an acknowledgment for work that was already documented.
The question now spans the commits your branch carries. The public
surface is also compared like with like — the previous revision read by
the same parser as the current one — so a capitalised local constant in
JavaScript no longer reports itself removed on every run.

**An epic that was never seeded says so.** A `Spec:` line carrying
anything a fingerprint could not produce read as spec drift forever,
pointing at a comparison that had never happened.

## 1.0.1 — 2026-08-20

**A tag is the release now.** Twenty-seven tags were pushed while exactly
one GitHub Release was ever created by hand, so everything that reads
"latest release" — the documentation site's own header among them — sat
on a version from weeks earlier while the site body reported the current
one.

CI publishes the Release when a `v*` tag is pushed, with the changelog
entry as the notes and the five platform binaries attached, so the
manual install no longer means cloning the repository for one file. It
runs only after the suite and the gate pass on the tagged tree: the
Release is what people download, so it ships on the same evidence as
everything else.

A missing changelog entry fails the job rather than publishing empty
notes. `procoder release` already refuses to tag without one, so its
absence at this point means something went wrong upstream, and a release
that looks finished but says nothing is worse than a red job.

## 1.0.0 — 2026-08-20

Procoder is 1.0. Not because anything new landed today, but because what
is here has been used long enough to be worth promising.

**What 1.0 means.** These are the public interface, and breaking any of
them takes a major version: command and subcommand names; exit codes (0
clean, 1 findings or refusal, 2 usage); what blocks versus what informs,
so a new blocking check cannot fail a build that passed yesterday; the
`.procoder/` formats a repository commits — config keys, the
spec/plan/todo/backlog/adr shapes, the epic `Spec:` fingerprint, the
rules files' machine-read sections; and the hook payloads and envelopes,
per host.

Deliberately not covered, because pinning them would freeze the product:
the wording of report lines (verdicts are for people; exit codes are the
contract), the default rules content every repository is meant to
override, the `internal/` Go packages, and the gitignored index format.
`.procoder/adr/0003-what-1-0-promises.md` carries the reasoning and the
alternatives that lost.

**What it is.** One Go binary, no runtime dependencies, cross-compiled
for five platforms and committed with the plugin — no npm, no network at
hook time, air-gapped installs included. Ten domains behind one gate.
The quality chain from spec to release, every link refusing rather than
advising. A universal agent layer serving twenty-odd hosts from one
`AGENTS.md`. A code index instead of grep. A lessons ledger where an
escape is not closed until the layer that missed it has been adapted.

**Where it stands.** 73.2% statement coverage from a mutation-driven
sweep; the whole-tree audit reports zero blocking findings; the deep
security scan reports zero findings; 25 lessons recorded with zero
unlearned; three debt markers, each with the condition that will bring
it back.

**One judgment reviewed for this release.** The spec fingerprint and the
tool-cache directory name both use SHA-1. Both are change detection
rather than signatures, and the fingerprint is persisted on every seeded
epic, so switching the digest would flag drift on work that has not
changed. Kept deliberately, marked at both call sites with the reason,
and now covered by the compatibility promise.

## 0.32.11 — 2026-08-20

**`procoder docs` now checks the anchor, not just the file.** A link
written as `architecture.md#contract-1-…` resolved as long as the file
existed; whether any heading generated that anchor went
unasked. mkdocs reports the mismatch at INFO, so `--strict` stays green
and a dead link ships — which is how one shipped from this repository in
0.32.9.

The check reproduces Python-Markdown's toc slug, which is what mkdocs
uses: punctuation dropped rather than turned into separators (an em dash
disappears), runs of separators collapsed, duplicate headings numbered.
Explicit ids count — attr_list's `{#custom}` and raw HTML `id="…"`. A
target that exists but cannot be read yields **no verdict**, because
answering "it has no anchors" would report every link into it as broken.

Proved against this repository: it catches the exact link that shipped,
and 218 Markdown files produce no false positive.

**And the half that cannot be mechanised is written down.** Twice in one
day a green assertion described what the code said while the rendered
page said otherwise — a table that was not a table, and an element the
DOM reported as hidden that the browser was still painting. The review
rubric and the documentation rules now both carry the same line: for
anything visual, look at it rendered, in both colour schemes, and when
the assertion and the screenshot disagree the screenshot is right.

## 0.32.10 — 2026-08-20

**The command reference's table was not a table.** The skills list
shipped in 0.32.9 with data rows and no header row, so Markdown never
made it a table — it rendered as a wall of text with literal pipes in
it, and the formatter reflowed it into a paragraph, which made it worse.
Fixed, with the header row it always needed and a command column that no
longer breaks `/procoder:agents` across two lines.

**The page filters now.** The reference is long by design — 33 skills
plus every binary subcommand — and the site search takes you to a
different page rather than narrowing the one you are on. A filter box
sits under the title: type `secrets` or `rename` or `sprint close` and
the page narrows to what matches, across the skills table and the
binary sections at once. Section headings whose contents all filtered
away hide too, so you never get an empty "Everyday commands". Escape
clears it, and with JavaScript off the page is simply the full list.

Two things found while testing it in a browser rather than by reading
the code: the `hidden` attribute loses to Material's own `display` rule
on tables, so the emptied table still painted its header; and a link
into `architecture.md` had an anchor that no heading generated.

## 0.32.9 — 2026-08-20

Documentation, after reading it as a user rather than as its author.

- **Serena joins the provenance map.** [Influences](https://procoder.azrty.com/influences/)
  covered superpowers and ponytail but not serena, whose navigation half
  Procoder took into the binary — `index find` / `refs` / `outline` /
  `callers` / `impls` / `rename`, plus `lint --types` and `.procoder/` as
  the memory that survives a lost context. Serena's symbol-level **write**
  tools are listed under what was deliberately not adopted: the binary
  computes the rename and hands over the diff. The README now says
  outright that all three plugins can be uninstalled, because running
  them alongside Procoder puts two sets of instructions in front of one
  agent.
- **The tutorial installs once.** It read as though the plugin install
  were step one of seven, with a clone and a `PATH` export after it. For
  Claude Code the marketplace install is the whole thing, and the tools
  step (`/procoder:init`) was missing entirely. The manual path — the
  binary on `PATH`, the agent contract, the git hook — is now its own
  how-to for the agents that need it.
- **The command reference leads with the commands you run.** All 33
  `/procoder:` skills in one table, then the binary underneath for
  scripting CI, other hosts, and debugging Procoder itself. The
  instructional pages call the skills; `procoder templates` is gone from
  the onboarding guide, since `/procoder:audit` writes those files.
- **Tone.** The word "honest" is out of the documentation. The behaviour
  it described stays exactly as it is — a file that could not be checked
  is never called clean — but a product that keeps calling itself honest
  is telling the reader what to think of it.

## 0.32.8 — 2026-08-20

**Documentation now has a shape, not just a checklist.** The docs domain
enforced that documentation exists — required files, badges, a README
first screen, no broken links. It said nothing about what a page should
be, so every page invented its own shape and drifted toward serving
three readers at once.

The shipped `.procoder/docs/RULES.md` now carries the
[Divio documentation system](https://docs.divio.com/documentation-system/):
four kinds of document, never mixed, the kind decided before the first
line.

| Kind         | Serves                        | Answers                 |
| ------------ | ----------------------------- | ----------------------- |
| Tutorial     | a newcomer learning           | "teach me"              |
| How-to guide | a competent user working      | "how do I X?"           |
| Reference    | someone looking a thing up    | "what are the options?" |
| Explanation  | someone wanting to understand | "why is it like this?"  |

Each has a characteristic failure, and the rules name them: a tutorial
that stops to explain trade-offs loses the learner; a how-to that
teaches from scratch wastes a reader who already knows; reference that
argues cannot be trusted to describe; explanation carrying steps rots,
because the steps then live in two places.

Alongside it, the writing rules that follow: answer first, examples over
prose about examples, real names rather than `foo`, sentences under
fifteen words, scannable structure, the searchable synonym included, and
an explicit "common pitfalls" list wherever a feature has a known
misuse. `/procoder:docs` now applies all of it when it WRITES, where
before it only checked what already existed.

Every word of this is repo-overridable (D-OVERRIDE) — replace it with
your own house style and your copy wins. None of it blocks a commit; it
is guidance the agent follows, and the reasoning is recorded in
ADR 0002 along with the mechanical enforcement that was considered and
rejected.

**The site was then rebuilt to follow it.** The nav is grouped by kind,
and every page says which kind it is in its opening sentence.

- `getting-started.md` is a tutorial: install, watch the gate refuse a
  commit over a merge conflict, fix it, watch it pass. Every shell
  command in it was executed in order in a clean repository and the
  output blocks are that run, verbatim.
- `workflow.md` became **How to ship a change** — eleven numbered steps
  and a common-pitfalls list, with no paragraph arguing for the design.
- `how-to-onboard.md` is new: the audit path for a repository Procoder
  has never governed, which used to be a step inside the tutorial.
- `quality-chain.md` became explanation only, and now admits what
  refusal costs when a controller is wrong — the section it was missing.
- `commands.md`, `configuration.md`, `domains.md` and `portability.md`
  are reference; `architecture.md` and `influences.md` are explanation.

**The diagrams were rebuilt to the brand.** The site had exactly one
diagram and three ASCII-art blocks, and the shared Mermaid theme was
still the old teal. There are now five, all Mermaid, all on the brand
palette in both light and dark: the write-hook loop on the overview, the
quality chain with its refusal loops, the three-layer architecture, the
ship-a-change sequence, and the onboarding triage order.

Colour comes from the theme and meaning comes from shape — rounded for a
start or end, rectangle for a step, diamond for something that decides.
Hard-coded fills were tried first and rejected: they cannot follow a
reader who switches to dark mode. The rules file now says so, along with
the rest of what makes a diagram worth having.

**A gate gap surfaced while writing the tutorial.** `procoder check`
blocked on the tutorial's own conflict-marker example, and every
workaround was bad: mangle the sample so a reader who copies it gets
broken text, drop the topic, or turn the check off. A document whose
subject is merge conflicts now declares it:

```
<!-- procoder:allow-conflict-markers this tutorial shows what a conflict looks like -->
```

The reason is required — a bare token exempts nothing, because that is
silencing a check rather than documenting an exception. The exemption is
file-scoped and explicit rather than "skip fenced code blocks", since a
real conflict lands inside a fence often enough that skipping fences
would be a silent miss.

## 0.32.7 — 2026-08-20

**The product is spelled Procoder**, taken from the wordmark. The logo
reads _Procoder_, so every word of text around it does too: the README,
the documentation site and its header, the brand guide, the rules every
agent reads, and the engineering principles injected at session start.
The artwork is the authority — a name is a picture people recognise
before it is a string they parse, and text is the cheaper thing to
change.

Everywhere a machine reads the name it stays `procoder`, unchanged and
unchangeable: the binary, the package, the plugin id, `.procoder/`,
every command, and every URL. That distinction is the whole rename;
nothing executable moved.

## 0.32.6 — 2026-08-20

**`procoder maintain` was silently dropping every function-length
finding.** golangci-lint keeps only the first issue per line by
default — and a long function is usually a branchy one, so funlen and
gocyclo land on the same line and funlen loses. On this repository that
meant 31 complexity findings, 0 length findings, over a dispatch
function 343 lines long. The generated config now sets
`uniq-by-line: false`, and the two linters say their different things
about the same function. Seven length findings appeared here the moment
it was fixed.

A report that quietly drops half of what it found is worse than one
that says NOT checked, because nothing tells the reader anything is
missing. Same family as the honesty rule, opposite direction.

Then the report was taken at its word:

- **One findings printer instead of four.** `ci`, `infra`, `security`
  and `lint` each carried their own copy of the same render loop —
  mark, location, message, count line, exit code — which is how three
  of them drift while the fourth gets fixed. They now share
  `printFindings`, which has tests of its own.
- **`procoder lint` and `procoder security` now print repository-relative
  paths**, as `ci` and `infra` already did. Paths outside the repository
  are still printed as given rather than as a climb of `../..`.
- **`adr`, `lint` and `test` moved out of the dispatch switch** into
  their own functions, following `indexCmd` and `backlogCmd`. `run` drops
  from cyclomatic complexity 113 to 73 and from 269 statements to 159.
- The `func(s string) { fmt.Println(s) }` closure, written out 23 times,
  is now `printLine`.

## 0.32.5 — 2026-08-20

**`procoder deps` no longer reports an empty shelf as an unread one.**
The honesty rule — a thing that was not checked is never reported as
clean — is what makes the tool worth reading, and it was firing in a
case it was never meant for. A repository with no third-party
dependencies has no license surface at all, so
`licenses (go): NOT checked — go-licenses is not installed` pointed the
reader at a gap that did not exist. A reader who learns to skim that
line will skim it in a repository where it means something.

The report now separates the two:

- **Nothing to check** — the manifest declares no third-party
  dependencies — reads `licenses (<eco>): no dependencies to report`.
- **Something unchecked** — dependencies exist and no tool read them —
  keeps the NOT-checked line and its install hint, exactly as before.

Where procoder cannot tell, it says NOT checked. Go reads `require`
directives (block and single-line, indirect included); js reads
`dependencies`, `devDependencies`, `peerDependencies` and
`optionalDependencies`; Rust reads the dependency tables in
`Cargo.toml`. Python answers only for a pyproject-only repository — a
`requirements.txt`, a `Pipfile`, or a `setup.py` computing its
`install_requires` at runtime cannot be read off the text, so procoder
declines to answer rather than guess "none". A manifest it cannot parse
answers the same way. And a dependency the native tool reports as
behind is a dependency whatever procoder made of the manifest text, so
the report can no longer contradict itself in the same breath.

## 0.32.4 — 2026-08-20

A test sweep across the codebase, driven by mutation rather than by
coverage: every test written here was proved by breaking the code it
covers and watching it fail. Total statement coverage 64.2% → 71.9%,
with the weakest packages carrying the change.

| package  | before | after  |
| -------- | ------ | ------ |
| host     | 0.0%   | 100.0% |
| doctor   | 0.0%   | 93.7%  |
| audit    | 8.8%   | 95.6%  |
| tools    | 20.8%  | 81.9%  |
| gitcmd   | 27.4%  | 83.7%  |
| deps     | 38.2%  | 77.5%  |
| security | 39.8%  | 89.2%  |
| config   | 47.1%  | 98.0%  |

- **`procoder debt` no longer calls sound debt rot.** A marker's revisit
  condition routinely lands on a continuation line, because the marker
  line is already full of what the ceiling is. The harvester judged the
  first line alone and flagged every such marker as having no trigger —
  including two written the same day, following the principles exactly.
  The whole comment block counts now; the ledger still shows the first
  line, which is the summary.
- **A fix from 0.32.3 was incomplete.** `maintain`'s file survey still
  swallowed an unreadable root, so the walk error it recorded was always
  nil and the ecosystem was silently skipped. The test written for that
  behaviour is what found it.
- Two branches that cannot be tested without adding a seam — a failed
  rename beside a successful write, and a failed Close — are recorded
  as debt with the condition that would make them testable, rather than
  covered by a test that proves nothing.

## 0.32.3 — 2026-08-20

The audit run on procoder itself, and the four honesty gaps it found in
procoder's own code.

- **The sweep no longer asks diff-scoped questions.** `audit` passes
  every tracked file, so doc-drift and the documentation obligation —
  both of which ask about a CHANGE — answered about everything at once:
  129 of the hygiene section's 138 findings were that noise. The
  diff-independent half is `docs.CollectSweep` now, and the hygiene
  section on this repository went from 138 findings to 9. The diff path
  is untouched.
- **A swallowed walk error made "could not look" read as "nothing
  there."** infra, docs and maintain skipped an unreadable directory and
  kept going, which is right, but they swallowed an unreadable ROOT the
  same way — producing an empty inventory a caller would take for a
  clean answer. The root is now distinguished and reported; `maintain`'s
  file predicate errs toward running the check rather than silently
  skipping an ecosystem.
- **A failed index rename left a stale index and litter.** The atomic
  swap's error is checked; the temporary file is removed so a failed
  write does not accumulate one beside the index on every attempt.
- **A rule the codebase already knew, applied consistently.** The review
  rubric says a failed Close after a write IS a failed write; `lintGo`
  honoured it and `lintJS` and `maintain` did not. All three do now.
  Read-handle closes are deliberately left: closing a file you only read
  cannot lose anything.

## 0.32.2 — 2026-08-20

Two defects found by using the tool, plus the first benchmarks.

- `procoder bench` reported a successful run with no benchmarks as
  `NOT run … exit 1`. The detector is a `git grep` for `func Benchmark`,
  and a grep cannot tell a benchmark from those words inside a fixture
  string — which internal/bench's own test file contains. The run is the
  authority now: zero rows from a successful run means there are no
  benchmarks, said plainly, exit 0.
- The documentation obligation could fire unclearably. procoder's own
  store under `.procoder/` was excluded from CLEARING an obligation but
  not from RAISING one, so a bug story naming the file it fixes demanded
  documentation that no edit to that story could ever supply. The
  exclusion is symmetric now.
- The first benchmarks, over the two paths that run on every write and
  scale with the repository rather than the change: `docs.Drift` across
  a 200-page corpus and `codeindex.Refresh` over a 1500-entry index.
  Both measure ~10ms; the baseline is committed so a future change that
  makes either ten times worse is caught rather than felt.

## 0.32.1 — 2026-08-20

- Internal cleanup, no behaviour change: four text helpers that had been
  copied across packages now have one definition each in
  `internal/textutil` — `slugify` (three byte-identical copies),
  `section` and `stripComments` (five each), and the seven `firstLine`
  copies whose semantics matched. The five `firstLine` variants that
  differ on purpose keep their own, because moving those would change
  output under the cover of a cleanup. Net: 371 lines deleted, 81 added,
  and the shared helpers have tests the copies never had.
- `docs.CollectOffline` and `docs.Run` are gone: both had zero callers
  after the gate moved to the config-aware variants.
- This repository now sets `[docs] policy = "block"`, which the 0.30.0
  notes claimed but the config never carried.

## 0.32.0 — 2026-08-19

- `procoder backlog close story <id>...` takes several ids and verifies
  ONCE. The gate and the test suite judge the tree, not a story, so
  asking them per story re-ran the same answer N times: closing a
  27-story sprint on this repository cost about 729 seconds of identical
  re-verification, and now costs 27. Each story is still judged on its
  own criteria and evidence, and an incomplete one is refused by name
  without costing the others their close. The single-id form is
  unchanged, asserted by a test comparing both forms' output;
  `close epic` and `close milestone` still take exactly one id.

## 0.31.0 — 2026-08-19

The loop: the daily-workflow gaps the analysis found.

- `procoder test --name <pattern>` narrows the run to matching tests
  (`-run`, `-k`, `--tests`, `-Dtest=`, cargo's positional, the pattern
  after `--` for a JS script). A runner that cannot express the pattern
  reports NOT filtered instead of silently running everything, and zero
  matches is an honest pass that says so.
- `procoder run [--exec]` answers "how do I run this project": every
  launch command the repository declares, with the file and line that
  declared it, most specific first. procoder does not manage processes
  — a server belongs to the shell that owns it — so `--exec` runs only
  a single one-shot candidate and refuses when there is a choice or the
  command looks like a server.
- `procoder status` — the state of play, computed fresh: branch, dirty
  files, the active sprint and its open stories, open tasks, unlearned
  lessons, index freshness. The same block is injected at session start
  inside a hard three-second budget (measured: 77ms on this repository),
  so a resumed session opens knowing where the project stands instead of
  re-deriving it.
- Stop and PreCompact hooks write `.procoder/state/handoff.md`: the same
  facts plus HEAD and a timestamp, with a Notes section the agent owns
  and the writer never touches. Facts only — the note never guesses at
  intent.
- `procoder env [--sync]` — what moved under you since the last sync:
  lockfile digests per ecosystem with the install command, migrations
  added or removed, and new keys declared in an `.env.example`. Key
  names only; no value from either file is ever printed. Files git
  ignores are never surveyed.
- `procoder ci --runs` — this branch's newest run per workflow via gh,
  with the failing job names, and the line that matters most: the newest
  run predates your latest push, so CI has not judged this commit.

## 0.30.0 — 2026-08-19

Enforcement: the two promises procoder made and did not keep.

- **The commit gate is no longer voluntary.** A PreToolUse hook
  intercepts `git commit` and stops it while the gate has blocking
  findings, handing the agent the exact work list. `git commit
--no-verify` still passes — loudly, never silently — and
  `[git] commit_gate = "report" | "off"` downgrades or disables the
  interception (default block). `procoder hook install-git` prints a
  `.git/hooks/pre-commit` script so the gate also holds for commits
  made outside any agent. A clean gate deliberately emits no decision
  at all, so your own permission prompt is never bypassed.
- **The documentation gate is universal.** The old command-coverage
  check only ran inside procoder's own source tree and only grepped for
  its own command names; it is replaced by a change-driven obligation
  that works in any repository: a public-surface change (exported
  symbol, CLI flag, config key) or a change to a file documentation
  names, with no documentation changed in the same diff, raises an
  obligation. It clears by editing a doc or by recording the decision —
  `docs: none — <reason>` in the commit message, which the hook reads
  at the moment of the commit (`procoder docs --ack "<reason>"` prints
  the line). Silence never clears it. `[docs] policy = "block"` opts a
  repository in; the default stays report.
- `SurfaceCoverage` replaces the identity-gated check: exported surface
  no document mentions, in any repository, reported in `procoder docs`
  and deliberately kept out of the gate so the gate stays readable.
  AGENTS.md and root-level Markdown now count as documentation.
- **The docs backfill this rot created**: AGENTS.md gained the 19
  commands it never mentioned (and the ten derived host rule files were
  regenerated), configuration.md gained six missing config keys,
  domains.md became ten domains with Testing added and bench, deps and
  adr placed, workflow.md now describes the real sequence, and
  index.md, getting-started.md, README.md and the nav caught up. Four
  false claims were removed, including "start in a worktree", which no
  code ever implemented.

## 0.29.0 — 2026-08-19

- The engineering principles gain two sections: **ADHD/ASD-friendly
  formatting** and **output preferences**. Complex responses (multiple
  issues, decisions to make, long context to synthesize, mixed item
  types) get a title and one-line summary, type-labeled problem cards
  (Enhancement, Defect, Question, Blocker), a small related-context
  block, a numbered "decisions needed" list marking independent ones,
  and noise filtered out — with short single-topic answers skipping all
  of it, and "plain version" / "just the answer" turning it off for a
  response. Output preferences: shorter than you think, 2-4 sentence
  paragraphs, TL;DR on long documents, prose for formal content, two
  explicit versions when two audiences need one document, full code
  blocks, tables for comparisons only. As ever, a repo replaces the
  principles wholesale via .procoder/PRINCIPLES.md.

## 0.28.0 — 2026-08-19

Daily practices, complete: the six remaining gaps from the
what-a-real-dev-does review, shipped as sprint 002 of the Daily
Practices milestone (32 stories, all closed with evidence, milestone
closed).

- `procoder backlog bug <title> [--epic] [--severity s1..s4]` — a
  defect is a story with a severity and a pre-seeded regression-test
  criterion; closing without a severity is refused; the board marks
  open bugs and counts them.
- Sprint retrospectives: `sprint close` scaffolds a Retro (what slowed
  us, what we change, one adaptation), and `sprint open` refuses while
  the last retro is empty — the retro is the price of the next sprint
  (`[sprint] retro = "off"` opts out).
- `procoder release [<version>]` — the pre-tag controller: version
  sync across `[release] files`, the changelog entry, a clean tree,
  the gate, and the suite under [test] policy — every failure listed,
  the tag command printed, never run. This repo lists its nine
  version files.
- `procoder adr new|list|check` — architecture decision records under
  .procoder/adr/: numbered, immutable, superseded rather than edited;
  check refuses hollow records, bad statuses, and dangling supersede
  references; the audit sweep includes them. ADR 0001 records the
  stories-vs-todo decision.
- `procoder deps` — the freshness report per ecosystem via native
  tools (go list -u, npm outdated, cargo-outdated and pip where
  available), licenses where a tool exists — honest NOT-checked lines
  everywhere else, report-only.
- `procoder bench [--save]` — Go benchmarks against a committed
  baseline: ns/op and B/op deltas, regressions beyond [bench]
  threshold marked and exit 1, cross-platform baselines compared with
  a warning. The perf skill now drives it.

## 0.27.0 — 2026-08-19

The test domain: "done" finally runs the tests.

- `procoder test [--coverage] [paths...]` — every detected ecosystem's
  canonical runner (go test, cargo test, the package.json test script
  via the lockfile's manager, pytest, gradle/maven), each reported
  honestly: PASS with counts, FAIL with the failing tests named, and
  NOT run when a runner is absent — which is never the same as green.
- `--coverage` reports the percentage where the runner measures it
  natively (Go; pytest with pytest-cov). A number, never a threshold.
- `[test] policy = "block"` wires the suite into the close controllers:
  `todo close` and `backlog close story` refuse while the suite is red
  — or unverifiable, because unknown is never done. Without the policy,
  closes behave exactly as before. procoder's own repository adopts the
  policy.
- Built and tracked through procoder's own backlog: sprint 001 of the
  Daily Practices milestone, seeded from the test-domain spec.

## 0.26.0 — 2026-08-19

The project layer: lean/agile backlogs with sprints, on the quality
chain. Built spec-first with procoder's own spec and plan controllers.

- `procoder backlog` — milestones → epics → user stories under
  `.procoder/backlog/`, the home of a spec-first project. The story is
  the execution unit and carries todo-task rigor; the todo list itself
  stays untouched as the standalone list for work not born from a spec.
- `procoder backlog seed <spec>` decomposes a COMPLETE spec into an
  epic plus one story per acceptance criterion — everything printed for
  the agent to review and write. The epic records the spec and a
  fingerprint; the board flags `⚠ spec drift` / `⚠ spec missing` when
  traceability breaks.
- Refusing controllers all the way up: story close refuses without
  checked criteria, evidence, and a clean gate; epic close refuses
  while a story is open (and warns on drift); milestone close refuses
  while an epic is open. Unreadable files block conservatively —
  unknown is never done.
- `procoder sprint` — scope-boxed sprints: one active sprint at a time
  (the WIP limit), `pull` commits stories, `carry <id> <reason>`
  returns unfinished work to the backlog with the reason recorded, and
  `close` refuses while a committed story is neither done nor carried,
  then writes committed/done/carried counts into the sprint file. No
  story points, no calendar enforcement — stories are counted, the
  goal is the commitment.
- `procoder backlog board` — the tree with statuses, sprint tags,
  orphans, drift flags, and a one-line summary.
- The plan checker's placeholder rule is now case-sensitive for TODO:
  lowercase "todo" legitimately names procoder's own task domain, and a
  plan touching internal/todo must be writable.

## 0.25.0 — 2026-08-19

- `procoder index impls <symbol>` — what implements an interface or its
  methods, from SCIP implementation relationships. Precise tier only:
  the relationship exists nowhere else, so without SCIP the answer is
  "not built", never a textual guess.
- The precise tier goes polyglot: every SCIP indexer the repository's
  layout calls for runs (not just the first manifest match) and the
  results merge into one index. A missing or failing indexer costs only
  its own ecosystem, reported per indexer.
- CI now verifies the committed dist binaries match the manifest
  version on every platform leg — a version bump that forgot the
  rebuild fails the build instead of shipping stale binaries.
- `.procoderignore` deleted: the file was read by nothing; dead config
  that looks live is the rot the docs domain polices, applied to
  ourselves.

## 0.24.0 — 2026-08-19

- The engineering principles gain a delegation section — you are a
  lead, not a lone hand: independent work fans out to parallel
  subagents (launched together, not one by one), delegation goes where
  a fresh context does better under a clear contract (scope, owned
  files, output shape, definition of done; no shared file ownership),
  launched agents are watched and redirected early, and nothing an
  agent produced merges unjudged — verify claims against the code and
  run the gate over anything an agent wrote. As with the rest of the
  principles, a repo replaces them wholesale via
  .procoder/PRINCIPLES.md.

## 0.23.0 — 2026-08-19

Serena parity: the two capabilities that still needed the serena MCP
plugin now live in procoder, without giving up P-CONTROL.

- `procoder index rename <symbol> <new> [--at path:line]` — the
  cross-file rename as a reviewable unified diff, computed by the
  language's own engine (Go via gopls, which doctor now requires on Go
  repositories). Nothing is written: the agent reviews and applies the
  diff, then verifies with `index refs`. Languages without an engine
  answer honestly with the reference worksheet instead of a half-right
  rewrite; an ambiguous name lists every definition and asks for `--at`.
- `procoder lint --types` — the type-checker where the canonical linter
  does not compile the code: `tsc --noEmit` for TypeScript (grouped
  under each file's nearest tsconfig; without one the file is declared
  out of scope, never silently skipped) and pyright for Python. Go and
  Rust need no flag — golangci-lint and clippy already compile what
  they lint. Doctor requires tsc under a project tsconfig and pyright
  where Python is a real project (pyproject/requirements).
- The index skill now says when to reach past the index: a textual refs
  answer, same-named symbols, or interface implementations are the
  language server's job — use the host's native LSP tool and come back
  to the index for repo-wide sweeps.

## 0.22.1 — 2026-08-19

- The CLI help (`procoder` with no arguments) now lists every command
  alphabetically instead of grouped by workflow, so a command is found
  by name at a glance. Descriptions are unchanged.

## 0.22.0 — 2026-08-19

The language matrix: procoder now covers the popular languages end to end.

- Formatting adds Java (google-java-format), Kotlin (ktfmt), Swift
  (swiftformat, verified live), Ruby (rubocop autocorrect), Dart
  (dart format), and C# (csharpier) — joining Go, Python, the prettier
  family (JS/TS/JSON/CSS/HTML/Markdown/YAML), Rust, C/C++, and shell.
  Same contract everywhere: the project's config wins, the result is
  printed for the agent, unchecked is never clean.
- Lint adds cargo clippy (Rust, workspace-scoped, filtered to changed
  files), ktlint, swiftlint, rubocop, and checkstyle (google_checks
  baseline; a repo checkstyle.xml wins).
- The dependency scan enumerates the wider lockfile matrix: Maven and
  Gradle, .NET packages.lock.json, Swift Package.resolved, Elixir
  mix.lock, Dart pubspec.lock — one list shared with doctor, so the
  scanner requirement and the scan can never disagree. (Podfile.lock is
  deliberately excluded: osv-scanner has no extractor for it, verified.)
- The precise index tier adds rust-analyzer (Rust) and scip-java
  (Java/Kotlin/Scala builds); doctor recommends each only where the
  repository's files call for it.
- The broad index tier gains Swift and Dart: universal-ctags ships no
  parser for either, so procoder supplies regex-based definitions
  (top-level symbols, verified live) — every matrix language can now be
  found, searched, and outlined.
- Docs: an Influences page maps every idea absorbed from the superpowers
  and ponytail plugins to exactly where it lives in procoder; the
  quality-chain page now speaks its own name (spec-based development,
  design documents, quality gates) and carries a real verbatim
  spec-check refusal.
- Honesty note: Go/Python/JS/shell remain the continuously-tested paths
  (they gate this repo's own CI); Swift was verified against the live
  tool; the rest follow each tool's documented interface and fail
  honest — a wrong flag surfaces as NOT-checked, never as clean.

## 0.21.0 — 2026-08-19

The documentation overhaul, and the gates that keep it from rotting again.

- README rewritten from scratch: the whole product told value-first —
  the gate, the quality chain (spec → plan → todo), the self-learning
  loop, the nine domains, the code index, principles and debt, every
  agent — instead of a release-one story eleven versions stale.
- The docs site grew three pages (Getting started, The quality chain,
  Architecture) and a restructured landing; navigation now tells the
  product's story before its reference.
- The rot guards, because presence checks let this happen: a repo can
  declare its feature families (`## README must mention` in the docs
  rules) and a family the README stops telling blocks the gate;
  /procoder:pr gains the mandatory docs-impact question ("what does this
  change alter about what a reader must be told?") answered before any
  PR opens; and the review rubric carries the product-story line. The
  escape is recorded in the lessons ledger with all three adaptations.

## 0.20.0 — 2026-08-19

procoder now works with every AI coding agent, not just Claude Code.

- One canonical `AGENTS.md` carries the always-on contract; ten rule-file
  hosts (Cursor, Windsurf, Cline, Kilo Code, Roo Code, Kiro, Antigravity,
  Qoder, Copilot editors, Codex) get byte-pinned copies, and drift blocks
  the gate exactly like the PR-template mirror. `procoder agents` (and
  `/procoder:agents`) prints the content for anything missing or drifted.
- Plugin-tier adapters for Codex CLI (shares Claude's hooks file — the
  binary detects the host and answers in its JSON shape), GitHub Copilot
  CLI (own hook schema, bash+powershell), Gemini CLI/Antigravity
  (`contextFileName: AGENTS.md`), OpenCode (a thin JS shim plus generated
  command twins, parity pinned by test), Grok Build, Devin CLI, Qoder,
  pi, and Hermes. Adapter rule: adapters stay thin — logic lives in the
  binary, content in `AGENTS.md` and `commands/`.
- Host detection in the binary (`COPILOT_PLUGIN_DATA` → `PLUGIN_DATA` →
  `QODER_SESSION_ID` → Claude, with the VS Code Copilot path heuristic);
  `procoder principles --hook` emits each host's session-start shape.
- Manifest versions are pinned to the plugin version by the gate, and a
  root `hooks/hooks.json` is forbidden outright (Gemini would auto-load
  it with incompatible event names). Claude Code remains the tested
  reference host; the docs say so plainly.

## 0.19.0 — 2026-08-19

Catch first, learn on escape: downstream reviewers become the fallback,
not the net.

- The pre-PR self-review: `/procoder:pr` now dispatches a fresh-context
  reviewer over the branch diff against `.procoder/github/REVIEW.md`
  BEFORE the PR is opened; Critical/Important findings are fixed first.
  The default rubric is seeded from every class bot reviewers actually
  caught on this repo.
- The reflection loop: `/procoder:merge` treats an escaped finding as a
  bug in our gates — each one names the layer that should have caught it,
  that layer is adapted in the same PR, and the lesson lands in
  `.procoder/github/LESSONS.md`. `procoder lessons` flags entries with no
  adaptation as UNLEARNED (exit 1) — recorded is not learned. Our own
  ledger ships seeded with the eight real escapes to date: the PR #17/#18
  review findings, the CI mirror hang, and our own self-scan's fixture
  harvest.
- Go lint baseline: repositories without a golangci config get a curated
  default (standard set plus gosec, gocritic, errorlint, unparam,
  copyloopvar, nilerr) — the same pattern as the eslint baseline, and the
  repo's own config always wins.
- CI robustness: apt is repointed from the flaky Azure mirror to the
  canonical archive with fail-fast retries — a gate run once burned its
  whole timeout waiting on that mirror.
- Honesty fix from our own scanner: debt-marker test fixtures are now
  assembled at runtime so `procoder debt` on this repository reports a
  clean ledger instead of harvesting its own tests.

## 0.18.0 — 2026-08-19

Absorbed the best of the superpowers and ponytail plugins, so both can be
uninstalled.

- `procoder plan` and `/procoder:plan`: implementation plans under
  `.procoder/plans/` complete the spec → plan → todo chain. The quality
  controller blocks on placeholders ("TBD", "handle edge cases",
  "similar to task N"), empty sections, and tasks without `Files:` or
  checkbox steps — a plan is written, not promised.
- `procoder debt`: deliberate-simplification markers (`debt:` comments
  naming a ceiling and a revisit condition; marker configurable via
  `[debt]`) harvested into a ledger, with no-trigger markers flagged as
  rot.
- `procoder principles` plus a SessionStart hook: every session starts
  with the engineering principles (reuse → stdlib → platform → minimum
  code, root-cause bug fixing, deliberate corners marked as debt);
  `.procoder/PRINCIPLES.md` replaces them per repo.
- New skills: `/procoder:debug` (root cause before any fix, one
  hypothesis at a time, three strikes questions the architecture),
  `/procoder:tdd` (red before green, name the break each test catches,
  the mutation check), `/procoder:simplify` (the five-tag
  over-engineering review with an honest "Lean already. Ship." null
  result).
- Skill upgrades: `/procoder:spec` now classifies work as spike, bounded,
  or architectural with a one-way ratchet before interviewing;
  `/procoder:todo` defines what counts as evidence (fresh verification
  only, red-green proof for regression tests); `/procoder:merge` gains
  the review-receiving rules (verify claims before implementing, ask
  when unclear, facts instead of gratitude).

## 0.17.0 — 2026-08-19

Quality controllers for tasks and specs — done means verified.

- `procoder todo` and `/procoder:todo`: tasks live as Markdown files under
  `.procoder/todo/`, each with a real description, testable acceptance
  criteria, and an evidence section. `todo close` is the quality
  controller — it refuses to close a task until every criterion is
  checked, the evidence records what was run and what it proved, and the
  commit gate is clean, naming exactly what is missing.
- `procoder spec` and `/procoder:spec`: spec-first design under
  `.procoder/specs/`. The skill runs a gap-closing interview (problem,
  users, scope boundaries, constraints, interfaces, data, edge cases,
  failure modes, testable acceptance criteria, open questions);
  `spec check` blocks while sections are missing or empty, while any
  `OPEN:` question is unresolved, and while criteria are untestable.
  A complete spec seeds the todo list.
- The docs domain now requires CHANGELOG.md to carry an entry for the
  current version (blocking): a changelog that exists but skips the
  release being shipped is exactly how release notes go stale.

## 0.16.0 — 2026-08-19

The onboarding sweep, the comprehensive site, and a robustness batch.

- `procoder audit` and `/procoder:audit`: every domain's checks over the
  WHOLE tracked tree of a repository procoder has not governed before,
  aggregated into a triage-ordered scorecard. Its first run flagged our
  own pinned action SHAs as secrets — the false-positive flow
  (`gitleaks:allow` / `.gitleaksignore`, every allow a reviewed decision)
  is now part of the security rules.
- The docs site grew from one page to a real reference: the nine domains,
  every command, every configuration knob, and the workflow — and a new
  completeness check blocks a shipped command the documentation never
  mentions (usage text and the coverage list are pinned to each other by
  test).
- Robustness: CI runs once per change (push runs only on main), golangci
  caches are isolated per repository root (no more stale cross-worktree
  paths), the pr skill enforces ≤72-char titles, the merge skill deletes
  remote branches explicitly instead of trusting the flag's silent local
  failure, and the accepted Stdout.Write info line is excluded by config.

## 0.15.0 — 2026-08-19

Linters for all, without an asterisk — and the version tripwire now
guards every claims-bearing page.

- VersionSync generalizes from the README to a rules-driven list
  (## Version-tracked docs in .procoder/docs/RULES.md; default README.md
  and docs/index.md): the Pages site's index shipped eight releases
  stale because only the README was held to the version — the same
  prose-claims blind spot, now closed for every listed page. The site
  content itself is rewritten to the all-nine reality.

- Configless JavaScript now gets a procoder baseline: eslint's BUILT-IN
  core rules (no-undef, no-unused-vars, eqeqeq, no-var, …) via a
  generated temp flat config with common runtime globals — no npm
  packages installed, nothing written into the repo, and the project's
  own eslint config still always wins. Findings are labeled
  "(lint, procoder baseline)".
- TypeScript without a project config stays honestly out of scope: a TS
  parser is not built into eslint and installing one would be imposing.
- eslint v10 removed the unix formatter from core — both eslint paths now
  parse --format json, fixing config-carrying projects on v10 too.

## 0.14.1 — 2026-08-19

Morning review fixes, both dictated:

- hook.Run (complexity 25) and index Impact (25) refactored into named
  single-purpose helpers — both now under the threshold; the remaining
  switchboards (gate.Run 19 and friends) accepted as honest.
- Maintain thresholds are repo-overridable per D-OVERRIDE:
  `[maintain] gocyclo / funlen_lines / funlen_statements` in
  .procoder/config.toml, defaults 15/80/50.

## 0.14.0 — 2026-08-19

Domain 4, performance — and with it, all nine domains shipped.

- `/procoder:perf`: the measure-first discipline as a skill. Deterministic
  perf checks barely exist, so v1 teaches the real instruments (go test
  -bench/pprof/benchstat, cProfile/py-spy, node --cpu-prof) and the law:
  baseline, change, re-measure, report the delta with the command — a fix
  without a benchmark is a hope. Heavier tooling arrives when a real need
  shows.

## 0.13.0 — 2026-08-19

Domain 8, DevOps/IaaS/CaaS: each instrument only where its files exist.

- `procoder infra` and `/procoder:infra`: hadolint over Dockerfiles,
  `terraform fmt`/`validate`/tflint over *.tf directories (a failing
  validate BLOCKS — objectively broken; uninitialised dirs say NOT
  validated instead of failing on providers), kubeconform over Kubernetes
  manifests, `helm lint` over charts. Rides the gate and `procoder git`;
  a repo with no infrastructure pays nothing.
- doctor/init learn the five tools, each required only by inventory.

## 0.12.0 — 2026-08-19

Domain 7, CI/CD/CT: pipeline discipline as deterministic checks.

- `procoder ci` and `/procoder:ci`: mutable action refs (report by
  default, blocking via `[ci] pin_actions_policy = "block"`), missing
  per-job timeout-minutes, missing concurrency cancel-in-progress, and no
  tests anywhere. Rides `procoder git` and the gate too.
- Our own CI practices it: every action pinned to its commit SHA with the
  tag as a comment, and a concurrency group cancels stale runs.

## 0.11.0 — 2026-08-19

Domain 3, maintainability: a thin layer over the index and the linters.

- `procoder maintain` and `/procoder:maintain`: dead-code candidates from
  the precise index (exported API marked for judgment), cyclomatic
  complexity and function length from isolated linter runs with procoder's
  own thresholds (gocyclo 15, funlen 80/50, C901) — the repo's lint config
  is neither required nor touched. Everything reports; nothing blocks.

## 0.10.0 — 2026-08-19

Domain 1, security: the priority level, built on lint's rails and the index.

- Secrets (gitleaks): BLOCKING always — in the write hook the moment a
  secret lands in a file, in the gate over the changed set. The finding
  names rule and location, never the value, and orders a rotation.
- SAST (semgrep, community rules) and dependency vulnerabilities
  (osv-scanner): `procoder security --deep` and CI; ERROR severity and
  CVSS ≥ 7.0 block, the rest is judged.
- `/procoder:security` reviews from the index's entry points and call
  graph; rules live in .procoder/security/RULES.md.
- Missing scanners read as blocking NOT-checked — a security check that
  silently didn't run is worse than a red one.

## 0.9.0 — 2026-08-19

Domain 2, best practices: the canonical linter per ecosystem.

- `procoder lint` and `/procoder:lint`: golangci-lint (Go), ruff check
  (Python), shellcheck (shell), eslint (JS/TS, only where the project
  carries a config — procoder imposes no rules). The write hook lints the
  file just written in-turn; the gate lints the changed set.
- Report by default — lint is judgment, formatting was not; a repo opts
  into blocking with `[lint] policy = "block"` in .procoder/config.toml.
- Missing linters read as NOT checked, never clean; configless JS/TS is
  labeled out of scope.
- `/procoder:update`: update the plugin from the marketplace and verify
  the new version by direct invocation.

## 0.8.2 — 2026-08-19

- The README must carry the current version on its first screen — a
  blocking docs check, born from three releases shipping against a badge
  frozen at 0.7.0. Prose claims aren't file paths, so drift never fired;
  now a release without a reviewed README reds the gate. The README's
  domain list also caught up (documentation shipped, the index noted).
- The call graph dropped its noise: compiler-local temporaries and bare
  package descriptors are excluded from the edges (7,012 → 2,587 on this
  repo, all signal), `callers` shows each named call once with readable
  symbols (`io/ReadAll()`, not the SCIP provenance header).

## 0.8.1 — 2026-08-19

- The skills are back: command definitions moved from TOML with multiline
  strings — which the plugin loader silently drops — to Markdown with YAML
  frontmatter, the canonical format. Same nine commands, now actually
  registered.

## 0.8.0 — 2026-08-19

The code index (D-INDEX): the shared platform layer under the domains.

- `procoder index build|find|search|refs|outline|impact|stats` and the
  `/procoder:index` skill. Broad tier from universal-ctags (definitions,
  outlines, fuzzy search); precise tier from SCIP (scip-go and friends) for
  exact references — every answer says which tier produced it, and a stale
  index says so out loud.
- `index impact`: the blast radius of the working-tree change — which
  symbols it defines and which files reference them; the gate prints it
  and /procoder:pr makes the agent verify the named files.
- The security/maintainability surface, built now: `index callers` (the
  call graph from SCIP occurrences), `index unused` (dead-code
  candidates, exported API marked), `index entrypoints` (mains and the
  exported surface), and `index graph` (the machine-readable edge list
  future domains walk).
- The write hook keeps the broad tier current for each file written; the
  gate rebuilds a stale index at the finishing moment, covering editor
  edits and the precise tier the hook cannot reach.
- Tool resolution got honest: a probe rejects macOS's BSD ctags impostor,
  and `~/go/bin` / `~/.local/bin` count as installed.

## 0.7.1 — 2026-08-19

- The docs scan now asks git which Markdown belongs to the repository
  (tracked plus untracked-but-not-ignored) instead of walking every
  directory — gitignored scratch is no longer scanned.
- The PR-template mirror is enforced: drift between .github/ (the path
  GitHub reads) and the .procoder/github/ master now blocks the gate.
- The merge watcher got a protocol: calibrate against previous runs, poll
  per job in the foreground, report the first failure immediately with its
  log excerpt, poll dynamically — never a fire-and-forget monitor.
- Issue templates caught up with the reset: no more dropped "levels",
  Node-era fields, or renamed config paths.

## 0.7.0 — 2026-08-19

Domain 5, documentation: docs treated as a product.

- `procoder docs [--external]` and `/procoder:docs`: broken relative
  references and non-compiling Mermaid diagrams block; doc drift, missing API
  doc comments (Go/Python/TypeScript), required docs, badges, and README
  first-screen structure are reported; `--external` adds `lychee` link
  checking and GitHub Pages health.
- The write hook now checks Markdown references and diagrams in-turn, and
  reports which docs mention a code file the agent just changed.
- New repo-owned rules: `.procoder/docs/RULES.md` and the shared Mermaid
  theme `.procoder/docs/mermaid.json` (printed by `procoder templates`).
- Docs site: MkDocs Material built and deployed to GitHub Pages by CI.
- `doctor`/`init` learn `lychee`, `mmdc`, and `mkdocs`.

## 0.6.0 — 2026-08-18

Repo-overridable workflow rules (D-OVERRIDE begins here).

- `.procoder/github/WORKFLOW.md`: feature work in git worktrees, PR polling
  delegated to a watch-only background agent, full local+remote cleanup after
  a successful merge. The repo's file wins over the skills' defaults.
- Fixed `RepoRoot` to recognize a worktree's `.git` file — commands run
  inside a worktree no longer report against the parent checkout.

## 0.5.x — 2026-08-17

Domain 9, GitOps/GitHub, hardened by its own dogfood runs.

- The gate's git slice: conflict markers, junk files, oversized files
  (5 MB default), AI-attribution lines in commit messages (blocking — the
  work is the author's), commit subject shape, default-branch policy.
- `actionlint` on every workflow file written, findings in the same turn.
- PR and commit templates under `.procoder/github/`, `/procoder:pr` and
  `/procoder:merge` skills, `procoder scrub`.
- 0.5.1 fixed the gate exiting 0 over blocking findings; 0.5.2 fixed Windows
  test stubs and stopped prettier flagging the commit template's functional
  blank lines.

## 0.4.0 — 2026-08-17

- `procoder init [--yes]`: the binary computes the install plan per machine,
  the agent (or `--yes`) executes it, and the survey re-runs afterwards —
  an installer's exit 0 is a claim; the tool resolving is the fact.

## 0.1.0 – 0.3.0 — 2026-08-17

The Go rewrite and the plumbing proof (domain 6, formatting).

- One static binary per platform, committed in `dist/`, installed via the
  Claude Code marketplace; hooks and skills call a thin launcher.
- Formatting via each ecosystem's canonical tool (gofmt, ruff, prettier,
  rustfmt, clang-format, shfmt) with three honest verdicts: clean,
  unformatted (formatted bytes handed to the agent), unchecked (said out
  loud, never silent). The write hook hands the agent the formatted code and
  never touches the file (P-CONTROL).

Before 0.1.0 the project was a TypeScript analyzer engine; that history is in
git. The design reset that produced the current harness is recorded in the
design contract.
