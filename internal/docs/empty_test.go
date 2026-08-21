package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The clobber that made this necessary, reproduced: `procoder format` on an
// ALREADY FORMATTED file prints one header line and nothing after it, so
// `procoder format X | tail -n +2 > X` empties the file on the SUCCESS
// path, exit 0, nothing on stderr. docs/commands.md was destroyed exactly
// that way — 551 lines — and shipped in a release, because review, the
// gate, `mkdocs build --strict` and the documentation obligation all had
// nothing to say about a page that had stopped saying anything.
//
// The obligation was the cruellest part: it asks whether a documentation
// file CHANGED in the diff, and emptying one is a change. Destroying the
// page satisfied the check meant to protect it.
// proved by: dropped EmptyDocs from CollectOfflineFor — an emptied doc
// passes the gate again, which is how 551 lines reached a release.
func TestAnEmptiedDocumentationFileBlocks(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "commands.md")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := EmptyDocs(root, []string{empty})
	if len(got) != 1 {
		t.Fatalf("an emptied documentation file must be reported: %+v", got)
	}
	if !got[0].Blocking {
		t.Error("an emptied page must block — it passed a release without doing so")
	}
	// A refusal a person cannot act on is a wall: the message names the way
	// back.
	if !strings.Contains(got[0].Message, "git checkout") {
		t.Errorf("the refusal must say how to restore it: %q", got[0].Message)
	}

	// Whitespace only is the shape the clobber actually leaves behind when
	// the pipeline writes a trailing newline.
	ws := filepath.Join(root, "guide.md")
	if err := os.WriteFile(ws, []byte("\n\n  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(EmptyDocs(root, []string{ws})) != 1 {
		t.Error("a page holding only whitespace is a page that says nothing")
	}
}

// And it must not fire on the ordinary tree, or it becomes noise that gets
// switched off — the failure mode every blocking rule has to avoid.
// proved by: made EmptyDocs report any file it could not read — every
// non-Markdown path in a changed set then blocks the gate.
func TestARealDocumentationFileIsNotReportedEmpty(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "README.md")
	if err := os.WriteFile(real, []byte("# Title\n\nSomething to read.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code := filepath.Join(root, "main.go")
	if err := os.WriteFile(code, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// An empty .go file is not this check's business — it is a Markdown
	// rule, and widening it would be a different decision than the one
	// this fixes.
	if got := EmptyDocs(root, []string{real, code, filepath.Join(root, "gone.md")}); len(got) != 0 {
		t.Errorf("only emptied Markdown counts: %+v", got)
	}
}
