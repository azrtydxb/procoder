package release

import (
	"strings"
	"testing"
)

// bodyOf must drop the frontmatter, or a version bump alone reads as a
// contract change — the version IS the announcement, not the thing being
// announced, and a check that fired on its own fix would be useless.
//
// proved by: bodyOf made to return its argument unchanged — the two
// fixtures below stop comparing equal.
func TestBodyIgnoresTheFrontmatter(t *testing.T) {
	a := "---\nname: x\nmetadata:\n  contract: \"1\"\n---\n\n# Rules\n\nSame body.\n"
	b := "---\nname: x\nmetadata:\n  contract: \"2\"\n---\n\n# Rules\n\nSame body.\n"
	if bodyOf(a) != bodyOf(b) {
		t.Fatalf("a version bump alone read as a body change:\n%q\n%q", bodyOf(a), bodyOf(b))
	}
	if !strings.Contains(bodyOf(a), "Same body") {
		t.Errorf("the body was lost: %q", bodyOf(a))
	}
}

// And the version has to be readable, or nothing can compare it.
//
// proved by: the `contract:` prefix test changed to an exact match on the
// unindented key — the indented frontmatter value is never found.
func TestContractVersionIsRead(t *testing.T) {
	got := contractVersionOf("---\nname: x\nmetadata:\n  contract: \"7\"\n---\n\n# Rules\n")
	if got != "7" {
		t.Fatalf("contract version = %q, want 7", got)
	}
	if v := contractVersionOf("---\nname: x\n---\n\n# Rules\n"); v != "" {
		t.Errorf("a file with no contract version reported %q", v)
	}
}

// The scan stops at the body. A document whose PROSE mentions a contract
// must not be read as declaring one.
//
// proved by: the `strings.HasPrefix(t, "# ")` break removed — the body's
// text is scanned and a sentence becomes a version.
func TestTheVersionScanStopsAtTheBody(t *testing.T) {
	if v := contractVersionOf("---\nname: x\n---\n\n# Rules\n\ncontract: this is prose, not frontmatter\n"); v != "" {
		t.Fatalf("prose in the body was read as a contract version: %q", v)
	}
}

// No previous tag is the first release: nothing to compare against, and
// nothing to report.
//
// The `previousTag == ""` guard is an early return, not the protection:
// with no tag, `git show :path` fails and the function returns nil anyway.
// Verified — removing it fails nothing. This pins the behaviour so a
// future version that treats an empty ref as HEAD fails here.
func TestNoPreviousTagReportsNothing(t *testing.T) {
	if got := ContractDrift(t.TempDir(), ""); len(got) != 0 {
		t.Fatalf("the first release reported contract drift: %v", got)
	}
}
