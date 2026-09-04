package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/api"
	"procoder/internal/host"
)

// fixtureRepo is a real git repository in a temp directory: the commands
// under test read git, and a bare .git directory is not enough for the
// ones that ask it questions.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "T"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git is not usable here: %v (%s)", err, out)
		}
	}
	// A repository with no .procoder/ is somebody else's: the gate runs in
	// its universal scope and checks no formatting at all (ADR 0005). The
	// fixture adopts procoder, because what these tests are about is the
	// full gate.
	if err := os.MkdirAll(filepath.Join(dir, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The same command, asked at either door, answers the same bytes.
//
// This is the whole promise of the second door, asserted on one command
// here and on every command in parity_test.go. It is here as well because
// this is the test that fails first when apiRunner stops building the
// session the request described.
//
// proved by: having apiRunner ignore req.Cwd — the two runs then read two
// different repositories and their config output diverges.
func TestApiRunnerMatchesTheCLI(t *testing.T) {
	dir := fixtureRepo(t)
	env := host.Env{}

	var direct bytes.Buffer
	directCode := run([]string{"config"}, session{
		stdin: strings.NewReader(""), stdout: &direct, stderr: &bytes.Buffer{}, cwd: dir, env: env,
	})

	res := api.Serve(api.Request{Protocol: api.Protocol, Argv: []string{"config"}, Cwd: dir}, apiRunner)

	if res.Exit == nil || *res.Exit != directCode {
		t.Fatalf("exit codes differ: in-process %d, over the envelope %v", directCode, res.Exit)
	}
	if res.Stdout != direct.String() {
		t.Fatalf("stdout differs:\n in-process: %q\n envelope:   %q", direct.String(), res.Stdout)
	}
}

// The request's environment is what the command sees, and the daemon's own
// is not consulted. One process answers for two hosts.
//
// proved by: pointing host.Detect back at os.Getenv — both requests then
// answer in the shape of whatever host started the test binary.
func TestRequestEnvironmentPicksTheHost(t *testing.T) {
	dir := fixtureRepo(t)
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("QODER_SESSION_ID", "")

	qoder := api.Serve(api.Request{
		Protocol: api.Protocol, Argv: []string{"principles", "--hook"}, Cwd: dir,
		Env: map[string]string{"QODER_SESSION_ID": "s"},
	}, apiRunner)
	claude := api.Serve(api.Request{
		Protocol: api.Protocol, Argv: []string{"principles", "--hook"}, Cwd: dir,
	}, apiRunner)

	if qoder.Stdout == claude.Stdout {
		t.Fatal("two hosts got the same envelope — the request's environment was not read")
	}
	if !strings.Contains(qoder.Stdout, "hookSpecificOutput") {
		t.Fatalf("the Qoder request did not get the envelope shape: %q", oneLineOf(qoder.Stdout))
	}
}

func oneLineOf(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

// A file the request never mentioned is never read: the session carries no
// handle, so the redirect check and the terminal check both answer nil.
func TestApiSessionHasNoFileHandles(t *testing.T) {
	dir := fixtureRepo(t)
	path := filepath.Join(dir, "x.md")
	if err := os.WriteFile(path, []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := api.Serve(api.Request{Protocol: api.Protocol, Argv: []string{"format", path}, Cwd: dir}, apiRunner)
	if res.Exit == nil {
		t.Fatal("format returned no exit code")
	}
	if strings.Contains(res.Stderr, "would empty") {
		t.Fatalf("the redirect check fired without a redirect: %q", res.Stderr)
	}
}

// The gate's verdict reaches a caller as findings, and the bytes it prints
// do not change because somebody collected them.
//
// proved by: dropping the collect call from gate.RunCollecting's
// formatting pass — the unformatted file is then printed and not reported,
// and a client acting on the result would call the tree clean.
func TestFindingsResultMatchesBytes(t *testing.T) {
	dir := fixtureRepo(t)
	bad := filepath.Join(dir, "bad.md")
	// Two headings on one line is something prettier will rewrite, so the
	// file is unformatted without being unparseable.
	if err := os.WriteFile(bad, []byte("#    ragged   heading\n\n\n\ntext\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var direct bytes.Buffer
	directCode := run([]string{"check", bad}, session{
		stdin: strings.NewReader(""), stdout: &direct, stderr: &bytes.Buffer{}, cwd: dir, env: host.Env{},
	})

	res := api.Serve(api.Request{
		Protocol: api.Protocol, Argv: []string{"check", bad}, Cwd: dir,
	}, apiRunner)

	if res.Stdout != direct.String() {
		t.Fatalf("collecting changed the bytes:\n in-process: %q\n envelope:   %q", direct.String(), res.Stdout)
	}
	if res.Exit == nil || *res.Exit != directCode {
		t.Fatalf("exit codes differ: %d vs %v", directCode, res.Exit)
	}
	if res.Result == nil {
		t.Fatal("the gate reported nothing to a caller that asked for data")
	}
	if res.Result.Kind != api.KindFindings {
		t.Fatalf("kind is %q, want %q", res.Result.Kind, api.KindFindings)
	}
	var found bool
	for _, f := range res.Result.Findings {
		if strings.HasSuffix(f.File, "bad.md") && f.Blocking {
			found = true
		}
	}
	if !found {
		t.Fatalf("the unformatted file is not in the result: %+v", res.Result.Findings)
	}
}

// An empty findings list and no result at all are different answers, and a
// client that flattened them would read a clean gate and a version number
// the same way.
func TestEmptyFindingsIsNotNull(t *testing.T) {
	dir := fixtureRepo(t)
	clean := api.Serve(api.Request{
		Protocol: api.Protocol, Argv: []string{"check", filepath.Join(dir, "nothing.md")}, Cwd: dir,
	}, apiRunner)
	if clean.Result == nil {
		t.Fatal("a gate run that found nothing returned no result at all")
	}
	if clean.Result.Findings == nil {
		t.Fatal("the findings list is nil rather than empty — a client cannot tell it from absent")
	}

	// principles prints text and computes no value of its own, so it is a
	// command with genuinely no result. version used to stand here and
	// acquired a kind of its own in Task 5, which is exactly the drift
	// this assertion is watching for.
	text := api.Serve(api.Request{Protocol: api.Protocol, Argv: []string{"principles"}, Cwd: dir}, apiRunner)
	if text.Result != nil {
		t.Fatalf("a command that reports nothing returned a result: %+v", text.Result)
	}
}

// config answers with its settings as values, and the loosened one says
// so — which is the whole point of the command, and a client reading only
// the table would have to parse a column to learn it.
func TestConfigResultCarriesTheSettings(t *testing.T) {
	dir := fixtureRepo(t)
	cfg := filepath.Join(dir, ".procoder", "config.toml")
	if err := os.WriteFile(cfg, []byte("[git]\nmax_file_mb = 10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := api.Serve(api.Request{Protocol: api.Protocol, Argv: []string{"config"}, Cwd: dir}, apiRunner)
	if res.Result == nil || res.Result.Kind != api.KindConfig {
		t.Fatalf("config did not answer in its own kind: %+v", res.Result)
	}
	var relaxed bool
	for _, set := range res.Result.Settings {
		if strings.Contains(set.Key, "max_file_mb") {
			if set.Value != "10" {
				t.Errorf("max_file_mb reads %q, want 10", set.Value)
			}
			relaxed = set.Relaxed
		}
	}
	if !relaxed {
		t.Errorf("a setting weaker than the default is not marked relaxed: %+v", res.Result.Settings)
	}
}

// todo list answers with the tasks, and an empty list is still the todo
// kind rather than no result — the list being empty is an answer.
func TestTodoResultListsTheTasks(t *testing.T) {
	dir := fixtureRepo(t)
	todoDir := filepath.Join(dir, ".procoder", "todo")
	if err := os.MkdirAll(todoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20260101-first.md", "20260102-second.md"} {
		body := "# " + name + "\n\nStatus: open\nCreated: 2026-01-01\n"
		if err := os.WriteFile(filepath.Join(todoDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res := api.Serve(api.Request{Protocol: api.Protocol, Argv: []string{"todo", "list"}, Cwd: dir}, apiRunner)
	if res.Result == nil || res.Result.Kind != api.KindTodo {
		t.Fatalf("todo list did not answer in its own kind: %+v", res.Result)
	}
	if len(res.Result.Tasks) != 2 {
		t.Fatalf("want 2 tasks, got %d: %+v", len(res.Result.Tasks), res.Result.Tasks)
	}
	for _, task := range res.Result.Tasks {
		if task.ID == "" || task.State == "" {
			t.Errorf("a task came back without an id or a state: %+v", task)
		}
	}
}

// version answers with what is running, and with no latest — nobody asked
// GitHub, and reporting the running version as the newest would be an
// answer this command did not compute.
func TestVersionResultDoesNotInventALatest(t *testing.T) {
	dir := fixtureRepo(t)
	res := api.Serve(api.Request{Protocol: api.Protocol, Argv: []string{"version"}, Cwd: dir}, apiRunner)
	if res.Result == nil || res.Result.Version == nil {
		t.Fatalf("version answered with no version: %+v", res.Result)
	}
	if res.Result.Version.Latest != "" {
		t.Errorf("a bare version reported a latest it never asked for: %q", res.Result.Version.Latest)
	}
}

// A caller who has an answer reaches the asking path; a caller who does
// not takes the path a command takes today with nothing on its stdin.
//
// proved by: having apiRunner drop req.Confirm — the confirmed request
// then takes the non-interactive path too, and the two outputs match.
func TestConfirmationReachesTheAskingPath(t *testing.T) {
	dir := fixtureRepo(t)
	no := "no"

	confirmed := api.Serve(api.Request{
		Protocol: api.Protocol, Argv: []string{"copilot-leak", "--from-copilot"},
		Cwd: dir, Confirm: &no,
	}, apiRunner)
	silent := api.Serve(api.Request{
		Protocol: api.Protocol, Argv: []string{"copilot-leak", "--from-copilot"}, Cwd: dir,
	}, apiRunner)

	if confirmed.Exit == nil || silent.Exit == nil {
		t.Fatal("one of the runs returned no exit code")
	}
	// Both take a defined path and neither crashes; what must not happen
	// is the confirmation being refused as an unknown field.
	if strings.Contains(confirmed.Stderr, "unknown") {
		t.Fatalf("the confirmation was refused: %q", confirmed.Stderr)
	}
}

// A confirmation sent to a command that never asks is ignored, not
// refused. A client that sets it everywhere is clumsy, not wrong, and
// refusing would make the field a per-command lookup for every caller.
func TestUnusedConfirmationIsIgnored(t *testing.T) {
	dir := fixtureRepo(t)
	yes := "yes"
	with := api.Serve(api.Request{
		Protocol: api.Protocol, Argv: []string{"config"}, Cwd: dir, Confirm: &yes,
	}, apiRunner)
	without := api.Serve(api.Request{
		Protocol: api.Protocol, Argv: []string{"config"}, Cwd: dir,
	}, apiRunner)

	if with.Stdout != without.Stdout {
		t.Fatalf("a confirmation changed a command that never asks:\n with:    %q\n without: %q",
			with.Stdout, without.Stdout)
	}
	if *with.Exit != *without.Exit {
		t.Fatalf("exit codes differ: %d vs %d", *with.Exit, *without.Exit)
	}
}

// status, spec check and the index lookups answer with values too, and
// each value is the one its own lines are rendered from.
//
// proved by: computing any of the three separately from its output — the
// value and the text drift the first time either is reworded, and a
// client acting on one gets a different answer from a person reading the
// other.
func TestStatusSpecIndexResultsAreTyped(t *testing.T) {
	dir := fixtureRepo(t)

	t.Run("status", func(t *testing.T) {
		res := api.Serve(api.Request{Protocol: api.Protocol, Argv: []string{"status"}, Cwd: dir}, apiRunner)
		if res.Result == nil || res.Result.Kind != api.KindStatus {
			t.Fatalf("status did not answer in its own kind: %+v", res.Result)
		}
		if res.Result.Status == nil {
			t.Fatal("the status kind carries no status")
		}
		if res.Result.Status.Branch != "main" {
			t.Errorf("branch is %q, want main", res.Result.Status.Branch)
		}
		if !strings.Contains(res.Stdout, "branch: main") {
			t.Errorf("the value and the lines disagree:\n%s", trimTo(res.Stdout, 300))
		}
	})

	t.Run("spec", func(t *testing.T) {
		specDir := filepath.Join(dir, ".procoder", "specs")
		if err := os.MkdirAll(specDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// A spec with nothing in it: the controller must refuse it and
		// say why, and the value must carry the same gaps.
		if err := os.WriteFile(filepath.Join(specDir, "hollow.md"),
			[]byte("# hollow\n\nStatus: draft\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := api.Serve(api.Request{
			Protocol: api.Protocol, Argv: []string{"spec", "check", "hollow"}, Cwd: dir,
		}, apiRunner)
		if res.Result == nil || res.Result.Kind != api.KindSpec {
			t.Fatalf("spec check did not answer in its own kind: %+v", res.Result)
		}
		if len(res.Result.Specs) != 1 {
			t.Fatalf("want one verdict, got %d", len(res.Result.Specs))
		}
		v := res.Result.Specs[0]
		if v.Name != "hollow" || v.Verdict != "NOT ready" {
			t.Errorf("verdict is %+v, want hollow NOT ready", v)
		}
		if len(v.Gaps) == 0 {
			t.Error("a refused spec carries no gaps — the caller is told no and not why")
		}
		if !strings.Contains(res.Stdout, "NOT ready") {
			t.Errorf("the value and the lines disagree:\n%s", trimTo(res.Stdout, 300))
		}
	})

	t.Run("index", func(t *testing.T) {
		res := api.Serve(api.Request{
			Protocol: api.Protocol, Argv: []string{"index", "find", "nothingcalledthis"}, Cwd: dir,
		}, apiRunner)
		// No index in the fixture: the lookup could not run, so there is
		// no symbol list — an empty one would say the index answered and
		// found none.
		if res.Result != nil && res.Result.Kind == api.KindIndex && res.Result.Symbols != nil {
			t.Errorf("a lookup that could not run returned a symbol list: %+v", res.Result.Symbols)
		}
	})
}

func trimTo(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// There is no fallback: a machine configured for the server does not
// quietly run the command itself when the server is not there.
//
// The alternative was tried and removed. A silent fallback means two
// possible answers to "where did this verdict come from", identical from
// the outside — and a machine configured for the daemon can spend weeks
// never reaching it with nothing saying so.
//
// proved by: running the command in-process when the client errors — this
// test then sees exit 0 and the command's output instead of a refusal.
func TestNoDaemonIsAFailureNotAFallback(t *testing.T) {
	dir := fixtureRepo(t)
	cfg := filepath.Join(dir, ".procoder", "config.toml")
	if err := os.WriteFile(cfg, []byte("[service]\nmode = \"local\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Point the run directory somewhere with no daemon in it.
	t.Setenv("HOME", t.TempDir())

	var out, errs bytes.Buffer
	s := session{
		stdin: strings.NewReader(""), stdout: &out, stderr: &errs,
		cwd: dir, env: host.Env{},
	}
	code, handled := viaDaemon([]string{"config"}, s)
	if !handled {
		t.Fatal("a machine set to mode=local did not have its command taken by the daemon path")
	}
	if code == 0 {
		t.Fatal("no daemon answered and the command still reported success")
	}
	if out.Len() != 0 {
		t.Fatalf("the command produced output without a daemon: %q", trimTo(out.String(), 200))
	}
	if !strings.Contains(errs.String(), "procoder serve") {
		t.Errorf("the failure does not say how to fix it: %q", trimTo(errs.String(), 300))
	}
}

// mode = off is the CLI, and the daemon path does not take the command at
// all.
func TestModeOffLeavesTheCommandAlone(t *testing.T) {
	dir := fixtureRepo(t)
	var out, errs bytes.Buffer
	s := session{
		stdin: strings.NewReader(""), stdout: &out, stderr: &errs,
		cwd: dir, env: host.Env{},
	}
	if _, handled := viaDaemon([]string{"config"}, s); handled {
		t.Fatal("a machine with no [service] mode had its command taken by the daemon path")
	}
	if errs.Len() != 0 {
		t.Errorf("the CLI path complained about a daemon it was never asked to use: %q", errs.String())
	}
}

// The commands that run what a repository declared are refused on a
// server machine that has not opened the exec socket — not quietly run
// here instead.
func TestExecutingCommandsRefusedWithoutTheExecSocket(t *testing.T) {
	dir := fixtureRepo(t)
	cfg := filepath.Join(dir, ".procoder", "config.toml")
	if err := os.WriteFile(cfg, []byte("[service]\nmode = \"local\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())

	var out, errs bytes.Buffer
	s := session{
		stdin: strings.NewReader(""), stdout: &out, stderr: &errs,
		cwd: dir, env: host.Env{},
	}
	code, handled := viaDaemon([]string{"run", "--exec"}, s)
	if !handled || code != 2 {
		t.Fatalf("want exit 2 and handled, got %d handled=%v", code, handled)
	}
	if !strings.Contains(errs.String(), "exec = true") {
		t.Errorf("the refusal does not name the setting that opens the door: %q", trimTo(errs.String(), 300))
	}
}
