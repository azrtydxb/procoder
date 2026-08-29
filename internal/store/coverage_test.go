package store

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// treeRoot is this package's directory two levels up: the repository.
const treeRoot = "../.."

// knownPaths is every .procoder/ path literal the tree is allowed to carry,
// each one served by this package.
//
// It is a list somebody has to edit, on purpose. A new .procoder/ path is a
// new thing procoder owns, and it should not be possible to add one without
// deciding which store operation serves it — which is a decision, not a
// detail. The test below fails when a literal appears that is not here.
var knownPaths = map[string]string{
	".procoder":                                 "the root marker (gate adoption; stat only)",
	".procoder/":                                "the root prefix (docs obligation; prefix test only)",
	".procoder-upgrade-*":                       "a temp directory name, not a path under .procoder/",
	".procoder/ is here":                        "prose, not a path",
	".procoder/PRINCIPLES.md":                   "LoadDoc",
	".procoder/adr":                             "ListDir, LoadIn",
	".procoder/analysis":                        "ListDir, LoadDoc via Rel",
	".procoder/ask":                             "LoadIn, SaveDoc via Rel",
	".procoder/backlog":                         "ListDir, LoadIn",
	".procoder/backlog/epics":                   "ListDir",
	".procoder/backlog/sprints":                 "ListDir",
	".procoder/bench":                           "LoadIn, SaveIn",
	".procoder/config.toml":                     "LoadDoc",
	".procoder/config.toml:%d":                  "a message format, not a path",
	".procoder/config.toml:%d — %s (%s)":        "a message format, not a path",
	".procoder/context.md":                      "LoadDoc",
	".procoder/docs/RULES.md":                   "LoadDoc",
	".procoder/docs/mermaid.json":               "passed to mmdc as a path; stat only",
	".procoder/github/COMMIT_TEMPLATE.md":       "LoadDoc",
	".procoder/github/COPILOT-LEAKS.md":         "LoadDoc, SaveDoc",
	".procoder/github/LESSONS.md":               "LoadDoc",
	".procoder/github/PULL_REQUEST_TEMPLATE.md": "LoadDoc",
	".procoder/github/REVIEW.md":                "LoadIn",
	".procoder/github/WORKFLOW.md":              "LoadDoc",
	".procoder/index":                           "ListDir, LoadIn, SaveIn, OpenIn",
	".procoder/index/":                          "a gitignore prefix, not a path",
	".procoder/lint/RULES.md":                   "LoadDoc",
	".procoder/plans":                           "ListDir, LoadDoc via Rel",
	".procoder/review/lenses":                   "LoadIn",
	".procoder/review/perspectives":             "LoadIn",
	".procoder/security/RULES.md":               "LoadDoc",
	".procoder/specs":                           "ListDir, LoadIn, LoadDoc via Rel",
	".procoder/state":                           "the state directory (markers)",
	".procoder/state/":                          "a gitignore prefix, not a path",
	".procoder/state/claims.json":               "LoadClaims, SaveClaims",
	".procoder/state/dispatch.json":             "LoadDispatch, SaveDispatch",
	".procoder/state/env.json":                  "LoadEnvState, SaveEnvState",
	".procoder/templates":                       "LoadIn",
	".procoder/todo":                            "ListDir, LoadIn, LoadDoc/SaveDoc via Rel",
	".procoder/wizards":                         "ListDir, LoadIn",
}

func goFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(treeRoot, dir), func(p string, d fs.DirEntry, err error) error {
			switch {
			case err != nil, d.IsDir(), !strings.HasSuffix(p, ".go"), strings.HasSuffix(p, "_test.go"):
				return nil
			}
			if strings.Contains(filepath.ToSlash(p), "/internal/store/") {
				return nil
			}
			out = append(out, p)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(out)
	return out
}

// proved by: adding a new .procoder/ path anywhere in the tree without
// deciding which store operation serves it fails this.
func TestStoreCoversEveryPathConstant(t *testing.T) {
	fset := token.NewFileSet()
	for _, p := range goFiles(t) {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				return true
			}
			v, err := strconv.Unquote(bl.Value)
			if err != nil || !strings.HasPrefix(v, ".procoder") {
				return true
			}
			if _, known := knownPaths[v]; !known {
				t.Errorf("%s:%d path %q has no store pair — add it to knownPaths in this file and say which operation serves it",
					p, fset.Position(bl.Pos()).Line, v)
			}
			return true
		})
	}
}

// procoderPathIdents are identifiers from OTHER packages that hold a
// .procoder/ path. Within a package the constants are discovered; across
// one they are named here, because guessing which `Dir` a qualified
// selector meant is how a guard starts failing open.
var procoderPathIdents = []string{
	"codeindex.Dir", "spec.Dir", "ask.Dir", "answers.Dir",
}

// ioFuncs are the calls that read, write, replace or remove a file.
//
// os.Stat and os.Lstat are absent deliberately: asking whether something
// exists is not reaching past the store, and gate adoption legitimately
// stats .procoder/ to decide whether a repository opted in. Removal and
// mode changes ARE here — a delete past the store is as much a write as a
// write is.
var ioFuncs = map[string]bool{
	"ReadFile": true, "WriteFile": true, "Create": true, "CreateTemp": true,
	"OpenFile": true, "Open": true, "ReadDir": true, "Rename": true,
	"Remove": true, "RemoveAll": true, "Truncate": true, "Chmod": true,
	"Symlink": true, "Link": true,
}

// packageProcoderConsts collects the .procoder/ path constants declared
// anywhere in a package.
//
// Per PACKAGE, not per file: Go constants are package-scoped, and a guard
// that only looked in the violating file missed a call in todo.go using a
// Dir declared three lines up in the same file's package but a different
// file. That bypass was found by review, not by the guard.
func packageProcoderConsts(t *testing.T, dir string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			vs, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				bl, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || bl.Kind != token.STRING {
					continue
				}
				if v, err := strconv.Unquote(bl.Value); err == nil && isProcoderPath(v) {
					out[name.Name] = true
				}
			}
			return true
		})
	}
	return out
}

// isProcoderPath matches `.procoder` and `.procoder/...` and NOT
// `.procoder-...`, which is how the self-upgrade names a temp directory
// and has nothing to do with the store.
func isProcoderPath(v string) bool {
	return v == ".procoder" || strings.HasPrefix(v, ".procoder/")
}

// proved by: reintroducing a direct os.ReadFile on a .procoder/ path
// anywhere outside this package fails this, which is what stops the seam
// eroding one convenient call at a time.
func TestNoDirectProcoderFileIO(t *testing.T) {
	fset := token.NewFileSet()
	byDir := map[string][]string{}
	for _, p := range goFiles(t) {
		d := filepath.Dir(p)
		byDir[d] = append(byDir[d], p)
	}
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		consts := packageProcoderConsts(t, dir)
		for _, p := range byDir[dir] {
			f, err := parser.ParseFile(fset, p, nil, 0)
			if err != nil {
				t.Fatalf("%s: %v", p, err)
			}
			checkFile(t, fset, p, f, consts)
		}
	}
}

// checkFile flags every IO call in one file that names a .procoder/ path,
// directly or through a variable that was assigned one.
func checkFile(t *testing.T, fset *token.FileSet, path string, f *ast.File, consts map[string]bool) {
	t.Helper()
	render := func(n ast.Node) string {
		var b strings.Builder
		if printer.Fprint(&b, fset, n) != nil {
			return ""
		}
		return b.String()
	}
	// names a .procoder path, either as a literal or through a constant
	// this package or another one declares as one.
	names := func(text string) bool {
		if procoderLiteral.MatchString(text) {
			return true
		}
		for c := range consts {
			if identRe(c).MatchString(text) {
				return true
			}
		}
		for _, c := range procoderPathIdents {
			if identRe(c).MatchString(text) {
				return true
			}
		}
		return false
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		// Variables holding a .procoder path. Without this the guard is
		// bypassed by one line — `p := filepath.Join(root, Dir, name)`
		// followed by `os.ReadFile(p)` — which review demonstrated.
		tainted := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			var lhs []ast.Expr
			var rhs []ast.Expr
			switch a := n.(type) {
			case *ast.AssignStmt:
				lhs, rhs = a.Lhs, a.Rhs
			case *ast.ValueSpec:
				for _, id := range a.Names {
					lhs = append(lhs, id)
				}
				rhs = a.Values
			default:
				return true
			}
			for i, l := range lhs {
				id, ok := l.(*ast.Ident)
				if !ok || i >= len(rhs) {
					continue
				}
				text := render(rhs[i])
				if names(text) {
					tainted[id.Name] = true
				}
			}
			return true
		})

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "os" || !ioFuncs[sel.Sel.Name] {
				return true
			}
			text := render(call)
			hit := names(text)
			for v := range tainted {
				if !hit && identRe(v).MatchString(text) {
					hit = true
				}
			}
			if hit {
				t.Errorf("%s:%d reaches past the store: %s",
					path, fset.Position(call.Pos()).Line, strings.Join(strings.Fields(text), " "))
			}
			return true
		})
	}
}

// procoderLiteral matches a `".procoder"` or `".procoder/..."` string in
// rendered source, and not `".procoder-..."`.
var procoderLiteral = regexp.MustCompile(`"\.procoder(/[^"]*)?"`)

// identRe matches name as a whole identifier, so "Dir" does not match
// "SkipDir" and "spec.Dir" does not match "myspec.Dir".
func identRe(name string) *regexp.Regexp {
	if re, ok := identCache[name]; ok {
		return re
	}
	re := regexp.MustCompile(`(^|[^\w.])` + regexp.QuoteMeta(name) + `($|[^\w.])`)
	identCache[name] = re
	return re
}

var identCache = map[string]*regexp.Regexp{}
