// Package stall tells work that is moving from work that is being edited.
//
// A story carried across three sprints, touched in nine commits, its
// criteria still unchecked and its evidence still empty, looks busy in
// every report procoder produces. `sprint status` says "carried over";
// nothing says "carried over and never actually advanced".
//
// The difference is what changed. A timestamp, a reflowed paragraph, a
// reordered list: the file is different and the WORK is not. So the state
// is hashed over the fields that carry status — the Status line, which
// criteria are checked, what evidence says — and everything else is
// deliberately left out (#205).
//
// A detection aid, never an automatic anything. A story can legitimately
// sit for weeks; what this reports is that it sat while looking otherwise,
// and a person decides what that means.
package stall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Item is one file's stall verdict.
type Item struct {
	Path    string
	Commits int    // how many times the file changed
	Hash    string // the semantic state, now
	Stalled bool   // changed repeatedly, meant the same thing throughout
}

var (
	statusLine  = regexp.MustCompile(`(?m)^Status:\s*(.+?)\s*$`)
	criterion   = regexp.MustCompile(`(?m)^\s*-\s*\[([ xX])\]`)
	sectionHead = regexp.MustCompile(`(?m)^##\s+(.+?)\s*$`)
)

// Semantic reduces a backlog or todo file to what it MEANS: its status,
// the checked-ness of each criterion in order, and whether its evidence
// says anything.
//
// Everything else is dropped on purpose. Prose is reworded constantly
// without the work moving, and a hash that changed when a sentence was
// tightened would report every story as progressing.
//
// Evidence is reduced to present-or-absent rather than hashed: its text is
// rewritten as it is gathered, and what matters for "did this move" is
// that it went from nothing to something.
func Semantic(text string) string {
	var parts []string
	if m := statusLine.FindStringSubmatch(text); m != nil {
		parts = append(parts, "status="+strings.ToLower(strings.TrimSpace(m[1])))
	}
	var boxes []string
	for _, m := range criterion.FindAllStringSubmatch(text, -1) {
		if strings.TrimSpace(m[1]) == "" {
			boxes = append(boxes, "0")
		} else {
			boxes = append(boxes, "1")
		}
	}
	parts = append(parts, "criteria="+strings.Join(boxes, ""))
	parts = append(parts, fmt.Sprintf("evidence=%v", hasEvidence(text)))
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])[:16]
}

// hasEvidence reports whether the Evidence section says anything a person
// wrote, as opposed to being absent or still holding its template comment.
func hasEvidence(text string) bool {
	locs := sectionHead.FindAllStringSubmatchIndex(text, -1)
	for i, loc := range locs {
		if !strings.EqualFold(strings.TrimSpace(text[loc[2]:loc[3]]), "Evidence") {
			continue
		}
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		body := text[loc[1]:end]
		// The template's own comment is not evidence of anything.
		for {
			a := strings.Index(body, "<!--")
			if a < 0 {
				break
			}
			b := strings.Index(body[a:], "-->")
			if b < 0 {
				body = body[:a]
				break
			}
			body = body[:a] + body[a+b+3:]
		}
		return strings.TrimSpace(body) != ""
	}
	return false
}

// Check reports the files that changed repeatedly without ever meaning
// something different.
//
// minCommits is the bar for "repeatedly": one edit is not a pattern, and
// reporting every file touched twice would bury the ones that matter.
func Check(root string, paths []string, minCommits int, out func(string)) int {
	var stalled []Item
	var unknown []string
	for _, p := range paths {
		hashes, err := semanticHistory(root, p)
		if err != nil {
			// git could not answer for this file. Not evidence of
			// anything, and certainly not evidence of progress.
			unknown = append(unknown, fmt.Sprintf("%s NOT checked (%v)", p, err))
			continue
		}
		if len(hashes) < minCommits {
			continue
		}
		distinct := map[string]bool{}
		for _, h := range hashes {
			distinct[h] = true
		}
		if len(distinct) == 1 {
			stalled = append(stalled, Item{Path: p, Commits: len(hashes), Hash: hashes[0], Stalled: true})
		}
	}
	sort.Slice(stalled, func(i, j int) bool { return stalled[i].Commits > stalled[j].Commits })
	for _, u := range unknown {
		out("  " + u)
	}
	if len(stalled) == 0 {
		out(fmt.Sprintf("no stalled items — nothing changed %d+ times while meaning the same thing throughout", minCommits))
		return 0
	}
	for _, s := range stalled {
		out(fmt.Sprintf("  %s — %d commits, same status, same criteria, same evidence throughout", s.Path, s.Commits))
	}
	out(fmt.Sprintf("%d item(s) look active and are not — a detection aid, not a verdict: an item can legitimately wait, and this says only that it waited while looking otherwise", len(stalled)))
	return 0
}

// semanticHistory is the file's semantic hash at each commit that touched
// it, oldest first.
func semanticHistory(root, path string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	log := exec.CommandContext(ctx, "git", "-C", root, "log", "--format=%H", "--", path) // nosemgrep -- a path from the repository's own listing
	raw, err := log.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %v", firstLine(err))
	}
	var out []string
	shas := strings.Fields(string(raw))
	for i := len(shas) - 1; i >= 0; i-- { // oldest first
		ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		show := exec.CommandContext(ctx2, "git", "-C", root, "show", shas[i]+":"+path) // nosemgrep -- a sha from git's own log
		content, serr := show.Output()
		cancel2()
		if serr != nil {
			continue // the file did not exist at that commit
		}
		out = append(out, Semantic(string(content)))
	}
	return out, nil
}

func firstLine(err error) string {
	s := err.Error()
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}
