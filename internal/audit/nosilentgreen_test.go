package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// findingLiteral matches a gitx.Finding composite literal with no nested
// braces — the shape every domain uses to report one problem.
var findingLiteral = regexp.MustCompile(`(?s)gitx\.Finding\{[^{}]*?\}`)

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
// One deliberate exception, named rather than matched by pattern: D-7 in
// .procoder/specs/no-silent-green.md governs a language procoder formats
// and has no linter for AT ALL, where there is no `procoder init` remedy
// to point at. unlinted.go is the only source of "NOT linted", so it is
// excluded by path — a second such finding appearing anywhere else is
// exactly the drift this audit exists to catch, and stays caught.
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
	const exemptD7 = "lint/unlinted.go"
	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		if strings.HasSuffix(filepath.ToSlash(path), exemptD7) {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		src := string(raw)
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
			offenders = append(offenders,
				filepath.ToSlash(path)+":"+itoa(strings.Count(src[:lit[0]], "\n")+1))
		}
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
