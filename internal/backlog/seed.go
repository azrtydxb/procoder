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
	"procoder/internal/store"
	"procoder/internal/textutil"
)

// criterionRe captures the text of one acceptance-criteria checkbox, checked
// or not — the same shape the spec checker accepts, so the two controllers
// never disagree about what counts as a criterion.
var criterionRe = regexp.MustCompile(`(?m)^\s*- \[[ xX]\]\s*(.+)$`)

// fingerprint is the first 12 hex characters of the SHA-1 of a spec's
// acceptance criteria — the contract the stories were seeded from, not the
// file's bytes. Rewrapping a paragraph, fixing a typo in Problem, or
// reflowing one criterion over two lines leaves it unchanged; adding,
// removing, or rewording a criterion changes it. The board compares it
// against the spec file to flag drift.
func fingerprint(b []byte) string {
	// Hashing the whole file flagged a cosmetic edit exactly as loudly as a
	// deleted criterion, which trains readers to ignore the flag — and a
	// flag nobody reads is worse than none. Change detection, not a
	// signature: nobody gains anything by colliding their own spec file.
	sum := sha1.Sum([]byte(strings.Join(specCriteria(b), "\n"))) // nosemgrep: use-of-sha1
	return hex.EncodeToString(sum[:])[:12]
}

// specCriteria is the criteria list of a whole spec file — the one reading
// seed, board, and close all fingerprint, so they never disagree about what
// the contract is.
func specCriteria(b []byte) []string {
	return criteria(textutil.Section(string(b), "Acceptance criteria"))
}

// recordedFingerprint matches what fingerprint produces: twelve lowercase hex
// characters. An epic whose Spec: line carries anything else never had a
// fingerprint recorded — the binary prints the epic and the agent writes it,
// so a placeholder can be transcribed in place of the digest — and that is a
// different fact from a spec that changed. Reporting it as drift sends the
// reader to compare a spec against a seeding that never happened.
var recordedFingerprint = regexp.MustCompile(`^[0-9a-f]{12}$`)

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
	raw, err := store.LoadIn(root, spec.Dir, specName+".md")
	if err != nil {
		out("spec " + specName + " unreadable: " + err.Error())
		return 2
	}
	crits := specCriteria(raw)
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
		"\nSeeded from " + filepath.ToSlash(filepath.Join(spec.Dir, specName+".md")) + " — one story per acceptance criterion.\n"
	out("== write this to " + epicRel + ":")
	out(epic)

	date := time.Now().UTC().Format("20060102")
	taken := map[string]bool{}
	for i, c := range crits {
		// The scope ids a criterion cites are traceability, not part of
		// the requirement: leaving them in puts "[S-1]" in the story's
		// title and its file name, where it means nothing to a reader
		// and changes the slug if the spec is ever renumbered.
		c = spec.StripScopeIDs(c)
		slug := textutil.Slug(c)
		if slug == "" {
			// A criterion of pure punctuation still deserves a story; the
			// index keeps its file name unique and non-empty.
			slug = fmt.Sprintf("criterion-%d", i+1)
		}
		// Two criteria can slug alike — "reads foo.bar" and "reads foo-bar"
		// differ by punctuation the file name does not keep. Emitting the
		// same path twice tells the agent to write one story over the
		// other, and a criterion disappears from the backlog with nothing
		// on screen to say it did. Suffixing keeps every criterion, which
		// refusing here would not.
		base := slug
		for n := 2; taken[slug]; n++ {
			slug = fmt.Sprintf("%s-%d", base, n)
		}
		taken[slug] = true
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
	for _, line := range strings.Split(textutil.StripComments(body), "\n") {
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
