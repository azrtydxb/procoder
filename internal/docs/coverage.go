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
	"agents", "audit", "backlog", "check", "ci", "debt", "docs", "doctor",
	"format", "git", "hook", "index", "infra", "init", "lessons", "lint",
	"maintain", "plan", "principles", "scrub", "security", "spec",
	"sprint", "templates", "test", "todo", "version",
}

// CommandCoverage reports commands the documentation never mentions —
// correctness checks can't see absence, so completeness gets its own
// check. This is a self-check: Commands is procoder's own list, so it
// only runs inside the procoder source tree itself (go.mod declares
// `module procoder`) — any other repo does not ship these commands and
// must not be gated on documenting them. It also needs a docs/ site
// directory; a repo without one is not claiming comprehensive docs.
func CommandCoverage(root string) []gitx.Finding {
	if !isProcoderRepo(root) {
		return nil
	}
	dir := filepath.Join(root, "docs")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil
	}
	// only the site and the README count as documentation — a mention in
	// a rules file or template must not mask a missing reference page
	var corpus strings.Builder
	for _, md := range MarkdownFiles(root) {
		rel, err := filepath.Rel(root, md)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel != "README.md" && !strings.HasPrefix(rel, "docs/") {
			continue
		}
		if raw, err := os.ReadFile(md); err == nil {
			corpus.Write(raw)
			corpus.WriteByte('\n')
		}
	}
	text := corpus.String()
	var out []gitx.Finding
	for _, cmd := range Commands {
		if !strings.Contains(text, "procoder "+cmd) {
			out = append(out, gitx.Finding{Blocking: true,
				Message: "documentation never mentions `procoder " + cmd + "` — a shipped command a reader cannot discover (docs)"})
		}
	}
	return out
}

// isProcoderRepo reports whether root is the procoder source tree: the one
// repository that ships the Commands list and therefore owes each command a
// documentation mention.
func isProcoderRepo(root string) bool {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return false
	}
	for _, line := range strings.SplitN(string(raw), "\n", 4) {
		if strings.TrimSpace(line) == "module procoder" {
			return true
		}
	}
	return false
}
