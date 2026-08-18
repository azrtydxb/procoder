// Package gitx is the git-hygiene half of the GitOps domain: everything a
// senior developer glances at before calling work finished, computed
// deterministically from the repository itself. Every function answers with
// information; acting on it is the agent's job (P-CONTROL).
package gitx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Finding is one hygiene problem: where it is and what it is. This is the
// shape every non-formatter domain reports in — a diagnosis the agent
// investigates, not content it can paste.
type Finding struct {
	File    string
	Line    int // 0 when the finding is about the file as a whole
	Message string
	// Blocking findings fail the gate; the rest are information.
	Blocking bool
}

func git(root string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// ChangedFiles is the set a commit is about to contain: modified, added,
// renamed, untracked — deleted files are not checkable. Paths are absolute.
func ChangedFiles(root string) ([]string, error) {
	out, err := git(root, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 || strings.Contains(line[:2], "D") {
			continue
		}
		name := strings.TrimSpace(line[3:])
		if i := strings.Index(name, " -> "); i >= 0 {
			name = name[i+4:]
		}
		name = strings.Trim(name, `"`)
		files = append(files, filepath.Join(root, name))
	}
	return files, nil
}

// CurrentBranch, or "" when detached.
func CurrentBranch(root string) string {
	b, err := git(root, "branch", "--show-current")
	if err != nil {
		return ""
	}
	return b
}

// DefaultBranch asks the remote first (origin/HEAD), then falls back to the
// conventional names. "" means it could not be determined, and the caller
// treats that as "not on it" — a hygiene check must not block on ignorance.
func DefaultBranch(root string) string {
	if ref, err := git(root, "symbolic-ref", "refs/remotes/origin/HEAD", "--short"); err == nil {
		return strings.TrimPrefix(ref, "origin/")
	}
	for _, name := range []string{"main", "master"} {
		if _, err := git(root, "rev-parse", "--verify", "refs/heads/"+name); err == nil {
			return name
		}
	}
	return ""
}

// UnpushedMessages returns the full message of every commit not yet on the
// upstream — the set whose wording the gate can still save. No upstream means
// no basis for "unpushed", so the last commit alone is checked: better one
// honest data point than a guess.
func UnpushedMessages(root string) []string {
	rangeSpec := "@{upstream}..HEAD"
	if _, err := git(root, "rev-parse", "--verify", "@{upstream}"); err != nil {
		rangeSpec = "-1"
	}
	out, err := git(root, "log", "--format=%B%x00", rangeSpec)
	if err != nil {
		return nil
	}
	var msgs []string
	for _, m := range strings.Split(out, "\x00") {
		if m = strings.TrimSpace(m); m != "" {
			msgs = append(msgs, m)
		}
	}
	return msgs
}

// --- the checks -------------------------------------------------------------

// Conflict markers. `<<<<<<< ` and `>>>>>>> ` at line start are unambiguous;
// a bare `=======` is a legitimate Markdown underline and is deliberately not
// matched on its own — the two ends of the conflict are always present when
// the middle is.
var conflictMarker = regexp.MustCompile(`(?m)^(<{7}( |$)|>{7}( |$))`)

func ConflictMarkers(files []string) []Finding {
	var out []Finding
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil || !utf8ish(data) {
			continue
		}
		for _, idx := range conflictMarker.FindAllIndex(data, -1) {
			line := 1 + bytes.Count(data[:idx[0]], []byte("\n"))
			out = append(out, Finding{File: f, Line: line, Blocking: true,
				Message: "merge conflict marker left in the file"})
		}
	}
	return out
}

// Junk files that only ever land in a commit by accident.
var junkNames = map[string]bool{".DS_Store": true, "Thumbs.db": true}

func JunkFiles(files []string) []Finding {
	var out []Finding
	for _, f := range files {
		base := filepath.Base(f)
		ext := filepath.Ext(base)
		if junkNames[base] || ext == ".orig" || ext == ".rej" {
			out = append(out, Finding{File: f, Blocking: true,
				Message: "junk file staged — this never belongs in a commit"})
		}
	}
	return out
}

// Oversized files about to be committed.
func Oversized(files []string, maxMB int) []Finding {
	limit := int64(maxMB) << 20
	var out []Finding
	for _, f := range files {
		if info, err := os.Stat(f); err == nil && info.Size() > limit {
			out = append(out, Finding{File: f, Blocking: true,
				Message: fmt.Sprintf("%d MB staged — over the %d MB limit; large artifacts belong in releases or LFS, not history", info.Size()>>20, maxMB)})
		}
	}
	return out
}

// AI attribution in commit messages. The work is the author's; a line crediting
// the tool is noise at best and a policy violation at worst, and it BLOCKS.
var attribution = regexp.MustCompile(`(?im)^[ \t]*co-authored-by:.*\b(claude|anthropic)\b|generated with.*\bclaude\b|noreply@anthropic\.com|🤖`)

func Attribution(messages []string) []Finding {
	var out []Finding
	for _, m := range messages {
		if loc := attribution.FindString(m); loc != "" {
			out = append(out, Finding{Blocking: true,
				Message: fmt.Sprintf("commit message carries an AI-attribution line: %q — the work is the author's; remove it (git commit --amend / rebase)", strings.TrimSpace(firstLineOf(loc)))})
		}
	}
	return out
}

// ScrubText applies the same attribution patterns to arbitrary text — the PR
// skill runs a drafted body through this before anything is sent.
func ScrubText(text string) []Finding {
	return Attribution([]string{text})
}

// Commit subject shape: non-empty, <=72 chars, blank line before any body.
// Reported, never blocking — the commit template makes this mostly automatic.
func SubjectShape(messages []string) []Finding {
	var out []Finding
	for _, m := range messages {
		lines := strings.Split(m, "\n")
		subject := strings.TrimSpace(lines[0])
		switch {
		case subject == "":
			out = append(out, Finding{Message: "commit has an empty subject line"})
		case len(subject) > 72:
			out = append(out, Finding{Message: fmt.Sprintf("commit subject is %d chars (aim for <=72): %q", len(subject), subject[:40]+"...")})
		}
		if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
			out = append(out, Finding{Message: fmt.Sprintf("no blank line between subject and body: %q", subject)})
		}
	}
	return out
}

// OnDefaultBranch reports (or blocks, per config) work happening directly on
// the default branch.
func OnDefaultBranch(root string, block bool) []Finding {
	current, def := CurrentBranch(root), DefaultBranch(root)
	if current == "" || def == "" || current != def {
		return nil
	}
	return []Finding{{Blocking: block,
		Message: fmt.Sprintf("working directly on the default branch (%s) — a senior habit is a branch per change; set [git] default_branch_policy in .procoder/config.toml to choose report or block", def)}}
}

func utf8ish(data []byte) bool {
	return !bytes.Contains(data[:min(len(data), 8000)], []byte{0})
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
