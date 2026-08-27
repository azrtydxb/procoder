package surgical

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func planAt(t *testing.T, body string) (root, rel string) {
	t.Helper()
	root = t.TempDir()
	rel = ".procoder/plans/p.md"
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, rel
}

func collect() (func(string), *[]string) {
	var l []string
	return func(s string) { l = append(l, s) }, &l
}

// Both spellings, because both exist in this repository: one plan writes
// backticked names, three write plain comma-separated ones. The first
// version accepted only backticks and found nothing in three of four while
// the line itself matched.
//
// proved by: the plain-path branch removed from paths() — the unticked
// declaration yields nothing.
func TestBothDeclarationSpellingsAreRead(t *testing.T) {
	for name, body := range map[string]string{
		"plain":      "Files: internal/a/x.go, internal/a/x_test.go\n",
		"backticked": "**Files:** `internal/a/x.go`, `internal/a/x_test.go`\n",
	} {
		root, rel := planAt(t, body)
		got, declaring := Declared(root, []string{rel})
		if declaring != 1 {
			t.Errorf("%s: no plan counted as declaring", name)
		}
		if !got["internal/a/x.go"] || !got["internal/a/x_test.go"] {
			t.Errorf("%s: parsed %v", name, got)
		}
	}
}

// An absolute path is as valid as a relative one — plan.Files hands back
// absolute paths, and joining root onto those produced a path that does
// not exist, so every plan was skipped in silence.
//
// proved by: the IsAbs branch removed from Declared — the absolute path is
// joined onto root and nothing is read.
func TestAnAbsolutePlanPathIsRead(t *testing.T) {
	root, rel := planAt(t, "Files: internal/a/x.go\n")
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if _, declaring := Declared(root, []string{abs}); declaring != 1 {
		t.Fatal("an absolute plan path was skipped — every plan would be, in silence")
	}
}

// A file no plan names is the finding. That is the drive-by edit: the
// change does what was asked and also touches something nobody reviewed as
// part of it.
//
// proved by: the `declared[rel]` test inverted — the declared files are
// reported and the stray one is not.
func TestAFileNoPlanNamesIsReported(t *testing.T) {
	root, rel := planAt(t, "Files: internal/a/x.go\n")
	out, lines := collect()
	Check(root, []string{"internal/a/x.go", "internal/b/drive_by.go"}, []string{rel}, out)
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "drive_by.go") {
		t.Errorf("the undeclared file was not reported:\n%s", joined)
	}
	if strings.Contains(joined, "internal/a/x.go —") {
		t.Errorf("a declared file was reported as stray:\n%s", joined)
	}
}

// A plan naming a directory covers what is under it. Demanding every file
// by name would make the declaration a chore nobody keeps current, and a
// stale declaration reports noise.
//
// proved by: underDeclaredDir made to return false — the file beneath a
// declared directory is reported as stray.
func TestADeclaredDirectoryCoversWhatIsUnderIt(t *testing.T) {
	root, rel := planAt(t, "Files: internal/gate/\n")
	out, lines := collect()
	Check(root, []string{"internal/gate/adoption.go"}, []string{rel}, out)
	if strings.Contains(strings.Join(*lines, "\n"), "adoption.go —") {
		t.Fatalf("a file under a declared directory was reported as stray:\n%s", strings.Join(*lines, "\n"))
	}
}

// No declared scope is not a pass. There is genuinely nothing to compare
// against, and saying so is different from saying the change was surgical.
//
// proved by: the `declaring == 0` branch changed to report success — a
// repository with no plans reports every change as in scope.
func TestNoDeclaredScopeSaysSo(t *testing.T) {
	root, rel := planAt(t, "## A plan\n\nNo file declarations at all.\n")
	out, lines := collect()
	Check(root, []string{"anything.go"}, []string{rel}, out)
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "NOT checked") {
		t.Fatalf("a plan declaring nothing reported a verdict:\n%s", joined)
	}
}
