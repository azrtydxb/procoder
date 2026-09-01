package hook

import (
	"strings"
	"testing"

	"procoder/internal/format"
)

// proved by: joining every part unconditionally — what this did before —
// produces a payload the host truncates, and what is lost is whatever
// finding happened to be built last.
func TestFitStopsAtTheDeliveryBudget(t *testing.T) {
	big := strings.Repeat("x", maxContextBytes)
	got := fit([]string{"first", big, "third"})

	if !strings.Contains(got, "first") {
		t.Fatal("the first part was dropped")
	}
	if strings.Contains(got, big) {
		t.Fatal("a part that did not fit was included anyway")
	}
	if len(got) > maxContextBytes+200 {
		t.Fatalf("payload is %d bytes, past the %d budget", len(got), maxContextBytes)
	}
	if !strings.Contains(got, "NOT shown") {
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
	got := fit([]string{a, b, c, d})
	for _, part := range []string{a, b, c, d} {
		if strings.Contains(got, part[:50]) && !strings.Contains(got, part) {
			t.Fatal("a part appears in the output but not in full")
		}
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
