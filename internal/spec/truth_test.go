package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/docs"
)

// specWith builds a spec document with the given section bodies.
func truthSpec(sections map[string]string) string {
	var b strings.Builder
	b.WriteString("# A spec\n\nStatus: draft\n\n")
	for _, s := range Sections {
		b.WriteString("## " + s + "\n\n")
		if body, ok := sections[s]; ok {
			b.WriteString(body + "\n\n")
		} else {
			b.WriteString("Something.\n\n")
		}
	}
	return b.String()
}

// S-1: a citation to something that does not exist is the mechanical
// stand-in for a claim that is not true. A spec may assert anything; one
// that NAMES what it asserts about can be checked.
//
// proved by: `resolves` made to return true always — both bogus citations
// pass and the test says so.
func TestUnresolvedCitationsAreReported(t *testing.T) {
	doc := truthSpec(map[string]string{
		"In scope": "- [S-1] The gate calls `nosuchpkg.NoSuchSymbolAnywhere` before anything else.\n" +
			"- [S-2] It reads `internal/nowhere/absent.go` for the rules.",
	})
	got := UnresolvedCitations("../..", doc)
	if len(got) != 2 {
		t.Fatalf("want 2 unresolved citations, got %d: %+v", len(got), got)
	}
	for _, c := range got {
		if c.Line == 0 {
			t.Errorf("citation %q carries no line number — a refusal must point at something", c.Text)
		}
	}
}

// The other half, and the one that matters more: a citation that resolves
// must be silent. A checker that refuses correct citations is one people
// switch off — the failure this project keeps relearning (#172, #185).
//
// proved by: `fileExtensions` emptied — `AGENTS.md` then parses as the
// pkg.Symbol shape, the symbol `md` is looked up, and a real file is
// refused. That was this checker's own first false positive.
func TestResolvingCitationsAreSilent(t *testing.T) {
	doc := truthSpec(map[string]string{
		"In scope": "- [S-1] `gitx.Attribution` runs over `internal/gate/gate.go`.\n" +
			"- [S-2] An `AGENTS.md` naming procoder counts as adoption.",
	})
	if got := UnresolvedCitations("../..", doc); len(got) != 0 {
		t.Fatalf("citations that exist were refused: %+v", got)
	}
}

// A fenced block is not a claim. A spec showing example output, or naming
// a symbol that deliberately does not exist, must not be refused for it.
//
// proved by: the fenceRe replacement removed — the fenced citation is
// refused.
func TestCitationsInsideFencesAreIgnored(t *testing.T) {
	doc := truthSpec(map[string]string{
		"In scope": "- [S-1] The output looks like this:\n\n```\n`nosuchpkg.NotReal` was here\n```\n",
	})
	if got := UnresolvedCitations("../..", doc); len(got) != 0 {
		t.Fatalf("a citation inside a code fence was refused: %+v", got)
	}
}

// S-2: a criterion nobody can run is an agreement, not a test. It passes
// review, becomes a story, gets ticked, and the thing it promised was
// never checked by anything.
//
// proved by: `observableRe.MatchString` negated — everything is accepted.
func TestACriterionWithNoObservableIsReported(t *testing.T) {
	doc := truthSpec(map[string]string{
		"Acceptance criteria": "- [ ] [S-1] The gate is correct.\n- [ ] [S-2] Users are happy with the result.",
	})
	got := UncheckableCriteria(doc)
	if len(got) != 2 {
		t.Fatalf("want 2 uncheckable criteria, got %d: %+v", len(got), got)
	}
}

// Naming the command, the test, or the file all count. Three shapes,
// because a rule that only accepted one would push people into writing
// criteria that fit the checker rather than criteria that are true.
//
// proved by: the `procoder ` alternative removed from observableRe — the
// command-shaped criterion is refused.
func TestACriterionNamingItsObservableIsAccepted(t *testing.T) {
	for _, c := range []string{
		"- [ ] [S-1] `procoder check` over the fixture exits 1.",
		"- [ ] [S-2] `TestTheGateBlocks` asserts it.",
		"- [ ] [S-3] `internal/gate/gate.go` carries the branch.",
	} {
		doc := truthSpec(map[string]string{"Acceptance criteria": c})
		if got := UncheckableCriteria(doc); len(got) != 0 {
			t.Errorf("criterion %q was refused despite naming an observable: %+v", c, got)
		}
	}
}

// A criterion wraps, and its observable is usually on the continuation
// line. Reading only the first line was this checker's own first bug: it
// refused three criteria in its own spec.
//
// proved by: `joinWrapped(lines, i)` reverted to `t` — the wrapped
// criterion is refused.
func TestAWrappedCriterionIsReadWhole(t *testing.T) {
	doc := truthSpec(map[string]string{
		"Acceptance criteria": "- [ ] [S-1] The same command over a fixture spec refuses and\n" +
			"      names it, asserted by `TestSomethingSpecific`.",
	})
	if got := UncheckableCriteria(doc); len(got) != 0 {
		t.Fatalf("a wrapped criterion was refused because only its first line was read: %+v", got)
	}
}

// S-3: the criteria that were green whatever the code did. The docs domain
// needs a built index — without one it reports "public surface NOT
// computed" and never reaches a finding, so a criterion about a
// documentation obligation on an unindexed fixture cannot fail.
//
// proved by: the docs entry removed from `prerequisites` — the criterion
// is accepted and the sprint-021 defect class goes unreported.
func TestACriterionMissingItsPrerequisiteIsReported(t *testing.T) {
	doc := truthSpec(map[string]string{
		"Acceptance criteria": "- [ ] [S-1] `procoder check` reports no documentation obligation for the fixture.",
	})
	got := UncheckableCriteria(doc)
	if len(got) != 1 {
		t.Fatalf("want the missing prerequisite reported, got %+v", got)
	}
	if !strings.Contains(got[0].Why, "index") {
		t.Errorf("the refusal does not name the index as the missing setup: %q", got[0].Why)
	}
}

// And naming the prerequisite clears it — otherwise the rule is
// unsatisfiable and people delete the criterion instead of fixing it.
//
// proved by: the `containsAny(lower, p.names)` escape removed — a
// criterion that DID account for the index is still refused.
func TestNamingThePrerequisiteClearsIt(t *testing.T) {
	doc := truthSpec(map[string]string{
		"Acceptance criteria": "- [ ] [S-1] With `procoder index build` run first, `procoder check` reports no documentation obligation.",
	})
	if got := UncheckableCriteria(doc); len(got) != 0 {
		t.Fatalf("a criterion that named its prerequisite was still refused: %+v", got)
	}
}

// S-5, and the criterion this test was promised by name in the spec:
// every refusal names a route out. A checker that refuses without one is a
// gate people learn to route around, which is how `--no-verify` becomes
// muscle memory.
//
// proved by: the trailing advice removed from either message — the test
// names which one lost it.
func TestEveryRefusalNamesTheFix(t *testing.T) {
	doc := truthSpec(map[string]string{
		"In scope":            "- [S-1] It calls `nosuchpkg.NoSuchSymbolAnywhere`.",
		"Acceptance criteria": "- [ ] [S-1] The gate is correct.",
	})
	gaps := TruthGaps("../..", doc)
	if len(gaps) < 2 {
		t.Fatalf("want a refusal of each kind, got %d: %v", len(gaps), gaps)
	}
	// Every kind, enumerated rather than counted: a rule added later
	// without a route out fails here rather than shipping.
	routes := []string{
		"cite something that exists", // an unresolved citation
		"say the command",            // a criterion with no observable
		"add `fails if",              // a criterion with no falsifier
		"cite where that lives",      // an uncited domain claim
	}
	for _, g := range gaps {
		ok := false
		for _, r := range routes {
			if strings.Contains(g, r) {
				ok = true
			}
		}
		if !ok {
			t.Errorf("this refusal offers no route out:\n%s", g)
		}
	}
}

// S-6: the regression fixture. The spec whose deviations motivated this
// must actually be caught by it — a check that cannot find the defects it
// was built for has not been shown to work.
//
// This asserts the criteria-with-no-observable class, which is the one
// that is mechanically reachable. Which classes are NOT reachable is
// recorded in the story's evidence rather than left implied.
//
// proved by: `UncheckableCriteria` made to return nil — the spec that
// motivated the check reports clean.
func TestTheSpecThatMotivatedThisIsCaughtByIt(t *testing.T) {
	path := filepath.Join("..", "..", ".procoder", "specs", "adoption-aware-gate.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the motivating spec is not in the tree: %v", err)
	}
	faults := UncheckableCriteria(string(raw))
	if len(faults) == 0 {
		t.Fatal("the spec whose deviations motivated this check reports clean — the check has not been shown to work")
	}
	t.Logf("adoption-aware-gate.md: %d criterion/criteria name no observable", len(faults))
}

// A cited command that does not exist is a claim about this tool that is
// simply false.
//
// This test exists because the feature it covers was written, verified by
// hand, and then silently lost: a mutation-testing restore reverted it and
// the commit went out without it, because nothing failed. An untested
// feature is one that can disappear between two green runs.
//
// proved by: the commandRe loop removed from UnresolvedCitations — the
// invented command passes.
func TestACitedCommandThatDoesNotExistIsReported(t *testing.T) {
	doc := truthSpec(map[string]string{
		"In scope": "- [S-1] `procoder nosuchcommand` reports the result.",
	})
	got := UnresolvedCitations("../..", doc)
	if len(got) != 1 {
		t.Fatalf("want the invented command reported, got %+v", got)
	}
	if !strings.Contains(got[0].Text, "nosuchcommand") {
		t.Errorf("the refusal does not name the command: %q", got[0].Text)
	}
}

// And a real one is silent, in both the bare and flagged forms.
//
// proved by: `Commands` emptied — every cited command is refused, which is
// the failure direction that teaches people to ignore the checker.
func TestACitedCommandThatExistsIsSilent(t *testing.T) {
	doc := truthSpec(map[string]string{
		"In scope": "- [S-1] `procoder spec check` refuses it and `procoder prune --apply` removes them.",
	})
	if got := UnresolvedCitations("../..", doc); len(got) != 0 {
		t.Fatalf("real commands were refused: %+v", got)
	}
}

// The command list here and the docs-coverage list are the same set. Two
// lists that must agree and are never compared is how a true citation
// starts reading as false.
//
// proved by: `prune` deleted from spec.Commands — the test names it.
func TestCommandsMatchTheDocsCoverageList(t *testing.T) {
	for _, c := range docs.Commands {
		if !Commands[c] {
			t.Errorf("%q is a documented command but spec.Commands does not know it — a spec citing it would be refused", c)
		}
	}
	for c := range Commands {
		found := false
		for _, d := range docs.Commands {
			if d == c {
				found = true
			}
		}
		if !found {
			t.Errorf("spec.Commands knows %q but the docs-coverage list does not — one of the two is stale", c)
		}
	}
}

// The deviation that cost sprint 021 most, in mechanical form. Its S-3
// listed nine domains — formatting among them — and cited nothing, so
// nobody looked at where the code decides formatting and nobody noticed
// the format loop ran before the scope decision. Honouring that one word
// meant restructuring RunWith and repairing four fixtures, discovered
// mid-sprint.
//
// The rule does not verify the claim. It puts the author in the file,
// which is where the discovery happens.
//
// proved by: `citationRe.MatchString(bullet) || commandRe.MatchString(bullet)`
// negated — every domain claim passes uncited.
func TestAPromiseNamingADomainMustCiteIt(t *testing.T) {
	doc := truthSpec(map[string]string{
		"In scope": "- [S-1] The gate runs none of the domains that encode procoder's conventions: formatting, linting and debt.",
	})
	got := UncitedClaims(doc)
	if len(got) != 1 {
		t.Fatalf("want the uncited domain claim reported, got %+v", got)
	}
	if !strings.Contains(got[0].Why, "formatting") {
		t.Errorf("the refusal does not name the domain it saw: %q", got[0].Why)
	}
}

// Citing it clears the rule — otherwise it is unsatisfiable and the
// promise gets reworded to dodge the checker rather than checked.
//
// proved by: the `continue` on a matched citation removed — a claim that
// DID cite is still refused.
func TestACitedDomainPromiseIsAccepted(t *testing.T) {
	for _, bullet := range []string{
		"- [S-1] Formatting does not run there — see `internal/gate/gate.go`.",
		"- [S-2] The debt domain is skipped, as `procoder debt` would otherwise report.",
	} {
		doc := truthSpec(map[string]string{"In scope": bullet})
		if got := UncitedClaims(doc); len(got) != 0 {
			t.Errorf("a promise that cited its domain was refused: %q -> %+v", bullet, got)
		}
	}
}

// A promise that names no domain is left alone. The rule is about claims
// on existing code, not about every sentence in the section.
//
// proved by: the `len(named) == 0` skip removed — ordinary promises are
// refused and the checker becomes noise.
func TestAPromiseNamingNoDomainIsLeftAlone(t *testing.T) {
	doc := truthSpec(map[string]string{
		"In scope": "- [S-1] A person can force either mode from configuration.",
	})
	if got := UncitedClaims(doc); len(got) != 0 {
		t.Fatalf("a promise naming no domain was refused: %+v", got)
	}
}

// The project's own mutation discipline, applied to the criterion rather
// than to the test. Two of sprint 021's five deviations were criteria that
// could not fail: one about narrowing junk findings to the diff — a
// failure that cannot happen, because those findings carry no line
// number — and one about a typo falling back to detection, on a fixture
// where an accepted typo and a correct fallthrough are indistinguishable.
//
// Stating the falsifier is what surfaces both: you cannot say what change
// would break a criterion without constructing the case that separates
// pass from fail, and when you cannot, that is the answer.
//
// proved by: `falsifierRe.MatchString(criterion)` negated — every
// criterion passes and the class goes unreported.
func TestACriterionWithNoFalsifierIsReported(t *testing.T) {
	doc := truthSpec(map[string]string{
		"Acceptance criteria": "- [ ] [S-1] `procoder check` exits 1 for the fixture.",
	})
	got := CriteriaWithoutFalsifiers(doc)
	if len(got) != 1 {
		t.Fatalf("want the criterion reported for having no falsifier, got %+v", got)
	}
	if !strings.Contains(got[0].Why, "fails if") {
		t.Errorf("the refusal does not say what to write: %q", got[0].Why)
	}
}

// Several phrasings count, because a rule that accepted one exact form
// would be satisfied by pasting that form rather than by thinking.
//
// proved by: the alternation in falsifierRe cut to `fails if` alone — the
// other phrasings are refused.
func TestACriterionNamingItsFalsifierIsAccepted(t *testing.T) {
	for _, c := range []string{
		"- [ ] [S-1] `procoder check` exits 1; fails if the branch is made unconditional.",
		"- [ ] [S-2] `procoder check` exits 1, proved by removing the guard.",
		"- [ ] [S-3] `procoder check` exits 1 — breaks if the deadline is dropped.",
	} {
		doc := truthSpec(map[string]string{"Acceptance criteria": c})
		if got := CriteriaWithoutFalsifiers(doc); len(got) != 0 {
			t.Errorf("criterion %q was refused despite naming a falsifier: %+v", c, got)
		}
	}
}

// S-6 completed: all five of the deviations that motivated this check are
// now mechanically reported against the spec they came from.
//
// Each class is asserted separately and by line, so a rule that stopped
// working could not be hidden by another rule still firing.
//
// proved by: any one of the three checks made to return nil — the class it
// covers is named here as unreported.
func TestAllFiveMotivatingDefectsAreReported(t *testing.T) {
	path := filepath.Join("..", "..", ".procoder", "specs", "adoption-aware-gate.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the motivating spec is not in the tree: %v", err)
	}
	doc := string(raw)

	// Defects 3 and 4: criteria naming an observable no fixture produces.
	if got := UncheckableCriteria(doc); len(got) == 0 {
		t.Error("no criterion reported for naming no observable — defects 3 and 4 unreported")
	}
	// Defect 1: S-3 listed nine domains, formatting among them, and cited
	// nothing.
	uncited := UncitedClaims(doc)
	formattingClaim := false
	for _, f := range uncited {
		if strings.Contains(strings.ToLower(f.Text), "formatting") {
			formattingClaim = true
		}
	}
	if !formattingClaim {
		t.Errorf("the formatting claim that cost the restructure is unreported — defect 1 unreached: %+v", uncited)
	}
	// Defects 2 and 5: criteria that cannot fail.
	noFalsifier := CriteriaWithoutFalsifiers(doc)
	junk, override := false, false
	for _, f := range noFalsifier {
		lower := strings.ToLower(f.Text)
		if strings.Contains(lower, "junk file") {
			junk = true
		}
		if strings.Contains(lower, "gate] scope") || strings.Contains(lower, "procoder_gate_scope") {
			override = true
		}
	}
	if !junk {
		t.Error("the junk-narrowing criterion is unreported — defect 2 unreached")
	}
	if !override {
		t.Error("the override criterion is unreported — defect 5 unreached")
	}
}

// A Windows checkout has CRLF. The section marker is then "## Name\r\n",
// the lookup for "## Name\n" finds nothing, and every check reports a
// clean document — a silent green that passed on macOS and Linux and
// failed only on Windows CI.
//
// Asserted over the same document twice, converted rather than written
// out separately, so the two cannot drift and the ONLY difference is the
// line ending.
//
// proved by: the normaliseEOL call removed from sectionOf — the CRLF
// document reports clean and the test names it.
func TestTheChecksReadCRLFDocuments(t *testing.T) {
	doc := truthSpec(map[string]string{
		"In scope":            "- [S-1] The gate calls `nosuchpkg.NoSuchSymbolAnywhere`.",
		"Acceptance criteria": "- [ ] [S-1] The gate is correct.",
	})
	crlf := strings.ReplaceAll(doc, "\n", "\r\n")

	lf := TruthGaps("../..", doc)
	if len(lf) == 0 {
		t.Fatal("the LF fixture reports nothing — this test would prove nothing")
	}
	got := TruthGaps("../..", crlf)
	if len(got) != len(lf) {
		t.Fatalf("CRLF gave %d findings, LF gave %d — the checks do not read a Windows checkout:\nCRLF: %v\nLF:   %v",
			len(got), len(lf), got, lf)
	}
}

// A criterion that names a test already carries the discipline: the test
// must name the mutation that makes it fail, and `procoder test` asks for
// that. Demanding the clause here too would be the same question twice,
// and a rule that asks twice is one people satisfy by pasting.
//
// This is what keeps the falsifier rule affordable — name the test you
// would write, which a good criterion does anyway, and there is nothing
// further to add.
//
// proved by: the namesATest exemption removed — a criterion citing a test
// is refused for lacking a clause the test already carries.
func TestACriterionNamingATestNeedsNoSeparateFalsifier(t *testing.T) {
	doc := truthSpec(map[string]string{
		"Acceptance criteria": "- [ ] [S-1] `procoder check` exits 1, asserted by `TestTheGateBlocks`.",
	})
	if got := CriteriaWithoutFalsifiers(doc); len(got) != 0 {
		t.Fatalf("a criterion naming its test was still asked for a falsifier: %+v", got)
	}
}

// And the exemption must not swallow the rule: a criterion that names
// neither is still caught, which is what sprint 021's two unfailable
// criteria were.
//
// proved by: namesATest widened to match any backtick — every criterion is
// exempt and the rule does nothing.
func TestACriterionNamingNeitherIsStillCaught(t *testing.T) {
	doc := truthSpec(map[string]string{
		"Acceptance criteria": "- [ ] [S-1] `procoder check` exits 1 for the fixture.",
	})
	if got := CriteriaWithoutFalsifiers(doc); len(got) != 1 {
		t.Fatalf("a criterion naming neither a test nor a falsifier was not caught: %+v", got)
	}
}

// #198's remaining half, after 3.2.0 took the unfalsifiable case. Three
// ways a criterion looks measured and is not.
//
// proved by: each of the three regexps in turn made to match nothing —
// the case it covers is named here as unreported.
func TestWeakOraclesAreReported(t *testing.T) {
	for name, tc := range map[string]struct{ criterion, want string }{
		"fixed output": {
			"- [ ] [S-1] `echo ok` prints ok after the change; fails if it does not.",
			"no failing branch",
		},
		"hedged": {
			"- [ ] [S-1] `procoder check` mostly reports the right findings; fails if it regresses.",
			"hedged",
		},
		"unmeasured": {
			"- [ ] [S-1] `procoder check` runs fast enough on a large repo; fails if it slows down.",
			"without giving one",
		},
	} {
		doc := truthSpec(map[string]string{"Acceptance criteria": tc.criterion})
		got := WeakOracles(doc)
		if len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d: %+v", name, len(got), got)
			continue
		}
		if !strings.Contains(got[0].Why, tc.want) {
			t.Errorf("%s: the finding does not explain the fault: %q", name, got[0].Why)
		}
	}
}

// And a criterion that measures something real is silent — the rule is
// worthless if it fires on the criteria it is meant to encourage.
//
// proved by: `hedged` widened to match any word — every good criterion is
// refused and the checker becomes noise.
func TestAMeasuredCriterionIsSilent(t *testing.T) {
	for _, c := range []string{
		"- [ ] [S-1] `procoder check` exits 1 over the fixture; fails if the branch is made unconditional.",
		"- [ ] [S-2] The report names all three findings, asserted by `TestAllThree`.",
		"- [ ] [S-3] `procoder prune --apply` reclaims 1.03 GB on the fixture; fails if the sweep is skipped.",
	} {
		doc := truthSpec(map[string]string{"Acceptance criteria": c})
		if got := WeakOracles(doc); len(got) != 0 {
			t.Errorf("a measured criterion was refused: %q -> %+v", c, got)
		}
	}
}
