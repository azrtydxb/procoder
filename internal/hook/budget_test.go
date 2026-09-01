package hook

import (
	"strings"
	"testing"
	"unicode/utf8"

	"procoder/internal/format"
)

// proved by: joining every part unconditionally — what this did before —
// produces a payload the host truncates, and what is lost is whatever
// finding happened to be built last.
func TestFitStopsAtTheDeliveryBudget(t *testing.T) {
	big := strings.Repeat("x", maxContextBytes)
	got := fit([]part{{text: "first"}, {text: big}, {text: "third"}})

	if !strings.Contains(got, "first") {
		t.Fatal("the first part was dropped")
	}
	if strings.Contains(got, big) {
		t.Fatal("a part that did not fit was included anyway")
	}
	// No tolerance: the notice is reserved INSIDE the budget, so the budget
	// is the budget. A +200 slack here is what let a 2144-byte payload pass
	// against a 2000-byte constant.
	if len(got) > maxContextBytes {
		t.Fatalf("payload is %d bytes, past the %d budget", len(got), maxContextBytes)
	}
	if !strings.Contains(got, "omitted") {
		t.Fatalf("findings were dropped and the output does not say so:\n%s", got)
	}
}

// proved by: truncating a part instead of dropping it hands the agent half
// a finding — and half a formatted file is one somebody may write back over
// the whole file.
func TestFitNeverTruncatesAPart(t *testing.T) {
	a := strings.Repeat("a", 500)
	b := strings.Repeat("b", 500)
	c := strings.Repeat("c", 500)
	d := strings.Repeat("d", 500)
	got := fit([]part{{text: a}, {text: b}, {text: c}, {text: d}})
	for _, p := range []string{a, b, c, d} {
		if strings.Contains(got, p[:50]) && !strings.Contains(got, p) {
			t.Fatal("a part appears in the output but not in full")
		}
	}
	// The positive half. Without it this passes on an empty return, or on a
	// return carrying nothing but the notice — a test that cannot fail.
	for _, p := range []string{a, b, c} {
		if !strings.Contains(got, p) {
			t.Fatal("a part that fits was dropped")
		}
	}
	if strings.Contains(got, d) {
		t.Fatal("a part past the budget was kept")
	}
	if !strings.Contains(got, "omitted") {
		t.Fatal("a part was dropped and the output does not say so")
	}
}

// proved by: ordering the parts by consequence is one fix for the
// starvation; a must-keep set is the other. Whichever is in use, a secret
// must not be the finding the budget discards.
func TestASecretSurvivesTheBudget(t *testing.T) {
	filler := strings.Repeat("f", maxContextBytes-100)
	secret := "== secrets: 3 finding(s) in the file you just wrote"
	got := fit([]part{{text: filler}, {text: secret, keep: true}})

	if !strings.Contains(got, secret) {
		t.Fatalf("the secret finding was dropped to make room for a larger part:\n%s", got)
	}
	if len(got) > maxContextBytes {
		t.Fatalf("keeping it pushed the payload to %d, past the %d budget", len(got), maxContextBytes)
	}
}

// proved by: slicing a byte index through an em-dash returns invalid UTF-8,
// which json.Marshal replaces with U+FFFD — the question reaches the agent
// as mojibake. The ASCII-only case cannot catch it.
func TestOneLineDoesNotSplitARune(t *testing.T) {
	got := oneLine(strings.Repeat("é", 400), 160)
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8: %q", got)
	}
	if n := len([]rune(got)); n != 160 {
		t.Fatalf("truncated to %d runes, want 160", n)
	}
}

// proved by: leaving q.Text as written lets one decision record carry its
// whole body — blank lines and all — into a line the code calls a summary.
// Five of those ran to eight kilobytes in this repository and crowded every
// other finding out of the payload.
func TestOneLineFlattensAndTruncates(t *testing.T) {
	in := "a question\n\nwith a body\nover several lines"
	got := oneLine(in, 160)
	if strings.Contains(got, "\n") {
		t.Fatalf("still multi-line: %q", got)
	}
	if got != "a question with a body over several lines" {
		t.Fatalf("got %q", got)
	}
	if long := oneLine(strings.Repeat("z", 400), 160); len([]rune(long)) != 160 {
		t.Fatalf("not truncated to the limit: %d runes", len([]rune(long)))
	}
}

// proved by: the old 48KB threshold — twenty-four times the host's delivery
// budget — let a formatted body be inlined under an instruction to write it
// back, with only its first 2KB actually arriving.
func TestFormattedBodyHasABoundedShare(t *testing.T) {
	if maxInlineFormatted >= maxContextBytes {
		t.Fatal("the formatted body may consume the whole payload, starving every finding after it")
	}
	msg := message(format.Result{
		Verdict:   format.Unformatted,
		File:      "big.go",
		Tool:      "gofmt",
		Formatted: []byte(strings.Repeat("x", maxInlineFormatted+1)),
	})
	if strings.Contains(msg, strings.Repeat("x", 100)) {
		t.Fatal("a body over the share was inlined instead of pointed at")
	}
	if !strings.Contains(msg, "procoder format") {
		t.Fatalf("no pointer to the full result:\n%s", msg)
	}
}
