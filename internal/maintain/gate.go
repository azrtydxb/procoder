package maintain

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"procoder/internal/gitx"
)

// gateLine matches the lines Run prints: "  complexity  file:line  text".
var gateLine = regexp.MustCompile(`^\s*complexity\s+(.+?):(\d+)\s+(.+)$`)

// ComplexityChanged is the commit gate's complexity leg: the same scan
// Run performs, with its findings narrowed to the files this commit
// carries.
//
// Narrowed by FINDING rather than by target, for the reason the SAST leg
// learned the hard way — a tool given a file list does not behave the
// same as one given a tree, and a developer blocked by a finding CI does
// not report is worse than a slower gate. golangci-lint also caches, so
// the whole-tree run costs about 340ms here.
//
// Reported, not blocking, unless the repository asks for block. The
// findings are judgement calls — a switch with twenty arms is sometimes
// exactly right — and this repository's own cmd/procoder/main.go carries
// a function of 181 statements against a threshold of 50. A default that
// blocks would stop anyone committing to that file at all, including the
// people who would have to refactor it. procoder never blocks a repo by
// surprise; `[maintain] policy = "block"` is how a team asks for it.
func ComplexityChanged(root string, files []string, block bool) []gitx.Finding {
	changed := map[string]bool{}
	for _, f := range files {
		if rel, ok := gitx.RepoRel(root, f); ok {
			changed[rel] = true
		}
	}
	if len(changed) == 0 {
		return nil
	}

	var out []gitx.Finding
	Run(root, func(line string) {
		m := gateLine.FindStringSubmatch(line)
		if m == nil {
			// A line that is not a located finding is either the summary
			// or a check that could not run. The latter must reach the
			// commit whatever it touched, or the gate would drop exactly
			// the reports that say nothing was measured.
			if strings.Contains(line, "NOT checked") || strings.Contains(line, "did NOT run") {
				out = append(out, gitx.Finding{Blocking: true,
					Message: strings.TrimSpace(line) + " (maintain)"})
			}
			return
		}
		file := filepath.ToSlash(m[1])
		if !changed[file] {
			return
		}
		n, _ := strconv.Atoi(m[2])
		out = append(out, gitx.Finding{File: file, Line: n, Blocking: block,
			Message: strings.TrimSpace(m[3]) + " (maintain)"})
	})
	return out
}
