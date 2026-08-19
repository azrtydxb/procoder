package audit

import (
	"fmt"
	"testing"

	"procoder/internal/gitx"
)

// A BLOCK finding sitting past the section cap must still be shown:
// orderForDisplay puts blocking findings first, so truncation only ever
// drops info lines.
func TestOrderForDisplayBlockingSurvivesCap(t *testing.T) {
	var findings []gitx.Finding
	for i := range maxPerSection + 5 {
		findings = append(findings, gitx.Finding{Message: fmt.Sprintf("info %d", i)})
	}
	findings = append(findings, gitx.Finding{Blocking: true, Message: "block A"})
	findings = append(findings, gitx.Finding{Blocking: true, Message: "block B"})

	ordered := orderForDisplay(findings)
	if len(ordered) != len(findings) {
		t.Fatalf("ordering changed length: got %d, want %d", len(ordered), len(findings))
	}
	if ordered[0].Message != "block A" || ordered[1].Message != "block B" {
		t.Fatalf("blocking findings not first (stable): got %q, %q", ordered[0].Message, ordered[1].Message)
	}
	for i, f := range ordered[2:] {
		if want := fmt.Sprintf("info %d", i); f.Message != want {
			t.Fatalf("info order not stable at %d: got %q, want %q", i, f.Message, want)
		}
	}
}
