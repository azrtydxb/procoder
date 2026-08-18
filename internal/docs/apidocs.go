package docs

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"procoder/internal/gitx"
)

// MissingAPIDocs reports exported/public symbols without a doc comment in the
// given source files — Go, Python, and TypeScript. Reported, never blocking:
// retrofitting a whole codebase would drown the agent, and what the harness
// reports is what the agent works on.
func MissingAPIDocs(files []string) []gitx.Finding {
	var out []gitx.Finding
	for _, f := range files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".go":
			if !strings.HasSuffix(f, "_test.go") {
				out = append(out, goAPIDocs(f)...)
			}
		case ".py":
			out = append(out, pyAPIDocs(f)...)
		case ".ts", ".tsx":
			if !strings.HasSuffix(f, ".d.ts") {
				out = append(out, tsAPIDocs(f)...)
			}
		}
	}
	return out
}

func goAPIDocs(file string) []gitx.Finding {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return nil // a file that does not parse is the compiler's finding, not ours
	}
	var out []gitx.Finding
	report := func(pos token.Pos, kind, name string) {
		out = append(out, gitx.Finding{File: file, Line: fset.Position(pos).Line,
			Message: fmt.Sprintf("exported %s %s has no doc comment", kind, name)})
	}
	for _, decl := range af.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() && d.Doc == nil && d.Recv == nil {
				report(d.Pos(), "function", d.Name.Name)
			}
		case *ast.GenDecl:
			if d.Doc != nil {
				continue
			}
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() && s.Doc == nil {
						report(s.Pos(), "type", s.Name.Name)
					}
				case *ast.ValueSpec:
					if s.Doc != nil {
						continue
					}
					for _, n := range s.Names {
						if n.IsExported() {
							report(n.Pos(), "value", n.Name)
						}
					}
				}
			}
		}
	}
	return out
}

// pyDef matches a top-level public def/class.
var pyDef = regexp.MustCompile(`^(def|class)\s+([A-Za-z][A-Za-z0-9_]*)`)

func pyAPIDocs(file string) []gitx.Finding {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var out []gitx.Finding
	for i, line := range lines {
		m := pyDef.FindStringSubmatch(line)
		if m == nil || strings.HasPrefix(m[2], "_") {
			continue
		}
		// find the first non-empty line after the (possibly multi-line) signature
		j := i
		for ; j < len(lines); j++ {
			if strings.HasSuffix(strings.TrimSpace(lines[j]), ":") {
				break
			}
		}
		doc := false
		for k := j + 1; k < len(lines) && k <= j+2; k++ {
			t := strings.TrimSpace(lines[k])
			if t == "" {
				continue
			}
			doc = strings.HasPrefix(t, `"""`) || strings.HasPrefix(t, "'''")
			break
		}
		if !doc {
			out = append(out, gitx.Finding{File: file, Line: i + 1,
				Message: fmt.Sprintf("public %s %s has no docstring", m[1], m[2])})
		}
	}
	return out
}

// tsExport matches an exported function/class/interface/type declaration.
var tsExport = regexp.MustCompile(`^export\s+(?:default\s+)?(?:abstract\s+)?(?:async\s+)?(function|class|interface|type|enum)\s+([A-Za-z][A-Za-z0-9_]*)`)

func tsAPIDocs(file string) []gitx.Finding {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var out []gitx.Finding
	for i, line := range lines {
		m := tsExport.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		doc := false
		for k := i - 1; k >= 0 && k >= i-2; k-- {
			t := strings.TrimSpace(lines[k])
			if t == "" {
				continue
			}
			doc = strings.HasSuffix(t, "*/") || strings.HasPrefix(t, "//")
			break
		}
		if !doc {
			out = append(out, gitx.Finding{File: file, Line: i + 1,
				Message: fmt.Sprintf("exported %s %s has no doc comment", m[1], m[2])})
		}
	}
	return out
}
