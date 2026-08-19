package backlog

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"procoder/internal/spec"
)

// criterionRe captures the text of one acceptance-criteria checkbox, checked
// or not — the same shape the spec checker accepts, so the two controllers
// never disagree about what counts as a criterion.
var criterionRe = regexp.MustCompile(`(?m)^\s*- \[[ xX]\]\s*(.+)$`)

// fingerprint is the first 12 hex characters of the SHA-1 of the spec file
// bytes: enough to detect that the spec changed after seeding, short enough
// to live on the epic's `Spec:` header line. The board compares it against
// the current spec file to flag drift.
func fingerprint(b []byte) string {
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:])[:12]
}

// Seed decomposes a COMPLETE spec into one epic plus one story per
// acceptance criterion, everything printed for the agent to review and
// write — the binary creates no files (P-CONTROL). The epic slug is the
// spec name and its header records the spec plus a content fingerprint, so
// the board can flag drift when the spec changes after seeding. An
// incomplete spec is refused with the checker's own gap list; a spec whose
// criteria are placeholders or commented out is refused because an epic
// with no stories decomposes nothing; an existing epic is refused because
// re-seeding after a spec change is a manual decision.
func Seed(root, specName, milestone string, out func(string)) int {
	var gaps []string
	if code := spec.Check(root, specName, func(s string) { gaps = append(gaps, s) }); code != 0 {
		out("spec " + specName + " is not COMPLETE — the backlog is built from finished specs:")
		for _, g := range gaps {
			out(g)
		}
		// The checker's own verdict carries the exit semantics: 1 for a
		// spec with gaps, 2 for a spec that does not exist.
		return code
	}
	raw, err := os.ReadFile(filepath.Join(root, spec.Dir, specName+".md"))
	if err != nil {
		out("spec " + specName + " unreadable: " + err.Error())
		return 2
	}
	crits := criteria(section(string(raw), "Acceptance criteria"))
	if len(crits) == 0 {
		out("an epic with no stories is not a decomposition — the spec needs acceptance criteria")
		return 1
	}
	epicRel := filepath.ToSlash(filepath.Join(Dir, KindEpic, specName+".md"))
	if _, err := os.Stat(filepath.Join(root, epicRel)); err == nil {
		out(epicRel + " already exists — re-seeding after a spec change is a manual decision: update or remove that epic first")
		return 2
	}

	now := time.Now().UTC().Format("2006-01-02")
	extra := ""
	if milestone != "" {
		extra += "\nMilestone: " + milestone
	}
	extra += "\nSpec: " + specName + " @ " + fingerprint(raw)
	epic := fmt.Sprintf(epicTemplate, specName, now, extra) +
		"\nSeeded from " + filepath.Join(spec.Dir, specName+".md") + " — one story per acceptance criterion.\n"
	out("== write this to " + epicRel + ":")
	out(epic)

	date := time.Now().UTC().Format("20060102")
	for i, c := range crits {
		slug := slugify(c)
		if slug == "" {
			// A criterion of pure punctuation still deserves a story; the
			// index keeps its file name unique and non-empty.
			slug = fmt.Sprintf("criterion-%d", i+1)
		}
		rel := filepath.ToSlash(filepath.Join(Dir, KindStory, date+"-"+slug+".md"))
		story := fmt.Sprintf(storyTemplate, c, now, specName)
		// The seeded criterion replaces the template placeholder: the story
		// starts life with its contract already written, unchecked.
		story = strings.Replace(story, "- [ ] ...", "- [ ] "+c, 1)
		out("== write this to " + rel + ":")
		out(story)
	}
	return 0
}

// criteria extracts the real acceptance criteria from a section body:
// commented-out boxes and the `...` template placeholder are not stories,
// and a criterion wrapped over several lines collapses to one line, which
// becomes the story title.
func criteria(body string) []string {
	var list []string
	continuing := false
	for _, line := range strings.Split(stripComments(body), "\n") {
		if m := criterionRe.FindStringSubmatch(line); m != nil {
			text := strings.TrimSpace(m[1])
			if text == "..." {
				continuing = false
				continue
			}
			list = append(list, text)
			continuing = true
			continue
		}
		t := strings.TrimSpace(line)
		if t == "" {
			continuing = false
			continue
		}
		if continuing {
			list[len(list)-1] += " " + t
		}
	}
	return list
}
