package testrun

import (
	"strings"
	"testing"
)

// nestedModuleFixture is a root module plus a nested one (its own go.mod,
// `replace => ../`), each with one package and one test.
func nestedModuleFixture(t *testing.T, nestedPasses bool) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module demo\n\ngo 1.22\n")
	write(t, root, "a/a.go", "package a\n")
	write(t, root, "a/a_test.go", "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n")
	write(t, root, "sub/go.mod", "module demo/sub\n\ngo 1.22\n\nrequire demo v0.0.0\n\nreplace demo => ../\n")
	write(t, root, "sub/x/x.go", "package x\n")
	body := "t.Fatal(\"deliberately red\")"
	if nestedPasses {
		body = "_ = t"
	}
	write(t, root, "sub/x/x_test.go", "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) { "+body+" }\n")
	return root
}

// A package in a nested module used to be named from the repo root and
// came back as "[setup failed] main module does not contain package": a
// correct commit blocked. Narrowed to it, the suite must run from the
// module's own directory.
//
// proved by: have runGo always run from root — the nested package fails to
// set up and this reads Fail with "does not contain package".
func TestGoNestedModulePackageIsTestedFromItsModule(t *testing.T) {
	requireGo(t)
	root := nestedModuleFixture(t, true)
	r := runGo(root, []string{"sub/x"}, false, "")
	if r.Verdict != Pass {
		t.Fatalf("nested module package must pass from its own dir, got %s: %s", r.Verdict, r.Detail)
	}
	if !strings.Contains(r.Detail, "sub: pass") {
		t.Errorf("the detail must say which module ran: %q", r.Detail)
	}
}

// Both modules in one commit run, and a red test in either one is red.
//
// proved by: drop the Fail branch of the merge in runGoModules — a failing
// nested module is reported as a pass.
func TestGoMixedModulesRunBothAndFailIfEitherFails(t *testing.T) {
	requireGo(t)
	root := nestedModuleFixture(t, false)
	r := runGo(root, []string{"a", "sub/x"}, false, "")
	if r.Verdict != Fail {
		t.Fatalf("a red test in the nested module must fail the run, got %s: %s", r.Verdict, r.Detail)
	}
	if !strings.Contains(r.Detail, ".: pass") || !strings.Contains(r.Detail, "sub: ") {
		t.Errorf("both modules must be reported: %q", r.Detail)
	}
	if !strings.Contains(r.Output+r.Detail, "TestX") {
		t.Errorf("the failing test must be named: %q / %q", r.Detail, r.Output)
	}
}
