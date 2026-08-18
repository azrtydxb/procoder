// Package maintain is domain 3: maintainability, as a thin layer over what
// already exists — dead code from the index's precise tier, complexity and
// function length from the linters run with just those rules. Everything
// REPORTS: maintainability is judgment, and the agent judges (P-CONTROL).
package maintain

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"procoder/internal/codeindex"
	"procoder/internal/lint"
	"procoder/internal/tools"
)

const maintainTimeout = 120 * time.Second

var findingLine = regexp.MustCompile(`^(.+?):(\d+)(?::\d+)?:\s+(.+)$`)

// Run is `procoder maintain`: the full maintainability report. Dead-code
// candidates come from the index (precise tier); complexity and length from
// isolated linter runs so the repo's own lint config is neither required nor
// disturbed.
func Run(root string, out func(string)) int {
	count := 0

	// dead code: defined, never referenced — exported API marked, the agent
	// judges (a library's public surface is legitimately unreferenced)
	unusedLines := 0
	code := codeindex.Unused(root, func(s string) {
		out("  unused  " + s)
		unusedLines++
	})
	if code != 0 && unusedLines > 0 {
		out("  (build the index with `procoder index build` for dead-code candidates)")
	}
	count += unusedLines

	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		count += complexityGo(root, out)
	}
	if hasFiles(root, ".py") {
		count += complexityPy(root, out)
	}
	out(fmt.Sprintf("procoder maintain: %d line(s) of findings — all judgment calls, none blocking", count))
	return 0
}

// complexityGo runs golangci-lint with ONLY the complexity rules, isolated
// from the repo's config so nothing is required or overridden.
func complexityGo(root string, out func(string)) int {
	bin := tools.Resolve(lint.GolangciLint, root)
	if bin == "" {
		out("  complexity  NOT checked — golangci-lint is not installed; run `procoder init`")
		return 1
	}
	// an isolated config carries the thresholds: golangci's CLI cannot set
	// linter settings, and the defaults (gocyclo 30) report almost nothing
	cfg := filepath.Join(os.TempDir(), fmt.Sprintf("procoder-maintain-%d.yml", os.Getpid()))
	if err := os.WriteFile(cfg, []byte(maintainGolangciCfg), 0o644); err != nil {
		out("  complexity  NOT checked — cannot write the isolated config")
		return 1
	}
	defer os.Remove(cfg)
	return runTool(root, bin, []string{"run", "--config", cfg,
		"--output.text.path=stdout", "--show-stats=false", "./..."}, "complexity", out)
}

// maintainGolangciCfg is the isolated complexity/length ruleset — never the
// repo's lint config, which domain 2 already honours.
const maintainGolangciCfg = `version: "2"
linters:
  default: none
  enable:
    - gocyclo
    - funlen
  settings:
    gocyclo:
      min-complexity: 15
    funlen:
      lines: 80
      statements: 50
`

// complexityPy runs ruff with only the McCabe complexity rule.
func complexityPy(root string, out func(string)) int {
	bin := tools.Resolve(lint.Ruff, root)
	if bin == "" {
		out("  complexity  NOT checked — ruff is not installed; run `procoder init`")
		return 1
	}
	return runTool(root, bin, []string{"check", "--select", "C901", "--output-format=concise", "."}, "complexity", out)
}

func runTool(root, bin string, args []string, label string, out func(string)) int {
	ctx, cancel := context.WithTimeout(context.Background(), maintainTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- resolved from the fixed tool table, never user input
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	_ = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		out(fmt.Sprintf("  %s  NOT checked — %s gave no answer in %s", label, filepath.Base(bin), maintainTimeout))
		return 1
	}
	count := 0
	for _, line := range strings.Split(buf.String(), "\n") {
		m := findingLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || !strings.Contains(m[1], ".") {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		file := m[1]
		// golangci renders paths relative to its config file (the temp dir
		// here) — normalise back to repo-relative
		if strings.HasPrefix(file, "..") {
			if abs, err := filepath.Abs(filepath.Join(os.TempDir(), file)); err == nil {
				if rel, err2 := filepath.Rel(root, abs); err2 == nil && !strings.HasPrefix(rel, "..") {
					file = rel
				}
			}
		}
		out(fmt.Sprintf("  %s  %s:%d  %s", label, file, n, m[3]))
		count++
	}
	return count
}

func hasFiles(root, ext string) bool {
	found := false
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".claude" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ext) {
			found = true
		}
		return nil
	})
	return found
}
