package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"procoder/internal/api"
	"procoder/internal/host"
)

// usageCommands reads the command names out of the usage text.
//
// Read rather than listed, so a command added to procoder without a
// parity case fails here instead of being quietly untested. The shape is
// the usage text's own: two spaces, then a lowercase name.
func usageCommands(t *testing.T) []string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^  ([a-z][a-z-]+)`)
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllStringSubmatch(usage, -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	if len(out) < 40 {
		t.Fatalf("only %d commands found in the usage text — the parity table is not reading it", len(out))
	}
	return out
}

// parityRepo is a real repository with procoder adopted: the fixture the
// commands are compared in.
func parityRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "T"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("git is not usable here: %v (%s)", err, out)
		}
	}
	return dir
}

// Every command answers the same at both doors.
//
// This is the whole promise of the second door and the only test that can
// make it for all of procoder at once. The comparison is byte-identical
// on stdout, stderr and the exit code — anything less would let the two
// implementations drift in exactly the place nobody looks.
//
// The four executing commands are asserted refused rather than compared:
// they are not served on the work socket at all, and comparing them would
// mean running them.
//
// proved by: making apiRunner build its session differently from
// processSession in any respect that reaches a command — the offending
// command's bytes diverge and this test names it.
func TestParityAcrossEveryCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("the parity table runs every command twice")
	}
	dir := parityRepo(t)
	env := host.Env{}

	for _, name := range usageCommands(t) {
		name := name
		t.Run(name, func(t *testing.T) {
			argv := []string{name}

			// Not compared: served on the exec socket or not at all, and
			// comparing them would mean running them.
			// TestExecutingCommandsAreNotServedOnTheWorkSocket asserts
			// that each is refused.
			if api.Executes(argv) {
				return
			}
			// serve blocks until its listener closes; there is nothing to
			// compare and running it here would hang the suite.
			if name == "serve" {
				return
			}

			var directOut, directErr bytes.Buffer
			directCode := run(argv, session{
				stdin: strings.NewReader(""), stdout: &directOut, stderr: &directErr,
				cwd: dir, env: env,
			})

			res := api.Serve(api.Request{
				Protocol: api.Protocol, Argv: argv, Cwd: dir,
			}, apiRunner)

			if res.Exit == nil {
				t.Fatalf("%s returned no exit code over the envelope", name)
			}
			if *res.Exit != directCode {
				t.Errorf("%s: exit %d in-process, %d over the envelope", name, directCode, *res.Exit)
			}
			if res.Stdout != directOut.String() {
				t.Errorf("%s: stdout differs\n in-process: %q\n envelope:   %q",
					name, trim(directOut.String()), trim(res.Stdout))
			}
			if res.Stderr != directErr.String() {
				t.Errorf("%s: stderr differs\n in-process: %q\n envelope:   %q",
					name, trim(directErr.String()), trim(res.Stderr))
			}
		})
	}
}

// The four that execute are refused on the work socket, every one of
// them, rather than compared.
func TestExecutingCommandsAreNotServedOnTheWorkSocket(t *testing.T) {
	for _, argv := range [][]string{
		{"run", "--exec"}, {"evidence", "record", "true"}, {"init", "--yes"}, {"self-upgrade"},
	} {
		if !api.Executes(argv) {
			t.Errorf("%v would be served on the work socket — it runs what a repository declared", argv)
		}
	}
}

// One daemon, two hosts, two shapes. The environment travels in the
// request, so the answer follows the caller and not whichever session
// started the process.
//
// This is the case #117's specified parity test would have passed while
// missing: the payload is identical and only the environment differs.
//
// proved by: pointing host.DetectIn back at os.Getenv — both requests then
// answer in one shape, and a daemon serving two hosts is wrong for one of
// them with nothing to see.
func TestParityVariesTheEnvironmentNotJustThePayload(t *testing.T) {
	dir := parityRepo(t)
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("QODER_SESSION_ID", "")
	t.Setenv("PLUGIN_DATA", "")

	shapes := map[string]string{}
	for name, env := range map[string]map[string]string{
		"claude": {},
		"qoder":  {"QODER_SESSION_ID": "s"},
		"codex":  {"PLUGIN_DATA": "d"},
	} {
		res := api.Serve(api.Request{
			Protocol: api.Protocol, Argv: []string{"principles", "--hook"}, Cwd: dir, Env: env,
		}, apiRunner)
		shapes[name] = res.Stdout
	}

	if shapes["claude"] == shapes["qoder"] {
		t.Error("Claude and Qoder got the same envelope — the request's environment was not read")
	}
	if !strings.Contains(shapes["qoder"], "hookSpecificOutput") {
		t.Errorf("the Qoder request did not get the envelope shape: %q", trim(shapes["qoder"]))
	}
	if !strings.Contains(shapes["codex"], "hookSpecificOutput") {
		t.Errorf("the Codex request did not get the envelope shape: %q", trim(shapes["codex"]))
	}
}

func trim(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
