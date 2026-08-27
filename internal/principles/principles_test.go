package principles

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"procoder/internal/releases"
	"procoder/internal/status"
)

// hostEnv puts the process in one host's environment: every sniffed variable
// is set explicitly, so an ambient one cannot decide the answer.
func hostEnv(t *testing.T, want string) {
	t.Helper()
	for _, v := range []string{"COPILOT_PLUGIN_DATA", "CLAUDE_PLUGIN_ROOT", "PLUGIN_DATA", "QODER_SESSION_ID"} {
		t.Setenv(v, "")
	}
	switch want {
	case "copilot":
		t.Setenv("COPILOT_PLUGIN_DATA", "x")
	case "codex":
		t.Setenv("PLUGIN_DATA", "x")
	case "qoder":
		t.Setenv("QODER_SESSION_ID", "x")
	}
}

func hookOutput(t *testing.T, want, root string) string {
	t.Helper()
	hostEnv(t, want)
	var got []string
	if code := RunHook(root, nil, func(s string) { got = append(got, s) }); code != 0 {
		t.Fatalf("hook exit %d — a SessionStart hook must never fail the session", code)
	}
	return strings.Join(got, "\n")
}

// The status block must arrive in every envelope: a host that gets the
// principles without the state is a session that opens blind, which is the
// whole failure this work exists to close.
func TestHookCarriesTheStatusBlockInEveryEnvelope(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		host    string
		extract func(string) string
	}{
		{"claude", func(s string) string { return s }},
		{"copilot", func(s string) string {
			var m map[string]string
			if json.Unmarshal([]byte(s), &m) != nil {
				t.Fatalf("copilot envelope is not JSON: %s", s)
			}
			return m["additionalContext"]
		}},
		{"codex", hookSpecific(t)},
		{"qoder", hookSpecific(t)},
	} {
		t.Run(tc.host, func(t *testing.T) {
			text := tc.extract(hookOutput(t, tc.host, root))
			if !strings.Contains(text, "Engineering principles") {
				t.Fatalf("%s envelope lost the principles text:\n%s", tc.host, text)
			}
			if !strings.Contains(text, status.Header) {
				t.Fatalf("%s envelope carries no status block:\n%s", tc.host, text)
			}
			if i, j := strings.Index(text, "Engineering principles"), strings.Index(text, status.Header); j < i {
				t.Fatalf("%s envelope puts the state before the principles", tc.host)
			}
			if !strings.Contains(text, "open tasks:") {
				t.Fatalf("%s envelope has the header but no facts:\n%s", tc.host, text)
			}
		})
	}
}

func hookSpecific(t *testing.T) func(string) string {
	return func(s string) string {
		t.Helper()
		var m struct {
			HookSpecificOutput struct {
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal([]byte(s), &m) != nil {
			t.Fatalf("envelope is not JSON: %s", s)
		}
		return m.HookSpecificOutput.AdditionalContext
	}
}

// `procoder principles` is the plain read of the philosophy and stays that:
// the state block belongs to the session hook, not to the document.
func TestPlainRunStaysPrinciplesOnly(t *testing.T) {
	var got []string
	if code := Run(t.TempDir(), func(s string) { got = append(got, s) }); code != 0 {
		t.Fatalf("principles exit %d", code)
	}
	if text := strings.Join(got, "\n"); strings.Contains(text, status.Header) {
		t.Fatalf("`procoder principles` must not carry the status block:\n%s", text)
	}
}

// The SessionStart path is what a session waits on, so it is timed here on
// the repository the hook actually runs in.
func TestSessionStartStaysInsideTheBudget(t *testing.T) {
	hostEnv(t, "claude")
	// The version check is the slowest thing the hook can do, and with
	// Version left at dev it never runs at all — the budget guard would
	// cover everything except the part most likely to blow it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer srv.Close()
	prevHost, prevVer, prevErr := releases.APIHost, Version, Stderr
	releases.APIHost, Version, Stderr = srv.URL, "1.0.0", io.Discard
	defer func() { releases.APIHost, Version, Stderr = prevHost, prevVer, prevErr }()

	start := time.Now()
	RunHook("../..", nil, func(string) {})
	elapsed := time.Since(start)
	if elapsed > status.Budget {
		t.Fatalf("SessionStart took %s — the budget is %s", elapsed, status.Budget)
	}
	t.Logf("SessionStart took %s (budget %s)", elapsed, status.Budget)
}

// N-03: the version check must not hold a session start open. The hook
// prints its payload and the warning arrives after it, or not at all — a
// GitHub that never answers costs the timeout once, not the session.
// proved by: awaited the check before hookText — a hanging GitHub then
// blocks every session start behind it.
func TestTheVersionCheckNeverHoldsTheSessionOpen(t *testing.T) {
	// Long enough to outlast the one-second check by a wide margin, short
	// enough that httptest's Close — which waits for the handler — does not
	// make this test the slowest thing in the suite.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer slow.Close()
	prevHost, prevVer := releases.APIHost, Version
	releases.APIHost = slow.URL
	Version = "0.0.1"
	defer func() { releases.APIHost, Version = prevHost, prevVer }()

	var buf bytes.Buffer
	prevErr := Stderr
	Stderr = &buf
	defer func() { Stderr = prevErr }()

	root := t.TempDir()
	var lines []string
	var firstOut time.Duration
	start := time.Now()
	if code := RunHook(root, nil, func(s string) {
		if len(lines) == 0 {
			firstOut = time.Since(start)
		}
		lines = append(lines, s)
	}); code != 0 {
		t.Fatalf("the hook must answer: exit %d", code)
	}
	if len(lines) == 0 {
		t.Fatal("the principles payload must be printed regardless")
	}
	// The property is not "the hook finishes eventually" — a fully
	// synchronous hook finishes inside the check's own one-second cap, so a
	// five-second bound passes on the very mutation this test exists to
	// reject. What must hold is that the PAYLOAD is out before the check
	// has had time to answer: the session is not waiting on GitHub.
	if firstOut >= releases.Timeout {
		t.Errorf("the payload waited %s for a check capped at %s — the session start was held behind GitHub",
			firstOut, releases.Timeout)
	}
	if buf.Len() != 0 {
		t.Errorf("a check that did not answer says nothing: %q", buf.String())
	}
}

// R-07: the warning goes to stderr, never into a payload three of the four
// hosts parse as JSON.
func TestTheVersionWarningStaysOutOfTheHookPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer srv.Close()
	prevHost, prevVer := releases.APIHost, Version
	releases.APIHost = srv.URL
	Version = "1.0.0"
	defer func() { releases.APIHost, Version = prevHost, prevVer }()

	var buf bytes.Buffer
	prevErr := Stderr
	Stderr = &buf
	defer func() { Stderr = prevErr }()

	var lines []string
	RunHook(t.TempDir(), nil, func(s string) { lines = append(lines, s) })
	if !strings.Contains(buf.String(), "9.9.9") {
		t.Errorf("the warning must reach stderr: %q", buf.String())
	}
	if strings.Contains(strings.Join(lines, "\n"), "9.9.9") {
		t.Error("the warning must never land in the hook's stdout payload")
	}
}

// [version] check = "off" asks GitHub nothing at all.
func TestTheConfigKnobSilencesTheCheckEntirely(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("check = off must not query GitHub")
	}))
	defer srv.Close()
	prevHost, prevVer := releases.APIHost, Version
	releases.APIHost = srv.URL
	Version = "1.0.0"
	defer func() { releases.APIHost, Version = prevHost, prevVer }()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".procoder", "config.toml"),
		[]byte("[version]\ncheck = \"off\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	prevErr := Stderr
	Stderr = &buf
	defer func() { Stderr = prevErr }()
	RunHook(root, nil, func(string) {})
	if buf.Len() != 0 {
		t.Errorf("check = off says nothing: %q", buf.String())
	}
}

// S-5: the decision rule is the deliverable, so its absence must fail here
// rather than be noticed months later by somebody wondering why the agent
// never asks.
//
// The rule exists because of a concrete failure: two decisions were put to
// the user as prose at the end of a long status report while the work
// continued underneath them, and the user's response was that they had not
// been asked. Nothing in the principles covered a decision — only questions
// arising from findings — so nothing had gone wrong by the rules as
// written. That is what this pins.
//
// Phrases, not the whole paragraph: pinning the prose would make every
// wording change a test failure, and the rule would get weakened to keep
// the suite green.
//
// proved by: deleted the decision bullets from the principles text — each
// missing phrase is named.
func TestThePrinciplesCarryTheDecisionRule(t *testing.T) {
	// Default, not Effective: what is pinned is the rule procoder SHIPS.
	// Effective would read this repository's own override if it had one,
	// and the test would then pass on a machine where the shipped text had
	// lost the rule entirely.
	text := Default
	for _, phrase := range []string{
		"A DECISION about what to do next is not yours",
		"STOP means stop",
		"structured question tool",
		// The queue is useless if nothing tells an agent the file exists.
		// docs/commands.md documents it, but an agent reads the principles
		// at session start and the docs never.
		".procoder/ask/decisions.md",
		// The mutation-restore rule. It exists because a feature was lost
		// exactly that way in this repository, so its absence must fail
		// rather than be rediscovered the same way.
		"TAKE THE SNAPSHOT IMMEDIATELY BEFORE EACH MUTATION",
		"untested by definition",
		// Merge-conflict discipline (#193). The last clause is the one
		// that came from a real failure here: git split a conflict through
		// the middle of a function, "keep both sides" truncated a test,
		// and only the compiler noticed.
		"resolved hunk by hunk",
		// Both commands named, not a bare --abort: the shipped text must
		// be as specific as AGENTS.md and the generated rule files, or the
		// same rule says two different things depending where it is read.
		// Raised in review on #216.
		"git merge --abort",
		"git rebase --abort",
		"middle of a function",
		// Relaying procoder's own findings, and the override that keeps
		// brevity away from consequences.
		"Relaying a procoder report",
		"never for consequences",
	} {
		if !strings.Contains(text, phrase) {
			t.Errorf("the principles no longer say %q — the rule is the deliverable", phrase)
		}
	}
}

// The principles block is ~7KB and SessionStart fires on startup, resume,
// clear and compact. On resume and compact the text is already in the
// conversation, so sending it again pays for telling the model what it can
// already read — ~187k tokens across one day of resumed sessions (#175).
//
// proved by: dropping the host.Resumed branch from RunHook — the resumed
// cases then emit the full text and this test sees it.
func TestAResumedSessionGetsAPointerNotTheWholeText(t *testing.T) {
	root := t.TempDir()
	full := func(payload string) string {
		var b strings.Builder
		RunHook(root, strings.NewReader(payload), func(s string) { b.WriteString(s + "\n") })
		return b.String()
	}

	startup := full(`{"source":"startup"}`)
	resumed := full(`{"source":"resume"}`)
	compacted := full(`{"source":"compact"}`)

	if len(startup) < 1000 {
		t.Fatalf("a fresh start must carry the whole text, got %d bytes", len(startup))
	}
	for name, got := range map[string]string{"resume": resumed, "compact": compacted} {
		if len(got) >= len(startup) {
			t.Errorf("%s sent %d bytes against startup's %d — it is not a pointer", name, len(got), len(startup))
		}
		if !strings.Contains(got, "procoder principles") {
			t.Errorf("%s must still say the rules are in force: %q", name, got)
		}
		if !strings.Contains(got, "procoder principles`") && !strings.Contains(got, "`procoder principles`") {
			t.Errorf("%s must say where to read them: %q", name, got)
		}
	}
}

// Everything that is not a resume or a compact gets the whole text,
// including a payload that could not be read at all. Saying too much costs
// tokens; saying too little leaves a session governed by rules nobody sent,
// and only one of those is recoverable by the reader.
//
// proved by: treating an unknown source as resumed — the last three cases
// then get a pointer to rules that were never sent.
func TestAnythingOtherThanAResumeGetsTheWholeText(t *testing.T) {
	root := t.TempDir()
	for _, payload := range []string{
		`{"source":"startup"}`,
		`{"source":"clear"}`,
		`{"source":"something-new"}`,
		`not json at all`,
		``,
	} {
		var b strings.Builder
		RunHook(root, strings.NewReader(payload), func(s string) { b.WriteString(s + "\n") })
		if len(b.String()) < 1000 {
			t.Errorf("payload %q got %d bytes — the rules must arrive in full when the start is not known to be resumed",
				payload, b.Len())
		}
	}
}

// The four-pass discipline (#203). Nothing on disk can verify an agent did
// four distinct passes, which is why this is a rule and not a check — and
// exactly why its absence has to fail something, or it quietly stops being
// part of the contract.
//
// proved by: the four-pass bullet deleted from AGENTS.md — the phrases are
// named here as missing.
func TestTheAgentContractCarriesTheFourPasses(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "AGENTS.md"))
	if err != nil {
		t.Skipf("no AGENTS.md here: %v", err)
	}
	// Whitespace-normalised: the file is prettier-formatted, so any phrase
	// long enough to be worth pinning will eventually be reflowed across a
	// line break. Matching raw text would make this fail on a wrap rather
	// than on the rule going missing.
	text := strings.Join(strings.Fields(string(raw)), " ")
	for _, phrase := range []string{
		"four passes",
		"reviewer who did not write it",
		"adversarial",
		"edge-case",
		"not from asking the same one harder",
	} {
		if !strings.Contains(text, phrase) {
			t.Errorf("AGENTS.md no longer says %q — the discipline is the deliverable", phrase)
		}
	}
}
