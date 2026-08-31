package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsGitCommit(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"git commit -m x", true},
		{"git commit", true},
		{"git -C /path commit", true},
		{"git -C \"/path with spaces\" commit -m x", true},
		{"env FOO=1 git commit", true},
		{"FOO=1 git commit -m x", true},
		{"cd x && git commit -m \"y\"", true},
		{"git   commit", true},
		{"go build && git commit -m x", true},
		{"/usr/bin/git commit", true},
		{"git -c user.name=x commit -m y", true},
		{"git --git-dir=.git commit", true},
		{"git commit --no-verify", true},

		{"echo \"commit\"", false},
		{"echo commit", false},
		{"echo \"git commit\"", false},
		{"gh pr merge", false},
		{"git merge --continue", false},
		{"git log --format=%h commit", false},
		{"cat notes/commit-message.txt", false},
		{"rg commit internal/", false},
		{"git push", false},
		{"", false},
	}
	for _, c := range cases {
		if got := parseCommand(c.command).isCommit; got != c.want {
			t.Errorf("parseCommand(%q).isCommit = %v, want %v", c.command, got, c.want)
		}
	}
}

func TestParseCommandPicksUpBypassAndDirectory(t *testing.T) {
	if p := parseCommand("git commit --no-verify -m x"); !p.isCommit || !p.noVerify {
		t.Fatalf("--no-verify not seen: %+v", p)
	}
	if p := parseCommand("git commit -n -m x"); !p.noVerify {
		t.Fatalf("-n is git's short --no-verify: %+v", p)
	}
	if p := parseCommand("git commit -m \"--no-verify\""); p.noVerify {
		t.Fatalf("a quoted message is not a flag: %+v", p)
	}
	if p := parseCommand("git -C /repo commit"); p.dir != "/repo" {
		t.Fatalf("-C directory lost: %+v", p)
	}
	if p := parseCommand("cd sub && git commit"); p.dir != "sub" {
		t.Fatalf("cd directory lost: %+v", p)
	}
}

// The acknowledgment must reach the gate from wherever git reads the
// message, not only from -m: an agent writing a body with a preformatted
// block reaches for a heredoc, and a `docs: none — <reason>` line that the
// gate cannot see sends the user to a remedy that cannot work.
// proved by: kept the -m-only scan — the heredoc and -F messages come back
// empty and the obligation stands with a correct acknowledgment in hand.
func TestTheMessageIsReadFromEveryFormGitReadsItFrom(t *testing.T) {
	heredoc := parseCommand("git commit -F - <<'EOF'\nchore: adopt the config\n\ndocs: none — the file documents itself.\nEOF")
	if !strings.Contains(heredoc.message, "docs: none") {
		t.Errorf("heredoc message lost: %q", heredoc.message)
	}
	// an indented line that reads like the delimiter is body, not the end of
	// it: only `<<-` allows a tab-indented terminator, and neither form
	// allows a space-indented one
	indented := parseCommand("git commit -F - <<'EOF'\nchore: x\n\n    EOF\n\ndocs: none — reason.\nEOF")
	if !strings.Contains(indented.message, "docs: none") {
		t.Errorf("an indented look-alike ended the message early: %q", indented.message)
	}
	tabbed := parseCommand("git commit -F - <<-EOF\nchore: x\n\ndocs: none — reason.\n\tEOF")
	if !strings.Contains(tabbed.message, "docs: none") {
		t.Errorf("<<- accepts a tab-indented terminator: %q", tabbed.message)
	}
	for _, cmd := range []string{
		"git commit -F msg.txt",
		"git commit --file msg.txt",
		"git commit --file=msg.txt",
		"git commit -Fmsg.txt",
	} {
		if p := parseCommand(cmd); p.messageFile != "msg.txt" {
			t.Errorf("%s: message file lost: %+v", cmd, p)
		}
	}
	// a quoted escape inside the message keeps the argument whole — the
	// tokenizer used to end the word at the backslash and lose every -m
	// after it, the acknowledgment included
	esc := parseCommand(`git commit -m "feat: say \"no\" politely" -m "docs: none — behaviour only."`)
	if !strings.Contains(esc.message, "docs: none") {
		t.Errorf("escaped quote swallowed the rest: %q", esc.message)
	}
	multi := parseCommand("git commit -m \"chore: x\" -m \"body:\n\n  key  value\" -m \"docs: none — reason.\"")
	if !strings.Contains(multi.message, "docs: none") {
		t.Errorf("multi-line -m lost the ack: %q", multi.message)
	}
}

// -F names a file the gate must actually read, in the directory the command
// runs in — the message only clears the obligation if it arrives at the
// check.
// proved by: left readMessageFile out of PreToolUse — the acknowledgment in
// the file is invisible and the commit stays blocked.
func TestAnAcknowledgmentInAMessageFileClearsTheObligation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "msg.txt"), []byte("chore: x\n\ndocs: none — reason.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readMessageFile(dir, "msg.txt"); !strings.Contains(got, "docs: none") {
		t.Errorf("message file not read: %q", got)
	}
	if got := readMessageFile(dir, "absent.txt"); got != "" {
		t.Errorf("an unreadable file is no message, got %q", got)
	}
}

// --- PreToolUse ---

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	return root
}

// commitAll leaves the tree with nothing changed, so the gate has nothing
// to judge and cannot depend on which tools the machine happens to carry.
func commitAll(t *testing.T, root string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "-A")
	run("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "fixture")
}

func writeAt(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitPayload(t *testing.T, root, command string) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"cwd":        root,
		"tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewBuffer(b)
}

// decisionOf reads the envelope back, and insists the hook never fails the host.
func decisionOf(t *testing.T, stdin *bytes.Buffer) (verdict, reason string) {
	t.Helper()
	var out bytes.Buffer
	if code := PreToolUse(stdin, &out); code != 0 {
		t.Fatalf("PreToolUse exit %d — the decision lives in the payload, never the exit code", code)
	}
	if strings.TrimSpace(out.String()) == "" {
		return "", ""
	}
	var resp decisionOutput
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("output is not the host's decision envelope: %v\n%s", err, out.String())
	}
	if resp.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("wrong hook event name: %s", resp.HookSpecificOutput.HookEventName)
	}
	return resp.HookSpecificOutput.PermissionDecision, resp.HookSpecificOutput.PermissionDecisionReason
}

func TestNonBashToolIsUntouched(t *testing.T) {
	b, err := json.Marshal(map[string]any{
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": "/tmp/a.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := decisionOf(t, bytes.NewBuffer(b)); v != "" {
		t.Fatalf("a non-Bash tool must produce no output, got %q", v)
	}
}

func TestNonCommitCommandIsUntouched(t *testing.T) {
	if v, _ := decisionOf(t, commitPayload(t, t.TempDir(), "echo \"commit\"")); v != "" {
		t.Fatalf("a non-commit command must produce no output, got %q", v)
	}
}

// A tree with nothing changed is the only "clean" a machine without the
// toolchain can honestly produce: with changed files present, a missing
// formatter or scanner is UNCHECKED, and unchecked is never clean. That is
// the product working, so the test uses the committed-and-untouched tree
// the spec names instead of assuming an installed toolchain.
func TestCleanGateLetsTheCommitThrough(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeAt(t, root, "README.md", "# demo\n\nA line of prose.\n")
	commitAll(t, root)
	if v, r := decisionOf(t, commitPayload(t, root, "git commit -m x")); v == "deny" {
		t.Fatalf("a tree with nothing changed must not be stopped: %s", r)
	}
}

func TestBlockingFindingStopsTheCommitAndNamesIt(t *testing.T) {
	requireGit(t)
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not installed")
	}
	root := gitRepo(t)
	// Formatting is a house rule since #172, so the fixture has to say it
	// adopted procoder — in somebody else's repository the gate does not
	// check formatting at all, and this test would be asserting the wrong
	// mode.
	writeAt(t, root, ".procoder/config.toml", "")
	writeAt(t, root, "bad.go", "package main\nfunc  main( ){}\n")
	v, r := decisionOf(t, commitPayload(t, root, "go build && git commit -m x"))
	if v != "deny" {
		t.Fatalf("an unformatted file must stop the commit, got %q (%s)", v, r)
	}
	if !strings.Contains(r, "bad.go") {
		t.Fatalf("the refusal must name the finding:\n%s", r)
	}
	if !strings.Contains(r, "blocking finding") {
		t.Fatalf("the refusal must say what it is:\n%s", r)
	}
}

func TestNoVerifyPassesThroughLoudly(t *testing.T) {
	requireGit(t)
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not installed")
	}
	root := gitRepo(t)
	writeAt(t, root, "bad.go", "package main\nfunc  main( ){}\n")
	v, r := decisionOf(t, commitPayload(t, root, "git commit --no-verify -m x"))
	if v != "allow" {
		t.Fatalf("--no-verify must pass through, got %q", v)
	}
	if !strings.Contains(r, "bypassed") {
		t.Fatalf("the bypass must be visible, never silent:\n%s", r)
	}
}

func TestReportPolicyAllowsAndStillNamesTheFindings(t *testing.T) {
	requireGit(t)
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not installed")
	}
	root := gitRepo(t)
	writeAt(t, root, "bad.go", "package main\nfunc  main( ){}\n")
	writeAt(t, root, ".procoder/config.toml", "[git]\ncommit_gate = \"report\"\n")
	v, r := decisionOf(t, commitPayload(t, root, "git commit -m x"))
	if v != "allow" {
		t.Fatalf("report must let the commit proceed, got %q", v)
	}
	if !strings.Contains(r, "bad.go") {
		t.Fatalf("report must still hand back the findings:\n%s", r)
	}
}

func TestOffPolicySkipsTheCheckEntirely(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeAt(t, root, "bad.go", "package main\nfunc  main( ){}\n")
	writeAt(t, root, ".procoder/config.toml", "[git]\ncommit_gate = \"off\"\n")
	if v, r := decisionOf(t, commitPayload(t, root, "git commit -m x")); v != "" {
		t.Fatalf("off must say nothing at all, got %q (%s)", v, r)
	}
}

func TestMalformedPayloadAllowsAndSaysItCouldNotJudge(t *testing.T) {
	v, r := decisionOf(t, bytes.NewBufferString("{\"tool_name\": \"Bash\", trunc"))
	if v != "allow" {
		t.Fatalf("a broken payload must never wedge the session, got %q", v)
	}
	if !strings.Contains(r, "did not judge") {
		t.Fatalf("the reason must say the gate could not judge:\n%s", r)
	}
}

// --- InstallGit ---

func TestInstallGitPrintsAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	var lines []string
	if code := InstallGit(root, func(s string) { lines = append(lines, s) }); code != 0 {
		t.Fatalf("InstallGit exit %d", code)
	}
	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"#!/bin/sh",
		"procoder check",
		"cat > .git/hooks/pre-commit <<'PROCODER_EOF'",
		"chmod +x .git/hooks/pre-commit",
		"exit 1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("printed hook is missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\\") {
		t.Fatalf("printed paths must use forward slashes:\n%s", text)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("InstallGit must write nothing; found %v (%v)", entries, err)
	}
}

// The message file a commit names with -F is often the command's own
// output — `cat > msg.txt <<'EOF' … EOF` then `git commit -F msg.txt` — and
// the gate judges the command BEFORE the shell has run, so the file does
// not exist yet. The acknowledgment was in the heredoc all along, in the
// command line the gate was handed, and this is the shape that blocked a
// real merge with "no commit message reached this check".
//
// proved by: the heredocInto call removed from PreToolUse — the case that
// carries the acknowledgment in its own heredoc re-blocks on the obligation.
func TestACommandOwnedMessageFileClearsTheObligation(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	// the obligation only BLOCKS when the repository says docs are blocking;
	// the default is report, and a report would let both the control and the
	// negative case through, so the fixture opts in the way the procoder
	// repository's own config does.
	writeAt(t, root, ".procoder/config.toml", "[docs]\npolicy = \"block\"\n")
	writeAt(t, root, "README.md", "# demo\n\n.toolpin pins the tool versions.\n")
	commitAll(t, root)
	writeAt(t, root, ".toolpin", "RUFF=0.1\n")

	// Assert the OBLIGATION, not a clean gate: a machine without the security
	// tools carries NOT-checked findings in an adopted repository whether or
	// not this test exists, and those are environment, not the mechanism
	// under test.
	ack := "cat > msg.txt <<'EOF'\nchore: pin bump\n\ndocs: none — the pin file is self-describing\nEOF\ngit commit -F msg.txt"
	if _, r := decisionOf(t, commitPayload(t, root, ack)); strings.Contains(r, "documentation obligation") {
		t.Fatalf("an acknowledgment in the command's own heredoc must clear the obligation:\n%s", r)
	}

	noAck := "cat > msg2.txt <<'EOF'\nchore: pin bump\nEOF\ngit commit -F msg2.txt"
	v, r := decisionOf(t, commitPayload(t, root, noAck))
	if !strings.Contains(r, "documentation obligation") {
		t.Fatalf("without the acknowledgment line the obligation must still block (decision %q):\n%s", v, r)
	}

	other := "cat > other.txt <<'EOF'\ndocs: none — not for this commit\nEOF\ngit commit -F msg3.txt"
	if _, r := decisionOf(t, commitPayload(t, root, other)); !strings.Contains(r, "documentation obligation") {
		t.Fatalf("a heredoc that writes a different file must not answer for this commit's message:\n%s", r)
	}
}

func TestHeredocIntoReadsOnlyTheHeredocThatWritesTheNamedFile(t *testing.T) {
	cmd := "cat > msg.txt <<'EOF'\nchore: x\n\ndocs: none — reason\nEOF\ngit commit -F msg.txt"
	if got, ok := heredocInto(cmd, "msg.txt"); !ok || !strings.Contains(got, "docs: none") {
		t.Errorf("the heredoc that writes msg.txt must read as its message: ok=%v got=%q", ok, got)
	}
	if _, ok := heredocInto(cmd, "other.txt"); ok {
		t.Error("a heredoc that writes msg.txt must not answer for other.txt")
	}
	if _, ok := heredocInto("git commit -F msg.txt", "msg.txt"); ok {
		t.Error("a command with no heredoc has no heredoc message")
	}
	// an append leaves the file's prior content unknown: the gate cannot
	// vouch for a message it cannot see in full
	appe := "cat >> msg.txt <<'EOF'\nx\nEOF"
	if _, ok := heredocInto(appe, "msg.txt"); ok {
		t.Error("an append must not be read as the whole message")
	}
}
