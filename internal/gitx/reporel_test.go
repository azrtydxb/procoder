package gitx

import (
	"path/filepath"
	"testing"
)

// The same file must get the same verdict however it was named. A gate
// leg that calls filepath.Rel directly drops every relative path, because
// Rel refuses an absolute base against a relative target — so
// `procoder check somefile.py` skipped its scan entirely while `procoder
// check` on the same file, whose paths come from git as absolute, scanned
// it normally.
// proved by: called filepath.Rel without joining first — the relative
// form returns not-ok and the file falls out of the commit's set.
func TestAPathIsTheSameFileHoweverItArrives(t *testing.T) {
	root := filepath.FromSlash("/repo")
	abs, ok := RepoRel(root, filepath.Join(root, "cmd", "main.go"))
	if !ok || abs != "cmd/main.go" {
		t.Errorf("absolute form: got %q ok=%v", abs, ok)
	}
	rel, ok := RepoRel(root, filepath.FromSlash("cmd/main.go"))
	if !ok || rel != "cmd/main.go" {
		t.Errorf("relative form: got %q ok=%v", rel, ok)
	}
	if abs != rel {
		t.Errorf("the same file must normalise the same way: %q vs %q", abs, rel)
	}
}

// A path that genuinely leaves the repository is refused; a directory
// named like an escape is not. A prefix test for ".." reads "..foo" as
// outside and quietly drops a real file from the commit's set.
// proved by: replaced the boundary test with strings.HasPrefix(rel, "..")
// — "..foo/a.go" is refused and its findings stop reaching the commit.
func TestOnlyRealEscapesAreRefused(t *testing.T) {
	root := filepath.FromSlash("/repo")
	for path, want := range map[string]bool{
		filepath.Join(root, "a.go"):           true,
		filepath.Join(root, "..foo", "a.go"):  true,
		filepath.FromSlash("..foo/a.go"):      true,
		filepath.FromSlash("/elsewhere/a.go"): false,
	} {
		if _, ok := RepoRel(root, path); ok != want {
			t.Errorf("RepoRel(%q) ok=%v, want %v", path, ok, want)
		}
	}
}
