package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// findingLiteral matches a gitx.Finding composite literal, permitting
// exactly one level of brace nesting inside it — Go's elided-type slice
// form, `[]gitx.Finding{{...}}`, opens with a second brace the flat
// pattern this replaced could not see past at all: not matched-and-
// passed, structurally invisible, so seven genuine "must always block"
// findings written that way were unexercised by this audit (#153). A
// literal referring to the bare, unqualified `Finding{}` — only possible
// from inside package gitx itself — is still outside what this can see;
// nothing there emits a "NOT checked"/"NOT run"/"NOT linted" message
// today, so it is a known boundary rather than a proven gap.
var findingLiteral = regexp.MustCompile(`(?s)gitx\.Finding\{(?:[^{}]|\{[^{}]*\})*?\}`)

// blocking matches "a check that did not run always blocks", which means
// Blocking: true literally — not a caller-supplied variable, however it
// is named. A `Blocking: block` naming a policy parameter would satisfy
// this by coincidence of variable naming rather than by proof, and that
// coincidence is exactly the gap D-7 needed a real exemption for: before
// it, every "NOT checked"/"NOT run" finding in the tree used a literal
// true; unlinted.go is the first to carry a real, possibly-false policy
// value, and it is excluded by path below rather than by loosening this
// regex to accept it.
var blocking = regexp.MustCompile(`Blocking:\s*true\b`)

// Every domain, not just the two that were fixed. A check that did not
// happen must block wherever it is reported — security, lint, formatting,
// infrastructure, docs, workflows — because the rule is about what a green
// gate MEANS, and one domain quietly exempting itself brings the whole
// meaning down with it.
//
// Two deliberate exceptions, named rather than matched by pattern —
// widening this audit's own regex (#153) made both visible for the first
// time, and each is a decision recorded in
// .procoder/specs/no-silent-green.md rather than a hole to close:
//
//   - lint/unlinted.go (D-7): a language procoder formats and has no
//     linter for AT ALL, where there is no `procoder init` remedy to
//     point at, so [lint] policy governs it like any other lint finding.
//   - lessons/copilot.go (D-8): LeakReminder is deliberately the gate's
//     offline half of the Copilot loop — a reminder that an adaptation
//     is unwritten, never a check that something is broken — and its own
//     doc comment already said so before this spec existed.
//
// A second, unrelated "NOT checked" appearing in either file is exactly
// the drift this audit exists to catch, and stays caught: the exemption
// matches the specific message each decision names, not the file — see
// TestASecondOffenderInAnExemptFileIsStillCaught below.
//
// This is a source audit rather than a behavioural test on purpose: the
// failure it guards against is a NEW domain, or a new branch of an old one,
// reporting "NOT checked" as info. No behavioural test can cover code that
// does not exist yet; reading the tree can.
// proved by: removed `Blocking: true` from the tflint branch in
// internal/infra — this test names infra/infra.go and the line, where the
// whole suite otherwise stayed green.
func TestNoDomainReportsAnUnrunCheckAsMerelyInformational(t *testing.T) {
	root := filepath.Join("..", "..", "internal")
	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		offenders = append(offenders, offendersIn(filepath.ToSlash(path), string(raw))...)
		return nil
	})
	if err != nil {
		t.Fatalf("the tree must be readable for this audit to mean anything: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("a check that did not happen must block, not inform:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// exempt names the ONE known literal per file this audit does not require
// to block — matched against a substring unique to that literal's own
// message, not against the file as a whole. A whole-file exemption was
// the first version of this and review on #159 caught what it actually
// meant: a second, unrelated "NOT checked" added anywhere else in either
// file would have gone unseen, exactly the drift the comment above
// claimed stayed caught. Matching the specific message text instead
// means only the literal D-7 or D-8 actually names is excused; anything
// else in the same file is scanned like any other file in the tree.
var exempt = []struct{ path, substr string }{
	{"lint/unlinted.go", "NOT linted — %s (lint)"},
	{"lessons/copilot.go", "captured Copilot findings cannot be counted"},
}

// offendersIn is the walk's per-file check, pulled out so it can be proven
// against a source string directly rather than only through the real tree
// — the real tree cannot demonstrate what the widened regex in #153 was
// FOR, because every genuine offender the widening found has since been
// fixed (docs.go, infra.go — see the fix commit). A synthetic fixture is
// what is left to show the narrow regex would still be blind to this
// shape today.
func offendersIn(path, src string) []string {
	var out []string
	for _, lit := range findingLiteral.FindAllStringIndex(src, -1) {
		text := src[lit[0]:lit[1]]
		if !strings.Contains(text, "NOT checked") &&
			!strings.Contains(text, "NOT run") &&
			!strings.Contains(text, "NOT linted") {
			continue
		}
		if blocking.MatchString(text) {
			continue
		}
		if isExemptLiteral(path, text) {
			continue
		}
		out = append(out, path+":"+itoa(strings.Count(src[:lit[0]], "\n")+1))
	}
	return out
}

func isExemptLiteral(path, literalText string) bool {
	for _, e := range exempt {
		if strings.HasSuffix(path, e.path) && strings.Contains(literalText, e.substr) {
			return true
		}
	}
	return false
}

// The regex this audit scans with has to see through Go's elided-type
// slice-literal shape, `[]gitx.Finding{{...}}` — a second, immediately-
// following brace the narrower pattern this replaced could not match at
// all. Proven against a synthetic fixture rather than the real tree: every
// real offender the widening found (docs.go, infra.go) is already fixed,
// so the tree alone cannot show what the narrow regex would still miss.
// proved by: reverting findingLiteral to
// `gitx\.Finding\{[^{}]*?\}` — this fixture's non-blocking "NOT checked"
// literal, written in the nested slice shape, goes unseen and the test
// passes on code that should fail it.
func TestFindingLiteralSeesTheNestedSliceLiteralShape(t *testing.T) {
	src := `package fake

import "procoder/internal/gitx"

func probe() []gitx.Finding {
	return []gitx.Finding{{File: "x",
		Message: "NOT checked — pretend tool is not installed"}}
}
`
	got := offendersIn("internal/fake/probe.go", src)
	if len(got) != 1 {
		t.Fatalf("a non-blocking NOT-checked finding in the nested slice shape must be caught: %v", got)
	}
	if !strings.Contains(got[0], "probe.go:6") {
		t.Errorf("the offender must name its file and line: %v", got)
	}
}

// itoa without importing strconv for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// The exemption for D-7 and D-8 must not become a hiding place for a
// different bug. It matches the one literal each decision names, by a
// substring unique to that literal's message — a second, unrelated
// "NOT checked" in the same file has to be caught like any other file
// in the tree.
// proved by: matching exempt by path alone (the file-wide form review
// caught on #159) — a synthetic second offender in unlinted.go goes
// unseen, in the same file the real, narrow exemption already excuses.
func TestASecondOffenderInAnExemptFileIsStillCaught(t *testing.T) {
	src := `package lint

import "procoder/internal/gitx"

func lintUnlinted(files []string, block bool) []gitx.Finding {
	out = append(out, gitx.Finding{Blocking: block, File: f,
		Message: fmt.Sprintf("NOT linted — %s (lint)", why)})
	// a second, unrelated finding sharing the file — not the one D-7 names
	out = append(out, gitx.Finding{File: f,
		Message: "NOT checked — some other tool is not installed (lint)"})
	return out
}
`
	got := offendersIn("internal/lint/unlinted.go", src)
	if len(got) != 1 {
		t.Fatalf("the D-7 literal stays excused, the unrelated one does not: %v", got)
	}
	if !strings.Contains(got[0], "unlinted.go:9") {
		t.Errorf("the caught offender must be the unrelated line, not D-7's: %v", got)
	}
}
