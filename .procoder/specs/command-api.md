# command-api

Status: complete

## Problem

Every procoder command reaches its caller the same way: a host spawns
`hooks/launcher.sh`, the launcher execs the binary, the binary writes lines
to stdout and exits. That is the whole interface. An agent that wants the
gate's verdict runs `procoder check` and parses text; a session that wants
the task list runs `procoder todo list` and parses text; a tool that wants
to know whether the suite is green has no option but to spawn a process and
wait however long the suite takes.

Three things follow from that, and only the first is obvious.

**Every caller pays the spawn.** Five hook wirings fire on every session
start, every Bash call and every write. Each is an independent process with
no memory of the last: the config is re-read, the code index is re-opened
from disk, and every domain re-discovers which tools exist on this machine.
Nothing is shared between calls, between sessions, or between repositories.

**A long command cannot be started and left.** `procoder test` runs the
whole suite, `audit` runs every domain over the whole tracked tree, and
`security --deep` adds semgrep and osv-scanner. The caller holds the process
for the duration or loses the run. The host hook timeouts — 120s for
PreToolUse, 60s for PostToolUse, 15s for SessionStart, 10s for Stop — are
the ceiling those commands are measured against today, and they are the
reason the slow half of procoder is never called from a hook at all.

**There is no interface to call that is not text.** Exit codes carry three
values (ADR 0003) and everything else is prose intended for a person. A
caller that wants a structured answer parses the prose, which makes every
line of output a compatibility surface nobody declared.

The goal is feature parity for all 47 commands over an API call, so a
session can drive procoder as functions rather than as a subprocess. The
four hook entrypoints are the first consumers of the socket this spec
defines and not its only ones: a caller who wants the other 43 may have
them.

One property is not traded away for any of it. No CLI command requires a
daemon, in CI or on a fresh clone — a second door is only useful if the
first one is always open.

The rewrite left the engine unusually ready for this. Nothing under
`internal/` writes to `os.Stdout` or calls `fmt.Print`; 123 functions
already take an `out func(string)`; there is no `os.Chdir` and no `os.Exit`
outside `main`. The seam an API needs exists. What does not exist is a
transport, a request envelope, and an answer for the four commands that run
what a repository declared.

## Users

- **An agent in a session** — needs the gate's verdict, the task list and
  the spec controller as calls that return a value, not as processes whose
  stdout it parses. It also needs to start the suite and keep working.
- **A hook** — needs what it needs today, unchanged: a JSON envelope in the
  running host's shape, within the host's timeout, degrading to silence
  rather than taking the session down.
- **A person at a terminal** — needs `procoder check` to behave identically
  with no daemon, no socket and no setup, in CI and on a fresh clone.
  A second door is only useful if the first one is always open.
- **Whoever writes the second client** — an editor extension, a CI step, a
  test harness — needs one documented envelope, not 47 output formats.
- **Whoever reviews this change** — needs to see that no command's behaviour
  moved, which is why byte-identical parity is an acceptance criterion.

## In scope

- [S-1] `procoder serve`: a daemon listening on a unix socket at
  `~/.procoder/run/procoder.sock`, mode 0600, a named pipe of the same name
  on Windows. One daemon per machine, serving every repository. File
  permissions are the authentication — no port, no token.
- [S-2] One request envelope and one response envelope, for every command.
  The request carries argv, the working directory, the environment the
  caller wants applied, stdin bytes and a protocol version. The response
  carries the exit code and the output, with stdout and stderr kept apart.
- [S-3] The request's working directory and environment are what the
  command sees. `host.Detect` takes the environment as an argument rather
  than reading the process's, and the three `os.Getwd()` sites
  (`internal/doctor/doctor.go:167`, `internal/format/format.go:170`,
  `internal/hook/commit.go:186`) take the request's directory. A daemon
  started by one host serves another host correctly.
- [S-4] Per-root serialisation inside the daemon: requests against one
  repository identity are queued, not run concurrently, so `store.Lock`
  never contends with another goroutine of its own process.
- [S-5] Two response shapes, chosen per command. A fast command answers
  inline. A long-running one — `test`, `audit`, `security --deep`,
  `index build`, `release`, `bench`, `deps`, `docs --external`, `ci --runs`
  — returns a job id immediately; the caller polls for status and output,
  and the job survives the connection that started it.
- [S-6] A second socket for the executing commands. `run --exec`,
  `evidence record`, `init --yes` and `self-upgrade` are served only there;
  it has its own opt-in, its own path, and the hook transport is never told
  its address. The work socket refuses those four with an error naming the
  boundary.
- [S-7] A version handshake: the daemon reports its version, and a client
  built from a different one refuses to use it. `self-upgrade` stops a
  running daemon as part of its own run, so the next call starts a fresh
  one.
- [S-8] The daemon holds one code index per repository in memory. Each
  repository's warm state is evicted after its own idle window; the daemon
  exits when it holds none.
- [S-9] The SessionStart hook starts the daemon when the socket is dead,
  single-flight via a lock file. No launchd, no systemd, no install
  step.
- [S-10] A parity test over every command: the same argv, working
  directory, environment and stdin produce a byte-identical exit code and
  output whether the command ran in-process or over the socket. The test
  varies the environment as well as the payload.
- [S-12] Every response carries a typed result alongside the human bytes.
  For the reporting commands that result is a findings list in the shape
  `internal/gitx`'s `Finding` already has — file, line, message, blocking —
  plus the domain that raised it. The bytes are unchanged, so the parity
  test in S-10 still compares them.
- [S-13] A command whose answer is not findings — `status`, `config`,
  `todo list`, `spec check`, the `index` queries, `version` — carries a
  typed result named for that command instead. Each is declared once, in
  one place, so a client reads a schema rather than a paragraph.
- [S-14] The request may carry a confirmation, so the six asking commands
  are reachable over the API: `ask`, `copilot-leak`, `version --check`,
  `wizard run`, `init` and `self-upgrade`. A request with no confirmation
  takes the non-interactive path exactly as it does today, and the response
  says which path it took.
- [S-11] `[service] mode` in `.procoder/config.toml` — `off` by default, so
  no repository changes behaviour because it upgraded — and
  `procoder init` asks whether this machine runs the local CLI or the local
  server, rather than leaving the key to be hand-edited.

## Out of scope

- **The team server.** No network transport, no RBAC, no shared state, no
  multi-user. Designed and parked in #248.
- **Removing or thinning the CLI.** Every command runs in-process exactly
  as it does today when there is no daemon, with no setup, in CI and on a
  fresh clone. The API is a second door, never the only one.
- **Proactive work.** The daemon does nothing unasked: no filesystem
  watching, no scheduled checks, no cross-repo coordination.
- **Changing what any command decides.** The gate reaches the same verdict
  and every controller refuses the same things. Only the transport is new.
- **Inventing a result shape per command where one already fits.** The
  typed result in S-12 is `gitx.Finding`, which 32 files already produce.
  A command whose answer is genuinely not findings gets its own shape
  (S-13); a command whose answer is findings does not get a second one.
- **Asking a person anything the daemon decides on its own.** The
  confirmation in S-14 is carried from the caller, never synthesised. A
  client that sends no answer gets the non-interactive path, which is what
  happens today.
- **Reaching a repository the caller could not already reach.** The daemon
  runs as the user who started it and grants no access the binary did not
  have (P-CONTROL).

## Constraints

- **Zero dependencies.** The module file go.mod has no require block and
  this change adds none. The socket, the envelope encoding and the job table are stdlib.
- **P-CONTROL.** The daemon acquires no right the binary lacked. It is
  transport, not permission.
- **#201's boundary holds structurally, not by rule.** A file an agent
  session could have written is never executed unattended. The hook
  transport and the executing commands must not share a door, which is what
  S-6 buys: `internal/hook/noexec_test.go` asserts no file in the hook
  package imports `runcmd`, and that assertion must stay true of the client
  transport too.
- **A hook must never wedge a session.** No wait is unbounded, and a daemon
  that cannot be reached costs the caller nothing but the in-process path.
- **The 0600 socket authenticates the user, not the process.** Every
  process running as that user — including an agent session's own Bash tool
  — can reach the work socket. Nothing served there may do what #201
  forbids doing unattended.
- **Windows, macOS, Linux.** The named pipe on Windows carries the same
  envelope and the same semantics as the unix socket elsewhere.
- **No behaviour change is a testable claim, not a promise.** See S-10.

## Interfaces

**`procoder serve`** — starts the daemon in the foreground. Prints the
socket path and the version it serves, then serves until idle or signalled.

```
procoder serve [--socket <path>] [--idle <duration>]
```

**The envelope.** One request shape for every command, newline-delimited
JSON over the socket:

```json
{
  "protocol": 1,
  "argv": ["check", "--paths-from", "-"],
  "cwd": "/Users/x/src/thing",
  "env": { "CLAUDE_PLUGIN_ROOT": "/Users/x/.claude/plugins/procoder" },
  "stdin": "<base64>",
  "confirm": null
}
```

The response is the same shape for an inline answer and for a job's result:

```json
{
  "protocol": 1,
  "exit": 1,
  "stdout": "...",
  "stderr": "...",
  "job": null,
  "result": {
    "kind": "findings",
    "findings": [
      {
        "file": "internal/hook/hook.go",
        "line": 0,
        "message": "is not formatted",
        "blocking": true,
        "domain": "format"
      }
    ]
  }
}
```

`result` is the typed answer and `stdout` is the same prose the CLI prints.
Both, not one: the bytes are what the parity test compares and what a person
reads, and the result is what a client acts on. A response that carried only
the result would make every caller a renderer, and one that carried only the
bytes would make every caller a parser.

`kind` is `findings` for the reporting commands — the shape `gitx.Finding`
already has in 32 files, plus the domain that raised it, which
`printFindings` already knows and currently spends on a label. A command
whose answer is not findings names its own kind (`status`, `config`,
`index.find`, and so on) and declares its fields once. A command with no
answer beyond an exit code carries `"result": null`, which is not the same
as an empty findings list: null is "this command does not report findings",
`[]` is "it does, and found none".

`confirm` carries what a person would have typed. Absent or null means no
person, and the six asking commands take the non-interactive path they take
today — `copilot.CanAsk` answers false over a socket because there is no
character device, and that stays true. The confirmation is a field the
caller sets, never something the daemon infers from having been asked.

`env` is a map and not the process environment because the daemon serves
several sessions: what the caller sends is what the command sees, and
nothing else. Only the keys procoder reads are meaningful —
`COPILOT_PLUGIN_DATA`, `PLUGIN_DATA`, `QODER_SESSION_ID`,
`PI_CODING_AGENT`, `CLAUDE_PLUGIN_ROOT`, `VIRTUAL_ENV`,
`PROCODER_PUPPETEER_CONFIG`, `PATH` — and a caller that sends more is not
an error, because the set will grow.

`stdout` and `stderr` stay apart because they already do: `store.Notice`,
`principles.Stderr` and `hook.Stop`'s writer are separate channels today,
and merging them at the transport would lose the distinction the hook
contract depends on.

**A job.** A long-running command answers with a job rather than a result:

```json
{ "protocol": 1, "exit": null, "job": { "id": "j7f3a2", "state": "running" } }
```

Polled with a request whose argv is empty and whose job id is set; the
reply is the same envelope, with `exit` filled in once `state` is `done`.
Output accumulated so far is returned on every poll, so a caller can follow
a suite without holding the connection that started it.

**The exec socket.** Same envelope, different path
(`~/.procoder/run/procoder-exec.sock`), enabled by its own config key. The
four executing commands are served there and nowhere else. Asked for on the
work socket, they return exit 2 and a message naming the boundary and the
issue that drew it.

**`.procoder/config.toml`** gains:

```toml
[service]
mode = "off"       # off | local — off by default (D4)
exec = false       # the second socket, opt-in on its own
```

**`procoder config`** prints both, with their provenance, alongside the
identity line the seam already added.

## Data

**The sockets** live under `~/.procoder/run/`, created 0700, with the
sockets themselves 0600. Not under `.procoder/` in a repository: one daemon
serves many repositories and a socket in one of them would make that
repository's checkout a dependency of every other's.

**The single-flight lock** for auto-start is a lock file beside the socket,
holding the pid of the starting process and the time it started, judged
stale on the same rule `internal/store` already uses.

**The job table** lives in memory only. A job is the command, its state, the
output so far, the exit code once there is one, and the identity it ran
against. Nothing is written to disk: a job that outlives the daemon would
have to be re-attached to a process that no longer exists, and a caller
whose daemon died needs to hear that rather than read a stale result.

**The warm state** is one code index per repository identity, plus that
repository's parsed config. Held in memory, evicted per repository on its
own idle window, never written anywhere the on-disk index does not already
live.

**Nothing in `.procoder/` changes shape.** Every file keeps its path, name
and format, so a repository that opts in and back out sees no diff.

## Edge cases

- **No daemon running and `mode = "off"`.** Every command runs in-process.
  This is the default and the CI path, and it is the one that must never
  need setup.
- **The socket exists but nothing is listening** — a daemon that died. The
  client removes the stale socket and either starts a new daemon (from
  SessionStart) or runs in-process, and says which.
- **A daemon of a different version.** The handshake fails and the client
  runs in-process rather than serving stale behaviour. A hook does not wedge.
- **Two sessions in one repository at once.** S-4 queues them. Without it
  the second's `store.Lock` waits out `lockTimeout` — five seconds — and
  returns "the write was NOT made" for a lock the same process holds.
- **Two clones of one repository on one machine.** They share an identity
  (the seam's deliberate choice) and therefore share a queue. Correct but
  slower than necessary; noted rather than solved.
- **A command that asks, called with no `confirm`.** `copilot.CanAsk` takes
  a concrete `*os.File` and tests for a character device, so over a socket
  it answers false and the command takes its non-interactive path. That path
  is defined for all six, and the response says which path it took rather
  than leaving the caller to infer it from silence.
- **A `confirm` on a command that never asks.** Ignored, not refused. A
  client that sets it everywhere is clumsy, not wrong, and refusing would
  make the field a per-command lookup for every caller.
- **A `confirm` that says yes to `self-upgrade` or `init --yes`.** Served
  only on the exec socket (S-6). The confirmation makes the answer
  reachable; it does not move the command to the other door.
- **A command whose findings are empty.** `"findings": []` with exit 0. A
  client must not read an absent result and an empty one as the same thing,
  which is why a command that reports no findings at all carries null.
- **A finding whose line is unknown.** `line` is 0, exactly as
  `gitx.Finding` already means it: the finding is about the file as a whole.
- **An executing command on the work socket.** Refused with exit 2, naming
  the exec socket and #201. Never served, never silently downgraded.
- **A job polled after the daemon restarted.** The id is unknown; the reply
  says the job was lost, not that it failed.
- **A job whose caller never polls again.** Evicted with its repository's
  warm state, on the same idle window.
- **`cwd` outside any repository.** `tools.RepoRoot` returns the directory
  it was given, exactly as it does for the in-process path.
- **`cwd` the caller cannot read, or that does not exist.** Refused before
  the command runs, with the path named.
- **An oversized stdin.** Bounded, and a request over the bound is refused
  with the limit named rather than buffered until the daemon dies.
- **Windows.** The named pipe replaces the socket; the lock file, the
  envelope and the job table are unchanged.

## Failure modes

- **The socket cannot be created** (no `~`, a read-only home, a stale
  directory owned by somebody else). `serve` refuses and says why. Clients
  run in-process.
- **The daemon is unreachable mid-request.** The client runs the command
  in-process and reports that it did. A degraded transport never costs a
  caller nothing it could have had: the command did not run, and the
  reason says so.
- **The daemon is slow.** Every client wait is bounded, and a wait that
  runs out is reported as one. The command is not then run here instead:
  it may already be half-done inside the daemon, and running it twice is
  worse than saying it did not finish.
- **A command's typed result cannot be built.** The bytes are still
  returned, `result` is null, and stderr says the result was unavailable.
  Losing the typed answer must never lose the human one — the prose is the
  older contract and the one a person is reading.
- **A client on an older protocol.** `protocol` is in both envelopes. A
  client that does not know a `kind` treats the result as absent and reads
  the bytes, which is why the bytes are not optional.
- **A command panics inside the daemon.** The panic is contained to that
  request, returned as a non-zero exit with the message on stderr, and the
  daemon keeps serving. One repository's bad state does not take down the
  other nine.
- **The in-memory index is stale.** The index has an on-disk truth and a
  freshness rule already; the warm copy is invalidated by the same rule, and
  a warm copy that cannot be proven fresh is dropped rather than served.
- **Memory grows with the repository count.** Eviction is per repository
  (S-8), and the daemon exits when it holds none. A footprint that is still
  a problem is a reason to change the eviction window, which is why it is
  configurable rather than a constant.
- **`self-upgrade` runs while the daemon serves other sessions.** It stops
  the daemon; their next call starts the new one. Requests in flight are
  finished first.
- **The exec socket is enabled by something other than a person.**
  `.procoder/config.toml` is a tracked file, so enabling it is a diff in a
  commit rather than a silent state change. This is the whole defence and it
  is stated so it is reviewed.

## Acceptance criteria

- [ ] [S-1] With `mode = "local"`, `procoder serve` creates a socket at
      `~/.procoder/run/procoder.sock` whose mode is 0600 inside a 0700
      parent, and a request over it returns the exit code the same command
      returns in-process. Asserted by `TestServeSocketPermissions` and
      `TestSocketExitMatchesInProcess`; fails if the socket is created with
      any wider mode, or if the two exit codes diverge for any command.
- [ ] [S-2] A request carrying argv, cwd, env and stdin returns a response
      whose stdout and stderr are separate fields, both populated for a
      command that writes to both. Asserted by `TestEnvelopeSeparatesStreams`;
      fails if either stream is merged into the other or dropped.
- [ ] [S-3] Two requests differing only in `env` — one carrying
      `CLAUDE_PLUGIN_ROOT`, one carrying `QODER_SESSION_ID` — return
      `principles --hook` output in the two hosts' different JSON shapes,
      from one daemon whose own environment carries neither. Asserted by
      `TestHostShapeFollowsRequestEnv`; fails if `host.Detect` reads the
      process environment again.
- [ ] [S-4] Fifty concurrent requests against one repository all succeed and
      a claims ledger written by all fifty holds fifty entries. Asserted by
      `TestPerRootSerialisation`; fails if any request returns "the write was
      NOT made", which is what `store.Lock` returns when it contends with
      its own process.
- [ ] [S-5] `procoder test` over the socket returns a job id in under one
      second, the connection can be closed, and polling that id returns
      accumulated output and finally the suite's real exit code. Asserted by
      `TestJobSurvivesItsConnection`; fails if the call blocks for the
      suite's duration, or if closing the connection ends the run.
- [ ] [S-6] `run --exec` on the work socket returns exit 2 naming the
      boundary; the same request on the exec socket runs it; with
      `exec = false` the exec socket does not exist. Asserted by
      `TestExecutingCommandsRefusedOnWorkSocket`; fails if any of the four
      executing commands is served on the work socket.
- [ ] [S-6] A structural test asserts the client transport does not import
      `internal/runcmd`, in the shape `internal/hook/noexec_test.go` already
      uses for the hook package. Asserted by
      `TestClientTransportExecutesNothing`; fails if that import is added,
      and the test names the file that added it.
- [ ] [S-7] A client whose version differs from the daemon's refuses to use
      it and says so, running nothing, and `self-upgrade` leaves no daemon
      running. Asserted by `TestVersionSkewIsRefused` and
      `TestSelfUpgradeStopsTheDaemon`; fails if a mismatched daemon serves
      the request, or if a daemon survives the upgrade.
- [ ] [S-8] After serving two repositories and idling past the window the
      daemon holds neither index, and having held none for a window it
      exits. Asserted by `TestPerRepoEviction`; fails if warm state outlives
      its window, or if a daemon holding nothing stays alive.
- [ ] [S-9] With the socket absent, one SessionStart hook leaves exactly one
      daemon running, and two racing SessionStart hooks also leave exactly
      one. Asserted by `TestAutoStartIsSingleFlight`; fails if a second
      daemon is started, or if none is.
- [ ] [S-10] For every command in the usage list a fixed argv, cwd, env and
      stdin produce a byte-identical exit code, stdout and stderr in-process
      and over the socket, with the executing four asserted refused rather
      than compared. Asserted by `TestParityAcrossEveryCommand`; fails if
      any command's bytes differ, and — because the table is built from the
      usage text — if a command is added without a parity case.
- [ ] [S-11] A repository with no `[service]` block behaves exactly as it
      does today, and `procoder config` prints `mode` and `exec` with their
      provenance. Asserted by `TestServiceModeDefaultsOff` and the existing
      `procoder config` report tests; fails if the default is anything but
      `off`, or if either key is printed without its source.
- [ ] [S-12] A command that reports findings returns them in `result` with
      file, line, message, blocking and domain, and the same response's
      `stdout` is byte-identical to what the CLI prints for the same input.
      Asserted by `TestFindingsResultMatchesBytes`; fails if a finding
      reaches the bytes but not the result, or the reverse, and fails if
      building the result changes a single byte of the prose.
- [ ] [S-12] A command that reports no findings carries `"findings": []`,
      and a command that does not report findings at all carries
      `"result": null`. Asserted by `TestEmptyFindingsIsNotNull`; fails if
      the two are rendered the same way.
- [ ] [S-13] `status`, `config`, `todo list`, `spec check`, the `index`
      queries and `version` each return a typed result with a declared
      `kind`, and every declared kind has exactly one declaration.
      Asserted by `TestTypedResultsAreDeclaredOnce`; fails if a command
      invents a kind that is not declared, or two commands declare the same
      kind with different fields.
- [ ] [S-14] A request carrying a confirmation reaches the asking path of
      all six asking commands, and the same request without one takes the
      non-interactive path and says so in the response. Asserted by
      `TestConfirmationReachesTheAskingPath`; fails if a command asks
      without a confirmation, or ignores one that was sent.
- [ ] [S-14] A confirmation sent to a command that never asks is ignored
      and the command behaves as it does without one. Asserted by
      `TestUnusedConfirmationIsIgnored`; fails if such a request is refused.
- [ ] [S-11] `procoder init` asks whether this machine runs the local CLI or
      the local server and writes the answer to `[service] mode`; declining
      writes nothing. Asserted by `TestInitAsksAboutTheServer`; fails if init
      writes the key without asking, or writes it after a decline.

## Open questions

<!-- Empty deliberately: any non-empty line in this section counts as an
     open question, "None." included.

     The four questions this design left open are settled in the sections
     above — the index
     is held in memory per repository and evicted per repository with the
     daemon persisting (S-8), `procoder init` asks rather than assumes
     (S-11), and `self-upgrade` stops the daemon explicitly (S-7). The
     executing boundary is settled by S-6 and the response shape for
     long-running commands by S-5. -->
