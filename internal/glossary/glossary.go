// Package glossary is the project's shared vocabulary: what this team
// calls things, which is not always what the code calls them.
//
// It exists because the same idea gets described three different ways
// across three specs, each one re-explaining it, and the naming drifts
// apart as it goes. A term everybody already agreed on is shorter to write
// and unambiguous to read (#195).
//
// Deliberately not an ADR and deliberately not blocking. An ADR is a
// decision with reasoning; this is vocabulary. And a glossary that stops
// work over a wording disagreement is worse than no glossary — so every
// finding here reports and none of them refuse.
package glossary

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Path is where the glossary lives, beside the other things a repository
// tells procoder about itself.
const Path = ".procoder/context.md"

// Term is one entry: the word the team uses, and what it means here.
type Term struct {
	Name       string
	Definition string
	Line       int
}

// Load reads the glossary. A repository without one has no vocabulary
// recorded yet, which is the ordinary case and not a problem.
func Load(root string) []Term {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Path)))
	if err != nil {
		return nil
	}
	var out []Term
	var current *Term
	for i, line := range strings.Split(normaliseEOL(string(raw)), "\n") {
		if heading := strings.TrimPrefix(line, "## "); heading != line {
			if current != nil {
				current.Definition = strings.TrimSpace(current.Definition)
				out = append(out, *current)
			}
			current = &Term{Name: strings.TrimSpace(heading), Line: i + 1}
			continue
		}
		if current != nil {
			current.Definition += line + "\n"
		}
	}
	if current != nil {
		current.Definition = strings.TrimSpace(current.Definition)
		out = append(out, *current)
	}
	return out
}

func normaliseEOL(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// Check reports what is wrong with the glossary itself: an entry with no
// definition, and two entries that are the same term written differently.
//
// Both report. Neither refuses — see the package comment.
func Check(root string, out func(string)) int {
	terms := Load(root)
	if len(terms) == 0 {
		out("no glossary — " + Path + " does not exist or records nothing")
		return 0
	}
	var problems []string
	seen := map[string]Term{}
	for _, t := range terms {
		if t.Name == "" {
			problems = append(problems, fmt.Sprintf("line %d: a heading with no term", t.Line))
			continue
		}
		if strings.TrimSpace(t.Definition) == "" {
			problems = append(problems, fmt.Sprintf("line %d: %q has no definition — a term nobody defined is one everybody will define differently", t.Line, t.Name))
		}
		key := normalise(t.Name)
		if first, ok := seen[key]; ok {
			problems = append(problems, fmt.Sprintf("line %d: %q is line %d's %q written differently — one term, one entry", t.Line, t.Name, first.Line, first.Name))
			continue
		}
		seen[key] = t
	}
	for _, t := range terms {
		out(fmt.Sprintf("  %s", t.Name))
	}
	if len(problems) == 0 {
		out(fmt.Sprintf("%d term(s), all defined", len(terms)))
		return 0
	}
	for _, p := range problems {
		out("  " + p)
	}
	out(fmt.Sprintf("%d term(s), %d problem(s) — reported, never blocking", len(terms), len(problems)))
	return 0
}

// List prints the vocabulary.
func List(root string, out func(string)) int {
	terms := Load(root)
	if len(terms) == 0 {
		out("no glossary — write " + Path + " with a `## <term>` section per entry")
		return 0
	}
	sort.Slice(terms, func(i, j int) bool { return terms[i].Name < terms[j].Name })
	for _, t := range terms {
		out(t.Name + " — " + firstLine(t.Definition))
	}
	return 0
}

// Add PRINTS the entry to write. The binary does not touch the file:
// P-CONTROL, the same as `adr new` and every other authoring command here.
func Add(root, term, definition string, out func(string)) int {
	if strings.TrimSpace(term) == "" {
		out("a glossary entry needs a term")
		return 2
	}
	if strings.TrimSpace(definition) == "" {
		out("a glossary entry needs a definition — a term nobody defined is one everybody will define differently")
		return 2
	}
	for _, existing := range Load(root) {
		if normalise(existing.Name) == normalise(term) {
			out(fmt.Sprintf("%q is already defined at %s:%d — edit that entry rather than adding a second", existing.Name, Path, existing.Line))
			return 2
		}
	}
	out("== append this to " + Path + ":")
	out("")
	out("## " + strings.TrimSpace(term))
	out("")
	out(strings.TrimSpace(definition))
	return 0
}

// Near reports glossary terms a piece of prose seems to be reinventing:
// close to an existing term without using it.
//
// Conservative on purpose, and reporting rather than blocking. The cost of
// a miss is one duplicated word; the cost of firing on every document is a
// check people stop reading.
func Near(terms []Term, prose string) []Term {
	lower := strings.ToLower(prose)
	var out []Term
	for _, t := range terms {
		name := normalise(t.Name)
		if name == "" || len(name) < 4 {
			// A short term matches too much to be evidence of anything.
			continue
		}
		if strings.Contains(lower, strings.ToLower(t.Name)) {
			continue // the prose uses the term, which is the point
		}
		if singular := strings.TrimSuffix(name, "s"); singular != name && strings.Contains(normalise(prose), singular) {
			out = append(out, t)
			continue
		}
		if strings.Contains(normalise(prose), name) {
			out = append(out, t)
		}
	}
	return out
}

// normalise makes two spellings of one term comparable: case, spacing and
// the punctuation people vary without meaning anything by it.
func normalise(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
