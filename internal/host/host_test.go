package host

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// clear blanks every variable Detect consults, so a case declares its whole
// world. The test process itself runs under an AI agent that may well export
// CLAUDE_PLUGIN_ROOT, and a case that only sets what it cares about would
// otherwise inherit the developer's machine and answer differently in CI.
func clear(t *testing.T) {
	t.Helper()
	for _, k := range []string{"COPILOT_PLUGIN_DATA", "CLAUDE_PLUGIN_ROOT", "PLUGIN_DATA", "QODER_SESSION_ID", "PI_CODING_AGENT"} {
		t.Setenv(k, "")
	}
}

// Detect decides which JSON envelope every hook emits, so a wrong answer
// breaks hooks silently on that host — one case per signal, and one per
// collision between signals, because the ORDER of the checks is the contract.
//
// proved by: reordering Detect so the PLUGIN_DATA (Codex) check runs before
// the Copilot check — the "copilot wins over codex" case then returns codex.
// Or moving the PI_CODING_AGENT check above Copilot's, which the last two cases
// catch.
func TestDetectSelectsHostBySignalAndPrecedence(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Host
	}{
		{
			name: "nothing set falls back to claude",
			env:  nil,
			want: Claude,
		},
		{
			name: "COPILOT_PLUGIN_DATA names copilot",
			env:  map[string]string{"COPILOT_PLUGIN_DATA": "/some/where"},
			want: Copilot,
		},
		{
			name: "PLUGIN_DATA alone names codex",
			env:  map[string]string{"PLUGIN_DATA": "/some/where"},
			want: Codex,
		},
		{
			name: "QODER_SESSION_ID alone names qoder",
			env:  map[string]string{"QODER_SESSION_ID": "abc123"},
			want: Qoder,
		},
		{
			name: "vs code copilot is spotted by its plugin root",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/Users/x/.vscode/extensions/agent-plugins/procoder"},
			want: Copilot,
		},
		{
			name: "claude's own plugin root is not mistaken for copilot",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/Users/x/.claude/plugins/procoder"},
			want: Claude,
		},
		{
			// Copilot exports the Claude variable too, so a root that carries
			// only ONE of the two markers must not be enough to claim copilot.
			name: "agent-plugins outside .vscode stays claude",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/opt/agent-plugins/procoder"},
			want: Claude,
		},
		{
			name: ".vscode without agent-plugins stays claude",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/Users/x/.vscode/extensions/procoder"},
			want: Claude,
		},
		{
			// Copilot sets both; whichever check runs first decides, and it
			// must be copilot's.
			name: "copilot wins over codex",
			env:  map[string]string{"COPILOT_PLUGIN_DATA": "/c", "PLUGIN_DATA": "/p"},
			want: Copilot,
		},
		{
			name: "vs code copilot root wins over codex",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/x/.vscode/agent-plugins", "PLUGIN_DATA": "/p"},
			want: Copilot,
		},
		{
			name: "codex wins over qoder",
			env:  map[string]string{"PLUGIN_DATA": "/p", "QODER_SESSION_ID": "abc"},
			want: Codex,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clear(t)
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			if got := Detect(); got != c.want {
				t.Errorf("Detect() = %q, want %q", got, c.want)
			}
		})
	}
}

// pi's own variable is a positive signal, which is why it beats the Claude
// default — and Copilot's is a positive signal too, and outranks it because a
// pi session run inside a Copilot one is answered by the host whose envelope
// actually gets parsed.
//
// proved by: moving the PI_CODING_AGENT check above Copilot's in Detect — the
// first case below then answers pi.
func TestPiRanksBelowThePositiveSignalsAboveIt(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Host
	}{
		{
			name: "PI_CODING_AGENT names pi",
			env:  map[string]string{"PI_CODING_AGENT": "true"},
			want: Pi,
		},
		{
			// CLAUDE_PLUGIN_ROOT alone is NOT a claim of being Claude Code: the
			// cases above exist because a copilot session carries it too, so
			// Detect already refuses to read it as a signal. It reaches the
			// default only because nothing answered, and a positive signal beats
			// a default. The cost, stated: a Claude Code session launched from
			// inside a pi terminal answers pi, because pi's variable leaked into
			// its environment and nothing here tells a leak from a host.
			name: "pi beats the claude root it already refuses to trust",
			env:  map[string]string{"CLAUDE_PLUGIN_ROOT": "/x/.claude/plugins/procoder", "PI_CODING_AGENT": "true"},
			want: Pi,
		},
		{
			name: "copilot wins over pi",
			env:  map[string]string{"COPILOT_PLUGIN_DATA": "/c", "PI_CODING_AGENT": "true"},
			want: Copilot,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clear(t)
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			if got := Detect(); got != c.want {
				t.Errorf("Detect() = %q, want %q", got, c.want)
			}
		})
	}
}

// The four host names are written into hook envelopes and matched by string
// elsewhere, so they are wire values: renaming one is a breaking change, not
// a refactor.
//
// proved by: changing the Copilot constant to "github-copilot" — the case
// below fails, as does every consumer that switches on the literal.
func TestHostNamesAreTheWireValues(t *testing.T) {
	for _, c := range []struct {
		got  Host
		want string
	}{
		{Claude, "claude"},
		{Codex, "codex"},
		{Copilot, "copilot"},
		{Qoder, "qoder"},
		{Pi, "pi"},
	} {
		if string(c.got) != c.want {
			t.Errorf("host constant = %q, want %q", c.got, c.want)
		}
	}
}

// The SessionStart payload carries why the session started. procoder needs
// it because the principles injection is ~7KB re-sent on every matched
// start, and on resume/compact that text is already in the conversation —
// one day of resumed sessions measured ~187k tokens of repetition (#175).
//
// proved by: returning the raw payload string instead of the parsed
// `source` — every case below then reports the whole JSON as the source.
func TestSessionSourceReadsWhyTheSessionStarted(t *testing.T) {
	for _, c := range []struct{ name, payload, want string }{
		{"startup", `{"hook_event_name":"SessionStart","source":"startup"}`, "startup"},
		{"resume", `{"source":"resume"}`, "resume"},
		{"compact", `{"source":"compact"}`, "compact"},
		{"clear", `{"source":"clear"}`, "clear"},
		{"mixed case", `{"source":"Resume"}`, "resume"},
		{"padded", `{"source":"  compact  "}`, "compact"},
		{"no source field", `{"hook_event_name":"SessionStart"}`, ""},
		{"not json", `this is not a payload`, ""},
		{"empty", ``, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := SessionSource(strings.NewReader(c.payload)); got != c.want {
				t.Errorf("SessionSource(%q) = %q, want %q", c.payload, got, c.want)
			}
		})
	}
	if got := SessionSource(nil); got != "" {
		t.Errorf("a nil reader is not a source, got %q", got)
	}
}

// Only the two starts that already carry the earlier context count as
// resumed. Everything else — including a source that could not be read —
// is a fresh start, because the fallback has to be sending the rules
// rather than a pointer to rules nobody sent.
//
// proved by: adding "" to the resumed set — the unknown case then gets a
// pointer, and a session whose payload failed to parse runs ungoverned.
func TestOnlyResumeAndCompactCountAsResumed(t *testing.T) {
	for _, c := range []struct {
		source string
		want   bool
	}{
		{"resume", true}, {"compact", true},
		{"startup", false}, {"clear", false}, {"", false}, {"unknown", false},
	} {
		if got := Resumed(c.source); got != c.want {
			t.Errorf("Resumed(%q) = %v, want %v", c.source, got, c.want)
		}
	}
}

// A SessionStart hook runs before the session can begin. A host that opens
// the pipe and sends nothing must not hold it open.
//
// proved by: removing the deadline from SessionSource — this test then
// hangs instead of returning.
func TestSessionSourceDoesNotWaitForeverOnASilentHost(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	done := make(chan string, 1)
	go func() { done <- SessionSource(pr) }()
	select {
	case got := <-done:
		if got != "" {
			t.Errorf("a silent host has no source, got %q", got)
		}
	case <-time.After(sessionSourceDeadline + 3*time.Second):
		t.Fatal("SessionSource did not give up on a host that sent nothing")
	}
}

// A terminal never sends EOF, so reading it costs the whole deadline and
// returns nothing anyway. Raised in review on #184: a host that invoked the
// hook with a terminal attached would pay 2s on every session start.
//
// The fake reports a character-device mode without needing a real tty,
// which no CI runner reliably has.
//
// proved by: the ModeCharDevice check removed from SessionSource — the
// test then takes the full deadline and fails the elapsed assertion.
func TestSessionSourceDoesNotReadATerminal(t *testing.T) {
	start := time.Now()
	if got := SessionSource(charDevice{}); got != "" {
		t.Errorf("SessionSource(terminal) = %q, want \"\"", got)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("reading a terminal took %s — the deadline was paid for an answer that was never coming", elapsed)
	}
}

// charDevice is a reader that says it is a terminal and blocks forever if
// anybody actually reads it — so a test that passes cannot be passing by
// having read it quickly.
type charDevice struct{}

func (charDevice) Read([]byte) (int, error) { select {} }

func (charDevice) Stat() (os.FileInfo, error) { return charDeviceInfo{}, nil }

type charDeviceInfo struct{ os.FileInfo }

func (charDeviceInfo) Mode() os.FileMode { return os.ModeCharDevice | 0o620 }

// DetectIn answers from the map it is given, never from the process.
//
// This is what makes one daemon able to serve two hosts: its own
// environment is whichever session happened to start it, and every request
// after the first would otherwise be shaped for the wrong host.
//
// proved by: pointing DetectIn back at os.Getenv — the Qoder request then
// answers Copilot, because the process carries a VS Code plugin root.
func TestDetectInReadsOnlyItsArgument(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_ROOT", "/x/.vscode/agent-plugins/copilot")
	if got := DetectIn(Env{"QODER_SESSION_ID": "x"}); got != Qoder {
		t.Fatalf("DetectIn read the process environment: want %q, got %q", Qoder, got)
	}
	if got := DetectIn(Env{}); got != Claude {
		t.Fatalf("an empty environment is the Claude default: want %q, got %q", Claude, got)
	}
}
