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
	script := `
import { ProcoderPlugin } from "file://` + plugin + `";
const logs = [];
const client = { app: { log: async ({ body }) => { logs.push(body.level + ":" + body.message); } } };
const hooks = await ProcoderPlugin({ client, directory: process.cwd() });
try {
  await hooks["tool.execute.before"]({ tool: "bash" }, { args: { command: process.argv[1] } });
  console.log("ALLOWED");
} catch (e) {
  console.log("DENIED:" + e.message);
}
console.log("LOGS:" + logs.join("|"));`
	cmd := exec.Command(node, "--input-type=module", "-e", script, command)
	cmd.Env = append(os.Environ(), "PATH="+pathDir+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
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
	if got := runHook(t, stub, "ls -la"); !strings.Contains(got, "ALLOWED") {
		t.Errorf("a non-commit command must not be judged, got %q", got)
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
