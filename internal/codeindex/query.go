package codeindex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"procoder/internal/store"
	"procoder/internal/textutil"
	"procoder/internal/tools"
)

// loadTags reads the broad tier. A missing index is an instruction, not an
// empty answer — the caller prints the error verbatim.
func loadTags(root string) ([]Tag, error) {
	f, err := store.OpenIn(root, Dir, tagsFile)
	if err != nil {
		return nil, fmt.Errorf("no index — run `procoder index build` first")
	}
	defer f.Close()
	var tags []Tag
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var t Tag
		if json.Unmarshal(sc.Bytes(), &t) == nil {
			tags = append(tags, t)
		}
	}
	if err := sc.Err(); err != nil {
		// a partial index answering as if whole is the lie the honesty rule
		// bans — better no answer with the reason than a wrong one
		return nil, fmt.Errorf("the index could not be read fully (%v) — run `procoder index build`", err)
	}
	return tags, nil
}

func loadMeta(root string) Meta {
	var m Meta
	raw, err := store.LoadIn(root, Dir, metaFile)
	if err == nil {
		json.Unmarshal(raw, &m)
	}
	return m
}

// staleNote is printed with every answer when the index no longer matches
// HEAD — a stale answer that looks current is the lie the honesty rule bans.
func staleNote(root string) string {
	m := loadMeta(root)
	if m.Commit == "" {
		return ""
	}
	if h := head(root); h != m.Commit {
		return fmt.Sprintf("note: index built at %s, HEAD is %s — run `procoder index build` to refresh", m.Commit, h)
	}
	return ""
}

func printTag(out func(string), t Tag) {
	sig := t.Signature
	if sig != "" {
		sig = " " + sig
	}
	scope := ""
	if t.Scope != "" {
		scope = "  [" + t.Scope + "]"
	}
	out(fmt.Sprintf("%s:%d  %s %s%s%s", t.Path, t.Line, t.Kind, t.Name, sig, scope))
}

// Find prints where symbol is defined. Exact match first; if nothing, it
// says so rather than guessing.
func Find(root, symbol string, out func(string)) int {
	tags, err := loadTags(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	hits := 0
	for _, t := range tags {
		if t.Name == symbol {
			printTag(out, t)
			hits++
		}
	}
	if hits == 0 {
		out(fmt.Sprintf("no definition of %q in the index — try `procoder index search %s`", symbol, symbol))
		return 1
	}
	if n := staleNote(root); n != "" {
		out(n)
	}
	return 0
}

const searchCap = 50

// Search is the fuzzy lookup: case-insensitive substring, exact and prefix
// matches ranked first, capped and saying so when capped.
func Search(root, query string, out func(string)) int {
	tags, err := loadTags(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	q := strings.ToLower(query)
	var exact, prefix, rest []Tag
	for _, t := range tags {
		n := strings.ToLower(t.Name)
		switch {
		case n == q:
			exact = append(exact, t)
		case strings.HasPrefix(n, q):
			prefix = append(prefix, t)
		case strings.Contains(n, q):
			rest = append(rest, t)
		}
	}
	hits := append(exact, append(prefix, rest...)...)
	if len(hits) == 0 {
		out(fmt.Sprintf("nothing matching %q in the index", query))
		return 1
	}
	shown := hits
	if len(shown) > searchCap {
		shown = shown[:searchCap]
	}
	for _, t := range shown {
		printTag(out, t)
	}
	if len(hits) > searchCap {
		out(fmt.Sprintf("… %d more matches not shown — narrow the query", len(hits)-searchCap))
	}
	if n := staleNote(root); n != "" {
		out(n)
	}
	return 0
}

// Outline prints a file's symbols in order — understanding a file without
// reading all of it.
func Outline(root, file string, out func(string)) int {
	tags, err := loadTags(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	rel := file
	if abs, err := filepath.Abs(file); err == nil {
		if r, err := filepath.Rel(root, abs); err == nil {
			rel = filepath.ToSlash(r)
		}
	}
	var hits []Tag
	for _, t := range tags {
		if t.Path == rel {
			hits = append(hits, t)
		}
	}
	if len(hits) == 0 {
		out(fmt.Sprintf("no symbols for %s in the index (not indexed, or the file has none)", rel))
		return 1
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Line < hits[j].Line })
	for _, t := range hits {
		printTag(out, t)
	}
	if n := staleNote(root); n != "" {
		out(n)
	}
	return 0
}

// scipDoc mirrors the fields of `scip print --json` this package reads.
type scipDoc struct {
	RelativePath string `json:"relative_path"`
	Occurrences  []struct {
		Range       []int  `json:"range"`
		Symbol      string `json:"symbol"`
		SymbolRoles int    `json:"symbol_roles"`
	} `json:"occurrences"`
}

// Refs prints the definition and every reference. Precise (SCIP) when the
// precise tier is built; textual (git grep) otherwise — and the output names
// which tier answered, always.
func Refs(root, symbol string, out func(string)) int {
	if code, ok := preciseRefs(root, symbol, out); ok {
		if n := staleNote(root); n != "" {
			out(n)
		}
		return code
	}
	return textualRefs(root, symbol, out)
}

// preciseRefs answers from the SCIP JSON when it exists and knows the symbol.
func preciseRefs(root, symbol string, out func(string)) (int, bool) {
	raw, err := store.LoadIn(root, Dir, refsFile)
	if err != nil {
		return 0, false
	}
	var idx struct {
		Documents []scipDoc `json:"documents"`
	}
	if json.Unmarshal(raw, &idx) != nil {
		return 0, false
	}
	found := 0
	for _, doc := range idx.Documents {
		for _, occ := range doc.Occurrences {
			if len(occ.Range) < 3 || !scipSymbolIs(occ.Symbol, symbol) {
				continue
			}
			role := "ref"
			if occ.SymbolRoles&1 != 0 {
				role = "def"
			}
			out(fmt.Sprintf("%s:%d  %s  (precise)", doc.RelativePath, occ.Range[0]+1, role))
			found++
		}
	}
	if found == 0 {
		return 0, false // unknown to the precise tier; fall back rather than answer "nothing"
	}
	return 0, true
}

// scipSymbolIs reports whether a SCIP symbol string's last descriptor names
// the queried symbol: `…/Name().` for functions, `…/Name#` for types,
// `…/Name.` for values, `…Type#Name().` for methods.
func scipSymbolIs(scipSym, name string) bool {
	for _, sep := range []string{"/", "#"} {
		for _, suffix := range []string{"().", "#", "."} {
			if strings.HasSuffix(scipSym, sep+name+suffix) {
				return true
			}
		}
	}
	return false
}

// textualRefs is the labeled fallback: word-boundary git grep.
func textualRefs(root, symbol string, out func(string)) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "grep", "--untracked", "-I", "-nw", "--", symbol)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	raw, err := cmd.Output()
	if err != nil {
		// git grep exits 1 for "no matches"; anything else is a failure and
		// must never read as "the symbol is unused"
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 && ctx.Err() == nil {
			out(fmt.Sprintf("no references to %q found (textual search)", symbol))
			return 1
		}
		out(fmt.Sprintf("textual search FAILED — references were NOT checked: %s", textutil.FirstLine(errb.String()+err.Error())))
		return 2
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	for _, l := range lines {
		out(l + "  (textual)")
	}
	out(fmt.Sprintf("%d textual match(es) — build the precise tier (`procoder index build` with the SCIP tools installed) for exact references", len(lines)))
	return 0
}

// tooCommon is the file count above which a symbol name is treated as too
// generic for textual impact matching.
const tooCommon = 15

// Impact answers "what does this change touch": the symbols defined in the
// changed files, and which other files refer to them. Textual matching,
// labeled as such — a blast-radius estimate, not a proof.
func Impact(root string, changed []string, out func(string)) int {
	tags, err := loadTags(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	changedSet := relSet(root, changed)
	if len(changedSet) == 0 {
		out("no changed files — nothing to assess")
		return 0
	}
	symbols := changedSymbols(tags, changedSet)
	if len(symbols) == 0 {
		out("the changed files define no indexed symbols")
		return 0
	}
	affected := affectedFiles(root, symbols, changedSet)
	printImpact(out, len(changedSet), len(symbols), affected)
	return 0
}

func relSet(root string, files []string) map[string]bool {
	set := map[string]bool{}
	for _, f := range files {
		if rel, err := filepath.Rel(root, f); err == nil {
			set[filepath.ToSlash(rel)] = true
		}
	}
	return set
}

// changedSymbols keeps the definition-shaped tags living in changed files.
func changedSymbols(tags []Tag, changedSet map[string]bool) []Tag {
	defKinds := map[string]bool{"func": true, "method": true, "type": true,
		"class": true, "interface": true, "struct": true}
	var symbols []Tag
	for _, t := range tags {
		if changedSet[t.Path] && defKinds[t.Kind] {
			symbols = append(symbols, t)
		}
	}
	return symbols
}

// affectedFiles maps each other file to the changed symbols it mentions —
// word-boundary git grep, with too-generic names skipped as noise.
func affectedFiles(root string, symbols []Tag, changedSet map[string]bool) map[string]map[string]bool {
	affected := map[string]map[string]bool{}
	for _, s := range symbols {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, "git", "-C", root, "grep", "--untracked", "-I", "-lw", "--", s.Name)
		raw, err := cmd.Output()
		cancel()
		if err != nil {
			continue
		}
		mentions := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		if len(mentions) > tooCommon {
			continue // a name matched all over the repo is noise, not signal
		}
		for _, f := range mentions {
			if f == "" || changedSet[f] {
				continue
			}
			if affected[f] == nil {
				affected[f] = map[string]bool{}
			}
			affected[f][s.Name] = true
		}
	}
	return affected
}

func printImpact(out func(string), changedCount, symbolCount int, affected map[string]map[string]bool) {
	out(fmt.Sprintf("%d changed file(s) define %d symbol(s)", changedCount, symbolCount))
	if len(affected) == 0 {
		out("no other file references them (textual) — the blast radius is the change itself")
		return
	}
	files := make([]string, 0, len(affected))
	for f := range affected {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		syms := make([]string, 0, len(affected[f]))
		for s := range affected[f] {
			syms = append(syms, s)
		}
		sort.Strings(syms)
		out(fmt.Sprintf("  %s  ← %s", f, strings.Join(syms, ", ")))
	}
	out(fmt.Sprintf("%d file(s) reference the changed symbols (textual) — verify them before finishing", len(affected)))
}

// Stats prints what the index knows and how fresh it is.
func Stats(root string, out func(string)) int {
	tags, err := loadTags(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	m := loadMeta(root)
	langs := map[string]int{}
	for _, t := range tags {
		l := t.Language
		if l == "" {
			l = "unknown"
		}
		langs[l]++
	}
	names := make([]string, 0, len(langs))
	for l := range langs {
		names = append(names, l)
	}
	sort.Strings(names)
	out(fmt.Sprintf("index built at commit %s (%s): %d files, %d symbols", m.Commit, m.BuiltAt, m.Files, m.Tags))
	for _, l := range names {
		out(fmt.Sprintf("  %-14s %d symbols", l, langs[l]))
	}
	if m.Precise {
		out("precise tier: built (refs are exact)")
	} else {
		out("precise tier: not built — refs are textual; install the SCIP tools and rebuild")
	}
	if n := staleNote(root); n != "" {
		out(n)
	}
	return 0
}

// Refresh re-indexes a single file in the broad tier — the write hook's way
// of keeping the index current without full rebuilds. It only acts when an
// index already exists: the hook maintains state the user asked for, it
// never creates it uninvited.
func Refresh(root, file string) {
	if _, err := os.Stat(filepath.Join(root, Dir, tagsFile)); err != nil {
		return
	}
	rel, err := filepath.Rel(root, file)
	if err != nil || skipExts[strings.ToLower(filepath.Ext(file))] {
		return
	}
	rel = filepath.ToSlash(rel)
	bin := tools.Resolve(Ctags, root)
	if bin == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, // nosemgrep -- resolved from the fixed tool table, never user input
		"--output-format=json", "--fields=+nSl", "-o", "-", rel)
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		return
	}
	fresh, _ := normalizeTags(raw)

	old, err := store.LoadIn(root, Dir, tagsFile)
	if err != nil {
		return
	}
	var buf strings.Builder
	sc := bufio.NewScanner(strings.NewReader(string(old)))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	needle := `"path":"` + rel + `"`
	for sc.Scan() {
		if !strings.Contains(sc.Text(), needle) {
			buf.WriteString(sc.Text())
			buf.WriteByte('\n')
		}
	}
	if sc.Err() != nil {
		return // leave the index untouched rather than write a truncated one
	}
	buf.Write(fresh)
	// The atomic swap this used to do by hand is now the store's, which
	// also cleans up its own temp file on failure and takes the index's
	// lock — so a Refresh firing from the write hook can no longer race an
	// `index build` running in another session.
	//
	// The failure stays silent: Refresh is fire-and-forget from the write
	// hook, the index is then stale, and staleNote is what says so.
	_ = store.SaveIn(root, Dir, tagsFile, []byte(buf.String()))
}
