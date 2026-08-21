package lint

import (
	"os"
	"path/filepath"
	"strings"
)

// RulesPath is the lint domain's repo-overridable rules file, following
// the pattern docs and security already use: prose for the agent, with
// list sections a machine reads. A section that is present replaces the
// default for that section; a section that is absent keeps it (D-OVERRIDE,
// and the "absent means default" rule the rest of .procoder/ lives by).
const RulesPath = ".procoder/lint/RULES.md"

// Rules is what the repository's RULES.md dictates. The zero value means
// "use procoder's defaults", so a repo with no file behaves exactly as it
// did before the file existed.
type Rules struct {
	// Checks replaces the curated clang-tidy check set. Empty means the
	// baseline stands.
	Checks []string
}

// LoadRules reads the repository's lint rules.
func LoadRules(root string) Rules {
	var r Rules
	data, err := os.ReadFile(filepath.Join(root, RulesPath))
	if err != nil {
		return r
	}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") {
			section = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(t, "## ")))
			continue
		}
		if !strings.HasPrefix(t, "- ") {
			continue
		}
		item := strings.TrimSpace(strings.Trim(strings.TrimPrefix(t, "- "), "`"))
		if item == "" {
			continue
		}
		if section == "checks" {
			r.Checks = append(r.Checks, item)
		}
	}
	return r
}

// checkSet is the clang-tidy --checks value: the repository's list when it
// wrote one, procoder's curated families otherwise.
func checkSet(root string) string {
	if c := LoadRules(root).Checks; len(c) > 0 {
		return strings.Join(c, ",")
	}
	return clangTidyBaseline
}
