package codeindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// The call graph is assembled from SCIP occurrences — the indexer's own
// facts, never our parsing (D-INDEX). A reference occurring inside the
// enclosing range of a definition is an edge from that definition to the
// referenced symbol. Security's source→sink reachability and
// maintainability's dead-code checks both walk these edges.

// Edge is one caller→callee fact, with where the call happens.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type scipIndex struct {
	Documents []scipGraphDoc `json:"documents"`
}

type scipGraphDoc struct {
	RelativePath string `json:"relative_path"`
	Occurrences  []struct {
		Range          []int  `json:"range"`
		Symbol         string `json:"symbol"`
		SymbolRoles    int    `json:"symbol_roles"`
		EnclosingRange []int  `json:"enclosing_range"`
	} `json:"occurrences"`
}

func loadScip(root string) (*scipIndex, error) {
	raw, err := os.ReadFile(filepath.Join(root, Dir, refsFile))
	if err != nil {
		return nil, fmt.Errorf("the precise tier is not built — run `procoder index build` with the SCIP tools installed (`procoder init`)")
	}
	var idx scipIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, fmt.Errorf("the precise index could not be read (%v) — rebuild it", err)
	}
	return &idx, nil
}

// shortSym trims a SCIP symbol to its readable tail: pkg/Name().
func shortSym(s string) string {
	if i := strings.Index(s, "`"); i >= 0 {
		s = s[i:]
	}
	return strings.TrimSuffix(strings.ReplaceAll(s, "`", ""), ".")
}

// Edges assembles the caller→callee graph from the precise tier.
func Edges(root string) ([]Edge, error) {
	idx, err := loadScip(root)
	if err != nil {
		return nil, err
	}
	var edges []Edge
	for _, doc := range idx.Documents {
		// definitions with a body range, ordered for containment lookup
		type def struct {
			sym        string
			start, end int
		}
		var defs []def
		for _, occ := range doc.Occurrences {
			if occ.SymbolRoles&1 != 0 && len(occ.EnclosingRange) >= 4 {
				defs = append(defs, def{occ.Symbol, occ.EnclosingRange[0], occ.EnclosingRange[2]})
			}
		}
		for _, occ := range doc.Occurrences {
			if occ.SymbolRoles&1 != 0 || len(occ.Range) < 3 {
				continue
			}
			line := occ.Range[0]
			// smallest enclosing definition wins (methods inside types)
			best := -1
			for i, d := range defs {
				if line >= d.start && line <= d.end {
					if best < 0 || d.end-d.start < defs[best].end-defs[best].start {
						best = i
					}
				}
			}
			if best < 0 || defs[best].sym == occ.Symbol {
				continue
			}
			edges = append(edges, Edge{From: defs[best].sym, To: occ.Symbol,
				File: doc.RelativePath, Line: line + 1})
		}
	}
	return edges, nil
}

// Callers prints who calls the symbol and what the symbol itself calls —
// the one-hop view; `graph` serves whole-graph consumers.
func Callers(root, symbol string, out func(string)) int {
	edges, err := Edges(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	callers, callees := 0, 0
	for _, e := range edges {
		if scipSymbolIs(e.To, symbol) {
			out(fmt.Sprintf("caller  %s:%d  %s  (precise)", e.File, e.Line, shortSym(e.From)))
			callers++
		}
	}
	for _, e := range edges {
		if scipSymbolIs(e.From, symbol) {
			out(fmt.Sprintf("callee  %s:%d  %s  (precise)", e.File, e.Line, shortSym(e.To)))
			callees++
		}
	}
	if callers+callees == 0 {
		out(fmt.Sprintf("no call edges for %q in the precise tier — the symbol may be unused, external, or the index stale", symbol))
		return 1
	}
	out(fmt.Sprintf("%d caller(s), %d callee edge(s)", callers, callees))
	if n := staleNote(root); n != "" {
		out(n)
	}
	return 0
}

// Graph dumps the full edge list as JSON — the machine-readable surface the
// security and maintainability domains consume for reachability walks.
func Graph(root string, out func(string)) int {
	edges, err := Edges(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	enc, err := json.Marshal(edges)
	if err != nil {
		out(err.Error())
		return 1
	}
	out(string(enc))
	return 0
}

// Unused lists symbols the precise tier defines but never references —
// dead-code candidates for the maintainability domain. Exported symbols are
// listed too, marked: a library's public API is legitimately unreferenced
// from inside, and only the agent can judge which is which.
func Unused(root string, out func(string)) int {
	idx, err := loadScip(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	type def struct {
		file string
		line int
	}
	defs := map[string]def{}
	refCount := map[string]int{}
	for _, doc := range idx.Documents {
		// documents outside the repo (Go build-cache artifacts scip-go also
		// visits) are not this repository's code
		outside := strings.HasPrefix(doc.RelativePath, "..") || filepath.IsAbs(doc.RelativePath)
		test := strings.Contains(doc.RelativePath, "_test.")
		for _, occ := range doc.Occurrences {
			if len(occ.Range) < 3 {
				continue
			}
			if occ.SymbolRoles&1 != 0 {
				callable := strings.HasSuffix(occ.Symbol, "().") || strings.HasSuffix(occ.Symbol, "#")
				runtimeCalled := strings.HasSuffix(occ.Symbol, "/init().") || strings.HasSuffix(occ.Symbol, "/main().")
				if !test && !outside && callable && !runtimeCalled {
					defs[occ.Symbol] = def{doc.RelativePath, occ.Range[0] + 1}
				}
			} else {
				refCount[occ.Symbol]++
			}
		}
	}
	var names []string
	for sym := range defs {
		if refCount[sym] == 0 {
			names = append(names, sym)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		return defs[names[i]].file+fmt.Sprint(defs[names[i]].line) < defs[names[j]].file+fmt.Sprint(defs[names[j]].line)
	})
	if len(names) == 0 {
		out("no unreferenced symbols in the precise tier")
		return 0
	}
	for _, sym := range names {
		d := defs[sym]
		tail := shortSym(sym)
		mark := ""
		if isExportedName(tail) {
			mark = "  [exported — may be public API]"
		}
		if strings.HasPrefix(tail, "main(") || strings.Contains(tail, "/main(") {
			continue // entry points are referenced by the runtime, not the code
		}
		out(fmt.Sprintf("%s:%d  %s%s", d.file, d.line, tail, mark))
	}
	out(fmt.Sprintf("%d unreferenced symbol(s) (precise) — dead-code candidates; the agent judges, exported API included", len(names)))
	if n := staleNote(root); n != "" {
		out(n)
	}
	return 0
}

func isExportedName(tail string) bool {
	// the symbol's own name is the segment after the last / or #
	name := tail
	for _, sep := range []string{"/", "#"} {
		if i := strings.LastIndex(name, sep); i >= 0 {
			name = name[i+1:]
		}
	}
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}

// Entrypoints lists where execution and data enter the program — the
// security domain's starting set: main functions and the exported surface.
// Heuristic over the broad tier, and labeled as such.
func Entrypoints(root string, out func(string)) int {
	tags, err := loadTags(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	count := 0
	for _, t := range tags {
		if t.Kind != "func" && t.Kind != "method" && t.Kind != "function" {
			continue
		}
		if strings.Contains(t.Path, "_test.") {
			continue
		}
		switch {
		case t.Name == "main":
			printTag(out, t)
			count++
		case isExportedName(t.Name):
			printTag(out, t)
			count++
		}
	}
	if count == 0 {
		out("no entry points found in the index")
		return 1
	}
	out(fmt.Sprintf("%d entry point(s): main functions and the exported surface (heuristic — the security domain refines this)", count))
	if n := staleNote(root); n != "" {
		out(n)
	}
	return 0
}
