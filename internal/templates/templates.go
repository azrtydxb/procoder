// Package templates resolves an embedded template against the
// repository's override.
//
// The five templates under .procoder/github/ have always been the
// repository's to edit; the nine that drive the quality chain — spec,
// plan, ADR, todo, milestone, epic, story, sprint, bug — were embedded
// constants with no way in. A team whose stories carry an extra field, or
// whose ADRs follow a house shape, had to choose between the domain and
// their own format.
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"procoder/internal/gitx"
)

// Dir is where a repository puts its own versions.
const Dir = ".procoder/templates"

// Resolve returns the template body for name, and where it came from.
//
// Absent means default — the rule everywhere else in .procoder/. Only an
// EMPTY file is an error, and it is a blocking one: `procoder format`
// prints a single header line for an already-formatted file, so a
// pipeline that strips the header and writes the rest empties the file on
// the success path. That destroyed a documentation page in this
// repository. Falling back to the default here would do the same thing to
// a team's customised template, silently, and they would find out when
// their next story came out in procoder's shape instead of theirs.
func Resolve(root, name, embedded string) (body, source string, problem *gitx.Finding) {
	path := filepath.Join(root, Dir, name+".md")
	rel := filepath.ToSlash(filepath.Join(Dir, name+".md"))
	data, err := os.ReadFile(path)
	if err != nil {
		return embedded, "default", nil
	}
	if strings.TrimSpace(string(data)) == "" {
		return embedded, rel, &gitx.Finding{Blocking: true, File: path,
			Message: fmt.Sprintf("this template is empty — procoder used its own instead. If it was emptied by accident, `git checkout <ref> -- %s` restores it; if it is meant to go, delete the file (templates)", rel)}
	}
	return string(data), rel, nil
}

// Names are the templates a repository may override.
var Names = []string{"spec", "plan", "adr", "todo", "milestone", "epic", "story", "sprint", "bug", "changelog"}
