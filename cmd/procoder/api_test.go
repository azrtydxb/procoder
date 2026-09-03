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

	version := api.Serve(api.Request{Protocol: api.Protocol, Argv: []string{"version"}, Cwd: dir}, apiRunner)
	if version.Result != nil {
		t.Fatalf("a command that reports no findings returned a result: %+v", version.Result)
	}
}
