package spec

import (
	"strings"
	"testing"
)

// fullSpec is a spec with every section answered except the two under
// test, so Check's other gaps do not mask the coverage one.
func fullSpec(scope, criteria string) string {
	out := "# probe\n\nStatus: draft\n"
	for _, s := range Sections {
		out += "\n## " + s + "\n\n"
		switch s {
		case "In scope":
			out += scope
		case "Acceptance criteria":
			out += criteria
		case "Open questions":
			out += "<!-- none -->\n"
		default:
			out += "Something a person actually wrote.\n"
		}
	}
	return out
}

func specWith(scope, criteria string) string {
	return "# probe\n\n## In scope\n\n" + scope +
		"\n## Acceptance criteria\n\n" + criteria + "\n## Open questions\n\n"
}

// A spec can promise five things, write criteria for three, and pass. That
// is not hypothetical: it happened to planning-methodology in this
// repository. `backlog seed` writes one story per criterion, so the two
// untested promises became no stories, got no sprint, and were never
// missed — the epic closed at "all stories done" while the feature
// shipped half-built, and nothing anywhere noticed until somebody asked.
// proved by: returned nil from ScopeCoverage — the spec that under-
// delivered passes again, and so does every future one.
func TestScopePromisedWithoutACriterionIsAGap(t *testing.T) {
	s := specWith(
		"- [S-1] the thing that got built\n- [S-2] the thing that did not\n",
		"- [ ] [S-1] a criterion citing the first\n")

	uncovered, declared := ScopeCoverage(s)
	if !declared {
		t.Fatal("ids are present, so coverage is declared and checkable")
	}
	if len(uncovered) != 1 || uncovered[0] != "S-2" {
		t.Fatalf("the untested promise is named: %v", uncovered)
	}

	// One criterion may genuinely cover several bullets, and citing both
	// is how it says so.
	both := specWith(
		"- [S-1] one\n- [S-2] two\n",
		"- [ ] [S-1] [S-2] a criterion that really does cover both\n")
	if uncovered, _ := ScopeCoverage(both); len(uncovered) != 0 {
		t.Errorf("a criterion citing both ids covers both: %v", uncovered)
	}
}

// Coverage is declared, never inferred. A spec whose bullets carry no ids
// cannot be checked, and a check that cannot run must not read as one
// that passed — the rule this repository already applies to every missing
// tool.
// proved by: returned declared=true for a spec with no ids at all — an
// unlabelled spec reports full coverage, which is the exact silence this
// exists to break.
func TestAnUnlabelledScopeIsNotCheckedRatherThanCovered(t *testing.T) {
	s := specWith("- a promise with no id\n- another\n", "- [ ] a criterion with no id\n")

	uncovered, declared := ScopeCoverage(s)
	if declared {
		t.Fatal("nothing here can be checked; saying so is the point")
	}
	if len(uncovered) != 0 {
		t.Errorf("an unchecked spec reports no uncovered ids — it reports that it is unchecked: %v", uncovered)
	}
	if n := ScopeBullets(s); n != 2 {
		t.Errorf("and says how many bullets would need labelling, got %d", n)
	}

	// A spec that promises nothing at all is not unchecked; there is
	// simply nothing to check, and a report saying otherwise is noise.
	if n := ScopeBullets(specWith("", "- [ ] one\n")); n != 0 {
		t.Errorf("no bullets is not a gap: %d", n)
	}
}

// The gap must reach the verdict, not merely be computable: `backlog
// seed` refuses a spec that is not COMPLETE, so a gap that never becomes
// a gap in Check leaves seed happy to build the half-spec.
// proved by: dropped the ScopeCoverage call from Check — the spec is
// COMPLETE again and seeds a backlog covering two thirds of what it
// promised.
func TestTheGapReachesTheVerdict(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "probe", fullSpec(
		"- [S-1] built\n- [S-2] forgotten\n",
		"- [ ] [S-1] a criterion a reviewer could verify\n"))

	var lines []string
	code := Check(root, "probe", func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if code == 0 {
		t.Fatalf("a spec that promises what it does not test is not ready:\n%s", joined)
	}
	if !strings.Contains(joined, "S-2") {
		t.Errorf("and the verdict names which promise: %s", joined)
	}
}
