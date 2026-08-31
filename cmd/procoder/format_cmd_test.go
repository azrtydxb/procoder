package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The contract these tests hold: stdout of `procoder format` is the bytes that
// belong in the file, whatever the verdict.
//
// This is a regression guard with a body count. The documented way to capture
// a formatted file was to redirect it, and there were two ways to lose the file
// that way. Redirecting onto the file itself is not recoverable by this command
// — the shell truncates the target before the process exists — so it is refused
// outright, which is what collidingOutput is for. The other way belonged to
// this command and is what these tests hold: for as long as the verdicts with no
// formatted text printed one header line and nothing after it, a caller that
// captured the output to a temporary path and wrote it back got a banner where a
// file used to be. That emptied a 551-line documentation page in this repository
// (#120) and three more files during the pi integration. The fix shipped then
// was a gate finding for emptied Markdown, which catches one extension after the
// fact; these hold the command itself, for every file type, on the success path.
//
// proved by: printing the banner to `out` instead of `notes`, or writing
// res.Formatted unconditionally so a Clean file prints nothing.

func writeFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A file the formatter already agrees with must come back byte for byte. This
// is the exact call that destroyed docs/commands.md.
func TestFormatFilesPrintsTheFileBackWhenItIsAlreadyFormatted(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n\nfunc main() {}\n"
	clean := writeFixture(t, dir, "clean.go", src)

	var out, notes bytes.Buffer
	if code := formatFiles([]string{clean}, &out, &notes); code != 0 {
		t.Fatalf("an already-formatted file exited %d:\n%s", code, notes.String())
	}
	if out.String() != src {
		t.Fatalf("the file did not come back intact, got %q", out.String())
	}
	if strings.Contains(out.String(), "already formatted") {
		t.Errorf("the verdict line is on stdout, which is where the bytes go:\n%s", out.String())
	}
	if !strings.Contains(notes.String(), "already formatted") {
		t.Errorf("the verdict never reached the reader: %q", notes.String())
	}
}

// The one verdict that has new bytes must still print exactly them, with no
// header in front: that is the shape `> file` was always safe with.
func TestFormatFilesPrintsTheToolOutputWhenTheFormatterDisagrees(t *testing.T) {
	dir := t.TempDir()
	messy := "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"x\")\n}\n"
	messyPath := writeFixture(t, dir, "messy.go", messy)
	want := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"x\")\n}\n"

	var out, notes bytes.Buffer
	if code := formatFiles([]string{messyPath}, &out, &notes); code != 0 {
		t.Fatalf("an unformatted file exited %d:\n%s", code, notes.String())
	}
	if out.String() != want {
		t.Fatalf("stdout is not the formatter's output:\n%q", out.String())
	}
	if !strings.Contains(notes.String(), "review and write it") {
		t.Errorf("the verdict never reached the reader: %q", notes.String())
	}
}

// A file nothing can check must still survive the pipeline. The type below is
// one procoder either does not claim or cannot run a tool for, and both of
// those print the file's own bytes — the difference between them is the exit
// code, asserted from whatever verdict the run actually got rather than from
// which formatters this machine happens to have.
func TestFormatFilesPrintsTheFileBackWhenNothingCheckedIt(t *testing.T) {
	dir := t.TempDir()
	src := "notes written down\nsecond line\n"
	other := writeFixture(t, dir, "notes.unknownext", src)

	var out, notes bytes.Buffer
	code := formatFiles([]string{other}, &out, &notes)
	if out.String() != src {
		t.Fatalf("an unchecked file did not come back intact, got %q", out.String())
	}
	if strings.Contains(notes.String(), "NOT checked") && code == 0 {
		t.Error("a file that was NOT checked exited green; the next command cannot tell")
	}
}

// Headers exist so a caller can tell five files apart, and they are the reason
// a multi-file run is read rather than redirected. One file therefore gets no
// header at all — anything on stdout would be bytes the file must not gain.
func TestFormatFilesLabelsOnlyWhenFilesShareTheStream(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n\nfunc main() {}\n"
	a := writeFixture(t, dir, "a.go", src)
	b := writeFixture(t, dir, "b.go", src)

	var solo bytes.Buffer
	formatFiles([]string{a}, &solo, &bytes.Buffer{})
	if strings.Contains(solo.String(), "== ") {
		t.Errorf("a single file carries a header, which would be written into it:\n%s", solo.String())
	}

	var pair bytes.Buffer
	formatFiles([]string{a, b}, &pair, &bytes.Buffer{})
	text := pair.String()
	for _, name := range []string{"a.go", "b.go"} {
		if !strings.Contains(text, "== "+filepath.Join(dir, name)) {
			t.Errorf("no header names %s when two files share one stream:\n%s", name, text)
		}
	}
	if strings.Count(text, src) != 2 {
		t.Errorf("both bodies must follow their own header:\n%s", text)
	}
	if !strings.Contains(text, "do not redirect") {
		t.Error("the labelled form is the unsafe one and does not say so")
	}
}

// The redirect the shell truncates before this process exists. The file cannot
// be saved by anything printed here — the point of the check is that the
// failure is loud, names the file, and exits non-zero instead of printing a
// verdict into the hole and calling it success.
func TestFormatRefusesWhenStdoutIsTheFileBeingRead(t *testing.T) {
	dir := t.TempDir()
	victim := writeFixture(t, dir, "victim.go", "package main\n\nfunc main() {}\n")
	other := writeFixture(t, dir, "other.go", "package main\n\nfunc main() {}\n")

	out, err := os.Create(victim)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	if got := collidingOutput([]string{other, victim}, out); got != victim {
		t.Errorf("stdout is %s and the check missed it, naming %q", victim, got)
	}
	// A pipe or a terminal is never the file: this is the case the check must
	// not fire on, or every ordinary run would be refused.
	if got := collidingOutput([]string{victim}, nil); got != "" {
		t.Errorf("no stdout to compare and it named %q", got)
	}
	// The order of the arguments must not decide which file gets named.
	if got := collidingOutput([]string{victim, other}, out); got != victim {
		t.Errorf("the first argument won over the real collision: %q", got)
	}
}
