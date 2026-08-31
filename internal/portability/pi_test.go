package portability

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// The pi adapter is the only non-Go glue in the host layer that carries real
// behaviour, and pi itself is not installed where `go test` runs. These tests
// therefore drive the adapter through internal/portability/testdata/pi-harness.mjs,
// which imports the adapter for real and hands it the object pi would hand it.
//
// What is faked is the host: registrations are collected instead of painted,
// and ctx.ui records instead of prompting. What is never faked is the adapter —
// it reads the real commands/, resolves its own launcher path from its own
// module location, and spawns the real hooks/launcher.sh. PROCODER_BIN is the
// launcher's own documented seam for tests, so the binary on the other side of
// that launcher is a fixture this file controls.

// piFixture is a procoder that answers from FIXTURE_MODE and appends every
// invocation to FIXTURE_LOG, so "did the gate spawn at all" is a question with
// an answer on disk rather than one inferred from timing.
//
// It is compiled rather than scripted. A #!/bin/sh fixture is a test that only
// exists on two of the three platforms the adapter ships to: on the Windows leg
// the fixture cannot be exec'd, which — once launcher.cmd was made to honour
// PROCODER_BIN the way launcher.sh always had — meant a fixture-driven
// assertion silently became an assertion about the released binary. The go tool
// is present by definition where `go test` runs, so the cost is one compile per
// test run and the gain is one fixture with one behaviour everywhere.
func piFixture(t *testing.T, dir string) (bin, log string) {
	t.Helper()
	log = filepath.Join(dir, "invocations.log")
	// Created empty rather than left absent: "the gate never spawned" has to be
	// an assertion that reads zero lines, not a ReadFile error.
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return piFixtureBin(t), log
}

const piFixtureSource = `package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Every invocation is logged with the payload it was handed, so "was the gate
// run at all, and told what" is answered from disk rather than inferred from
// timing. Only the hook subcommands are given a payload: the adapter leaves
// stdin inherited for the rest, and a fixture that waited on an inherited stdin
// would sit there until the caller's own timeout expired.
func main() {
	args := os.Args[1:]
	key := ""
	if len(args) > 1 {
		key = args[0] + " " + args[1]
	}
	payload := ""
	switch key {
	case "hook pre-tool-use", "hook post-tool-use", "hook stop":
		raw, _ := io.ReadAll(os.Stdin)
		payload = string(raw)
	}
	if path := os.Getenv("FIXTURE_LOG"); path != "" {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, "%s %s\n", strings.Join(args, " "), payload)
			f.Close()
		}
	}
	mode := os.Getenv("FIXTURE_MODE")
	if mode == "" {
		mode = "deny"
	}
	switch key {
	case "hook post-tool-use":
		fmt.Print("{\"hookSpecificOutput\":{\"hookEventName\":\"PostToolUse\",\"additionalContext\":\"procoder [format]: x.go is not formatted\"}}")
		return
	case "hook pre-tool-use":
		switch mode {
		case "deny":
			fmt.Print("{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"procoder gate: 2 blocking findings\"}}")
		case "allow":
			fmt.Print("{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"allow\",\"permissionDecisionReason\":\"commit message obligation cleared\"}}")
		}
		return
	case "hook stop":
		if os.Getenv("FIXTURE_STOP") == "block" {
			fmt.Fprintln(os.Stderr, "procoder: a decision is waiting on the user and was not asked")
			os.Exit(2)
		}
		return
	}
	if len(args) > 0 && args[0] == "principles" {
		fmt.Println("Build like a senior developer who has been paged at 3am.")
		return
	}
	if len(args) > 0 && args[0] == "check" {
		for i := 0; i < 3000; i++ {
			fmt.Printf("line %d\n", i)
		}
		return
	}
}
`

// The compile happens once for the package. A per-test build would add a
// second of compile to each of the eleven tests that ask for a binary, and a
// fixture is not interesting enough to earn that.
var (
	fixtureBuild    sync.Once
	fixtureBinPath  string
	fixtureBuildErr error
)

func piFixtureBin(t *testing.T) string {
	t.Helper()
	fixtureBuild.Do(func() {
		dir, err := os.MkdirTemp("", "procoder-pi-fixture")
		if err != nil {
			fixtureBuildErr = err
			return
		}
		// A module of its own, so the build is not read as part of this one:
		// the fixture imports nothing from the tree and must not be able to.
		write := func(name, body string) error {
			return os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
		}
		if err := write("go.mod", "module procoder-pi-fixture\n\ngo 1.21\n"); err != nil {
			fixtureBuildErr = err
			return
		}
		if err := write("main.go", piFixtureSource); err != nil {
			fixtureBuildErr = err
			return
		}
		bin := filepath.Join(dir, "procoder-fixture"+exeSuffix())
		cmd := exec.Command("go", "build", "-o", bin, ".")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			fixtureBuildErr = fmt.Errorf("building the pi fixture: %v\n%s", err, out)
			return
		}
		fixtureBinPath = bin
	})
	if fixtureBuildErr != nil {
		t.Fatal(fixtureBuildErr)
	}
	return fixtureBinPath
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// piHarness runs one harness mode against the real adapter and decodes its
// answer. The repo root is passed explicitly because the adapter derives its own
// paths from where it was loaded from, and a test that guessed would be testing
// a directory layout nobody installs.
func piHarness(t *testing.T, mode string, config map[string]any, env map[string]string) map[string]any {
	t.Helper()
	got, _ := piHarnessRaw(t, mode, config, env)
	return got
}

// piHarnessRaw is piHarness with the whole child output, because a finding the
// adapter reports on stderr — where a print-mode session is the only audience —
// is invisible in the decoded last line.
func piHarnessRaw(t *testing.T, mode string, config map[string]any, env map[string]string) (map[string]any, string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node on PATH")
	}
	root := repoRoot(t)
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "null" {
		raw = []byte("{}")
	}
	cmd := exec.Command(node, filepath.Join(root, "internal", "portability", "testdata", "pi-harness.mjs"), mode, string(raw))
	cmd.Env = append(os.Environ(), "PROCODER_REPO="+root)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("harness %s failed: %v\n%s", mode, err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var decoded map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &decoded); err != nil {
		t.Fatalf("harness %s printed no JSON:\n%s", mode, out)
	}
	return decoded, string(out)
}

func fixtureEnv(t *testing.T, mode string) (map[string]string, string) {
	t.Helper()
	dir := t.TempDir()
	bin, log := piFixture(t, dir)
	return map[string]string{
		"PROCODER_BIN": bin,
		"FIXTURE_LOG":  log,
		"FIXTURE_MODE": mode,
	}, log
}

func invocations(t *testing.T, log string) []string {
	t.Helper()
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// registryOnce runs the harness's registry mode and unpacks the three things
// it reports. One load of the adapter answers all three checks below, so the
// split into separate tests costs no extra node process.
func registryOnce(t *testing.T) ([]map[string]any, map[string]any, []string) {
	t.Helper()
	env, _ := fixtureEnv(t, "deny")
	got := piHarness(t, "registry", nil, env)

	pick := func(key string) []map[string]any {
		var out []map[string]any
		for _, item := range got[key].([]any) {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	tool, _ := got["tool"].(map[string]any)
	var events []string
	if list, ok := got["events"].([]any); ok {
		for _, e := range list {
			if s, ok := e.(string); ok {
				events = append(events, s)
			}
		}
	}
	return pick("commands"), tool, events
}

// commandsExpected is the registry the command directory asks for: one entry
// per file, namespaced, update.md aside.
func commandsExpected(t *testing.T) map[string]string {
	t.Helper()
	want := map[string]string{}
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), "commands"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || name == "update.md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "commands", name))
		if err != nil {
			t.Fatal(err)
		}
		want["procoder:"+strings.TrimSuffix(name, ".md")] = frontmatterDescription(t, string(raw))
	}
	return want
}

// The command set arrives whole, under the procoder namespace, with the
// description each file already carries. pi names a skill after its parent
// directory when the frontmatter has no name, which is how 34 files once
// arrived as one skill called "commands" — registering them is the only shape
// where each keeps its own identity.
//
// proved by: adding a file to commands/ without registering it, or dropping one
// of the two exclusions — this test names it.
func TestPiCommandRegistryMatchesCommandsDir(t *testing.T) {
	commands, _, _ := registryOnce(t)
	want := commandsExpected(t)

	if len(commands) != len(want) {
		t.Fatalf("registered %d commands, commands/ holds %d (minus update.md)", len(commands), len(want))
	}
	seen := map[string]bool{}
	for _, c := range commands {
		name, _ := c["name"].(string)
		seen[name] = true
		desc, ok := want[name]
		if !ok {
			t.Errorf("%s is registered with no file behind it", name)
			continue
		}
		if c["description"] != desc {
			t.Errorf("%s: description %v is not the file's %q", name, c["description"], desc)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("%s never registered", name)
		}
	}
}

// The callable tool is the one place pi gets a procoder without a shell, so its
// parameter list is the whole permission boundary: reporting commands only.
//
// proved by: adding "backlog" to REPORTING_COMMANDS — this names it, and
// TestPiToolRefusesMutatingSubcommand then fails on the refusal.
func TestPiToolOffersOnlyReportingCommands(t *testing.T) {
	_, tool, _ := registryOnce(t)
	if tool == nil {
		t.Fatal("no procoder tool registered")
	}
	subs, _ := tool["subcommands"].([]any)
	if len(subs) == 0 {
		t.Fatal("the procoder tool declares no subcommands, so it can be asked for anything")
	}
	for _, forbidden := range []string{"backlog", "release", "todo", "adr", "sprint", "init", "seed"} {
		for _, s := range subs {
			if s == forbidden {
				t.Errorf("the procoder tool offers %q, which mutates state and must stay a slash command", forbidden)
			}
		}
	}
}

// An event pi never fires is a hook that silently does nothing, and the four
// enforcement points are invisible until a session needs one.
//
// proved by: renaming any one of the pi event strings in the adapter — this
// test names the surface that went quiet.
func TestPiWiresEveryHostEvent(t *testing.T) {
	_, _, events := registryOnce(t)
	for _, want := range []string{
		"before_agent_start", "tool_call", "tool_result", "agent_settled",
		"session_before_compact", "session_start",
	} {
		found := false
		for _, e := range events {
			if e == want {
				found = true
			}
		}
		if !found {
			t.Errorf("no handler for %s — the host would never call it", want)
		}
	}
}

func frontmatterDescription(t *testing.T, body string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if d := strings.TrimPrefix(line, "description:"); d != line {
			return strings.TrimSpace(d)
		}
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "---") {
			break // frontmatter is over; the description was never there
		}
	}
	t.Fatal("command file carries no description in its frontmatter")
	return ""
}

// The launcher is found beside the installed package, never on PATH. The machine
// this adapter was written on keeps a 1.0.2 procoder on PATH while the package
// is 3.4.0, and a gate running a binary three majors behind reports a clean tree
// it has never seen.
//
// proved by: returning "procoder" from launcherPath, or dropping the win32 arm —
// this test names whichever broke.
func TestPiLauncherResolvesFromModuleLocation(t *testing.T) {
	env, _ := fixtureEnv(t, "deny")
	got := piHarness(t, "launcher", nil, env)

	posix, _ := got["posix"].(string)
	if !strings.HasSuffix(filepath.ToSlash(posix), "hooks/launcher.sh") {
		t.Errorf("posix launcher is %q, expected the hooks/ launcher beside the package", posix)
	}
	if posix == "procoder" || !filepath.IsAbs(posix) {
		t.Errorf("posix launcher %q is resolved through PATH rather than beside the package", posix)
	}
	win, _ := got["win32"].(string)
	if !strings.HasSuffix(win, "launcher.cmd") {
		t.Errorf("win32 launcher is %q, expected launcher.cmd", win)
	}

	text, _ := got["text"].(string)
	if strings.Contains(text, "CLAUDE_PLUGIN_ROOT") {
		t.Errorf("the transform left a plugin-root reference in the host command text:\n%s", text)
	}
	for _, want := range []string{`"/opt/pkg/hooks/launcher.sh" check`, `"/opt/pkg/hooks/launcher.sh" format <file>`} {
		if !strings.Contains(text, want) {
			t.Errorf("transformed command text is missing %q:\n%s", want, text)
		}
	}
	// The transform rewrites WHERE procoder lives and nothing else. $ARGUMENTS
	// is pi's own expansion, applied per invocation, and a rule that swallowed it
	// would hand every command the same half-finished sentence.
	if !strings.Contains(text, "$ARGUMENTS") {
		t.Error("the transform rewrote past the launcher and consumed $ARGUMENTS:", text)
	}
	// A host that names its own launcher must not be told to look it up on
	// PATH — the sentence the PATH hosts get would be false here.
	if strings.Contains(text, "binary on PATH") {
		t.Errorf("a host with an absolute launcher was told to use PATH:\n%s", text)
	}
	if !strings.Contains(text, `The launcher is: "/opt/pkg/hooks/launcher.sh"`) {
		t.Errorf("the launcher sentence was not carried over with the real path:\n%s", text)
	}
}

// The contract arrives once. pi loads AGENTS.md as a context file, so injecting
// it again was a 12,458-byte tax on every turn of every session — the reason
// this adapter's first version is being replaced.
//
// proved by: dropping the contextFiles guard and injecting unconditionally (the
// marker appears twice), or inverting it (the marker appears zero times and the
// session is governed by rules nobody sent).
func TestPiAdapterInjectsContractOnce(t *testing.T) {
	root := repoRoot(t)
	env, _ := fixtureEnv(t, "deny")

	loaded := piHarness(t, "inject", map[string]any{
		"contextFiles": []map[string]any{{"path": filepath.Join(root, "AGENTS.md")}},
	}, env)
	if loaded["first"] != float64(1) {
		t.Errorf("nothing was injected into a fresh session: %+v", loaded)
	}
	if loaded["markerInMessage"] != float64(0) {
		t.Errorf("AGENTS.md was injected although pi had already loaded it (%v copies of its marker)", loaded["markerInMessage"])
	}
	if loaded["principles"] != true {
		t.Error("the engineering principles did not reach the session — pi loads no .procoder/PRINCIPLES.md")
	}
	if loaded["second"] != float64(0) {
		t.Error("the contract was injected twice in one session, which is how #175 happened here")
	}

	missing := piHarness(t, "inject", map[string]any{"contextFiles": []any{}}, env)
	if missing["markerInMessage"] != float64(1) {
		t.Errorf("with context files unloaded the contract should arrive exactly once, got %v", missing["markerInMessage"])
	}

	resumed := piHarness(t, "inject", map[string]any{
		"reason":       "resume",
		"contextFiles": []map[string]any{{"path": filepath.Join(root, "AGENTS.md")}},
	}, env)
	if resumed["first"] != float64(0) {
		t.Error("a resumed session got a second copy of rules already in its transcript")
	}
}

// A commit is judged by the binary before the shell sees it, and an ordinary
// shell call pays nothing for that. --abbrev-commit is in every session and is
// not a commit by any reading.
//
// proved by: removing the isCommitish pre-filter (git log spawns the gate) or
// the deny branch (a blocked commit runs anyway).
func TestPiCommitGateBlocksThenAllows(t *testing.T) {
	deny, log := fixtureEnv(t, "deny")
	got := piHarness(t, "gate", map[string]any{"command": "git commit -m notes"}, deny)
	results, _ := got["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("a denied commit came back with %d results: %+v", len(results), got)
	}
	block, _ := results[0].(map[string]any)
	if block["block"] != true {
		t.Errorf("the gate said deny and the commit was not blocked: %+v", block)
	}
	if reason, _ := block["reason"].(string); !strings.Contains(reason, "blocking findings") {
		t.Errorf("the block carries %q, not the gate's reason", reason)
	}
	if len(invocations(t, log)) != 1 {
		t.Errorf("the gate ran %d times for one commit", len(invocations(t, log)))
	}

	quiet, log := fixtureEnv(t, "deny")
	got = piHarness(t, "gate", map[string]any{"command": "git log --oneline --abbrev-commit"}, quiet)
	if len(invocations(t, log)) != 0 {
		t.Errorf("git log spawned the gate: %v", invocations(t, log))
	}
	if len(got["results"].([]any)) != 0 {
		t.Errorf("an ordinary shell call was intercepted: %+v", got["results"])
	}

	allow, _ := fixtureEnv(t, "allow")
	got = piHarness(t, "gate", map[string]any{"command": "git commit -m notes"}, allow)
	if len(got["results"].([]any)) != 0 {
		t.Errorf("an allowed commit was blocked: %+v", got["results"])
	}
	notices, _ := got["notices"].([]any)
	if len(notices) != 1 {
		t.Errorf("the allow reason never reached the coder: %+v", got)
	}
}

// The write hook's findings land inside the write that caused them. pi can patch
// a tool result, which Claude Code cannot; the verdict itself belongs to
// internal/hook and is covered there — what would go unnoticed here is a verdict
// produced and then dropped on the way to the model.
//
// proved by: returning undefined instead of the content patch, or by patching
// even for a tool that is not a write.
func TestPiWriteResultCarriesFormatVerdict(t *testing.T) {
	root := repoRoot(t)
	env, log := fixtureEnv(t, "deny")
	got := piHarness(t, "write", map[string]any{
		"path":    "internal/gate/gate.go",
		"content": []map[string]any{{"type": "text", "text": "wrote it"}},
	}, env)

	results, _ := got["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("the write result was not patched: %+v", got)
	}
	patch, _ := results[0].(map[string]any)
	content, _ := patch["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("patched content has %d blocks, expected the original plus the verdict", len(content))
	}
	last, _ := content[1].(map[string]any)
	if text, _ := last["text"].(string); !strings.Contains(text, "is not formatted") {
		t.Errorf("the verdict never reached the model: %q", text)
	}
	calls := invocations(t, log)
	if len(calls) != 1 {
		t.Fatalf("the hook ran %d times for one write", len(calls))
	}
	if !strings.Contains(calls[0], `"file_path":"`+filepath.Join(root, "internal", "gate", "gate.go")+`"`) {
		t.Errorf("the hook was not told which file, as an absolute path: %s", calls[0])
	}

	env["FIXTURE_MODE"] = "silent"
	silent := piHarness(t, "write", map[string]any{"toolName": "read", "path": "AGENTS.md"}, env)
	if len(silent["results"].([]any)) != 0 {
		t.Error("a read was intercepted as though it were a write")
	}
	if calls := invocations(t, log); len(calls) != 1 {
		t.Errorf("a read still spawned the hook: %v", calls)
	}
}

// The gate as a callable tool, with pi's own output limits. A procoder report
// that is longer than the context it is being inserted into is not a report.
//
// proved by: forwarding res.stdout uncut, or dropping the temp file write.
func TestPiToolTruncatesWithTempFile(t *testing.T) {
	env, _ := fixtureEnv(t, "deny")
	got := piHarness(t, "tool", map[string]any{"params": map[string]any{"command": "check"}}, env)

	text, _ := got["text"].(string)
	if strings.Count(text, "\n") > 2100 {
		t.Errorf("the tool forwarded %d lines with no truncation", strings.Count(text, "\n")+1)
	}
	if !strings.Contains(text, "truncated") {
		t.Error("truncated output does not say it was truncated")
	}
	details, _ := got["details"].(map[string]any)
	full, _ := details["fullOutput"].(string)
	if full == "" {
		t.Fatal("truncation names no file holding the rest")
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("the file the tool named is not there: %v", err)
	}
	if strings.Count(string(raw), "\n") < 2999 {
		t.Errorf("the file holds %d lines, not the whole report", strings.Count(string(raw), "\n")+1)
	}
}

// What the tool may not do. Closing work, seeding the backlog and releasing are
// slash commands a human types, because an agent session could have written the
// sentence that asks for them — the same line procoder run draws for launch
// commands.
//
// proved by: widening REPORTING_COMMANDS with a mutating verb, or returning the
// refusal instead of throwing it.
func TestPiToolRefusesMutatingSubcommand(t *testing.T) {
	env, log := fixtureEnv(t, "deny")
	for _, forbidden := range []string{"backlog", "release", "todo", "sprint"} {
		got := piHarness(t, "tool", map[string]any{"params": map[string]any{"command": forbidden}}, env)
		threw, _ := got["threw"].(string)
		if threw == "" {
			t.Errorf("%s was accepted by the procoder tool", forbidden)
			continue
		}
		if !strings.Contains(threw, "/procoder:") {
			t.Errorf("%s was refused without naming the route a human uses: %q", forbidden, threw)
		}
	}
	if calls := invocations(t, log); len(calls) != 0 {
		t.Errorf("a refused subcommand still spawned something: %v", calls)
	}
}

// A turn that ends with a decision deferred and unrecorded does not end. The
// binary decides this and remembers which message it last refused; the adapter
// only carries the answer back, because a dedupe held in adapter memory is
// forgotten by the next reload, which is #242.
//
// proved by: ignoring the exit code, or gating the follow-up on a local variable.
func TestPiTurnReportsAnUnaskedDecision(t *testing.T) {
	env, log := fixtureEnv(t, "deny")
	env["FIXTURE_STOP"] = "block"
	got := piHarness(t, "stop", map[string]any{
		"branch": []map[string]any{{
			"type": "message",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "Should I keep the trailer?"}},
			},
		}},
	}, env)
	sent, _ := got["sent"].([]any)
	if len(sent) != 1 {
		t.Fatalf("the block never reached the model: %+v", got)
	}
	msg, _ := sent[0].(map[string]any)
	options, _ := msg["options"].(map[string]any)
	if options["deliverAs"] != "followUp" {
		t.Errorf("the reason was delivered as %+v, which does not continue the turn", options)
	}
	calls := invocations(t, log)
	if len(calls) != 1 || !strings.Contains(calls[0], "Should I keep the trailer?") {
		t.Errorf("the hook was called without the last assistant message: %v", calls)
	}

	env["FIXTURE_STOP"] = "ok"
	clean := piHarness(t, "stop", nil, env)
	if len(clean["sent"].([]any)) != 0 {
		t.Error("an ordinary turn was interrupted")
	}

	// A refused turn in print mode has no turn to refuse: the session exits once
	// the agent settles, so the follow-up has nowhere to go and the reason leaves
	// with the process. Live, this was measured rather than imagined — the hook
	// refused, recorded its dedupe, and nothing was ever said about it.
	env["FIXTURE_STOP"] = "block"
	quiet, raw := piHarnessRaw(t, "stop", map[string]any{"hasUI": false}, env)
	if len(quiet["sent"].([]any)) != 0 {
		t.Error("a print-mode session was sent a message it cannot deliver")
	}
	if !strings.Contains(raw, "a decision is waiting on the user") {
		t.Errorf("the refusal vanished with the session:\n%s", raw)
	}
}

// No binary is not a clean tree. The launcher's own rule — a hook that cannot
// get its binary warns and lets the session continue — has to survive the
// translation into pi, or procoder becomes the thing that broke editing.
//
// proved by: treating an empty verdict as an allow with no notice, or as a block.
func TestPiHooksDegradeToAWarning(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{
		"PROCODER_BIN": filepath.Join(dir, "procoder-that-is-not-here"),
		"FIXTURE_LOG":  filepath.Join(dir, "invocations.log"),
	}
	got := piHarness(t, "gate", map[string]any{"command": "git commit -m notes"}, env)
	if len(got["results"].([]any)) != 0 {
		t.Errorf("a gate that could not run blocked the commit: %+v", got["results"])
	}
	notices, _ := got["notices"].([]any)
	if len(notices) == 0 {
		t.Fatal("a gate that could not run said nothing")
	}
	notice, _ := notices[0].(map[string]any)
	if message, _ := notice["message"].(string); !strings.Contains(message, "NOT") {
		t.Errorf("the notice reads %q, which could be mistaken for a pass", message)
	}

	tool := piHarness(t, "tool", map[string]any{"params": map[string]any{"command": "check"}}, env)
	if tool["isError"] != true {
		t.Error("a procoder that never started is not a report that came back clean")
	}
}

// The manifest is what a host reads before any code runs. It once pointed pi at
// commands/ as a skill directory, which collapsed 34 files into one skill named
// after the directory; skills/ holds the one file that is a skill.
//
// proved by: restoring pi.skills to ./commands, or dropping the pi-package
// keyword the gallery indexes on.
func TestPiManifestPointsAtSkillsAndNotCommands(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Keywords []string `json:"keywords"`
		Pi       struct {
			Extensions []string `json:"extensions"`
			Skills     []string `json:"skills"`
			Prompts    []string `json:"prompts"`
		} `json:"pi"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if strings.Join(manifest.Pi.Skills, ",") != "./skills" {
		t.Errorf("pi.skills is %v; commands/ is not a skill directory", manifest.Pi.Skills)
	}
	if len(manifest.Pi.Prompts) != 0 {
		t.Errorf("pi.prompts is %v; the command set registers at load and needs no template path", manifest.Pi.Prompts)
	}
	if strings.Join(manifest.Pi.Extensions, ",") != "./pi-extension/index.mjs" {
		t.Errorf("pi.extensions is %v, and the drift guard reads this file", manifest.Pi.Extensions)
	}
	keyed := false
	for _, k := range manifest.Keywords {
		if k == "pi-package" {
			keyed = true
		}
	}
	if !keyed {
		t.Error("no pi-package keyword, so the package gallery will not index it")
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "procoder", "SKILL.md")); err != nil {
		t.Errorf("pi.skills names ./skills, which holds no SKILL.md: %v", err)
	}
}

// The host row is how the next reader learns what pi gets. A row that says
// "supported" while the four enforcement points go unnamed is how this spec got
// written in the first place.
//
// proved by: shortening the pi row in docs/portability.md to a host name.
func TestPiHostRowStatesItsSurfaces(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "portability.md"))
	if err != nil {
		t.Fatal(err)
	}
	row := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "| pi ") {
			row = line
		}
	}
	if row == "" {
		t.Fatal("no pi row in docs/portability.md")
	}
	for _, want := range []string{
		"tool_call", "tool_result", "agent_settled", "before_agent_start", "commands",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("the pi row does not name %s:\n%s", want, row)
		}
	}
}

// pi is detected rather than assumed. Without this, the envelope pi receives is
// whatever the Claude default happens to emit, and a future change to that
// default would change pi silently.
//
// proved by: removing the Pi case from Detect — TestDetectSelectsHostBySignalAnd
// Precedence in the host package catches that; this one pins the adapter's side
// of the same fact, which no Go test can reach.
func TestPiHostIsNamedInTheGate(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "internal", "host", "host.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `Pi Host = "pi"`) {
		t.Error("host.go names no pi host, so pi is answered by the Claude default")
	}
	if !strings.Contains(text, "PI_CODING_AGENT") {
		t.Error("nothing reads the variable pi actually sets")
	}
}
