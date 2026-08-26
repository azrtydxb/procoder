package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if len(gaps) != 2 {
		t.Fatalf("want both kinds of refusal, got %d: %v", len(gaps), gaps)
	}
	for _, g := range gaps {
		// A fix reads as an instruction: it tells you what to write.
		if !strings.Contains(g, "cite something that exists") && !strings.Contains(g, "say the command") {
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
