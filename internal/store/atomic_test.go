package store

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// proved by: replacing the temp-and-rename with a direct os.WriteFile makes
// the write destroy what was at the target instead of leaving it, and this
// fails.
//
// The rename is made to fail by pointing it at a NON-EMPTY DIRECTORY, which
// no platform will let a file be renamed over. That is why this test needs
// no seam in the production code: an atomicity claim nothing can check is
// not worth making, but a claim that needs a flag in the shipping binary to
// check is worse.
func TestAtomicWriteLeavesOriginalOnRenameFailure(t *testing.T) {
	root := t.TempDir()
	const rel = ".procoder/state/claims.json"

	target := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(child, []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(root, rel, []byte("replacement"), 0o644); err == nil {
		t.Fatal("WriteFile reported success though the rename could not have worked")
	}
	got, err := os.ReadFile(child)
	if err != nil || string(got) != "untouched" {
		t.Fatalf("a failed write damaged the target: %q %v", got, err)
	}
	// and it left no litter behind
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), tempPrefix) {
			t.Fatalf("a failed write left %s behind", e.Name())
		}
	}
}

// proved by: a direct os.WriteFile truncates first, so a reader catches a
// zero-length or half-written file and this fails.
func TestReaderNeverSeesPartialFile(t *testing.T) {
	root := t.TempDir()
	rel := ".procoder/state/claims.json"
	oldData, newData := strings.Repeat("a", 1<<16), strings.Repeat("b", 1<<16)
	if err := WriteFile(root, rel, []byte(oldData), 0o644); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				_ = WriteFile(root, rel, []byte(newData), 0o644)
				_ = WriteFile(root, rel, []byte(oldData), 0o644)
			}
		}
	}()
	defer func() { close(stop); <-done }()

	for i := 0; i < 2000; i++ {
		got, err := ReadFile(root, rel)
		if err != nil {
			continue // the file is momentarily absent between rename attempts
		}
		if s := string(got); s != oldData && s != newData {
			t.Fatalf("partial read of %d bytes", len(s))
		}
	}
}

// proved by: dropping the sweep leaves the stale temp file in place and this
// fails. Without it a crashed write litters .procoder/ forever.
func TestTempFilesAreSwept(t *testing.T) {
	root := t.TempDir()
	const rel = ".procoder/state/claims.json"

	// The litter is planted BEFORE the first write into this directory,
	// because the sweep runs once per directory per process — a second
	// write does not pay for a second ReadDir.
	dir := filepath.Join(root, ".procoder", "state")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	litter := filepath.Join(dir, tempPrefix+"stale")
	if err := os.WriteFile(litter, []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(litter, old, old); err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(root, rel, []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(litter); err == nil {
		t.Fatal("stale temp file survived a successful write")
	}
}

// proved by: sweeping on every write puts the size of the directory on the
// cost of writing one file in it.
func TestSweepRunsOncePerDirectory(t *testing.T) {
	root := t.TempDir()
	const rel = ".procoder/specs/a.md"
	if err := WriteFile(root, rel, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".procoder", "specs")
	litter := filepath.Join(dir, tempPrefix+"later")
	if err := os.WriteFile(litter, []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(litter, old, old); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(root, ".procoder/specs/b.md", []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(litter); err != nil {
		t.Fatal("the second write into the same directory swept again")
	}
}

// proved by: falling back to a direct write when the temp file cannot be made
// would let this succeed, which is the silent-green this package exists to
// remove.
func TestReadOnlyStateRefusesWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Chmod on Windows toggles a file's read-only attribute and does not restrict a DIRECTORY, so there is no read-only directory to test against here")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory mode, so there is nothing to test")
	}
	root := t.TempDir()
	rel := ".procoder/state/claims.json"
	if err := WriteFile(root, rel, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(root, ".procoder", "state")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := WriteFile(root, rel, []byte("replacement"), 0o644)
	if err == nil {
		t.Fatal("write succeeded into a read-only directory")
	}
	if !strings.Contains(err.Error(), rel) {
		t.Fatalf("error does not name the path: %v", err)
	}
	got, _ := ReadFile(root, rel)
	if string(got) != "original" {
		t.Fatalf("file changed under a read-only directory: %q", got)
	}
}
