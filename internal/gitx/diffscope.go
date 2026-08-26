package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// NarrowToDiff drops every finding that sits on a line this commit did
// not write.
//
// It exists for repositories that never adopted procoder. There, a file is
// somebody else's and the diff is the only part of it that is mine to
// answer for. The reported case was a constant whose name ends in
// `_STORE_KEY`, on line 4,423 of a file whose change sat 2,500 lines away:
// not a credential, not written by that commit, and blocking it anyway —
// which is how `--no-verify` becomes muscle memory and the gate stops
// protecting anything (#172).
//
// The scans themselves are unchanged. Whole files are still handed to
// gitleaks and to the conflict-marker reader, because a secret split
// across lines, or a marker whose meaning comes from the lines around it,
// needs those surroundings; what narrows is which findings survive.
//
// This applies only to checks that read file CONTENT. A file-level check —
// an oversized blob, a junk file — is about the file's presence, which
// this commit is adding whole, so those never narrow.
//
// In an adopting repository this is not used at all: there the file is
// yours, what is in it is yours, and you asked to be told.
func NarrowToDiff(root string, paths []string, all []Finding) []Finding {
	if len(all) == 0 {
		return nil
	}
	touched := addedLines(root, paths)
	if touched == nil {
		// The diff could not be read, so which lines are this commit's is
		// unknown. Report everything rather than nothing: a secret missed
		// because the narrowing could not be computed is the silent green
		// this project exists to prevent, and a false positive is merely
		// annoying.
		return all
	}
	var out []Finding
	for _, f := range all {
		key := lineKey{file: normalisePath(root, f.File), line: f.Line}
		// A finding with no line cannot be placed in or out of the diff.
		// It stays, for the same reason as above.
		if f.Line == 0 || touched[key] {
			out = append(out, f)
		}
	}
	return out
}

type lineKey struct {
	file string
	line int
}

// addedLines is every line this commit writes, by file. Nil when git could
// not answer — which the caller must treat as "unknown", never as "none".
func addedLines(root string, paths []string) map[lineKey]bool {
	// Staged first: at a pre-commit hook that is what is about to become
	// the commit. --unified=0 so each hunk covers only changed lines and
	// nothing either side of them.
	out := map[lineKey]bool{}
	for _, args := range [][]string{
		{"diff", "--cached", "--unified=0", "--no-color"},
		{"diff", "--unified=0", "--no-color"},
	} {
		raw, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
		if err != nil {
			return nil
		}
		collectAddedLines(root, string(raw), out)
	}
	// An untracked file appears in no diff, and every line in it is this
	// commit's — it did not exist before.
	for _, p := range paths {
		if !tracked(root, p) {
			markWholeFile(root, p, out)
		}
	}
	return out
}

// collectAddedLines walks a unified diff and records the line numbers of
// the added side. It reads the hunk headers rather than counting, because
// a hunk header states where the added side begins and how far it runs.
func collectAddedLines(root, diff string, out map[lineKey]bool) {
	file := ""
	line := 0
	for _, raw := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(raw, "+++ "):
			// "+++ b/path", or "+++ /dev/null" for a deletion
			name := strings.TrimPrefix(raw, "+++ ")
			if name == "/dev/null" {
				file = ""
				continue
			}
			file = normalisePath(root, strings.TrimPrefix(name, "b/"))
		case strings.HasPrefix(raw, "@@"):
			line = hunkStart(raw)
		case file != "" && line > 0 && strings.HasPrefix(raw, "+"):
			out[lineKey{file: file, line: line}] = true
			line++
		}
	}
}

// hunkStart reads the added side's first line number out of "@@ -a,b +c,d @@".
func hunkStart(header string) int {
	i := strings.Index(header, "+")
	if i < 0 {
		return 0
	}
	rest := header[i+1:]
	if j := strings.IndexAny(rest, ", @"); j >= 0 {
		rest = rest[:j]
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0
	}
	return n
}

func markWholeFile(root, p string, out map[lineKey]bool) {
	full := p
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, p)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return
	}
	name := normalisePath(root, p)
	for i := range strings.Split(string(raw), "\n") {
		out[lineKey{file: name, line: i + 1}] = true
	}
}

func tracked(root, p string) bool {
	rel := p
	if filepath.IsAbs(rel) {
		if r, ok := RepoRel(root, p); ok {
			rel = r
		}
	}
	err := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", rel).Run()
	return err == nil
}

// normalisePath brings both sides to the same shape:
// findings carry whatever path the scanner produced, and a diff carries
// repository-relative slashes.
func normalisePath(root, p string) string {
	if rel, ok := RepoRel(root, p); ok {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(p)
}
