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

// blocking matches the two honest ways a finding is marked blocking: set
// outright, or carried from the caller's policy.
var blocking = regexp.MustCompile(`Blocking:\s*(true|block)\b`)

// Every domain, not just the two that were fixed. A check that did not
// happen must block wherever it is reported — security, lint, formatting,
// infrastructure, docs, workflows — because the rule is about what a green
// gate MEANS, and one domain quietly exempting itself brings the whole
// meaning down with it.
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
