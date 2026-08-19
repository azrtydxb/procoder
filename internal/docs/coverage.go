package docs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"procoder/internal/codeindex"
	"procoder/internal/gitx"
)

// Commands is the canonical list of procoder commands. cmd/procoder pins its
// usage text against this list by test, so the two cannot drift. It is
// procoder's own inventory, not a rule any other repository is held to — no
// check reads it.
var Commands = []string{
	"adr", "agents", "audit", "backlog", "bench", "check", "ci", "debt",
	"deps", "docs", "doctor", "env",
	"format", "git", "hook", "index", "infra", "init", "lessons", "lint",
	"maintain", "plan", "principles", "release", "run", "scrub", "security", "spec",
	"sprint", "status", "templates", "test", "todo", "version",
}

// surfaceCoverageCap is how many undocumented symbols are worth listing; past
// twenty it is a project decision, not a checklist.
const surfaceCoverageCap = 20

// SurfaceCoverage reports the repository's own exported surface that no
// documentation file mentions at all — correctness checks cannot see absence,
// so completeness gets its own check. Universal: the surface comes from the
// repository's index, never from a list compiled into this binary, and the
// check is not gated on which repository it runs in.
//
// Never blocking: what deserves a document is the project's call. Ranked with
// entry points first, because what a reader reaches first is what an absent
// document costs most.
func SurfaceCoverage(root string) []gitx.Finding {
	tags, err := loadIndexTags(root)
	if err != nil {
		return []gitx.Finding{{
			Message: "public-surface coverage NOT computed — no index; run `procoder index build`: " + err.Error()}}
	}
	mentioned := documentedWords(root)
	type sym struct {
		name, kind, path string
		rank             int
	}
	seen := map[string]bool{}
	var missing []sym
	for _, t := range tags {
		if !surfaceKinds[t.Kind] || !isExportedSymbol(t.Name) || seen[t.Name] {
			continue
		}
		path := filepath.ToSlash(t.Path)
		if strings.Contains(path, "_test.") || languageOf(path) == "" {
			continue
		}
		seen[t.Name] = true
		if mentioned[t.Name] {
			continue
		}
		rank := 1
		if entrypointKinds[t.Kind] {
			rank = 0 // the index's own entry-point definition: main and the exported callables
		}
		if isInternalPath(path) {
			// a directory called internal (or private) says the project does
			// not consider this its surface — reported, but last
			rank += 2
		}
		missing = append(missing, sym{name: t.Name, kind: t.Kind, path: path, rank: rank})
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].rank != missing[j].rank {
			return missing[i].rank < missing[j].rank
		}
		return missing[i].name < missing[j].name
	})
	var out []gitx.Finding
	for i, m := range missing {
		if i == surfaceCoverageCap {
			out = append(out, gitx.Finding{
				Message: fmt.Sprintf("… %d more exported symbol(s) no documentation file mentions (docs)", len(missing)-surfaceCoverageCap)})
			break
		}
		out = append(out, gitx.Finding{File: filepath.Join(root, filepath.FromSlash(m.path)),
			Message: fmt.Sprintf("documentation never mentions exported %s %s (%s) — surface a reader cannot discover (docs)", m.kind, m.name, m.path)})
	}
	return out
}

// surfaceKinds are the index kinds this package can also read out of source,
// so the two sides of a comparison mean the same thing. Struct members and
// package tags are deliberately out: they are not the surface a document
// introduces, and the source reader does not see them either.
var surfaceKinds = map[string]bool{
	"func": true, "function": true, "method": true,
	"var": true, "variable": true, "const": true, "constant": true,
	"type": true, "typedef": true, "struct": true, "interface": true,
	"class": true, "enum": true, "trait": true, "union": true,
}

// entrypointKinds mirror `procoder index entrypoints`: execution enters
// through callables, so those rank first.
var entrypointKinds = map[string]bool{"func": true, "function": true, "method": true}

// documentedWords is every identifier-shaped word the documentation corpus
// contains. Word membership, not substring: "specific" must not document
// "spec".
func documentedWords(root string) map[string]bool {
	out := map[string]bool{}
	for _, md := range MarkdownFiles(root) {
		data, err := os.ReadFile(md)
		if err != nil {
			continue
		}
		for _, w := range strings.FieldsFunc(string(data), func(r rune) bool {
			return !(r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
		}) {
			out[w] = true
		}
	}
	return out
}

// indexTagsFile is the broad tier's file inside codeindex.Dir; this package
// reads it the way `maintain` consumes the index — the index owns building it.
const indexTagsFile = "tags.jsonl"

func loadIndexTags(root string) ([]codeindex.Tag, error) {
	f, err := os.Open(filepath.Join(root, codeindex.Dir, indexTagsFile))
	if err != nil {
		return nil, fmt.Errorf("the index has not been built")
	}
	defer f.Close()
	var tags []codeindex.Tag
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var t codeindex.Tag
		if json.Unmarshal(sc.Bytes(), &t) == nil {
			tags = append(tags, t)
		}
	}
	if err := sc.Err(); err != nil {
		// a partial index answering as if whole is the lie the honesty rule
		// bans — no answer with the reason beats a wrong one
		return nil, fmt.Errorf("the index could not be read fully (%v)", err)
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("the index is empty")
	}
	return tags, nil
}

// indexedSymbols is the index's exported surface grouped by file — the "was"
// side of the public-surface comparison.
func indexedSymbols(root string) (map[string]map[string]bool, error) {
	tags, err := loadIndexTags(root)
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]bool{}
	for _, t := range tags {
		path := filepath.ToSlash(t.Path)
		if _, ok := out[path]; !ok {
			out[path] = map[string]bool{}
		}
		if surfaceKinds[t.Kind] && isExportedSymbol(t.Name) {
			out[path][t.Name] = true
		}
	}
	return out, nil
}
