// Package maintain is domain 3: maintainability, as a thin layer over what
// already exists — dead code from the index's precise tier, complexity and
// function length from the linters run with just those rules. Everything
// REPORTS: maintainability is judgment, and the agent judges (P-CONTROL).
package maintain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"procoder/internal/codeindex"
	"procoder/internal/config"
	"procoder/internal/lint"
	"procoder/internal/textutil"
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
	code := codeindex.Unused(root, func(line string) {
		out("  unused  " + line)
		// only location-shaped lines are findings; status and summary
		// lines must not inflate the count
		if findingLine.MatchString(line) {
			unusedLines++
		}
	})
	if code != 0 && unusedLines == 0 {
		out("  (build the index with `procoder index build` for dead-code candidates)")
	}
	count += unusedLines

	cfg := config.Load(root)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		count += complexityGo(root, cfg, out)
	}
	if hasFiles(root, ".py") {
		count += complexityPy(root, out)
	}
	out(fmt.Sprintf("procoder maintain: %d line(s) of findings — all judgment calls, none blocking", count))
	return 0
}

// complexityGo runs golangci-lint with ONLY the complexity rules, isolated
// from the repo's config so nothing is required or overridden.
func complexityGo(root string, cfg config.Config, out func(string)) int {
	bin := tools.Resolve(lint.GolangciLint, root)
	if bin == "" {
		out("  complexity  NOT checked — golangci-lint is not installed; run `procoder init`")
		return 1
	}
	// an isolated config carries the thresholds: golangci's CLI cannot set
	// linter settings, and the defaults (gocyclo 30) report almost nothing.
	// The repo overrides them in .procoder/config.toml ([maintain] gocyclo,
	// funlen_lines, funlen_statements) — D-OVERRIDE.
	cfgFile, err := os.CreateTemp("", "procoder-maintain-*.yml")
	if err != nil {
		out("  complexity  NOT checked — cannot write the isolated config")
		return 1
	}
	cfgPath := cfgFile.Name()
	// a temp file that will not delete is nothing the report can act on
	defer func() { _ = os.Remove(cfgPath) }()
	// a failed Close can mean the write never hit disk — a config that is
	// not there produces a check that silently measured nothing
	_, werr := cfgFile.WriteString(golangciCfg(cfg))
	if cerr := cfgFile.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		out("  complexity  NOT checked — cannot write the isolated config")
		return 1
	}
	return runTool(root, bin, []string{"run", "--config", cfgPath,
		"--output.text.path=stdout", "--show-stats=false", "./..."}, "complexity", out)
}

// Default thresholds when the repo sets none.
const (
	defaultGocyclo          = 15
	defaultFunlenLines      = 80
	defaultFunlenStatements = 50
)

func golangciCfg(cfg config.Config) string {
	gocyclo, lines, stmts := cfg.Gocyclo, cfg.FunlenLines, cfg.FunlenStatements
	if gocyclo == 0 {
		gocyclo = defaultGocyclo
	}
	if lines == 0 {
		lines = defaultFunlenLines
	}
	if stmts == 0 {
		stmts = defaultFunlenStatements
	}
	return fmt.Sprintf(`version: "2"
linters:
  default: none
  enable:
    - gocyclo
    - funlen
  settings:
    gocyclo:
      min-complexity: %d
    funlen:
      lines: %d
      statements: %d
issues:
  # golangci keeps only the FIRST issue per line by default, and a long
  # function is usually a branchy one — so every funlen finding that
  # shares a line with a gocyclo finding was silently dropped. The two
  # say different things about the same function and the reader judges
  # both.
  uniq-by-line: false
`, gocyclo, lines, stmts)
}

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
	cmd.Env = append(os.Environ(), "GOLANGCI_LINT_CACHE="+lint.CacheDir(root))
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	runErr := cmd.Run()
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
	// a failed run that produced no findings must never read as clean —
	// exit 1 with output is the linters' ordinary "findings exist" answer
	if count == 0 && runErr != nil {
		var exit *exec.ExitError
		if !(errors.As(runErr, &exit) && exit.ExitCode() == 1 && buf.Len() > 0) {
			out(fmt.Sprintf("  %s  NOT checked — %s failed: %s", label, filepath.Base(bin), textutil.FirstLine(buf.String()+runErr.Error())))
			return 1
		}
	}
	return count
}

func hasFiles(root, ext string) bool {
	found := false
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		// an unreadable subdirectory is skipped, not fatal; an unreadable
		// ROOT is no survey at all, and must not answer "nothing here"
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}
		if found {
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
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil && !found {
		// the survey failed, so "no files of this type" is not something we
		// know — assume there may be some and let the ecosystem's own tool
		// answer, which reports NOT checked when it cannot
		return true
	}
	return found
}
