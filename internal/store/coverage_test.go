package store

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
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

// procoderPathIdents are the identifiers that hold a .procoder/ path. An
// os.* call naming one of these is reaching past the store.
//
// Written out rather than inferred: inferring it means guessing which `Dir`
// a call meant, and a guard that guesses is a guard that fails open.
var procoderPathIdents = []string{
	"RulesPath", "CopilotLeaksPath", "MermaidConfigPath", "PerspectiveDir",
	"StateDir", "DecisionsFile", "codeindex.Dir", "spec.Dir",
}

// ioFuncs are the calls that read or write a file's contents. os.Stat and
// os.Getwd are absent deliberately: asking whether something exists is not
// reaching past the store, and gate adoption legitimately stats .procoder/.
var ioFuncs = map[string]bool{
	"ReadFile": true, "WriteFile": true, "Create": true, "CreateTemp": true,
	"OpenFile": true, "Open": true, "ReadDir": true, "Rename": true,
}

// proved by: reintroducing a direct os.ReadFile on a .procoder/ path
// anywhere outside this package fails this, which is what stops the seam
// eroding one convenient call at a time.
func TestNoDirectProcoderFileIO(t *testing.T) {
	fset := token.NewFileSet()
	for _, p := range goFiles(t) {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		// The package's own .procoder path constants, by name.
		local := map[string]bool{}
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
				if v, err := strconv.Unquote(bl.Value); err == nil && strings.HasPrefix(v, ".procoder") {
					local[name.Name] = true
				}
			}
			return true
		})

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name != "os" || !ioFuncs[sel.Sel.Name] {
				return true
			}
			var b strings.Builder
			if err := printer.Fprint(&b, fset, call); err != nil {
				return true
			}
			text := b.String()

			// `".procoder/` or exactly `".procoder"` — NOT `.procoder-`,
			// which is how the self-upgrade names a temp directory and has
			// nothing to do with the store.
			hit := strings.Contains(text, `".procoder/`) || strings.Contains(text, `".procoder"`)
			for name := range local {
				if !hit && containsIdent(text, name) {
					hit = true
				}
			}
			for _, name := range procoderPathIdents {
				if !hit && containsIdent(text, name) {
					hit = true
				}
			}
			if hit {
				t.Errorf("%s:%d reaches past the store: %s",
					p, fset.Position(call.Pos()).Line, strings.Join(strings.Fields(text), " "))
			}
			return true
		})
	}
}

// containsIdent reports whether text uses name as a whole identifier, so
// "Dir" does not match "SkipDir".
func containsIdent(text, name string) bool {
	for i := 0; ; {
		j := strings.Index(text[i:], name)
		if j < 0 {
			return false
		}
		j += i
		before := byte(' ')
		if j > 0 {
			before = text[j-1]
		}
		after := byte(' ')
		if j+len(name) < len(text) {
			after = text[j+len(name)]
		}
		if !isIdentByte(before) && !isIdentByte(after) {
			return true
		}
		i = j + len(name)
	}
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '.' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
