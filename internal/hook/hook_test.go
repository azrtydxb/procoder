package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func payloadFor(t *testing.T, file string) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": file},
	})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewBuffer(b)
}

func TestUnformattedWriteHandsBackTheFormattedCode(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not installed")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "a.go")
	if err := os.WriteFile(p, []byte("package main\nfunc  main( ){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := Run(payloadFor(t, p), &out); code != 0 {
		t.Fatalf("hook exit %d — a hook must never fail the session", code)
	}
	var resp hookOutput
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("hook output is not the host's JSON shape: %v\n%s", err, out.String())
	}
	ctx := resp.HookSpecificOutput.AdditionalContext
	if !strings.Contains(ctx, "func main() {}") {
		t.Fatalf("context does not contain the formatted code:\n%s", ctx)
	}
	if !strings.Contains(ctx, "NOT modified") {
		t.Fatalf("context must tell the agent the file was not touched:\n%s", ctx)
	}
}

func TestCleanWriteStaysSilent(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not installed")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "a.go")
	if err := os.WriteFile(p, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	Run(payloadFor(t, p), &out)
	if out.Len() != 0 {
		t.Fatalf("clean file produced output:\n%s", out.String())
	}
}

func TestGarbagePayloadIsSilentlySurvived(t *testing.T) {
	var out bytes.Buffer
	if code := Run(strings.NewReader("not json"), &out); code != 0 || out.Len() != 0 {
		t.Fatalf("garbage payload: exit %d output %q — must be 0 and silent", code, out.String())
	}
}

// The bug this design paid for once: a pipe held open with no data must not
// hold the hook (and with it the session) open. A reader that never delivers
// and never closes must be abandoned at the deadline.
type neverReader struct{}

func (neverReader) Read([]byte) (int, error) { select {} }

func TestOpenPipeWithNoDataCannotHangTheHook(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the stdin deadline")
	}
	done := make(chan int, 1)
	go func() {
		var out bytes.Buffer
		done <- Run(neverReader{}, &out)
	}()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d, want 0", code)
		}
	case <-time.After(stdinDeadline + 3*time.Second):
		t.Fatal("the hook is hanging on an open, silent pipe — the exact bug this exists to prevent")
	}
}

// keep fmt import honest in both build modes
var _ = fmt.Sprint
var _ io.Reader = neverReader{}

// C-06: the write hook carries pending questions into the place the coder
// reads, with the instruction that matters — and carries nothing when there
// is nothing to ask.
// proved by: dropped the "do NOT guess" wording — the coder is then handed a
// list of open questions with no instruction, which is how they got answered
// by invention in the first place.
func TestTheHookCarriesPendingQuestionsAndTheInstruction(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := "# widget\n\nStatus: draft\n\n## Open questions\n\n- which database?\n"
	if err := os.WriteFile(filepath.Join(root, ".procoder", "specs", "widget.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}

	got := askPart(root)
	for _, want := range []string{"== q&a", "which database?", "Do NOT guess", "procoder ask --file"} {
		if !strings.Contains(got, want) {
			t.Errorf("the section must carry %q:\n%s", want, got)
		}
	}

	// Nothing pending: nothing said. A hook that speaks when it has nothing
	// to say trains the reader to skip it.
	if err := os.WriteFile(filepath.Join(root, ".procoder", "specs", "widget.md"),
		[]byte("# widget\n\nStatus: draft\n\n## Open questions\n\n<!-- none -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if quiet := askPart(root); quiet != "" {
		t.Errorf("no questions, no section: %q", quiet)
	}
}
