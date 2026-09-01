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

// TestRunNeverExceedsTheDeliveryBudget is the end-to-end half, and it exists
// because the unit test of fit does not catch the mutation that matters:
// putting strings.Join back at the call site leaves fit correct, tested and
// unused, with every payload oversized again.
//
// The fixture is tuned so the parts genuinely overflow — a formatted body
// just under its share, so it inlines rather than being pointed at, plus a
// full ask queue. Without that the payload lands under budget on its own and
// the test passes whether Run calls fit or not, which is the shape of a test
// that proves nothing.
//
// proved by: replacing `fit(parts)` in Run with `strings.Join(parts, "\n\n")`
// fails this on the budget assertion.
func TestRunNeverExceedsTheDeliveryBudget(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not installed")
	}
	root := t.TempDir()

	// An unformatted file whose FORMATTED form lands just under the share,
	// so it is inlined and takes up most of the budget.
	var src strings.Builder
	src.WriteString("package probe\n")
	for i := 0; i < 40; i++ {
		src.WriteString("func  Fn( ) {}\n")
	}
	p := filepath.Join(root, "a.go")
	if err := os.WriteFile(p, []byte(src.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// A decisions ledger with more questions than askPart shows, each with a
	// long body — the shape that put eight kilobytes of ask queue into every
	// write in this repository.
	if err := os.MkdirAll(filepath.Join(root, ".procoder", "ask"), 0o755); err != nil {
		t.Fatal(err)
	}
	var dec strings.Builder
	for i := 0; i < 8; i++ {
		dec.WriteString("## Question ")
		dec.WriteString(strings.Repeat("with a very long heading ", 12))
		dec.WriteString("?\n\n")
		dec.WriteString(strings.Repeat("Context that runs on and on. ", 40))
		dec.WriteString("\n\n- one option\n- another option\n\n")
	}
	if err := os.WriteFile(filepath.Join(root, ".procoder", "ask", "decisions.md"),
		[]byte(dec.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if code := Run(payloadFor(t, p), &out); code != 0 {
		t.Fatalf("hook exit %d — a hook must never fail the session", code)
	}
	if out.Len() == 0 {
		t.Fatal("the fixture produced no payload, so this test asserts nothing")
	}
	var resp hookOutput
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("hook output is not the host's JSON shape: %v\n%s", err, out.String())
	}
	ctx := resp.HookSpecificOutput.AdditionalContext

	if len(ctx) > maxContextBytes+200 {
		t.Fatalf("payload is %d bytes, past the %d the host actually delivers — "+
			"everything after the first 2KB reaches the agent as a file path nothing makes it read",
			len(ctx), maxContextBytes)
	}
	// The fixture is built to overflow. If nothing was dropped it stopped
	// overflowing, and this test went back to proving nothing — fail loudly
	// rather than pass quietly.
	if !strings.Contains(ctx, "NOT shown") {
		t.Fatalf("the fixture no longer overflows the budget, so this test no longer "+
			"exercises fit — re-tune it:\n%s", ctx)
	}
}
