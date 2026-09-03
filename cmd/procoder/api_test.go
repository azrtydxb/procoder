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
