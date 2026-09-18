package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// Every kind this package declares is in Kinds(), and no kind is in it
// twice.
//
// Asserted by reading the source rather than by exercising it: a kind
// nobody calls is exactly the one that reaches a client as an object it
// cannot name, and a behavioural test proves only the paths it ran.
//
// proved by: adding `const KindThing = "thing"` without adding it to
// Kinds() — the test names the constant.
func TestTypedResultsAreDeclaredOnce(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("could not read the package: %v", err)
	}

	declared := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, d := range file.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					continue
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if !strings.HasPrefix(name.Name, "Kind") || i >= len(vs.Values) {
							continue
						}
						lit, ok := vs.Values[i].(*ast.BasicLit)
						if !ok {
							continue
						}
						declared[strings.Trim(lit.Value, `"`)] = true
					}
				}
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("no Kind constants were found — the guard is not reading the source")
	}

	seen := map[string]bool{}
	for _, k := range Kinds() {
		if seen[k] {
			t.Errorf("Kinds() lists %q twice", k)
		}
		seen[k] = true
		if !declared[k] {
			t.Errorf("Kinds() lists %q, which no Kind constant declares", k)
		}
	}
	for k := range declared {
		if !seen[k] {
			t.Errorf("a Kind constant declares %q, which Kinds() does not list", k)
		}
	}
}
