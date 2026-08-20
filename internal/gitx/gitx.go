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

// FilesUnder lists the gate's file set beneath dir: tracked plus
// untracked-but-not-ignored. Paths are absolute.
func FilesUnder(root, dir string) []string {
	out, err := git(root, "ls-files", "--cached", "--others", "--exclude-standard", "--", dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			files = append(files, filepath.Join(root, line))
		}
	}
	return files
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

// GrepOn lists the files on a ref whose content matches a regular
// expression, as repo-relative paths. It is how a report asks what another
// branch holds without checking it out; err is a git that could not answer,
// which is never the same as a match of none.
func GrepOn(root, ref, pattern, pathspec string) ([]string, error) {
	out, err := git(root, "grep", "-l", "-e", pattern, ref, "--", pathspec)
	if err != nil {
		// git grep exits 1 for no matches, which is an answer, not a failure.
		if code, ok := err.(*exec.ExitError); ok && code.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if _, path, found := strings.Cut(line, ":"); found && path != "" {
			files = append(files, path)
		}
	}
	return files, nil
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

// conflictAllow exempts a whole file from the conflict-marker check, and
// only with a reason after the token: a document whose subject IS merge
// conflicts cannot show a reader one otherwise, and the alternatives are
// all worse — mangle the example so a copy of it is broken, drop the
// topic, or turn the check off everywhere.
//
// File-scoped and explicit rather than "skip fenced code blocks", because
// a real conflict lands inside a fence often enough that skipping them
// would be a silent miss — and a check that quietly misses things is the
// failure this product argues against. Same spirit as gitleaks:allow: one
// greppable line, a stated reason, and nothing implicit.
var conflictAllow = regexp.MustCompile(`procoder:allow-conflict-markers(.*)`)

// conflictAllowed reports whether the file carries the exemption WITH a
// reason. The comment terminator is stripped first, so the reason in
// `<!-- procoder:allow-conflict-markers -->` reads as empty — a bare
// token is someone silencing the check, not documenting an exception.
func conflictAllowed(data []byte) bool {
	m := conflictAllow.FindSubmatch(data)
	if m == nil {
		return false
	}
	reason := strings.TrimSpace(string(m[1]))
	for _, close := range []string{"-->", "*/", "#}", "-}"} {
		reason = strings.TrimSpace(strings.TrimSuffix(reason, close))
	}
	return reason != ""
}

// ConflictMarkers finds merge-conflict markers left in changed files.
func ConflictMarkers(files []string) []Finding {
	var out []Finding
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil || !utf8ish(data) {
			continue
		}
		if conflictAllowed(data) {
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

// Junk: OS droppings, merge leftovers, caches, editor swap — the garbage a
// senior developer never lets into history. Deliberate build artifacts a repo
// ships on purpose (this repo's dist/ binaries) are NOT junk and NOT listed.
var junkNames = map[string]bool{
	".DS_Store": true, "Thumbs.db": true, "desktop.ini": true,
	".lycheecache": true, "npm-debug.log": true, "yarn-error.log": true,
}

var junkExts = map[string]bool{
	".orig": true, ".rej": true, ".pyc": true, ".pyo": true,
	".swp": true, ".swo": true, ".tmp": true, ".log": true,
}

// junkDirs anywhere in the path mean the whole entry is cache, not source.
var junkDirs = map[string]bool{
	"__pycache__": true, "node_modules": true, ".venv": true, "venv": true,
	".cache": true, ".pytest_cache": true, ".mypy_cache": true,
	".ruff_cache": true, ".tox": true, ".gradle": true,
}

// JunkFiles flags staged garbage: OS droppings, caches, editor swap, logs.
func JunkFiles(files []string) []Finding {
	var out []Finding
	for _, f := range files {
		base := filepath.Base(f)
		if junkNames[base] || junkExts[filepath.Ext(base)] || inJunkDir(f) {
			out = append(out, Finding{File: f, Blocking: true,
				Message: "junk file staged — caches and garbage never belong in a commit"})
		}
	}
	return out
}

func inJunkDir(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if junkDirs[part] {
			return true
		}
	}
	return false
}

// gitignoreNeeds maps an ecosystem marker present in the repo to the ignore
// entry its cache/artifact garbage requires. Reported, not blocked: the fix
// is one line in .gitignore and the finding says which.
var gitignoreNeeds = []struct{ marker, entry string }{
	{"go.mod", "*.test"},
	{"package.json", "node_modules"},
	{"pyproject.toml", "__pycache__"},
	{"requirements.txt", "__pycache__"},
	{"Cargo.toml", "target"},
	{"mkdocs.yml", "site"},
	// procoder's own derived state: per-machine, rebuilt on demand, tracked
	// no more than .procoder/index/ is.
	{".procoder", ".procoder/state/"},
}

// IgnoreCoverage checks the repo has a .gitignore covering the garbage its
// present ecosystems generate.
func IgnoreCoverage(root string) []Finding {
	var present []struct{ marker, entry string }
	for _, n := range gitignoreNeeds {
		if _, err := os.Stat(filepath.Join(root, n.marker)); err == nil {
			present = append(present, n)
		}
	}
	if len(present) == 0 {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return []Finding{{Message: ".gitignore is missing — every repo ignores its garbage; add one covering the ecosystems present"}}
	}
	text := string(data)
	var out []Finding
	for _, n := range present {
		if !strings.Contains(text, n.entry) {
			out = append(out, Finding{File: filepath.Join(root, ".gitignore"),
				Message: fmt.Sprintf(".gitignore has no %q entry, but %s is present — its garbage will end up staged", n.entry, n.marker)})
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

// Attribution finds AI-attribution lines in the given commit messages.
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
