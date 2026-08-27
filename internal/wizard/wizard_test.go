package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func collect() (func(string), *[]string) {
	var got []string
	return func(s string) { got = append(got, s) }, &got
}

// proved by: drop the `Capture:` branch in Parse — the line falls through
// into Body and Capture stays empty, so this fails.
func TestParseReadsStepsAndTheirCaptures(t *testing.T) {
	steps := Parse("# title\n\npreamble\n\n## One\n\ndo a thing\n\n## Two\n\nCapture: TOKEN matching ^gh_\\w+$\n")
	if len(steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(steps))
	}
	if steps[0].Title != "One" || steps[1].Title != "Two" {
		t.Fatalf("titles: %q %q", steps[0].Title, steps[1].Title)
	}
	if steps[1].Capture != "TOKEN" {
		t.Errorf("capture = %q, want TOKEN", steps[1].Capture)
	}
	if steps[1].Shape != `^gh_\w+$` {
		t.Errorf("shape = %q", steps[1].Shape)
	}
	for _, b := range steps[1].Body {
		if strings.Contains(b, "Capture:") {
			t.Error("the Capture line leaked into the body")
		}
	}
}

// proved by: change normaliseEOL to return s unchanged — the CRLF heading
// keeps its \r, the title mismatches, and this fails.
func TestParseHandlesCRLF(t *testing.T) {
	steps := Parse("## One\r\n\r\nbody\r\n")
	if len(steps) != 1 || steps[0].Title != "One" {
		t.Fatalf("CRLF not normalised: %#v", steps)
	}
}

// proved by: make read() return "" with no message when the file is
// missing — Show then reports "no step headings" instead of naming the
// absent file, and this fails.
func TestAMissingWizardIsNamed(t *testing.T) {
	out, got := collect()
	if code := Show(t.TempDir(), "nope", out); code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(strings.Join(*got, "\n"), "no wizard named nope") {
		t.Errorf("did not name the missing wizard: %v", *got)
	}
}

// Windows found this one. os.ReadDir on a path that is a FILE reports
// "cannot find the path specified" there, which satisfies os.IsNotExist —
// so the first version of List answered "no wizards" for a path it could
// not read. Absent and unreadable are different answers on every platform.
//
// proved by: replace the Stat switch in List with a bare os.ReadDir whose
// error path returns the empty-state line — this fails on Windows, and the
// !info.IsDir() arm is what makes it fail on Unix too.
func TestAnUnreadableWizardDirIsNotReportedAsEmpty(t *testing.T) {
	root := t.TempDir()
	// A file where the directory belongs: ReadDir fails on every platform,
	// where chmod 000 does not deny reads on Windows.
	if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, Dir), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, got := collect()
	if code := List(root, out); code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	joined := strings.Join(*got, "\n")
	if !strings.Contains(joined, "NOT listed") {
		t.Errorf("an unreadable dir read as empty: %v", *got)
	}
	if strings.Contains(joined, "no wizards") {
		t.Errorf("an unreadable dir claimed there are none: %v", *got)
	}
}

// proved by: delete the `if v == ""` guard in capture — the empty line is
// accepted as TOKEN, Run reaches its success line, and this fails.
func TestRunRefusesAnEmptyCapture(t *testing.T) {
	root := t.TempDir()
	write(t, root, "w", "## Give it\n\nCapture: TOKEN\n")
	out, got := collect()
	// empty, then stop
	code := Run(root, "w", strings.NewReader("\nstop\n"), out)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(strings.Join(*got, "\n"), "cannot be empty") {
		t.Errorf("empty value was not refused: %v", *got)
	}
}

// proved by: drop the `!re.MatchString(v)` branch — the wrong token is
// accepted and Run exits 0, so this fails.
func TestRunEnforcesTheShape(t *testing.T) {
	root := t.TempDir()
	write(t, root, "w", "## Give it\n\nCapture: TOKEN matching ^gh_[a-z]+$\n")
	out, got := collect()
	code := Run(root, "w", strings.NewReader("wrong\ngh_ok\n"), out)
	if code != 0 {
		t.Fatalf("code = %d, want 0: %v", code, *got)
	}
	joined := strings.Join(*got, "\n")
	if !strings.Contains(joined, "does not match") {
		t.Errorf("the bad value was accepted: %v", *got)
	}
	if !strings.Contains(joined, "TOKEN accepted") {
		t.Errorf("the good value was not accepted: %v", *got)
	}
}

// The rule the package exists under: a captured value never reaches the
// terminal, right or wrong.
//
// proved by: add the value to either message in capture (e.g. `+ " ("+v+")"`)
// — it appears in the output and this fails.
func TestACapturedValueIsNeverEchoed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "w", "## Give it\n\nCapture: TOKEN matching ^gh_[a-z]+$\n")
	out, got := collect()
	Run(root, "w", strings.NewReader("sup3rsecret\ngh_ok\n"), out)
	joined := strings.Join(*got, "\n")
	for _, secret := range []string{"sup3rsecret", "gh_ok"} {
		if strings.Contains(joined, secret) {
			t.Errorf("the value %q was echoed: %v", secret, *got)
		}
	}
}

// proved by: swap regexp.Compile for regexp.MustCompile in capture — the
// bad pattern panics instead of reporting, and this fails.
func TestABadShapeIsAFindingNotAPanic(t *testing.T) {
	root := t.TempDir()
	write(t, root, "w", "## Give it\n\nCapture: TOKEN matching ^gh_[a-z\n")
	out, got := collect()
	if code := Run(root, "w", strings.NewReader("anything\n"), out); code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(strings.Join(*got, "\n"), "NOT checked") {
		t.Errorf("a bad shape did not say NOT checked: %v", *got)
	}
}

// proved by: have Run continue past a "stop" instead of returning — it
// reaches step 2 and the success line, and this fails.
func TestStopEndsTheWalkWhereItStood(t *testing.T) {
	root := t.TempDir()
	write(t, root, "w", "## One\n\na\n\n## Two\n\nb\n")
	out, got := collect()
	if code := Run(root, "w", strings.NewReader("stop\n"), out); code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	joined := strings.Join(*got, "\n")
	if !strings.Contains(joined, "stopped at step 1 of 2") {
		t.Errorf("stop was not honoured: %v", *got)
	}
	if strings.Contains(joined, "every step confirmed") {
		t.Error("a stopped walk reported success")
	}
}

// P-CONTROL: the binary prints, the agent writes.
//
// proved by: make Scaffold write the file it prints — the directory exists
// afterwards and this fails.
func TestScaffoldPrintsAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd) //nolint:errcheck
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	out, got := collect()
	if code := Scaffold("setup", out); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if _, err := os.Stat(filepath.Join(root, Dir)); !os.IsNotExist(err) {
		t.Error("Scaffold created something; it must only print")
	}
	if !strings.Contains(strings.Join(*got, "\n"), "## Generate a token") {
		t.Errorf("scaffold did not include a step: %v", *got)
	}
}

// proved by: drop the empty-name guard in Scaffold — it prints a wizard
// named "" and returns 0, so this fails.
func TestScaffoldRefusesAnEmptyName(t *testing.T) {
	out, _ := collect()
	if code := Scaffold("  ", out); code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
}
