package release

import (
	"fmt"
	"strings"
	"testing"
)

// para builds a changelog paragraph citing the given numbers, optionally
// crediting handles.
func para(text string, cites []int, credits ...string) string {
	var b strings.Builder
	b.WriteString("**Fixed — " + text + ".**\n")
	for _, n := range cites {
		fmt.Fprintf(&b, "([#%d](https://github.com/azrtydxb/procoder/pull/%d)) ", n, n)
	}
	b.WriteString("Prose about it.")
	for _, c := range credits {
		fmt.Fprintf(&b, " Reported by [@%s](https://github.com/%s).", c, c)
	}
	return b.String()
}

func fake(m map[int]origin) func(int) (origin, error) {
	return func(n int) (origin, error) {
		o, ok := m[n]
		if !ok {
			return origin{}, fmt.Errorf("no such number")
		}
		return o, nil
	}
}

// The maintainer's rule, case 1: a cited issue owes its author a credit,
// and the report names the handle to add rather than saying "wrong".
//
// proved by: the `credited[...]` skip inverted — the owed credit is
// reported as satisfied and the gap disappears.
func TestAnUncreditedIssueAuthorIsReported(t *testing.T) {
	got := missingCreditsWith(
		para("a thing", []int{42}),
		"maintainer", nil,
		fake(map[int]origin{42: {number: 42, login: "reporter", isPR: false}}),
	)
	if len(got) != 1 {
		t.Fatalf("want the uncredited reporter named, got %v", got)
	}
	if !strings.Contains(got[0], "@reporter") || !strings.Contains(got[0], "reported #42") {
		t.Errorf("the report does not name who and for what: %q", got[0])
	}
	// It must hand over the text, not just the complaint.
	if !strings.Contains(got[0], "https://github.com/reporter") {
		t.Errorf("the report does not give the line to add: %q", got[0])
	}
}

// Case 2: the same person opened the issue and the pull request. That is
// ONE credit, not two — a rule that demanded both would produce changelog
// entries thanking somebody twice in one paragraph.
//
// proved by: the `seen[...]` collapse removed — the same handle is
// reported twice.
func TestOnePersonWhoDidBothIsCreditedOnce(t *testing.T) {
	got := missingCreditsWith(
		para("a thing", []int{7, 8}),
		"maintainer", nil,
		fake(map[int]origin{
			7: {number: 7, login: "tobero", isPR: false},
			8: {number: 8, login: "tobero", isPR: true},
		}),
	)
	if len(got) != 1 {
		t.Fatalf("want exactly one credit owed, got %d: %v", len(got), got)
	}
}

// Case 3, the one that erases people: the reporter and the fixer are
// different. Both are owed. Crediting only the pull request quietly drops
// whoever found the problem.
//
// proved by: the loop over `owed` made to stop after the first — the
// second contributor vanishes and the test names them.
func TestAReporterAndADifferentFixerAreBothOwed(t *testing.T) {
	got := missingCreditsWith(
		para("a thing", []int{10, 11}),
		"maintainer", nil,
		fake(map[int]origin{
			10: {number: 10, login: "finder", isPR: false},
			11: {number: 11, login: "fixer", isPR: true},
		}),
	)
	if len(got) != 2 {
		t.Fatalf("want both owed, got %d: %v", len(got), got)
	}
	joined := strings.Join(got, "\n")
	for _, who := range []string{"@finder", "@fixer"} {
		if !strings.Contains(joined, who) {
			t.Errorf("%s is owed a credit and was not named:\n%s", who, joined)
		}
	}
	// And each is described by what they actually did.
	if !strings.Contains(joined, "reported #10") || !strings.Contains(joined, "contributed #11") {
		t.Errorf("the report does not distinguish reporting from contributing:\n%s", joined)
	}
}

// A credit already present is not reported again, or the rule is
// unsatisfiable and gets switched off.
//
// proved by: the `credited` lookup dropped — a correct paragraph is
// refused.
func TestAnAlreadyCreditedContributorIsSilent(t *testing.T) {
	if got := missingCreditsWith(
		para("a thing", []int{9}, "tobero"),
		"maintainer", nil,
		fake(map[int]origin{9: {number: 9, login: "tobero", isPR: true}}),
	); len(got) != 0 {
		t.Fatalf("a paragraph that already credits its contributor was refused: %v", got)
	}
}

// Whoever is cutting the release is excluded. Thanking yourself in your
// own release notes is noise, and a rule that demanded it would be ignored
// within one release — which is how a check stops being read at all.
//
// proved by: the `strings.EqualFold(o.login, me)` skip removed — the
// maintainer is owed a credit in every entry they write.
func TestTheReleaserIsNotOwedACredit(t *testing.T) {
	if got := missingCreditsWith(
		para("a thing", []int{1}),
		"maintainer", nil,
		fake(map[int]origin{1: {number: 1, login: "maintainer", isPR: false}}),
	); len(got) != 0 {
		t.Fatalf("the releaser was asked to credit themselves: %v", got)
	}
}

// GitHub not answering is not a pass. Who to credit is then unknown, and
// unknown must block like every other check here that could not run.
//
// proved by: the resolve-error branch made to `continue` — an
// unreachable API reads as "nobody is owed anything".
func TestAnUnresolvableNumberBlocks(t *testing.T) {
	got := missingCreditsWith(
		para("a thing", []int{99}),
		"maintainer", nil,
		fake(map[int]origin{}),
	)
	if len(got) != 1 {
		t.Fatalf("an unresolvable citation must block, got %v", got)
	}
	if !strings.Contains(got[0], "not a pass") {
		t.Errorf("the report does not say why it blocks: %q", got[0])
	}
}

// A paragraph citing nothing owes nothing — the rule is about crediting
// people for the things a paragraph names.
//
// The `len(nums) == 0` skip is an early return, not the protection — with
// no numbers the loop below has nothing to resolve and owes nothing
// anyway. Verified: removing it does not fail this test. What this pins is
// the behaviour, so a future version that starts scanning prose for
// handles fails here.
func TestAParagraphCitingNothingOwesNothing(t *testing.T) {
	if got := missingCreditsWith(
		"**Changed — something.** Prose with no citations at all.",
		"maintainer", nil,
		fake(map[int]origin{}),
	); len(got) != 0 {
		t.Fatalf("a paragraph citing nothing produced findings: %v", got)
	}
}

// Raised in review on #213: if `gh` cannot say who is cutting the release,
// the exclusion cannot be applied. An empty handle excludes nobody, so the
// rule would quietly start demanding the maintainer credit themselves in
// every entry — the rule silently not applying, which is the failure this
// file exists to end.
//
// The policy was already "unknown is not a pass". It was applied to the
// citation lookup and not to this one.
//
// proved by: the `meErr != nil || me == ""` guard removed — the fixture
// then reports the maintainer as owed a credit instead of blocking.
func TestAnUnknownReleaserBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		me   string
		err  error
	}{
		{"gh failed", "", fmt.Errorf("gh: not authenticated")},
		{"gh answered with nothing", "", nil},
	} {
		got := missingCreditsWith(
			para("a thing", []int{1}),
			tc.me, tc.err,
			fake(map[int]origin{1: {number: 1, login: "maintainer", isPR: false}}),
		)
		if len(got) != 1 {
			t.Fatalf("%s: want a blocking finding, got %v", tc.name, got)
		}
		if !strings.Contains(got[0], "could not be determined") {
			t.Errorf("%s: the finding does not say why it blocks: %q", tc.name, got[0])
		}
	}
	// Which is why MissingCredits refuses before calling this at all.
}
