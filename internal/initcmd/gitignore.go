package initcmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Root-anchored host directories must not hide similarly named application
// files. GitHub workflows and unrelated skills remain trackable.
var hostIgnores = []string{
	"/.agents/", "/.claude/", "/.clinerules/", "/.codex/", "/.cursor/",
	"/.kilo/", "/.kilocode/", "/.kiro/", "/.opencode/", "/.qoder/",
	"/.roo/", "/.windsurf/", "/.github/copilot-instructions.md",
	"/skills/procoder/",
}

// IgnoreHosts is the established init write: only selected local host paths
// are ignored; the shared .procoder directory always remains trackable.
func IgnoreHosts(root string, out io.Writer, hosts []string) error {
	var patterns []string
	for _, h := range hosts {
		switch h {
		case "all":
			patterns = append(patterns, hostIgnores...)
		case "agents":
		case "copilot":
			patterns = append(patterns, "/.github/copilot-instructions.md")
		case "skills":
			patterns = append(patterns, "/skills/procoder/")
		case "cline":
			patterns = append(patterns, "/.clinerules/")
		case "antigravity":
			patterns = append(patterns, "/.agents/")
		case "claude", "codex", "cursor", "kilo", "kiro", "opencode", "qoder", "roo", "windsurf":
			patterns = append(patterns, "/."+h+"/")
		}
	}
	if len(patterns) == 0 {
		return nil
	}
	path := filepath.Join(root, ".gitignore")
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(raw), "\n") {
		seen[strings.TrimSuffix(line, "\r")] = true
	}
	var missing []string
	for _, pattern := range patterns {
		if !seen[pattern] {
			missing = append(missing, pattern)
		}
	}
	if len(missing) == 0 {
		fmt.Fprintln(out, "procoder init: local AI host entries are in .gitignore (already tracked files are unchanged)")
		return nil
	}
	addition := ""
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		addition = "\n"
	}
	addition += "\n# Local AI coding hosts (procoder init); .procoder/ stays trackable.\n"
	addition += strings.Join(missing, "\n") + "\n"
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(f, addition)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Fprintln(out, "procoder init: local AI host entries are in .gitignore (already tracked files are unchanged)")
	return nil
}
