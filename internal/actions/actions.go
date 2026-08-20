// Package actions lints GitHub Actions workflow files with actionlint — the
// canonical, zero-config tool for the job. Its findings follow the same shape
// as gitx: a diagnosis with file and line, for the agent to investigate.
package actions

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"procoder/internal/gitx"
	"procoder/internal/tools"
)

// Actionlint and Gh are registered tools so doctor and init treat them like
// any formatter: presence checked, install commands known.
var Actionlint = &tools.Tool{
	Name:        "actionlint",
	Install:     "brew install actionlint   (or: go install github.com/rhysd/actionlint/cmd/actionlint@latest)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "actionlint"}},
		{Manager: "go", Args: []string{"install", "github.com/rhysd/actionlint/cmd/actionlint@latest"}},
		{Manager: "winget", Args: []string{"install", "--id", "rhysd.actionlint", "-e"}},
	},
}

// Gh is the GitHub CLI: doctor requires it where the repository pushes to
// a GitHub remote, and the skills drive PR, merge and CI-run work through
// it rather than reimplementing the API.
var Gh = &tools.Tool{
	Name:        "gh",
	Install:     "brew install gh   (https://cli.github.com)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "gh"}},
		{Manager: "apt-get", Args: []string{"install", "-y", "gh"}},
		{Manager: "dnf", Args: []string{"install", "-y", "gh"}},
		{Manager: "winget", Args: []string{"install", "--id", "GitHub.cli", "-e"}},
	},
}

// IsWorkflowFile: a yml/yaml under .github/workflows, anywhere in the path.
func IsWorkflowFile(path string) bool {
	norm := filepath.ToSlash(path)
	ext := strings.ToLower(filepath.Ext(norm))
	return (ext == ".yml" || ext == ".yaml") && strings.Contains(norm, ".github/workflows/")
}

const hungToolTimeout = 30 * time.Second

// actionlint's default output: file:line:col: message [kind]
var lintLine = regexp.MustCompile(`^(.+?):(\d+):\d+: (.+)$`)

// Lint runs actionlint over the given workflow files. The three-way honesty
// rule applies: findings, clean, or an Unchecked-style finding when the tool
// is missing or failed — never silence for a file nobody looked at.
func Lint(files []string) []gitx.Finding {
	if len(files) == 0 {
		return nil
	}
	bin := tools.Resolve(Actionlint, "")
	if bin == "" {
		var out []gitx.Finding
		for _, f := range files {
			out = append(out, gitx.Finding{File: f, Blocking: true,
				Message: "NOT checked — actionlint is not installed; run `procoder init`"})
		}
		return out
	}

	ctx, cancel := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel()
	// -no-color keeps the output parseable; actionlint exits 1 on findings,
	// which is not an error for us.
	cmd := exec.CommandContext(ctx, bin, append([]string{"-no-color"}, files...)...) // nosemgrep -- resolved from the fixed tool table, never user input
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return []gitx.Finding{{File: files[0], Blocking: true,
			Message: fmt.Sprintf("actionlint gave no answer in %s — the process was killed; the files were NOT checked", hungToolTimeout)}}
	}

	var out []gitx.Finding
	for _, line := range strings.Split(buf.String(), "\n") {
		m := lintLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		out = append(out, gitx.Finding{File: m[1], Line: n, Blocking: true, Message: m[3]})
	}
	// A non-zero exit with nothing parseable is a failure, not a clean run.
	if err != nil && len(out) == 0 {
		return []gitx.Finding{{File: files[0], Blocking: true,
			Message: "actionlint failed without reporting findings — the files were NOT checked: " + firstLine(buf.String())}}
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}
