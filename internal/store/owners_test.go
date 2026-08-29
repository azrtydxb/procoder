package store_test

import (
	"testing"

	"procoder/internal/hook"
	"procoder/internal/learn"
	"procoder/internal/store"
)

// proved by: letting a package declare its own copy of a path the store
// also declares lets the two drift, and the drift is silent until one
// writes where the other no longer reads.
//
// Most owners now NAME the store's constant, which makes drift impossible
// rather than merely detectable. These two compose theirs from parts, so
// they get a test instead.
func TestComposedStatePathsMatchTheStore(t *testing.T) {
	if got := learn.Dir + "/" + learn.File; got != store.LearnPath {
		t.Errorf("learn writes %q, the store reads %q", got, store.LearnPath)
	}
	if got := hook.StateDir + "/" + hook.HandoffFile; got != store.HandoffPath {
		t.Errorf("the stop hook writes %q, the store reads %q", got, store.HandoffPath)
	}
}
