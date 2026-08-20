// Package backlog is the project layer of the quality chain: milestones,
// epics, and user stories under .procoder/backlog/, worked in scope-boxed
// sprints. The story is the execution unit of spec-based work and carries
// the todo-task rigor (description, acceptance criteria, evidence); the
// todo domain itself stays untouched as the standalone list for work not
// born from a spec. The binary is the quality controller: creation PRINTS
// files for the agent to write, and only verified state transitions
// (close, pull, carry) rewrite files — all of them procoder-owned, under
// this directory (P-CONTROL).
package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"procoder/internal/textutil"
)

// Dir is where the backlog lives, under D-HOME.
const Dir = ".procoder/backlog"

// The four kinds, doubling as subdirectory names.
const (
	KindMilestone = "milestones"
	KindEpic      = "epics"
	KindStory     = "stories"
	KindSprint    = "sprints"
)

var (
	statusRe    = regexp.MustCompile(`(?m)^Status:\s*(.+?)\s*$`)
	milestoneRe = regexp.MustCompile(`(?m)^Milestone:\s*(\S+)`)
	epicRe      = regexp.MustCompile(`(?m)^Epic:\s*(\S+)`)
	sprintRe    = regexp.MustCompile(`(?m)^Sprint:\s*(\S+)`)
	typeRe      = regexp.MustCompile(`(?m)^Type:\s*(\S+)`)
	severityRe  = regexp.MustCompile(`(?m)^Severity:\s*(\S+)`)
	// The fingerprint half is optional to MATCH so that a Spec: line without
	// one still registers as a spec reference. It is not optional to HAVE:
	// an epic that names a spec but records no seeding is one the board must
	// be able to say that about, and a line that simply failed to parse is
	// invisible — the worst of the three states.
	specRe      = regexp.MustCompile(`(?m)^Spec:\s*(\S+)(?:\s*@\s*(\S+))?`)
	uncheckedRe = regexp.MustCompile(`(?m)^\s*- \[ \]`)
	checkedRe   = regexp.MustCompile(`(?m)^\s*- \[[xX]\]`)
	placeholder = regexp.MustCompile(`(?m)^\s*- \[.\]\s*\.\.\.`)
)

// Item is one parsed backlog file of any kind.
type Item struct {
	ID        string
	Kind      string // one of the Kind* constants
	Title     string
	Status    string // open | done ... | active | closed ... | unreadable
	Milestone string // epics: parent milestone slug, "" when none
	Epic      string // stories: parent epic slug
	Sprint    string // stories: sprint id, "-" or "" when in the backlog
	Type      string // stories: "bug" from a Type: header; "" means feature
	Severity  string // stories: s1..s4 from a Severity: header, "" when absent
	SpecName  string // epics: source spec name, "" when none
	SpecPrint string // epics: spec fingerprint recorded at seed time
	Path      string
}

// Done reports whether the item's status counts as finished. Anything the
// controller does not recognise stays open — refusals are conservative.
func (i Item) Done() bool {
	return strings.HasPrefix(i.Status, "done") || strings.HasPrefix(i.Status, "closed")
}

// ItemFile resolves an id to its path within a kind, refusing ids that
// would escape the backlog directory (separators, dot-dot).
func ItemFile(root, kind, id string) (string, error) {
	if id == "" || id != filepath.Base(id) || strings.Contains(id, "..") {
		return "", fmt.Errorf("invalid id %q — ids are plain file names", id)
	}
	return filepath.Join(root, Dir, kind, id+".md"), nil
}

// LoadAll reads every backlog file across the four kinds. Missing
// directories are an empty backlog, not an error; an unreadable file is
// an item with Status unreadable — never silently skipped.
func LoadAll(root string) ([]Item, error) {
	var items []Item
	for _, kind := range []string{KindMilestone, KindEpic, KindStory, KindSprint} {
		dir := filepath.Join(root, Dir, kind)
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("backlog directory unreadable: %v", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			it := Item{ID: strings.TrimSuffix(e.Name(), ".md"), Kind: kind, Path: path, Status: "open"}
			raw, err := os.ReadFile(path)
			if err != nil {
				it.Status = "unreadable"
				it.Title = err.Error()
				items = append(items, it)
				continue
			}
			text := string(raw)
			if m := statusRe.FindStringSubmatch(text); m != nil {
				it.Status = m[1]
			}
			if m := milestoneRe.FindStringSubmatch(text); m != nil {
				it.Milestone = m[1]
			}
			if m := epicRe.FindStringSubmatch(text); m != nil && m[1] != "-" {
				// Epic: - is a story that predates any epic — the same
				// as no Epic line at all, never a link to an epic "-".
				it.Epic = m[1]
			}
			if m := sprintRe.FindStringSubmatch(text); m != nil {
				it.Sprint = m[1]
			}
			if m := typeRe.FindStringSubmatch(text); m != nil {
				it.Type = m[1]
			}
			if m := severityRe.FindStringSubmatch(text); m != nil {
				it.Severity = m[1]
			}
			if m := specRe.FindStringSubmatch(text); m != nil {
				it.SpecName, it.SpecPrint = m[1], m[2]
			}
			if i := strings.Index(text, "# "); i >= 0 {
				line := text[i+2:]
				if j := strings.IndexByte(line, '\n'); j > 0 {
					it.Title = strings.TrimSpace(line[:j])
				}
			}
			items = append(items, it)
		}
	}
	sort.Slice(items, func(a, b int) bool {
		if items[a].Kind != items[b].Kind {
			return items[a].Kind < items[b].Kind
		}
		return items[a].ID < items[b].ID
	})
	return items, nil
}

const milestoneTemplate = `# %s

Status: open
Created: %s

## Goal

<!-- What reaching this milestone means, in the reader's terms — the
     state of the product, not a list of epics. -->
`

const epicTemplate = `# %s

Status: open
Created: %s%s

## Description

<!-- What this epic delivers and why it is one coherent unit of value.
     Stories reference this epic by its file name. -->
`

const storyTemplate = `# %s

Status: open
Created: %s
Epic: %s
Sprint: -

## Description

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] ...

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
`

// Milestone prints a milestone file for the agent to write.
func Milestone(root, title string, out func(string)) int {
	return printItem(root, KindMilestone, textutil.Slug(title), title, func(now string) string {
		return fmt.Sprintf(milestoneTemplate, title, now)
	}, out)
}

// Epic prints an epic file; milestone is the optional parent slug.
func Epic(root, title, milestone string, out func(string)) int {
	extra := ""
	if milestone != "" {
		extra = "\nMilestone: " + milestone
	}
	return printItem(root, KindEpic, textutil.Slug(title), title, func(now string) string {
		return fmt.Sprintf(epicTemplate, title, now, extra)
	}, out)
}

// Story prints a story file; epic is the required parent slug. Story ids
// are date-prefixed like todo tasks, so the backlog reads chronologically.
func Story(root, title, epic string, out func(string)) int {
	if epic == "" {
		out("a story belongs to an epic — pass --epic <id> (or use `procoder todo add` for standalone work)")
		return 2
	}
	slug := textutil.Slug(title)
	if slug != "" {
		slug = time.Now().UTC().Format("20060102") + "-" + slug
	}
	return printItem(root, KindStory, slug, title, func(now string) string {
		return fmt.Sprintf(storyTemplate, title, now, epic)
	}, out)
}

// printItem is the shared creation path: refuse empty slugs and existing
// files (a slug collision must not print an overwrite), else print the
// target path and content — the binary creates nothing (P-CONTROL).
func printItem(root, kind, slug, title string, fill func(now string) string, out func(string)) int {
	if slug == "" || title == "" {
		out("a " + strings.TrimSuffix(kind, "s") + " needs a title")
		return 2
	}
	rel := filepath.ToSlash(filepath.Join(Dir, kind, slug+".md"))
	if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
		out(rel + " already exists — pick a different title or edit the existing file")
		return 2
	}
	out("== write this to " + rel + ":")
	out(fill(time.Now().UTC().Format("2006-01-02")))
	return 0
}
