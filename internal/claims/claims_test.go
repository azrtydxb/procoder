package claims

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func collect() (func(string), *[]string) {
	var l []string
	return func(s string) { l = append(l, s) }, &l
}

var when = time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

// Overlap answers "could these collide", not "will they". A maybe is worth
// saying: a false conflict costs a question, a missed one costs two
// agents' work.
//
// proved by: Overlap made to return `a == b` only — every glob pair below
// stops conflicting and the check reports nothing useful.
func TestOverlapIsConservative(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"internal/gate/**", "internal/gate/adoption.go"},
		{"internal/gate/", "internal/gate/gate.go"},
		{"internal/*/x.go", "internal/gate/x.go"},
		{"docs/a.md", "docs/a.md"},
	} {
		if !Overlap(c.a, c.b) {
			t.Errorf("%q and %q could collide and were not reported", c.a, c.b)
		}
	}
}

// And unrelated paths must not conflict, or every claim collides with
// every other and the report says nothing.
//
// proved by: literalPrefix made to return "" always — both globs read as
// wide open and everything conflicts.
func TestUnrelatedPathsDoNotConflict(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"internal/gate/**", "internal/security/**"},
		{"docs/a.md", "docs/b.md"},
		{"cmd/**", "internal/**"},
	} {
		if Overlap(c.a, c.b) {
			t.Errorf("%q and %q are unrelated and were reported as a conflict", c.a, c.b)
		}
	}
}

// An agent does not conflict with itself — claiming twice is not a
// collision, and reporting it would train people to ignore the report.
//
// proved by: the EqualFold self-check removed from Conflicts — a second
// claim by the same agent conflicts with its own first.
func TestAnAgentDoesNotConflictWithItself(t *testing.T) {
	existing := []Claim{{By: "alice", Globs: []string{"internal/gate/**"}}}
	if got := Conflicts(existing, Claim{By: "alice", Globs: []string{"internal/gate/x.go"}}); len(got) != 0 {
		t.Fatalf("an agent conflicted with its own claim: %v", got)
	}
}

// An unreadable ledger is not an empty one. Reporting it as no claims
// would say "nobody else is working here" on the strength of not having
// looked — the failure this package exists to prevent, and the same shape
// found five times elsewhere in this codebase.
//
// proved by: the JSON error branch in Load made to return nil, nil — the
// corrupt ledger reads as no claims and Add proceeds.
func TestAnUnreadableLedgerIsNotAnEmptyOne(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(File))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("a corrupt ledger loaded as no claims at all")
	}
	out, lines := collect()
	if code := Add(root, "alice", []string{"x/**"}, when, out); code == 0 {
		t.Errorf("a claim was recorded against a ledger procoder could not read: %v", *lines)
	}
}

// A claim, its conflict, and the release that clears it.
//
// proved by: Release made to keep every claim — the released agent still
// holds theirs and the next claim still conflicts.
func TestAClaimConflictsUntilItIsReleased(t *testing.T) {
	root := t.TempDir()
	out, _ := collect()
	Add(root, "alice", []string{"internal/gate/**"}, when, out)

	out2, lines2 := collect()
	Add(root, "bob", []string{"internal/gate/adoption.go"}, when, out2)
	if !strings.Contains(strings.Join(*lines2, "\n"), "CONFLICT") {
		t.Fatalf("the overlap was not reported: %v", *lines2)
	}

	out3, _ := collect()
	Release(root, "alice", out3)

	out4, lines4 := collect()
	Add(root, "carol", []string{"internal/gate/gate.go"}, when, out4)
	if strings.Contains(strings.Join(*lines4, "\n"), "alice") {
		t.Fatalf("a released claim still conflicts: %v", *lines4)
	}
}
