package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// proved by: replacing the temp-and-rename with a direct os.WriteFile leaves
// the target truncated instead of untouched, and this fails.
func TestAtomicWriteLeavesOriginalOnRenameFailure(t *testing.T) {
	root := t.TempDir()
	rel := ".procoder/state/claims.json"
	if err := WriteFile(root, rel, []byte("original"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	forceRenameFailure = true
	t.Cleanup(func() { forceRenameFailure = false })

	if err := WriteFile(root, rel, []byte("replacement"), 0o644); err == nil {
		t.Fatal("WriteFile reported success though the rename failed")
	}
	got, err := ReadFile(root, rel)
	if err != nil {
		t.Fatalf("target unreadable after a failed write: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("target was modified: %q", got)
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
	rel := ".procoder/state/claims.json"
	if err := WriteFile(root, rel, []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}

	stale := filepath.Join(root, ".procoder", "state", tempPrefix+"stale")
	if err := os.WriteFile(stale, []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-31 * time.Second)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(root, rel, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale temp file survived a successful write")
	}
}

// proved by: falling back to a direct write when the temp file cannot be made
// would let this succeed, which is the silent-green this package exists to
// remove.
func TestReadOnlyStateRefusesWrite(t *testing.T) {
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
