package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQuestionsSurviveTheBudgetAsOneLineEach pins the flattening at its call
// site, which the unit test of oneLine does not.
//
// Dropping oneLine does not make the payload oversized — fit simply discards
// the whole ask part instead, and the budget assertion elsewhere still
// passes. What is lost is the questions: the coder stops being told a human
// decision is pending, which is the one thing askPart exists to do.
//
// So the assertion is that they SURVIVE, and survive as one line each.
//
// proved by: replacing `oneLine(q.Text, 160)` with `q.Text` makes the ask
// part too large to fit, fit drops it, and this fails on the missing q&a
// section.
func TestQuestionsSurviveTheBudgetAsOneLineEach(t *testing.T) {
	// No gofmt skip on purpose. Whatever verdict format.Check reaches
	// without it, the questions still have to survive the budget — and
	// without this, a machine with no gofmt has nothing exercising Run
	// against the budget at all.
	root := t.TempDir()

	// A CLEAN file, so the format part contributes nothing and the ask queue
	// is competing only with itself.
	p := filepath.Join(root, "a.go")
	if err := os.WriteFile(p, []byte("package probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".procoder", "ask"), 0o755); err != nil {
		t.Fatal(err)
	}
	var dec strings.Builder
	for i := 0; i < 6; i++ {
		dec.WriteString("## A question with a body?\n\n")
		dec.WriteString(strings.Repeat("Context that runs on and on. ", 60))
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
		t.Fatal("pending questions produced no payload at all")
	}
	var resp hookOutput
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	ctx := resp.HookSpecificOutput.AdditionalContext

	if !strings.Contains(ctx, "q&a:") {
		t.Fatalf("the questions did not survive the budget — the coder is not told a "+
			"human decision is pending, which is the one thing this part exists for:\n%s", ctx)
	}
	for _, line := range strings.Split(ctx, "\n") {
		if !strings.HasPrefix(line, "  - ") {
			continue
		}
		if len([]rune(line)) > 220 {
			t.Fatalf("a question line is %d runes — it was not flattened to a summary:\n%s",
				len([]rune(line)), line)
		}
	}
}
