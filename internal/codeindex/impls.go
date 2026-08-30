package codeindex

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"procoder/internal/store"
)

// implsDoc mirrors the SCIP fields Impls reads: the per-document symbol
// table carries the implementation relationships, the occurrences carry
// where each implementing symbol is defined.
type implsDoc struct {
	RelativePath string `json:"relative_path"`
	Occurrences  []struct {
		Range       []int  `json:"range"`
		Symbol      string `json:"symbol"`
		SymbolRoles int    `json:"symbol_roles"`
	} `json:"occurrences"`
	Symbols []struct {
		Symbol        string `json:"symbol"`
		Relationships []struct {
			Symbol           string `json:"symbol"`
			IsImplementation bool   `json:"is_implementation"`
		} `json:"relationships"`
	} `json:"symbols"`
}

// Impls prints what implements the named interface (or interface method).
// This is a precise-tier-only answer: implementation relationships exist
// nowhere else, so without SCIP the honest response is "not built", never
// a textual guess.
func Impls(root, symbol string, out func(string)) int {
	raw, err := store.LoadIn(root, Dir, refsFile)
	if err != nil {
		out("the precise tier is not built — implementations need SCIP; install the SCIP tools and run `procoder index build`")
		return 1
	}
	var idx struct {
		Documents []implsDoc `json:"documents"`
	}
	if json.Unmarshal(raw, &idx) != nil {
		out("the precise tier could not be read — run `procoder index build`")
		return 1
	}
	// pass 1: which SCIP symbols declare an implementation of the query
	implementing := map[string]bool{}
	for _, doc := range idx.Documents {
		for _, s := range doc.Symbols {
			for _, rel := range s.Relationships {
				if rel.IsImplementation && scipSymbolIs(rel.Symbol, symbol) {
					implementing[s.Symbol] = true
				}
			}
		}
	}
	if len(implementing) == 0 {
		out(fmt.Sprintf("no implementations of %q in the precise tier (unknown symbol, or the language's indexer emits no implementation relationships)", symbol))
		return 1
	}
	// pass 2: where each implementing symbol is defined
	var lines []string
	for _, doc := range idx.Documents {
		for _, occ := range doc.Occurrences {
			if len(occ.Range) < 3 || occ.SymbolRoles&1 == 0 || !implementing[occ.Symbol] {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s:%d  impl  %s", doc.RelativePath, occ.Range[0]+1, scipTail(occ.Symbol)))
		}
	}
	if len(lines) == 0 {
		out(fmt.Sprintf("%d implementation(s) of %q are known but defined outside the indexed tree (dependencies)", len(implementing), symbol))
		return 1
	}
	sort.Strings(lines)
	for _, l := range lines {
		out(l)
	}
	if n := staleNote(root); n != "" {
		out(n)
	}
	return 0
}

// scipTail reduces a SCIP symbol string to its readable member part:
// "scip-go gomod m v `pkg/path`/Type#Method()." -> "pkg/path Type#Method()."
func scipTail(sym string) string {
	if i := strings.Index(sym, "`"); i >= 0 {
		if j := strings.LastIndex(sym, "`"); j > i {
			return sym[i+1:j] + " " + strings.TrimPrefix(sym[j+1:], "/")
		}
	}
	if i := strings.LastIndex(sym, "/"); i >= 0 {
		return sym[i+1:]
	}
	return sym
}
