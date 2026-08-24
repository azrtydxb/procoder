// Package review is the judgment half of the gate. Every other check
// procoder runs is mechanical — formatting, secrets, linters, hygiene —
// and answers a question with one right answer. This one asks the
// questions that do not: is this the right shape, what breaks at the
// edges, would a test catch this regressing.
//
// The binary judges nothing. It cannot: it is not a language model. It
// prints the lens and the scope, the agent judges, and what comes back
// travels the same path as every other finding. That is the same contract
// `procoder format` has always held — the binary prints, the agent
// writes — and it is why this package has no opinion about the content it
// names.
package review

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"procoder/internal/gitx"
)

// Dir is where a repository puts its own lenses.
const Dir = ".procoder/review/lenses"

// Lens is one stance, and where its text came from.
type Lens struct {
	Name string
	Body string
	// Source is "default" for a shipped lens, or the repo-relative path of
	// the override that replaced it. Printed so a reader knows whose words
	// they are reading.
	Source string
}

// Resolve returns the lens set for this repository: the shipped lenses,
// each replaced by an override where one exists.
//
// An unreadable or empty override is a refusal, and NOTHING is returned —
// deliberately unlike templates.Resolve, which returns its embedded body
// alongside the finding. A template is content the agent reads before
// writing it somewhere, so showing procoder's version and blocking the
// commit loses nothing. A lens is an instruction that shapes a judgment,
// and `procoder review` is not gated by the commit gate: an agent reading
// printed output may act on it whatever the exit code says. Printing
// procoder's adversarial lens to a repository that wrote its own would
// produce a review claiming to be one thing and being another. Printing
// nothing leaves nothing to act on by mistake.
func Resolve(root string) ([]Lens, []gitx.Finding) {
	return resolveSet(root, Lenses, Dir)
}

// ResolvePerspectives is Resolve for the perspective set — who is
// reading, where a lens is how. Same override contract, same refusal:
// a perspective that could not be read is not replaced by procoder's.
func ResolvePerspectives(root string) ([]Lens, []gitx.Finding) {
	return resolveSet(root, PerspectiveSet, PerspectiveDir)
}

func resolveSet(root string, shippedSet []Lens, dir string) ([]Lens, []gitx.Finding) {
	var out []Lens
	var problems []gitx.Finding
	for _, shipped := range shippedSet {
		path := filepath.Join(root, dir, shipped.Name+".md")
		rel := filepath.ToSlash(filepath.Join(dir, shipped.Name+".md"))
		data, err := os.ReadFile(path)
		switch {
		case os.IsNotExist(err):
			shipped.Source = "default"
			out = append(out, shipped)
		case err != nil:
			// Present and unreadable is not absent. A permissions failure
			// reported as "no override here" runs procoder's lens under the
			// repository's name.
			problems = append(problems, gitx.Finding{Blocking: true, File: path,
				Message: fmt.Sprintf("%s %s NOT read — %v (review)", kindOf(dir), shipped.Name, err)})
		case strings.TrimSpace(string(data)) == "":
			problems = append(problems, gitx.Finding{Blocking: true, File: path,
				Message: fmt.Sprintf("%s %s is empty — procoder did NOT fall back to its own, because a review under your own name running procoder's words is worse than no review. If it was emptied by accident, `git checkout <ref> -- %s` restores it; if it is meant to go, delete the file (review)", kindOf(dir), shipped.Name, rel)})
		default:
			out = append(out, Lens{Name: shipped.Name, Body: string(data), Source: rel})
		}
	}
	return out, problems
}

// Select narrows a lens set to the named ones, in the order the caller
// asked for them. An unknown name is returned rather than guessed at: a
// reader who asks for one lens and silently gets the other four believes
// they got the one they asked for.
func Select(all []Lens, names []string) ([]Lens, []string) {
	byName := map[string]Lens{}
	for _, l := range all {
		byName[l.Name] = l
	}
	var out []Lens
	var unknown []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if l, ok := byName[n]; ok {
			out = append(out, l)
			continue
		}
		unknown = append(unknown, n)
	}
	return out, unknown
}

// Names lists the lens names in a set, for an error that has to say what
// was available.
func Names(all []Lens) []string {
	out := make([]string, 0, len(all))
	for _, l := range all {
		out = append(out, l.Name)
	}
	return out
}

// Print writes the review request: what is in scope, then each lens.
//
// The scope is named rather than quoted. The agent can already read the
// files; what only procoder knows is which ones the change touched and
// what the lenses are.
func Print(w io.Writer, scope []string, lenses []Lens) { print(w, scope, lenses, "lens(es)", "lens") }

// PrintPerspectives is Print for the perspective set, which reads the
// same way but is not a lens and must not say it is: a reader told they
// applied five lenses when they applied four perspectives cannot check
// the claim against what they did.
func PrintPerspectives(w io.Writer, scope []string, ps []Lens) {
	print(w, scope, ps, "perspective(s)", "perspective")
}

func print(w io.Writer, scope []string, lenses []Lens, plural, singular string) {
	fmt.Fprintf(w, "== procoder review — %d %s over %d file(s)\n\n", len(lenses), plural, len(scope))
	if len(scope) == 0 {
		fmt.Fprintln(w, "no files in scope — pass paths, or make a change for the diff to carry")
		return
	}
	fmt.Fprintln(w, "## In scope")
	fmt.Fprintln(w)
	for _, p := range scope {
		fmt.Fprintln(w, "- "+p)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Read each file. Then apply every %s below, one at a time — they are\n", singular)
	fmt.Fprintln(w, "different stances, not a checklist to merge. Overlap between two of them is")
	fmt.Fprintln(w, "signal rather than duplication: keep both findings and say they agree.")
	fmt.Fprintln(w)
	for _, l := range lenses {
		fmt.Fprintf(w, "---\n\n%s\n", l.Body)
		if l.Source != "default" {
			fmt.Fprintf(w, "\n(this lens comes from %s, not procoder's own)\n", l.Source)
		}
		fmt.Fprintln(w)
	}
}

// kindOf names what a directory holds, so a refusal says "perspective
// architect is empty" rather than calling everything a lens.
func kindOf(dir string) string {
	if dir == PerspectiveDir {
		return "perspective"
	}
	return "lens"
}
