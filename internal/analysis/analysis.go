// Package analysis is the phase before the spec.
//
// `spec check` has always judged whether a document is complete, never
// whether the idea in it is good — it will pass a thoroughly filled-in
// specification for the wrong feature, and nothing in the tree helped a
// person get from a notion to a spec worth checking. This is where that
// happens.
//
// It is deliberately not a tollgate. Nothing requires an analysis
// document to exist, and `spec check` names one only when it is there —
// the phase is the answer to "I do not know what I am building yet", not
// a new obligation for people who do.
package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"procoder/internal/textutil"
)

// Dir is where analysis documents live.
const Dir = ".procoder/analysis"

// Sections are what an analysis document answers. Fewer than a spec's,
// and different: a spec says what will be built, this says why anyone
// thinks it should be, and what was considered instead.
var Sections = []string{
	"Question",
	"What we know",
	"What we do not know",
	"Options",
	"Recommendation",
}

// Template is the document `procoder analyze brief` prints.
const Template = `# %s

Status: open
Created: %s

## Question

<!-- The thing that is actually undecided, in one sentence. Not the
     solution you already have in mind — the question it answers. -->

## What we know

<!-- Evidence, with where it came from. A number nobody measured and a
     claim nobody checked are both guesses; say which this is. -->

## What we do not know

<!-- The gaps that would change the recommendation if filled. Name what
     would resolve each one, so somebody can go and find out. -->

## Options

<!-- Each one a real option somebody could choose, with its cost. An
     option nobody would pick is not an option, it is padding. -->

## Recommendation

<!-- Which option, and the reason. If the honest answer is "we do not
     know enough yet", that is a recommendation too — say what to do
     next to find out. -->
`

// Files lists the analysis documents in root, sorted.
func Files(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, filepath.Join(root, Dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

// Gaps reports what is still unanswered in one analysis document.
//
// The standard is the one `spec check` already holds a spec to: a heading
// is not an answer, and a section still carrying its template comment has
// recorded nothing. A phase whose documents nobody has to fill in is a
// formality that costs time and buys nothing, so this refuses the same
// way and for the same reason.
func Gaps(text string) []string {
	var gaps []string
	for _, s := range Sections {
		body := textutil.Section(text, s)
		if body == "" && !strings.Contains(text, "## "+s) {
			gaps = append(gaps, "section missing: "+s)
			continue
		}
		if strings.TrimSpace(textutil.StripComments(body)) == "" {
			gaps = append(gaps, "section empty: "+s+" — a heading is not an answer")
		}
	}
	return gaps
}

// Check reports whether each analysis document has been filled in.
func Check(root, name string, out func(string)) int {
	var files []string
	switch {
	case name == "" || name == "all":
		files = Files(root)
		if len(files) == 0 {
			out("no analysis to check — nothing under " + Dir)
			return 0
		}
	default:
		if name != filepath.Base(name) || strings.Contains(name, "..") {
			out(fmt.Sprintf("invalid analysis name %q — names are plain file names", name))
			return 2
		}
		f := filepath.Join(root, Dir, name+".md")
		if _, err := os.Stat(f); err != nil {
			out("no analysis " + name + " — `procoder analyze list` shows what exists")
			return 2
		}
		files = []string{f}
	}

	worst := 0
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			out(filepath.Base(path) + ": unreadable — " + err.Error())
			worst = 2
			continue
		}
		id := strings.TrimSuffix(filepath.Base(path), ".md")
		gaps := Gaps(string(raw))
		if len(gaps) == 0 {
			out("analysis " + id + ": COMPLETE — every section answered")
			continue
		}
		out("analysis " + id + ": NOT ready — the quality controller found:")
		for _, g := range gaps {
			out("  - " + g)
		}
		if worst < 1 {
			worst = 1
		}
	}
	return worst
}

// SpecSource names the analysis a spec came from, when one exists.
//
// Matched by name: an analysis and the spec it produced share a slug,
// which is the convention `procoder analyze brief <name>` and
// `procoder spec template <name>` already put in place. Empty when there
// is none, because analysis is available and never required — a spec
// whose author already knew what to build owes nobody a document
// recording that they knew.
func SpecSource(root, specName string) string {
	path := filepath.Join(root, Dir, specName+".md")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return filepath.ToSlash(filepath.Join(Dir, specName+".md"))
}
