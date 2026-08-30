package gitx

import (
	"os/exec"
	"testing"
)

// proved by: parsing the remote name from the wrong dot in the config key —
// remote.origin.url has three parts — returns "origin.url" and this fails.
func TestRemotes(t *testing.T) {
	dir := repo(t)
	for _, args := range [][]string{
		{"remote", "add", "origin", "git@example.com:o/r.git"},
		{"remote", "add", "up.stream", "https://example.com/o/r2.git"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	got := Remotes(dir)
	if got["origin"] != "git@example.com:o/r.git" {
		t.Errorf("origin = %q, want git@example.com:o/r.git", got["origin"])
	}
	// A remote whose own name contains a dot is why the name is taken from
	// between the FIRST and LAST dot, not by splitting on dots.
	if got["up.stream"] != "https://example.com/o/r2.git" {
		t.Errorf("up.stream = %q, want https://example.com/o/r2.git", got["up.stream"])
	}
}

// proved by: returning an error instead of an empty map makes the identity
// ladder treat "no remotes" as a fault rather than as the answer it is.
func TestRemotesWithoutRepository(t *testing.T) {
	if got := Remotes(t.TempDir()); len(got) != 0 {
		t.Fatalf("Remotes on a non-repository returned %v", got)
	}
}
