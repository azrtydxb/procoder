package plugincache_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// S-7: nothing on a hook path reaches the sweep.
//
// Hooks fire on every write, every commit, every session start. A delete
// that removes a gigabyte is a deliberate action a person takes, not
// something that happens to them while they are typing. The issue asked
// for this explicitly and it is the kind of thing a later "just call it
// from self-upgrade's hook" change would undo without anybody noticing.
//
// Asserted by reading the hook sources rather than by running them: a
// behavioural test can only prove the paths it happened to exercise, and
// what is wanted here is that the call is absent from all of them.
//
// proved by: added `plugincache.Compute(...)` to internal/hook/hook.go —
// the test names the file and fails.
func TestNoHookPathReachesTheSweep(t *testing.T) {
	dir := filepath.Join("..", "hook")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the hook package could not be read: %v", err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s could not be read: %v", name, err)
		}
		checked++
		if strings.Contains(string(raw), "plugincache") {
			t.Errorf("internal/hook/%s reaches the plugin-cache sweep — a hook must never delete", name)
		}
	}
	// An empty set would make this pass while proving nothing: a rename of
	// the package, or a test run from an unexpected directory, would
	// otherwise read as "no hook calls it".
	if checked == 0 {
		t.Fatal("no hook source files were read — this test proved nothing")
	}
}
