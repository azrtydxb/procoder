package store

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// proved by: writing without taking the lock lets this succeed, which is the
// race the whole package exists to remove.
func TestSaveDispatchLocksAndWritesAtomically(t *testing.T) {
	impatient(t)
	root := t.TempDir()
	plant(t, root, DispatchPath, os.Getpid(), time.Now().Unix())

	err := SaveDispatch(root, []byte("{}\n"))
	if err == nil {
		t.Fatal("SaveDispatch wrote while the file was locked")
	}
	if !strings.Contains(err.Error(), DispatchPath) {
		t.Fatalf("error does not name the file: %v", err)
	}
	if _, rerr := ReadFile(root, DispatchPath); rerr == nil {
		t.Fatal("a refused save left a file behind")
	}
}

// proved by: removing the Lock from AppendLearn loses appends under
// concurrency — the exact defect this task exists to fix, and it fails today
// without the lock.
func TestConcurrentAppendsBothSurvive(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := AppendLearn(root, []byte(fmt.Sprintf("{\"n\":%d}\n", i))); err != nil {
				t.Errorf("append %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	got, err := LoadLearn(root)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(got, []byte("\n")); n != 20 {
		t.Fatalf("%d lines survived, want 20 — an append was lost", n)
	}
}

// proved by: reading through Lock would make a reader wait on a writer, which
// the spec rules out — the atomic rename is what makes reads safe.
func TestLoadsDoNotLock(t *testing.T) {
	root := t.TempDir()
	if err := SaveClaims(root, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	rel, err := Lock(root, ClaimsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer rel()

	done := make(chan struct{})
	go func() { defer close(done); _, _ = LoadClaims(root) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("LoadClaims blocked on a held write lock")
	}
}

// proved by: an absent file reported as an error rather than as absence turns
// every first run into a fault.
func TestAbsentStateReadsAsAbsent(t *testing.T) {
	root := t.TempDir()
	for name, load := range map[string]func(string) ([]byte, error){
		"dispatch": LoadDispatch,
		"claims":   LoadClaims,
		"env":      LoadEnvState,
		"learn":    LoadLearn,
		"handoff":  LoadHandoff,
	} {
		if _, err := load(root); !os.IsNotExist(err) {
			t.Errorf("%s: got %v, want an os.IsNotExist error", name, err)
		}
	}
	if _, err := LoadMarker(root, "last-unasked-decision"); !os.IsNotExist(err) {
		t.Errorf("marker: got %v, want an os.IsNotExist error", err)
	}
}

// proved by: joining name as a path lets a caller escape .procoder/state/,
// and a marker name is not a path.
func TestMarkerNameIsNotAPath(t *testing.T) {
	root := t.TempDir()
	if err := SaveMarker(root, "../../escape", []byte("x")); err == nil {
		t.Fatal("SaveMarker accepted a name containing a path")
	}
	if _, err := LoadMarker(root, "sub/dir"); err == nil {
		t.Fatal("LoadMarker accepted a name containing a separator")
	}
}
