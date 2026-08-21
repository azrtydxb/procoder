package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, Dir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A repository's template replaces the embedded one, and a repository
// without the file gets the embedded one unchanged — the "absent means
// default" rule the rest of .procoder/ lives by.
// proved by: made Resolve return the embedded body even when a file is
// present — a team's house format is silently ignored and every story
// comes out in procoder's shape.
func TestTheRepositorysTemplateWinsAndAbsentMeansDefault(t *testing.T) {
	root := t.TempDir()
	if body, source, _ := Resolve(root, "story", "EMBEDDED"); body != "EMBEDDED" || source != "default" {
		t.Errorf("no file means the default, got %q from %q", body, source)
	}
	write(t, root, "story", "HOUSE STYLE")
	body, source, problem := Resolve(root, "story", "EMBEDDED")
	if body != "HOUSE STYLE" {
		t.Errorf("the repository's template must win, got %q", body)
	}
	if !strings.Contains(source, "story.md") {
		t.Errorf("the source must name the file, got %q", source)
	}
	if problem != nil {
		t.Errorf("a template that exists and has content is not a problem: %v", problem)
	}
}

// An empty template BLOCKS rather than falling back quietly. Falling back
// is the dangerous option: `procoder format` prints one header line for an
// already-formatted file, so a pipeline that strips the header and writes
// the rest empties the file on the success path — that destroyed a
// documentation page in this repository. A team would find out their
// customised template was gone when their next story came out in
// procoder's shape instead of theirs.
// proved by: treated whitespace-only as absent — the emptied template is
// silently replaced by the default and nothing is reported.
func TestAnEmptyTemplateBlocksInsteadOfFallingBackQuietly(t *testing.T) {
	root := t.TempDir()
	for _, body := range []string{"", "\n\n   \n"} {
		write(t, root, "story", body)
		got, _, problem := Resolve(root, "story", "EMBEDDED")
		if problem == nil {
			t.Fatalf("an emptied template must be reported (body %q)", body)
		}
		if !problem.Blocking {
			t.Error("it must block — a silently reverted template is found weeks later")
		}
		if !strings.Contains(problem.Message, "git checkout") {
			t.Errorf("the refusal must say how to restore it: %q", problem.Message)
		}
		// Procoder still prints something usable: refusing to emit a
		// template at all would turn one bad file into a broken command.
		if got != "EMBEDDED" {
			t.Errorf("the embedded template is still used, got %q", got)
		}
	}
}
