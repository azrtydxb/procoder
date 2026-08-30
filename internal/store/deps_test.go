package store

import (
	"os"
	"strings"
	"testing"
)

// proved by: adding any module dependency fails this.
//
// The lockfile in this package is an O_EXCL file rather than flock because
// a portable flock/LockFileEx pair costs golang.org/x/sys. That reasoning
// is only worth anything while the module really has no dependencies, so
// the claim gets a test rather than a comment.
func TestNoModuleDependencies(t *testing.T) {
	raw, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "require") {
		t.Fatal("go.mod has grown a require block — procoder's zero-dependency claim, and the reason the lock is a lockfile, no longer hold")
	}
}
