package glossary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withGlossary(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(Path))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

// The glossary is a flat list of terms with definitions.
//
// proved by: the `## ` heading test in Load inverted — nothing parses and
// the glossary reads as empty.
func TestLoadReadsTermsAndDefinitions(t *testing.T) {
	root := withGlossary(t, "# Vocabulary\n\n## quality chain\n\nThe refusing controllers.\n\n## the gate\n\nWhat runs before a commit.\n")
	terms, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 2 {
		t.Fatalf("want 2 terms, got %d: %+v", len(terms), terms)
	}
	if terms[0].Name != "quality chain" || !strings.Contains(terms[0].Definition, "refusing controllers") {
		t.Errorf("first term parsed wrong: %+v", terms[0])
	}
}

// P-CONTROL: add PRINTS the entry. The binary never writes the file — the
// same rule as `adr new` and every other authoring command here.
//
// proved by: an os.WriteFile added to Add — the file appears and the test
// names it.
func TestAddWritesNothing(t *testing.T) {
	root := withGlossary(t, "## existing\n\nA thing.\n")
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Path)))
	if err != nil {
		t.Fatal(err)
	}
	out, lines := collect()
	if code := Add(root, "new term", "Its definition.", out); code != 0 {
		t.Fatalf("exit %d: %v", code, *lines)
	}
	after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Add wrote to the glossary — the binary prints, the agent writes")
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "## new term") {
		t.Errorf("the entry to write was not printed: %v", *lines)
	}
}

// A term already defined is edited, not added twice. Two entries for one
// word is the drift a glossary exists to stop.
//
// proved by: the duplicate check removed from Add — a second entry is
// printed for a term that already exists.
func TestAddRefusesATermAlreadyDefined(t *testing.T) {
	root := withGlossary(t, "## The Gate\n\nWhat runs before a commit.\n")
	out, lines := collect()
	// Different spelling, same term — case and spacing must not hide it.
	if code := Add(root, "the gate", "Something else.", out); code == 0 {
		t.Fatalf("a term already defined was added again: %v", *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "already defined") {
		t.Errorf("the refusal does not say why: %v", *lines)
	}
}

// A term with no definition is one everybody will define differently.
// Reported, never blocking — a glossary that refuses over wording is worse
// than no glossary.
//
// proved by: the empty-definition test removed from Check — the finding
// disappears.
func TestCheckReportsATermWithNoDefinition(t *testing.T) {
	root := withGlossary(t, "## orphan\n\n## defined\n\nA thing.\n")
	out, lines := collect()
	code := Check(root, out)
	joined := strings.Join(*lines, "\n")
	if code != 0 {
		t.Errorf("check must report, never block: exit %d", code)
	}
	if !strings.Contains(joined, "orphan") || !strings.Contains(joined, "no definition") {
		t.Errorf("the undefined term was not reported: %s", joined)
	}
}

// Near is the whole point of cross-referencing: prose reinventing a word
// the project already has. It must NOT fire when the prose uses the term,
// or every document that mentions the vocabulary gets flagged.
//
// proved by: the `strings.Contains(lower, ...)` skip removed — a spec that
// correctly uses the term is reported as reinventing it.
func TestNearIgnoresProseThatUsesTheTerm(t *testing.T) {
	terms := []Term{{Name: "quality chain", Definition: "x"}}
	if got := Near(terms, "This spec extends the quality chain with another controller."); len(got) != 0 {
		t.Fatalf("prose using the term was flagged: %+v", got)
	}
}

// And a short term matches too much to be evidence of anything.
//
// Recorded honestly: the `len(name) < 4` guard is NOT what makes this
// pass. "investigate" contains "gate", so the literal "prose uses the
// term" skip catches it first, and removing the length guard fails
// nothing. Verified, not assumed. The guard stays as defence for a short
// term whose only match appears after normalisation, and this test pins
// the behaviour rather than the mechanism.
//
// proved by: the literal `strings.Contains(lower, ...)` skip removed —
// then this fixture is reported and the test names it.
func TestNearIgnoresVeryShortTerms(t *testing.T) {
	terms := []Term{{Name: "gate", Definition: "x"}}
	if got := Near(terms, "We should investigate delegating this."); len(got) != 0 {
		t.Fatalf("a short term matched unrelated prose: %+v", got)
	}
}

// A glossary that EXISTS and cannot be read is not the same as no
// glossary. Reporting it as none makes this feature go quiet in the one
// place it is meant to help, and it is the fifth time that shape has
// appeared in this codebase — unknown treated as none. Raised in review on
// #217.
//
// proved by: the os.IsNotExist branch in Load made to swallow every error
// — the unreadable file reads as an empty glossary and nothing says so.
func TestAnUnreadableGlossaryIsNotAnEmptyOne(t *testing.T) {
	root := withGlossary(t, "## a term\n\nA definition.\n")
	p := filepath.Join(root, filepath.FromSlash(Path))
	if err := os.Chmod(p, 0o000); err != nil {
		t.Skipf("cannot make the file unreadable here: %v", err)
	}
	defer os.Chmod(p, 0o644)
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can read anything")
	}

	if _, err := Load(root); err == nil {
		t.Fatal("an unreadable glossary loaded without error — it reads as no glossary at all")
	}
	out, lines := collect()
	if code := Check(root, out); code == 0 {
		t.Errorf("check reported success on a glossary it could not read: %v", *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "could not be read") {
		t.Errorf("the finding does not say what happened: %v", *lines)
	}
}

// And an absent glossary is still the ordinary case: no error, no noise.
//
// proved by: the os.IsNotExist branch removed — every repository without a
// glossary starts reporting an error.
func TestNoGlossaryIsNotAnError(t *testing.T) {
	if terms, err := Load(t.TempDir()); err != nil || len(terms) != 0 {
		t.Fatalf("a repository with no glossary reported terms=%v err=%v", terms, err)
	}
}
