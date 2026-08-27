package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

// The output is never printed. Evidence gets committed, and a suite's
// output can carry a token, a customer name, or a path that identifies
// somebody's machine — so the fingerprint proves what ran without putting
// the result on disk.
//
// proved by: `out(string(raw))` added to Record — the secret appears and
// the test names it.
func TestRecordNeverPrintsTheOutput(t *testing.T) {
	// The secret must be in the OUTPUT, not the command: the Command line
	// deliberately echoes what was typed, because evidence has to say what
	// ran. The first version of this test put the secret in the argument
	// and failed for that reason — a fixture fault, but one worth keeping
	// the shape of, since it is exactly the mistake a user could make.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "out.txt"), []byte("SUPERSECRET-TOKEN\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, lines := collect()
	if code := Record(root, []string{"cat", "out.txt"}, out); code != 0 {
		t.Fatalf("exit %d: %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if strings.Contains(joined, "SUPERSECRET-TOKEN") {
		t.Fatalf("the command's output was printed:\n%s", joined)
	}
	if !strings.Contains(joined, "Fingerprint: sha256:") {
		t.Errorf("no fingerprint was produced:\n%s", joined)
	}
	if !strings.Contains(joined, "exit 0") {
		t.Errorf("the exit code was not recorded:\n%s", joined)
	}
}

// The same command producing the same output fingerprints the same, and a
// different output does not. Without that the fingerprint proves nothing.
//
// proved by: the sha256 replaced with a constant — the two fingerprints
// stop differing.
func TestTheFingerprintTracksTheOutput(t *testing.T) {
	fp := func(args ...string) string {
		out, lines := collect()
		Record(t.TempDir(), args, out)
		for _, l := range *lines {
			if strings.HasPrefix(l, "Fingerprint:") {
				return l
			}
		}
		t.Fatalf("no fingerprint from %v: %v", args, *lines)
		return ""
	}
	same1, same2 := fp("echo", "same"), fp("echo", "same")
	if same1 != same2 {
		t.Errorf("the same output fingerprinted differently:\n%s\n%s", same1, same2)
	}
	if other := fp("echo", "different"); other == same1 {
		t.Errorf("different output fingerprinted the same: %s", other)
	}
}

// A failing command is still evidence — of failure. Recording only
// successes would make the ledger a highlight reel.
//
// proved by: the ExitError branch changed to return 2 — a failing command
// records nothing and the test says so.
func TestAFailingCommandIsStillRecorded(t *testing.T) {
	out, lines := collect()
	code := Record(t.TempDir(), []string{"sh", "-c", "echo nope; exit 3"}, out)
	joined := strings.Join(*lines, "\n")
	if code != 0 {
		t.Fatalf("recording a failing command should still produce evidence: exit %d\n%s", code, joined)
	}
	if !strings.Contains(joined, "exit 3") {
		t.Errorf("the failure was not recorded as such:\n%s", joined)
	}
}

// #208: the difference between a measurement and somebody's account of
// one, made visible.
//
// proved by: fingerprintLine widened to match any line — prose classifies
// as measured and the distinction disappears.
func TestClassifyTellsMeasurementFromClaim(t *testing.T) {
	measured := "Fingerprint: sha256:3f9a1b2c3d4e5f60\nProduced: 214 bytes, exit 0\n"
	if got := Classify(measured); got != Measured {
		t.Errorf("a fingerprint classified as %v", got)
	}
	for _, prose := range []string{
		"I ran the suite and it passed.",
		"Verified by hand on macOS and Linux.",
		"sha256 of the file is unchanged.", // mentions sha256, is not a fingerprint line
	} {
		if got := Classify(prose); got != Claim {
			t.Errorf("%q classified as %v, want a claim", prose, got)
		}
	}
}

// Neither kind is refused. Most evidence is prose, and a sentence saying
// why a check was not needed is exactly right — what was missing is a
// reader being able to tell which they are looking at.
//
// proved by: Kind.String() made to return the same word for both — the
// note stops distinguishing them.
func TestBothKindsAreNamed(t *testing.T) {
	if Measured.String() == Claim.String() {
		t.Fatal("the two kinds render identically — a reader cannot tell them apart")
	}
	if !strings.Contains(Claim.String(), "claim") {
		t.Errorf("a claim does not say so: %q", Claim.String())
	}
}
