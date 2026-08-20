package portability

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stubProcoder puts a fake `procoder` on PATH that prints the given stdout
// and exits with code. The plugin must reach the binary the same way a host
// would — through PATH — so the stub is a real executable, not a mock.
func stubProcoder(t *testing.T, stdout string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncat >/dev/null\n"
	if stdout != "" {
		script += "printf '%s' '" + stdout + "'\n"
	}
	script += "exit 0\n"
	path := filepath.Join(dir, "procoder")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runHook drives the plugin's tool.execute.before exactly as a host does and
// reports what the hook did: DENIED with the reason, or ALLOWED.
func runHook(t *testing.T, pathDir, command string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a POSIX shell script")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node on PATH")
	}
	plugin := filepath.Join(repoRoot(t), ".opencode/plugins/procoder.mjs")
	// A payload larger than the pipe buffer is what exposes the write race,
	// and argv cannot carry one portably — so the harness builds it here.
	script := `
import { ProcoderPlugin } from "file://` + plugin + `";
const command = process.argv[1] === "__big__"
  ? "git commit -m " + "x".repeat(300000)
  : process.argv[1];
const logs = [];
const client = { app: { log: async ({ body }) => { logs.push(body.level + ":" + body.message); } } };
const hooks = await ProcoderPlugin({ client, directory: process.cwd() });
try {
  await hooks["tool.execute.before"]({ tool: "bash" }, { args: { command } });
  console.log("ALLOWED");
} catch (e) {
  console.log("DENIED:" + e.message);
}
console.log("LOGS:" + logs.join("|"));`
	cmd := exec.Command(node, "--input-type=module", "-e", script, command)
	// PATH is the stub directory and nothing else: a real procoder installed
	// on the machine would otherwise answer these tests, and the one about a
	// missing binary would silently stop testing anything
	cmd.Env = append(os.Environ(), "PATH="+pathDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// stubIgnoringStdin is a procoder that exits without reading its input. A
// binary that answers and leaves closes the pipe under the write still in
// flight, which is the shape of the failure below.
func stubIgnoringStdin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "procoder"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Writing to a gate that has already exited must not take the session with
// it. An unhandled 'error' on the child's stdin is a process-level throw, so
// the cost of a commit message too large for the pipe buffer would be the
// whole editor session — for a check that is meant to be advisory when it
// cannot run.
// proved by: removed the stdin error handler — this test dies with EPIPE
// instead of failing, which is exactly what it did on CI.
func TestAGateThatExitsEarlyDoesNotCrashTheSession(t *testing.T) {
	if got := runHook(t, stubIgnoringStdin(t), "__big__"); !strings.Contains(got, "ALLOWED") {
		t.Errorf("a gate that closed its input must not crash or block, got %q", got)
	}
}

// A blocking gate stops the commit in Kilo and OpenCode the same way it does
// in Claude Code — the hook throws, which is how those hosts deny a tool —
// and the reason the binary printed is what the agent is told.
// proved by: made the hook swallow the verdict — a denied commit then runs.
func TestTheGateDeniesACommitWithBlockingFindings(t *testing.T) {
	stub := stubProcoder(t, `{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"BLOCKING secret in config.toml"}}`)
	got := runHook(t, stub, "git commit -m x")
	if !strings.HasPrefix(got, "DENIED:") || !strings.Contains(got, "BLOCKING secret") {
		t.Errorf("a deny verdict must stop the tool and carry its reason, got %q", got)
	}
}

// A clean gate is silent: the commit runs.
// proved by: threw on every verdict — every commit is then blocked, which is
// the failure mode that gets a plugin uninstalled.
func TestACleanGateLetsTheCommitThrough(t *testing.T) {
	stub := stubProcoder(t, "")
	got := runHook(t, stub, "git commit -m x")
	if !strings.Contains(got, "ALLOWED") {
		t.Errorf("a clean gate must not block, got %q", got)
	}
	// a clean gate is also silent: a warning on every commit is noise the
	// reader learns to skip, and then misses the one that mattered
	if !strings.HasSuffix(got, "LOGS:") {
		t.Errorf("a clean gate must log nothing, got %q", got)
	}
}

// Only commits reach the binary. Spawning a process on every shell call
// would tax the whole session for one command's sake.
// proved by: dropped the prefilter — `ls` then consults the deny stub and
// the session cannot run an ordinary command.
func TestOrdinaryShellCommandsNeverConsultTheGate(t *testing.T) {
	stub := stubProcoder(t, `{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"should never be asked"}}`)
	for _, cmd := range []string{
		"ls -la",
		// the word appears, but as part of a flag: an agent runs this one
		// constantly and it creates nothing
		"git log --oneline --abbrev-commit",
	} {
		if got := runHook(t, stub, cmd); !strings.Contains(got, "ALLOWED") {
			t.Errorf("%s must not be judged, got %q", cmd, got)
		}
	}
}

// procoder not installed means the gate did NOT judge this commit. It says
// so through the host's log and lets the commit through: blocking every
// commit on a machine without the binary would be a broken session, not a
// gate.
// proved by: treated a missing binary as a deny — the plugin then bricks
// every commit for anyone who has not installed procoder.
func TestAMissingBinaryDoesNotWedgeTheSession(t *testing.T) {
	empty := t.TempDir() // no procoder here, and PATH is prefixed with it
	got := runHook(t, empty, "git commit -m x")
	if !strings.Contains(got, "ALLOWED") {
		t.Errorf("a gate that cannot run must not block the commit, got %q", got)
	}
	// unknown is never done: it must be said, not swallowed
	if !strings.Contains(got, "warn:") || !strings.Contains(got, "did NOT run") {
		t.Errorf("a gate that could not run must say so through the host log, got %q", got)
	}
}
