package docs

import (
	"os"
	"path/filepath"
	"strings"

	"procoder/internal/gitx"
)

// Commands is the canonical list of procoder commands. cmd/procoder pins
// its usage text against this list by test, so the two cannot drift; the
// coverage check below holds the documentation to the same list.
var Commands = []string{
	"audit", "check", "ci", "docs", "doctor", "format", "git", "index",
	"infra", "init", "lint", "maintain", "scrub", "security", "templates",
}

// CommandCoverage reports commands the documentation never mentions —
// correctness checks can't see absence, so completeness gets its own
// check. Only runs when the repo has a docs/ site directory; a repo
// without one is not claiming comprehensive docs.
func CommandCoverage(root string) []gitx.Finding {
	dir := filepath.Join(root, "docs")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil
	}
	var corpus strings.Builder
	for _, md := range MarkdownFiles(root) {
		if raw, err := os.ReadFile(md); err == nil {
			corpus.Write(raw)
			corpus.WriteByte('\n')
		}
	}
	text := corpus.String()
	var out []gitx.Finding
	for _, cmd := range Commands {
		if !strings.Contains(text, "procoder "+cmd) {
			out = append(out, gitx.Finding{
				Message: "documentation never mentions `procoder " + cmd + "` — a shipped command a reader cannot discover (docs)"})
		}
	}
	return out
}
