# command-api — implementation plan

Status: complete
Spec: .procoder/specs/command-api.md

## Goal

Serve every procoder command over a local socket, with a typed result
beside the human bytes, without changing what any command decides or
requiring a daemon for any of them.

## Architecture

`cmd/procoder/main.go` already funnels all 112 dispatch branches through
one `run(args []string) int`. That function is the whole seam: it is given a
session — stdin, stdout, stderr, working directory, environment — instead of
reaching for process globals, and everything above it becomes a matter of
who constructs the session. The CLI constructs one from the process; the
daemon constructs one from a request envelope.

A new package `internal/api` owns the envelope, the typed results and the
socket on both ends. It depends on `cmd/procoder` for nothing: the daemon
calls a runner function the main package registers with it, so the import
runs one way only.

## Constraints

Every task inherits these, taken from the spec.

- **Zero dependencies.** The module file go.mod has no require block and no
  task may add one. The socket, the envelope encoding and the job table are
  stdlib.
- **P-CONTROL.** The daemon acquires no right the binary lacked. It is
  transport, not permission.
- **#201's boundary holds structurally.** The hook transport and the
  executing commands never share a door. `internal/hook/noexec_test.go`
  asserts no file in the hook package imports `internal/runcmd`; the client
  transport inherits the same assertion.
- **The 0600 socket authenticates the user, not the process.** Every process
  running as that user can reach the work socket. Nothing served there may
  do what #201 forbids doing unattended.
- **No behaviour change.** The same inputs produce the same bytes on stdout
  and the same exit code, in-process and over the socket. Task 15 asserts
  it for every command.
- **A hook must never wedge a session.** No client wait is unbounded, and a
  daemon that cannot be reached costs the caller nothing but the in-process
  path.
- **Windows, macOS, Linux.** Paths compared and sorted as repo-relative
  slash-separated strings. The named pipe on Windows carries the same
  envelope as the unix socket elsewhere.
- **Literal values, fixed here, used verbatim by every task:**
  - protocol version: `1`
  - run directory: `~/.procoder/run/`, mode `0700`
  - work socket: `~/.procoder/run/procoder.sock`, mode `0600`
  - exec socket: `~/.procoder/run/procoder-exec.sock`, mode `0600`
  - auto-start lock: `~/.procoder/run/start.lock`
  - client dial timeout: 250ms; client request timeout: the caller's, never
    unbounded
  - default idle window per repository: 30 minutes
  - maximum request size: 8 MiB

## Task 1: The session — cwd and environment stop being process-global

Files:

- `internal/host/host.go` — `Detect` takes an environment.
- `internal/doctor/doctor.go` — `Root` takes a working directory.
- `internal/format/format.go` — the relative-path branch takes one too.
- `internal/hook/commit.go` — `commitDir` takes one too.
- `cmd/procoder/main.go` — `run` takes a session and passes it down.
- `internal/host/host_test.go`, `cmd/procoder/main_test.go` — the tests.

Interfaces this task produces, consumed by every later task:

```go
// internal/host
type Env map[string]string
func DetectIn(env Env) Host   // new: reads only what it is given
func Detect() Host            // kept: DetectIn(host.ProcessEnv())
func ProcessEnv() Env         // the keys procoder reads, from os.Getenv

// cmd/procoder
type session struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	cwd    string
	env    host.Env
}
func (s session) out(line string)      // replaces the package-level printLine
func (s session) root() string         // replaces doctor.Root()
func run(args []string, s session) int // was run(args []string) int
func processSession() session          // os.Stdin/Stdout/Stderr, os.Getwd, ProcessEnv
```

- [ ] Write the failing test `TestDetectInReadsOnlyItsArgument` in
      `internal/host/host_test.go`: call
      `host.DetectIn(host.Env{"QODER_SESSION_ID": "x"})` with
      `t.Setenv("CLAUDE_PLUGIN_ROOT", "/whatever")` set, and assert the
      result is the Qoder host. Run `go test ./internal/host/` and
      expect FAIL with "undefined: host.DetectIn".
- [ ] Add `type Env map[string]string`, `func ProcessEnv() Env` reading the
      eight keys procoder reads (`COPILOT_PLUGIN_DATA`, `PLUGIN_DATA`,
      `QODER_SESSION_ID`, `PI_CODING_AGENT`, `CLAUDE_PLUGIN_ROOT`,
      `VIRTUAL_ENV`, `PROCODER_PUPPETEER_CONFIG`, `PATH`), and
      `func DetectIn(env Env) Host` holding the body `Detect` has today
      with every `os.Getenv(k)` replaced by `env[k]`. Leave
      `func Detect() Host { return DetectIn(ProcessEnv()) }`.
- [ ] Run `go test ./internal/host/` and expect PASS.
- [ ] Write the failing test `TestRunUsesTheSessionNotTheProcess` in
      `cmd/procoder/main_test.go`: build a session whose `cwd` is a temp
      directory containing a `.git` directory, whose `stdout` is a
      `bytes.Buffer`, and call `run([]string{"config"}, s)`; assert the
      buffer is non-empty and that nothing was written to `os.Stdout`
      (captured by swapping it for a pipe). Run
      `go test ./cmd/procoder/` and expect FAIL with "too many arguments
      in call to run".
- [ ] Add the `session` struct with `out`, `root`, and `processSession`.
      Change `func run(args []string) int` to `func run(args []string, s
session) int`; change `main` to `os.Exit(run(os.Args[1:],
processSession()))`.
- [ ] Replace every `printLine` argument inside `run` with `s.out`, every
      `doctor.Root()` with `s.root()`, every `os.Stdin` with `s.stdin`,
      every `os.Stdout` with `s.stdout` and every `os.Stderr` with
      `s.stderr`. Delete the package-level `printLine`. Point
      `doctor.Root`'s remaining callers at `doctor.RootOf(cwd)` and keep
      `Root()` as `RootOf(wd)` from `os.Getwd`; do the same for the
      `os.Getwd` branches in `format.go:170` and `commit.go:186`, each
      taking the directory as a parameter from its caller.
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `refactor: the session, not the process, says where and who`.

## Task 2: The envelope

Files:

- `internal/api/envelope.go` — the request and response types and their
  encoding.
- `internal/api/result.go` — the `Result` and `Finding` types the response
  field is declared as. Declared here rather than in Task 4 because
  `Response.Result` does not compile without them; Task 4 fills the value,
  this task only names the shape.
- `internal/api/envelope_test.go` — the tests.

Interfaces this task produces:

```go
// internal/api
const Protocol = 1

type Request struct {
	Protocol int               `json:"protocol"`
	Argv     []string          `json:"argv"`
	Cwd      string            `json:"cwd"`
	Env      map[string]string `json:"env"`
	Stdin    []byte            `json:"stdin"`   // base64 by encoding/json
	Confirm  *string           `json:"confirm"` // nil = no person
	Job      string            `json:"job"`     // set = poll, argv empty
}

type Response struct {
	Protocol int     `json:"protocol"`
	Exit     *int    `json:"exit"` // nil while a job runs
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	Job      *Job    `json:"job"`
	Result   *Result `json:"result"`
}

const MaxRequestBytes = 8 << 20

func WriteRequest(w io.Writer, r Request) error
func ReadRequest(r io.Reader) (Request, error)
func WriteResponse(w io.Writer, r Response) error
func ReadResponse(r io.Reader) (Response, error)
```

- [ ] Write the failing test `TestEnvelopeRoundTrips` in
      `internal/api/envelope_test.go`: write a `Request` with every field
      set (including `Stdin: []byte{0x00, 0xff}` and a non-nil `Confirm`)
      into a `bytes.Buffer`, read it back, and assert `reflect.DeepEqual`.
      Run `go test ./internal/api/` and expect FAIL with "no required
      module provides package procoder/internal/api".
- [ ] Write `envelope.go` with the types above, each encoded as one JSON
      object followed by `\n`, read with a `bufio.Scanner` whose buffer is
      capped at `MaxRequestBytes`.
- [ ] Run `go test ./internal/api/` and expect PASS.
- [ ] Write the failing test `TestRequestOverTheCapIsRefused`: encode a
      request whose `Stdin` is `MaxRequestBytes+1` bytes, read it, and
      assert the error names the limit `8388608`. Run
      `go test ./internal/api/` and expect FAIL with "want an error, got
      nil".
- [ ] Return an error from `ReadRequest` when the scanner reports a token
      too long, worded `procoder: request over the 8388608-byte limit`.
- [ ] Run `go test ./internal/api/` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: one envelope every command answers in`.

## Task 3: The in-process runner behind the envelope

Files:

- `internal/api/runner.go` — the `Runner` type the daemon calls.
- `cmd/procoder/api.go` — the main package's registration of its `run`.
- `internal/api/runner_test.go`, `cmd/procoder/api_test.go` — the tests.

Interfaces this task produces:

```go
// internal/api
type Runner func(req Request, stdout, stderr io.Writer) int
func Serve(req Request, run Runner) Response // builds the Response, no socket

// cmd/procoder
func apiRunner(req api.Request, stdout, stderr io.Writer) int // adapts run()
```

- [ ] Write the failing test `TestServeCapturesBothStreams` in
      `internal/api/runner_test.go`: call `api.Serve` with a `Runner` that
      writes `"out"` to stdout, `"err"` to stderr and returns 3; assert the
      response has `Stdout == "out"`, `Stderr == "err"` and `*Exit == 3`.
      Run `go test ./internal/api/` and expect FAIL with "undefined:
      api.Serve".
- [ ] Write `Serve`: two `bytes.Buffer`s, call the runner, fill the
      response, set `Protocol: Protocol`.
- [ ] Run `go test ./internal/api/` and expect PASS.
- [ ] Write the failing test `TestApiRunnerMatchesTheCLI` in
      `cmd/procoder/api_test.go`: run `config` twice against the same temp
      repository — once through `run(args, processSession())` with stdout
      swapped for a buffer, once through
      `api.Serve(api.Request{Argv: []string{"config"}, Cwd: dir}, apiRunner)`
      — and assert the two stdout strings are identical. Run
      `go test ./cmd/procoder/` and expect FAIL with "undefined:
      apiRunner".
- [ ] Write `apiRunner` in `cmd/procoder/api.go`: build a `session` from
      the request's `cwd`, `env` and `stdin` with the given writers, and
      return `run(req.Argv, s)`.
- [ ] Run `go test ./cmd/procoder/` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: a command answers a request without a socket`.

## Task 4: The findings result

Files:

- `cmd/procoder/main.go` — `printFindings` also collects.
- `internal/api/result_test.go`, `cmd/procoder/api_test.go` — the tests.

Interfaces this task produces:

```go
// internal/api
type Finding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
	Domain   string `json:"domain"`
}
type Result struct {
	Kind     string     `json:"kind"`
	Findings []Finding  `json:"findings,omitempty"`
}
const KindFindings = "findings"

// cmd/procoder — the sink printFindings writes to when one is set
type collector struct{ findings []api.Finding }
func (s session) collect(domain string, f []gitx.Finding)
```

- [ ] Write the failing test `TestFindingsResultMatchesBytes` in
      `cmd/procoder/api_test.go`: in a temp repository containing one
      unformatted file, serve `check` through `apiRunner` and assert the
      response's `Result.Kind` is `"findings"`, that the findings list has
      one entry whose `File` is that file and whose `Blocking` is true, and
      that `Stdout` is byte-identical to the same command run through
      `run`. Run `go test ./cmd/procoder/` and expect FAIL with "Result is
      nil".
- [ ] Give `session` a `*collector` (nil for the CLI) and have
      `printFindings` append to it as well as writing its lines, taking the
      `label` it already receives as the domain. Fill `Response.Result`
      from the collector in `Serve` via a `Result() *Result` the runner
      returns; a nil collector yields a nil result.
- [ ] Run `go test ./cmd/procoder/` and expect PASS.
- [ ] Write the failing test `TestEmptyFindingsIsNotNull`: serve `check` in
      a clean repository and assert `Result.Kind == "findings"` with
      `len(Result.Findings) == 0`; serve `version` and assert
      `Result == nil`. Run `go test ./cmd/procoder/` and expect FAIL with
      "want a findings result, got nil".
- [ ] Set the collector on every command that calls `printFindings` and
      leave it unset elsewhere, so the two cases render differently.
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: findings reach a caller as findings, not as prose`.

## Task 5: The typed results that are not findings

Files:

- `internal/api/kinds.go` — one declaration per non-findings kind.
- `cmd/procoder/main.go` — the three commands fill theirs.
- `internal/api/kinds_test.go` — the declaration guard.

Three commands, not six. `config`, `todo` and `version` already compute
the values they print — `cfg.Settings` carries key, value, source and the
default it was relaxed from; `todo.List` returns `[]todo.Task`; the
version pair is two strings. Filling a typed result for those is reading a
value that exists.

`status`, `spec check` and the `index` queries are line-oriented all the
way down: `status.Report` returns `[]string` and its branch, dirty and
sprint lines are formatted inside six helpers; `spec.Check` and
`codeindex.Find` take an `out func(string)` and never hold a value.
Giving those three a typed result means restructuring their domains to
build data and render from it, which is a different change from adding an
envelope — and doing it here would mean either parsing our own output back
(fragile, and a second place to keep in step) or duplicating the git calls
`status` already makes. Task 16 does it properly.

Interfaces this task produces:

```go
// internal/api
type Setting struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Source  string `json:"source"`
	Relaxed bool   `json:"relaxed"`
	Default string `json:"default,omitempty"`
}
type Task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	State string `json:"state"`
}
type Version struct {
	Running string `json:"running"`
	Latest  string `json:"latest,omitempty"`
}
// Result gains, each omitempty: Settings []Setting, Tasks []Task, Version *Version
const (
	KindConfig  = "config"
	KindTodo    = "todo"
	KindVersion = "version"
)
func Kinds() []string // every declared kind, for the guard
```

- [ ] Write the failing test `TestTypedResultsAreDeclaredOnce` in
      `internal/api/kinds_test.go`: assert `api.Kinds()` has no duplicate
      entries, and that every `Kind*` constant declared in the package
      appears in it — found by parsing the package's own non-test files
      with `go/parser` and collecting the `Kind`-prefixed constant names.
      Run `go test ./internal/api/` and expect FAIL with "undefined:
      api.Kinds".
- [ ] Write `kinds.go` with the three types, the three constants, the
      three `Result` fields and `Kinds()` returning all four kind strings
      including `KindFindings`.
- [ ] Run `go test ./internal/api/` and expect PASS.
- [ ] Write the failing test `TestConfigResultCarriesTheSettings` in
      `cmd/procoder/api_test.go`: serve `config` in a fixture repository
      whose `.procoder/config.toml` sets `max_file_mb = 10`, and assert
      `Result.Kind == "config"`, that a setting with `Key` `git.max_file_mb`
      is present with `Value` `10`, and that its `Relaxed` is true. Run
      `go test ./cmd/procoder/` and expect FAIL with "Result is nil".
- [ ] Fill the result in the `config`, `todo list` and `version` branches
      from the values those commands already compute, without changing a
      line they print. `session.col` gains a `kind` and the three typed
      slices, so a command sets one or the other and never both.
- [ ] Write the failing test `TestTodoResultListsTheTasks`: serve
      `todo list` in a fixture repository holding two task files and assert
      `Result.Kind == "todo"` with two entries carrying their ids and
      states. Run `go test ./cmd/procoder/` and expect FAIL with "want 2
      tasks, got 0".
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: the commands whose answer is not a finding`.

## Task 6: The confirmation

Files:

- `cmd/procoder/api.go` — the request's confirmation reaches the commands.
- `internal/copilot/prompt.go` — an answer may be supplied.
- `cmd/procoder/api_test.go`, `internal/copilot/prompt_test.go` — the tests.

Interfaces this task produces:

```go
// internal/copilot
func CanAskWith(in *os.File, supplied *string) bool // true when supplied != nil
func ReadYesFrom(in io.Reader, supplied *string) int
```

- [ ] Write the failing test `TestSuppliedAnswerCountsAsAsking` in
      `internal/copilot/prompt_test.go`: call
      `copilot.CanAskWith(nil, ptr("yes"))` and assert true; call
      `copilot.CanAskWith(nil, nil)` and assert false. Run
      `go test ./internal/copilot/` and expect FAIL with "undefined:
      copilot.CanAskWith".
- [ ] Add `CanAskWith` and `ReadYesFrom`, leaving `CanAsk` and `ReadYes` as
      the `nil`-supplied calls so no existing caller changes.
- [ ] Run `go test ./internal/copilot/` and expect PASS.
- [ ] Write the failing test `TestConfirmationReachesTheAskingPath` in
      `cmd/procoder/api_test.go`: serve `version --check` with
      `Confirm: ptr("no")` and assert the output contains the declined
      line rather than the "no terminal" line; serve it with `Confirm: nil`
      and assert the reverse. Run `go test ./cmd/procoder/` and expect FAIL
      with "want the declined line".
- [ ] Carry `req.Confirm` on the session and pass it to the six asking
      commands at their `CanAsk`/`ReadYes` call sites.
- [ ] Write the failing test `TestUnusedConfirmationIsIgnored`: serve
      `config` with `Confirm: ptr("yes")` and assert the response is
      identical to serving it with `Confirm: nil`. Run
      `go test ./cmd/procoder/` and expect PASS once the previous step is
      in (it asserts the absence of a refusal, so it must not fail first).
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: a caller may answer what a terminal would have`.

## Task 7: `procoder serve` on the work socket

Files:

- `internal/api/server.go` — the listener and the accept loop.
- `internal/api/paths.go` — where the sockets live.
- `cmd/procoder/main.go` — the `serve` dispatch branch and its usage line.
- `internal/api/server_test.go` — the tests.

Interfaces this task produces:

```go
// internal/api
func RunDir() (string, error)        // ~/.procoder/run, created 0700
func WorkSocket() (string, error)    // <RunDir>/procoder.sock
func ExecSocket() (string, error)    // <RunDir>/procoder-exec.sock
type Server struct{ Run Runner; Version string }
func (s *Server) Listen(path string) (net.Listener, error) // socket at 0600
func (s *Server) Accept(l net.Listener)                    // serves until closed
```

- [ ] Write the failing test `TestServeSocketPermissions` in
      `internal/api/server_test.go`: listen on a path inside `t.TempDir()`,
      stat it, and assert the mode is `0600` and the parent is `0700`. Run
      `go test ./internal/api/` and expect FAIL with "undefined:
      api.Server".
- [ ] Write `paths.go` and `server.go`: `Listen` unlinks a stale path,
      calls `net.Listen("unix", path)`, then `os.Chmod(path, 0o600)`;
      `Accept` reads one request per connection, calls `Serve`, writes the
      response, closes.
- [ ] Run `go test ./internal/api/` and expect PASS.
- [ ] Write the failing test `TestSocketExitMatchesInProcess`: serve
      `config` over a real socket and in-process, and assert the exit codes
      and stdout are identical. Run `go test ./internal/api/` and expect
      FAIL until `Accept` is wired.
- [ ] Add the `serve` branch to `run` — `procoder serve [--socket <path>]
[--idle <duration>]` — printing the socket path and the version, then
      accepting until closed.
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: procoder serve, on a socket only its user can open`.

## Task 8: The client, the handshake, and the fallback that never costs an answer

Files:

- `internal/api/client.go` — dial, send, receive, give up.
- `cmd/procoder/main.go` — commands try the socket when `mode = "local"`.
- `internal/api/client_test.go` — the tests.

Interfaces this task produces:

```go
// internal/api
type Client struct{ Path, Version string; Dial time.Duration }
func (c Client) Do(req Request) (Response, error) // error = run in-process
var ErrNoDaemon = errors.New("procoder: no daemon")
var ErrVersionSkew = errors.New("procoder: daemon version differs")
```

- [ ] Write the failing test `TestVersionSkewFallsBackInProcess` in
      `internal/api/client_test.go`: start a server reporting version
      `"9.9.9"`, dial with a client whose version is `"1.0.0"`, and assert
      `Do` returns `ErrVersionSkew`. Run `go test ./internal/api/` and
      expect FAIL with "undefined: api.Client".
- [ ] Write `client.go`: dial with a 250ms timeout, send the request with
      the client's version in the envelope, refuse on a mismatched
      response version, return `ErrNoDaemon` when the socket is absent or
      dead.
- [ ] Run `go test ./internal/api/` and expect PASS.
- [ ] Write the failing test `TestDeadSocketCostsNothing`: create a socket
      file with no listener, call `Do`, and assert `ErrNoDaemon` inside
      500ms. Run `go test ./internal/api/` and expect PASS once the dial
      timeout is in.
- [ ] In `run`, when `[service] mode = "local"`, try the client first and
      fall through to the in-process path on any error, writing the reason
      to stderr.
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: a daemon that cannot be reached costs only its speed`.

## Task 9: Per-root serialisation

Files:

- `internal/api/serialise.go` — one queue per repository identity.
- `internal/api/serialise_test.go` — the contention test.

Interfaces this task produces:

```go
// internal/api
type queues struct{ mu sync.Mutex; m map[string]*sync.Mutex }
func (q *queues) do(identity string, f func())
// Server gains: Identity func(cwd string) string
```

- [ ] Write the failing test `TestPerRootSerialisation` in
      `internal/api/serialise_test.go`: start a server against one temp
      repository, fire fifty concurrent requests that each append to the
      claims ledger, and assert all fifty return exit 0, that none of the
      responses contains `"the write was NOT made"`, and that the ledger
      holds fifty entries. Run `go test ./internal/api/` and expect FAIL
      with fifty entries wanted and fewer found.
- [ ] Write `serialise.go` and have `Accept` wrap each request in
      `queues.do(identity)`, with the identity from
      `store.IdentityFor(root, cfgRepo).Key`.
- [ ] Run `go test -race ./internal/api/` and expect PASS.
- [ ] Run `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `fix: one repository, one queue — the store never fights itself`.

## Task 10: Jobs, for the commands that outlive a request

Files:

- `internal/api/job.go` — the job table.
- `internal/api/job_test.go` — the tests.

Interfaces this task produces:

```go
// internal/api
type Job struct {
	ID    string `json:"id"`
	State string `json:"state"` // running | done | lost
}
type jobs struct{ mu sync.Mutex; m map[string]*jobState }
func (j *jobs) start(req Request, run Runner, identity string) Job
func (j *jobs) poll(id string) Response
func IsLongRunning(argv []string) bool
```

- [ ] Write the failing test `TestIsLongRunning` in
      `internal/api/job_test.go`: assert true for `test`, `audit`,
      `security --deep`, `index build`, `release`, `bench`, `deps`,
      `docs --external` and `ci --runs`; assert false for `config`,
      `status`, `security` and `docs`. Run `go test ./internal/api/` and
      expect FAIL with "undefined: api.IsLongRunning".
- [ ] Write `IsLongRunning` matching the command and the flag that makes it
      long, per the list above.
- [ ] Run `go test ./internal/api/` and expect PASS.
- [ ] Write the failing test `TestJobSurvivesItsConnection`: submit a
      request whose runner sleeps 200ms then writes `"done"`, assert the
      response returns a job id with `Exit == nil` in under one second,
      close the connection, then poll the id until `State == "done"` and
      assert the output is `"done"` and the exit code is the runner's. Run
      `go test ./internal/api/` and expect FAIL with "want a job, got nil".
- [ ] Write `job.go`: a map of id to state, the runner in a goroutine
      writing into a mutex-guarded buffer, `poll` returning what has
      accumulated and the exit code once there is one, and an unknown id
      answering `State: "lost"`.
- [ ] Run `go test -race ./internal/api/` and expect PASS.
- [ ] Write the failing test `TestLostJobSaysLostNotFailed`: poll an id
      that was never issued and assert `State == "lost"` and `Exit == nil`.
      Run `go test ./internal/api/` and expect PASS.
- [ ] Run `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: start the suite and get on with something else`.

## Task 11: The exec socket

Files:

- `internal/api/exec.go` — which commands execute, and which door serves
  them.
- `internal/api/exec_test.go` — the refusal and the structural guard.
- `internal/config/config.go` — the `[service] exec` key.

Interfaces this task produces:

```go
// internal/api
func Executes(argv []string) bool // run --exec, evidence record, init --yes, self-upgrade
// Server gains: Exec bool  // true only for the exec socket's server
```

- [ ] Write the failing test `TestExecutesNamesTheFour` in
      `internal/api/exec_test.go`: assert true for
      `run --exec`, `evidence record echo`, `init --yes` and
      `self-upgrade`; assert false for `run`, `evidence`, `init` and
      `check`. Run `go test ./internal/api/` and expect FAIL with
      "undefined: api.Executes".
- [ ] Write `Executes` matching exactly those four argv shapes.
- [ ] Run `go test ./internal/api/` and expect PASS.
- [ ] Write the failing test `TestExecutingCommandsRefusedOnWorkSocket`:
      send `run --exec` to a server whose `Exec` is false and assert exit 2
      with stderr containing `procoder: run --exec is not served here` and
      the exec socket's path; send it to a server whose `Exec` is true and
      assert it runs. Run `go test ./internal/api/` and expect FAIL with
      "want exit 2, got 0".
- [ ] Refuse in `Accept` when `Executes(req.Argv) && !s.Exec`, and start the
      second listener from `serve` only when `[service] exec = true`.
- [ ] Write the failing test `TestClientTransportExecutesNothing` in
      `internal/api/exec_test.go`, in the shape
      `internal/hook/noexec_test.go` uses: parse every non-test file in
      `internal/api` with `go/parser` and assert none imports
      `procoder/internal/runcmd`, naming the file that does. Run
      `go test ./internal/api/` and expect PASS.
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: the executing four have their own door, or none`.

## Task 12: Warm state, per-repository eviction, and the exit

Files:

- `internal/api/warm.go` — the per-repository cache and its clock.
- `internal/api/warm_test.go` — the tests.

Interfaces this task produces:

```go
// internal/api
type warm struct{ mu sync.Mutex; m map[string]*repoState; window time.Duration }
func (w *warm) get(identity string) *repoState
func (w *warm) evict(now time.Time) (held int)
// Server gains: Idle time.Duration  // default 30 * time.Minute
```

- [ ] Write the failing test `TestPerRepoEviction` in
      `internal/api/warm_test.go`: warm two identities, advance the clock
      past the window for one of them only, call `evict`, and assert one
      is held and the other is gone. Run `go test ./internal/api/` and
      expect FAIL with "undefined: api.warm".
- [ ] Write `warm.go` holding one `repoState` per identity — the parsed
      config and the in-memory code index — each with its own last-used
      time, evicted independently.
- [ ] Run `go test -race ./internal/api/` and expect PASS.
- [ ] Write the failing test `TestDaemonExitsHoldingNothing`: run a server
      with a 50ms idle window, make one request, wait, and assert `Accept`
      returns within 500ms of the last eviction. Run
      `go test ./internal/api/` and expect FAIL with "server still
      accepting".
- [ ] Have `Accept` stop when `evict` reports zero held.
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: a daemon holding nothing does not stay`.

## Task 13: Auto-start, exactly once

Files:

- `internal/api/start.go` — the single-flight start.
- `internal/principles/principles.go` — SessionStart starts it.
- `internal/api/start_test.go` — the race test.

Interfaces this task produces:

```go
// internal/api
func EnsureDaemon(bin string) error // starts one if the socket is dead
```

- [ ] Write the failing test `TestAutoStartIsSingleFlight` in
      `internal/api/start_test.go`: call `EnsureDaemon` from ten goroutines
      against a fixture binary that appends its pid to a file, and assert
      the file holds exactly one line. Run `go test ./internal/api/` and
      expect FAIL with "undefined: api.EnsureDaemon".
- [ ] Write `start.go`: take `~/.procoder/run/start.lock` with
      `O_CREATE|O_EXCL`, judged stale on the rule `internal/store` uses,
      spawn `bin serve` detached, wait for the socket to answer, release.
- [ ] Run `go test -race ./internal/api/` and expect PASS.
- [ ] Call `EnsureDaemon` from `principles.RunHook` when
      `[service] mode = "local"`, ignoring its error — a hook that cannot
      start a daemon still prints its principles.
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: the session that finds no daemon starts one`.

## Task 14: The two config keys, and the question `init` asks

Files:

- `internal/config/config.go` — `mode` and `exec`.
- `internal/config/report.go` — both printed with their provenance.
- `internal/initcmd/initcmd.go` — the question.
- `internal/config/config_test.go`, `internal/initcmd/initcmd_test.go`.

Interfaces this task produces:

```go
// internal/config
func (c Config) ServiceMode() string // "off" | "local", default "off"
func (c Config) ServiceExec() bool   // default false
```

- [ ] Write the failing test `TestServiceModeDefaultsOff` in
      `internal/config/config_test.go`: load a config with no `[service]`
      block and assert `ServiceMode() == "off"` and `ServiceExec() == false`;
      load one with `mode = "local"` and assert it reads back. Run
      `go test ./internal/config/` and expect FAIL with "undefined:
      ServiceMode".
- [ ] Add both keys, defaulting as above, and add both to the report with
      their source in the shape `service.repo` already uses.
- [ ] Run `go test ./internal/config/` and expect PASS.
- [ ] Write the failing test `TestInitAsksAboutTheServer` in
      `internal/initcmd/initcmd_test.go`: run init with a supplied answer
      of `"server"` and assert `[service] mode = "local"` is written; run it
      with `"cli"` and assert nothing is written to `config.toml`. Run
      `go test ./internal/initcmd/` and expect FAIL with "config.toml
      unchanged".
- [ ] Ask the question in `init` through the same `CanAskWith` path Task 6
      added, so a non-interactive init writes nothing.
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: init asks whether this machine runs the server`.

## Task 15: Parity, over every command there is

Files:

- `cmd/procoder/parity_test.go` — the table and the assertion.

Interfaces this task consumes: `run`, `processSession`, `apiRunner`,
`api.Serve`, `api.Executes` — all from Tasks 1, 3 and 11.

- [ ] Write the failing test `TestParityAcrossEveryCommand` in
      `cmd/procoder/parity_test.go`: read the command names out of the
      `usage` constant with the same `^  [a-z][a-z-]+` shape the usage text
      uses, and for each one run it in a fixture repository twice — once
      through `run(argv, s)` with buffers, once through
      `api.Serve(api.Request{Argv: argv, Cwd: dir, Env: env}, apiRunner)` —
      asserting the exit code, stdout and stderr are byte-identical. For a
      command where `api.Executes(argv)` is true, assert the socket path
      refuses with exit 2 instead of comparing. Run
      `go test ./cmd/procoder/` and expect FAIL for any command whose bytes
      differ.
- [ ] Fix whatever the table finds, one command at a time, changing the
      transport and never the command.
- [ ] Add the environment case: run `principles --hook` twice with
      `Env: {"CLAUDE_PLUGIN_ROOT": "..."}` and
      `Env: {"QODER_SESSION_ID": "x"}` against one server process, and
      assert the two outputs are the two hosts' different JSON shapes. Run
      `go test ./cmd/procoder/` and expect FAIL if `host.Detect` reads the
      process again.
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `test: every command answers the same over either door`.

## Task 16: Structured answers for the three line-oriented commands

Files:

- `internal/status/status.go` — build a value, render from it.
- `internal/spec/spec.go` — `Check` returns its verdict as well as printing.
- `internal/codeindex/query.go` — `Find` and `Search` return their hits.
- `cmd/procoder/main.go` — the three branches fill their results.
- `internal/api/kinds.go` — three more kinds.
- `internal/status/status_test.go`, `internal/spec/spec_test.go`,
  `internal/codeindex/query_test.go`, `cmd/procoder/api_test.go`.

Interfaces this task produces:

```go
// internal/status
type Data struct {
	Branch  string
	Default string
	Dirty   []string
	Sprint  string
	Open    int
}
func Data(root string) Data   // what report() builds; Report renders it

// internal/spec
type Verdict struct{ Name, Verdict string; Gaps []string }
func CheckVerdict(root, name string) Verdict

// internal/codeindex
type Hit struct{ Symbol, File string; Line int }
func FindHits(root, symbol string) []Hit
func SearchHits(root, query string) []Hit

// internal/api
const (KindStatus = "status"; KindSpec = "spec"; KindIndex = "index")
```

- [ ] Write the failing test `TestStatusDataCarriesTheBranch` in
      `internal/status/status_test.go`: build a fixture repository on a
      branch named `probe` and assert `status.Data(root).Branch == "probe"`.
      Run `go test ./internal/status/` and expect FAIL with "undefined:
      status.Data".
- [ ] Restructure `report` to fill a `Data` and render its lines from it,
      so the value and the text cannot disagree. `Report` keeps its
      signature and its exact output; `TestReportStaysInsideTheBudget` and
      the existing content tests must pass unchanged.
- [ ] Run `go test ./internal/status/` and expect PASS.
- [ ] Write the failing test `TestCheckVerdictNamesItsGaps` in
      `internal/spec/spec_test.go`: run `CheckVerdict` against a spec with
      one unanswered question and assert `Verdict == "NOT ready"` with one
      gap naming the open question. Run `go test ./internal/spec/` and
      expect FAIL with "undefined: spec.CheckVerdict".
- [ ] Have `Check` call `CheckVerdict` and render it, so the printed lines
      and the verdict are the same computation.
- [ ] Run `go test ./internal/spec/` and expect PASS.
- [ ] Write the failing test `TestFindHitsCarriesTheLocation` in
      `internal/codeindex/query_test.go`: build an index over a fixture
      holding one known symbol and assert `FindHits` returns one hit with
      that file and a line above zero. Run `go test ./internal/codeindex/`
      and expect FAIL with "undefined: codeindex.FindHits".
- [ ] Have `Find` and `Search` render the hits their new siblings return.
- [ ] Write the failing test `TestStatusSpecIndexResultsAreTyped` in
      `cmd/procoder/api_test.go`: serve each of the three and assert the
      kind and one field of each. Run `go test ./cmd/procoder/` and expect
      FAIL with "Result is nil".
- [ ] Run `go test ./...` and expect PASS, then
      `./dist/darwin-arm64/procoder check` and expect 0 blocking.
- [ ] Commit: `feat: status, spec and index answer with values too`.
