package testrun

import (
	"strings"
	"testing"
)

// The gate hands runGo package DIRECTORIES (changedPackages maps a changed
// file to path.Dir(rel)). A changed sub/main.go arrives as "sub", the root
// of the nested module; the module lookup used to treat it as a file, start
// its ascent at the repo root, and pick the root module — which then failed
// with "main module does not contain package demo/sub".
//
// proved by: make gomod.ModuleDir start at filepath.Dir(path) for a
// directory too — "sub" resolves to the root module and this fails with
// "does not contain package".
func TestGoDirectoryAtANestedModuleRootIsTestedFromThatModule(t *testing.T) {
	requireGo(t)
	root := nestedModuleFixture(t, true)
	write(t, root, "sub/s.go", "package sub\n")
	write(t, root, "sub/s_test.go", "package sub\n\nimport \"testing\"\n\nfunc TestS(t *testing.T) {}\n")
	r := runGo(root, []string{"sub"}, false, "")
	if r.Verdict != Pass {
		t.Fatalf("a package at the nested module's root must pass from that module, got %s: %s", r.Verdict, r.Detail)
	}
	if !strings.Contains(r.Detail, "sub: pass") {
		t.Errorf("the detail must say the nested module ran: %q", r.Detail)
	}
}
