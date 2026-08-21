package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rootedLiteral matches a test assigning a path literal beginning with a
// slash to something used as a repository root.
var rootedLiteral = regexp.MustCompile(`root\s*:?=\s*(filepath\.FromSlash\()?"(/[^"]*)"`)

// A rooted path with no volume — "/repo" — is absolute on macOS and Linux
// and is not on Windows. gitx.RepoRel joins a non-absolute path onto the
// root, so a fixture written that way measures its own path arithmetic
// rather than the function, passes on two runners and fails on the third.
//
// This repository made that mistake three times in one afternoon, in
// internal/gitx and twice in internal/testrun, and each time CI caught
// what the local suite could not. t.TempDir gives a root that is genuinely
// absolute everywhere.
// proved by: put `root := filepath.FromSlash("/repo")` back into any test
// — this names the file and the line, on every platform, before the push.
func TestNoTestUsesARootedLiteralAsARepositoryRoot(t *testing.T) {
	base := filepath.Join("..", "..", "internal")
	var offenders []string
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		// This file quotes the pattern it forbids, in prose and in its own
		// regexp; excluding it by name keeps the guard from flagging its
		// own explanation.
		if strings.HasSuffix(filepath.ToSlash(path), "internal/audit/testroots_test.go") {
			return nil
		}
		src := string(raw)
		// The hazard is path ARITHMETIC, not the literal itself. A test
		// that hands a realistic absolute path to a sanitiser as sample
		// text — internal/copilot does, including a Windows one — is
		// correct and must not be flagged, or the guard becomes noise and
		// gets switched off. What breaks on Windows is joining or
		// relativising against a root the OS does not consider absolute.
		if !strings.Contains(src, "filepath.Join(root") &&
			!strings.Contains(src, "RepoRel(root") &&
			!strings.Contains(src, "filepath.Rel(root") {
			return nil
		}
		for _, m := range rootedLiteral.FindAllStringSubmatch(src, -1) {
			offenders = append(offenders, filepath.ToSlash(path)+`: root = "`+m[2]+`"`)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("the tree must be readable for this audit to mean anything: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("a repository root must come from t.TempDir, not a rooted literal — "+
			"\"/x\" is not absolute on Windows:\n  %s", strings.Join(offenders, "\n  "))
	}
}
