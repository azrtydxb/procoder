package store

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// proved by: locking the directory rather than the file makes the second
// save fail too, and two people editing two stories would wait on each
// other for no reason.
func TestSaveInLocksItsOwnFile(t *testing.T) {
	impatient(t)
	root := t.TempDir()
	const dir = ".procoder/backlog/stories"
	plant(t, root, dir+"/s1.md", os.Getpid(), time.Now().Unix())

	if err := SaveIn(root, dir, "s1.md", []byte("x")); err == nil {
		t.Fatal("SaveIn wrote while that file was locked")
	}
	if err := SaveIn(root, dir, "s2.md", []byte("y")); err != nil {
		t.Fatalf("a lock on s1.md blocked s2.md: %v", err)
	}
}

// proved by: taking the caller's argument order rather than sorted order
// deadlocks two operations spanning the same two files.
func TestMultiFileSaveTakesSortedLocks(t *testing.T) {
	root := t.TempDir()
	a := ".procoder/backlog/stories/s1.md"
	b := ".procoder/backlog/sprints/001.md"
	done := make(chan error, 2)
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		go func(p [2]string) {
			for i := 0; i < 50; i++ {
				release, err := Lock(root, p[0], p[1])
				if err != nil {
					done <- err
					return
				}
				release()
			}
			done <- nil
		}(pair)
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("lock failed: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("deadlock: locks were not taken in sorted order")
		}
	}
}

// proved by: returning the IsNotExist error makes every caller unwrap it,
// and "no specs yet" is an ordinary state, not a fault.
func TestListDirAbsentIsEmpty(t *testing.T) {
	names, err := ListDir(t.TempDir(), ".procoder/specs")
	if err != nil {
		t.Fatalf("absent directory reported as an error: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("got %v, want none", names)
	}
}

// proved by: returning entries in readdir order makes every listing depend
// on the filesystem, and two machines disagree about what `procoder spec
// list` prints.
func TestListDirIsSortedAndFilesOnly(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"c.md", "a.md", "b.md"} {
		if err := SaveIn(root, ".procoder/specs", n, []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(root+"/.procoder/specs/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	names, err := ListDir(root, ".procoder/specs")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "a.md,b.md,c.md" {
		t.Fatalf("got %v, want a.md,b.md,c.md with the directory excluded", names)
	}
}

// proved by: joining an unchecked name lets ".." walk out of the directory
// the caller named.
func TestInDirNameIsNotAPath(t *testing.T) {
	root := t.TempDir()
	if err := SaveIn(root, ".procoder/specs", "../escape.md", []byte("x")); err == nil {
		t.Fatal("SaveIn accepted a name containing a path")
	}
	if _, err := LoadIn(root, ".procoder/specs", "sub/thing.md"); err == nil {
		t.Fatal("LoadIn accepted a name containing a separator")
	}
}

// proved by: a doc write that skipped the lock would lose one of two
// concurrent edits to the same rules file.
func TestSaveDocLocks(t *testing.T) {
	impatient(t)
	root := t.TempDir()
	const doc = ".procoder/security/RULES.md"
	plant(t, root, doc, os.Getpid(), time.Now().Unix())
	if err := SaveDoc(root, doc, []byte("x")); err == nil {
		t.Fatal("SaveDoc wrote while the file was locked")
	}
}

// proved by: returning the path unchecked lets "read what I was handed"
// become "read anything on this machine".
func TestRelRefusesOutsideRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := Rel(root, "/etc/passwd"); err == nil {
		t.Fatal("Rel accepted a path outside root")
	}
	got, err := Rel(root, root+"/.procoder/specs/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != ".procoder/specs/a.md" {
		t.Fatalf("Rel = %q, want .procoder/specs/a.md", got)
	}
}

// proved by: comparing unresolved paths refuses a legitimate file — on
// macOS a temp root is /var/... while anything that has called
// EvalSymlinks holds /private/var/..., and every readUnder caller then
// reports a perfectly readable file as unreadable.
func TestRelResolvesSymlinksOnBothSides(t *testing.T) {
	root := t.TempDir()
	if err := SaveIn(root, ".procoder/specs", "a.md", []byte("x")); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == root {
		t.Skip("this temp root is not behind a symlink, so there is nothing to test here")
	}
	if _, err := Rel(root, filepath.Join(resolved, ".procoder", "specs", "a.md")); err != nil {
		t.Fatalf("a resolved path under an unresolved root was refused: %v", err)
	}
	if _, err := Rel(resolved, filepath.Join(root, ".procoder", "specs", "a.md")); err != nil {
		t.Fatalf("an unresolved path under a resolved root was refused: %v", err)
	}
}

// proved by: not resolving symlinks lets a link inside the root read and
// WRITE anywhere on the machine while producing a clean relative path.
func TestRelRefusesEscapeThroughASymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "esc")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("this platform will not make a symlink: %v", err)
	}
	if _, err := Rel(root, filepath.Join(link, "secret.txt")); err == nil {
		t.Fatal("Rel accepted a path that leaves the root through a symlink")
	}
}

// proved by: returning "." lets LoadDoc read, and SaveDoc write over, the
// repository root itself.
func TestRelRefusesTheRootItself(t *testing.T) {
	root := t.TempDir()
	if _, err := Rel(root, root); err == nil {
		t.Fatal("Rel accepted the root directory as a file")
	}
}

// proved by: without the fixed-point guard, a path whose root cannot be
// resolved recurses until the stack runs out.
func TestResolveTerminatesAtTheRoot(t *testing.T) {
	root := filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
	got, err := resolve(root)
	if err != nil {
		t.Fatalf("resolving the filesystem root errored: %v", err)
	}
	if got == "" {
		t.Fatal("resolving the filesystem root returned nothing")
	}
}

// TestReadOnlyStateBlocksContentWrites pins a real behaviour change, so it
// is a decision rather than a surprise.
//
// The lock for every file lives under .procoder/state/locks, so a
// read-only .procoder/state stops writes to content that has nothing to do
// with state: `procoder ask` used to write .procoder/ask/QA.md in that
// situation and now refuses.
//
// The condition is narrow, and worth being precise about rather than
// alarming: it bites only while .procoder/state/locks does not yet exist.
// Once it has been created — which the first successful write in a
// repository does — a read-only state directory blocks nothing, because
// no new entry has to be made inside it.
//
// Accepted deliberately. Locks beside their files show up in git status
// and in review; locks in the OS temp directory stop being mutually
// exclusive the moment two processes have different TMPDIR values, which
// on macOS is the ordinary case across sessions — and a lock that silently
// stops locking is the one failure this package cannot have.
//
// proved by: falling back to an unlocked write here makes this pass while
// reintroducing the race the package exists to remove.
func TestReadOnlyStateBlocksContentWrites(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Chmod on Windows toggles a file's read-only attribute and does not restrict a DIRECTORY, so there is no read-only directory to test against here")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory mode, so there is nothing to test here")
	}
	root := t.TempDir()
	// Seed the content file WITHOUT the store, so the lock directory has
	// never been created. That is the whole condition: once
	// .procoder/state/locks exists, a read-only state directory no longer
	// blocks anything, because nothing new has to be created inside it.
	ask := filepath.Join(root, ".procoder", "ask")
	if err := os.MkdirAll(ask, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ask, "QA.md"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, ".procoder", "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0o755) })

	err := SaveDoc(root, ".procoder/ask/QA.md", []byte("after"))
	if err == nil {
		t.Fatal("wrote a content file unlocked while the lock directory was unwritable")
	}
	if !strings.Contains(err.Error(), "lock directory") {
		t.Fatalf("the error does not name the lock directory, so the coupling is not diagnosable: %v", err)
	}
	got, _ := ReadFile(root, ".procoder/ask/QA.md")
	if string(got) != "before" {
		t.Fatalf("the refused write changed the file anyway: %q", got)
	}
}
