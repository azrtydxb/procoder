package portability

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The OpenCode/Kilo shim (`.opencode/plugins/procoder.mjs`, byte-identical
// to `.kilo/plugin/procoder.js`) cannot refuse a turn — the host offers a
// plugin no such surface — but on `session.idle` and `session.compacted`
// it must still run `procoder hook stop`: the handoff gets written and
// the unasked-decision report lands where the host keeps records. This
// test drives the plugin's `event` hook through node the way pi-harness
// drives the pi adapter, with the fixture binary standing in for
// procoder.
//
// The plugin resolves the binary as `procoder` on PATH — the
// everywhere-binary contract — so the fixture is placed under that name.
// FIXTURE_STOP=block makes the fixture answer the way a real binary
// answers a turn that buried a decision: exit 2, reason on stderr. The
// assertions are that the hook logged the reminder (not that it stopped
// anything — this host stops nothing, docs/portability.md says so) and
// that a blocking verdict did not take the event handler down with it:
// a throwing event handler is how a hook breaks a session.
func TestOpenCodeTurnEndRunsHookStop(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node on PATH")
	}
	root := repoRoot(t)
	plugin := filepath.Join(root, ".opencode", "plugins", "procoder.mjs")

	fixtureDir := t.TempDir()
	fixture := piFixtureBin(t)
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("reading the fixture binary: %v", err)
	}
	binName := "procoder" + exeSuffix()
	if err := os.WriteFile(filepath.Join(fixtureDir, binName), raw, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(fixtureDir, "invocations.log")
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	cwd := t.TempDir()
	driver := filepath.Join(fixtureDir, "driver.mjs")
	src := "import plugin from '" + plugin + "';\n" +
		"const hooks = await plugin({ client: null, directory: process.env.PROCODER_OCWD });\n" +
		"await hooks.event({ event: { type: 'session.idle' } });\n" +
		"await hooks.event({ event: { type: 'session.compacted' } });\n" +
		"await hooks.event({ event: { type: 'message.updated' } });\n" +
		"process.stdout.write('settled\\n');\n"
	if err := os.WriteFile(driver, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(node, driver)
	cmd.Env = append(os.Environ(),
		"PROCODER_OCWD="+cwd,
		"FIXTURE_LOG="+log,
		"FIXTURE_STOP=block",
	)
	var key string
	if runtime.GOOS == "windows" {
		key = "Path"
	} else {
		key = "PATH"
	}
	for i, e := range cmd.Env {
		k, v, _ := strings.Cut(e, "=")
		if strings.EqualFold(k, key) {
			cmd.Env[i] = k + "=" + fixtureDir + string(os.PathListSeparator) + v
		}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("driver failed: %v\n%s", err, out)
	}

	// Exactly the two turn-end events spawn the binary; the unrelated
	// event must cost nothing.
	invoked := invocations(t, log)
	if len(invoked) != 2 {
		t.Fatalf("expected two invocations (session.idle, session.compacted), got %d:\n%s", len(invoked), strings.Join(invoked, "\n"))
	}
	type stopPayload struct {
		Cwd string `json:"cwd"`
	}
	for i, line := range invoked {
		if !strings.HasPrefix(line, "hook stop ") {
			t.Fatalf("invocation %d is not `hook stop`:\n%s", i, line)
		}
		body := strings.TrimPrefix(line, "hook stop ")
		var p stopPayload
		if err := json.Unmarshal([]byte(body), &p); err != nil {
			t.Fatalf("invocation %d payload is not JSON (a Windows path would escape it twice): %v\n%s", i, err, body)
		}
		if p.Cwd != cwd {
			t.Fatalf("invocation %d was told %q, want the session directory %q", i, p.Cwd, cwd)
		}
	}
}
