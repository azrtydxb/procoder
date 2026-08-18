// Package lint is domain 2: best practices, delivered by each ecosystem's
// canonical linter. The project's own linter config always wins; procoder
// adds no rules of its own. Findings are REPORTED by default — lint is
// judgment, formatting was not — and a repo opts into blocking via
// `[lint] policy = "block"` in .procoder/config.toml (D-OVERRIDE).
package lint

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

	"procoder/internal/gitx"
	"procoder/internal/tools"
)

// GolangciLint is the canonical Go linter aggregation.
var GolangciLint = &tools.Tool{
	Name:        "golangci-lint",
	Install:     "brew install golangci-lint   (or: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest)",
	VersionArgs: []string{"version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "golangci-lint"}},
		{Manager: "go", Args: []string{"install", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"}},
	},
}

// Shellcheck lints shell scripts.
var Shellcheck = &tools.Tool{
	Name:        "shellcheck",
	Install:     "brew install shellcheck   (or: apt install shellcheck)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "shellcheck"}},
		{Manager: "apt-get", Args: []string{"install", "-y", "shellcheck"}},
	},
}

// Ruff is invoked here in `check` mode; the formatter registry holds its own
// instance for `format`.
var Ruff = &tools.Tool{
	Name:        "ruff",
	Install:     "brew install ruff   (or: pipx install ruff)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "ruff"}},
		{Manager: "pipx", Args: []string{"install", "ruff"}},
	},
}

// Eslint lints JS/TS, and only where the project carries an eslint config —
// without one procoder would be imposing rules, which it never does.
var Eslint = &tools.Tool{
	Name:               "eslint",
	NeedsProjectConfig: "eslint.config.js",
	Install:            "npm i -D eslint   (project-local preferred)",
	VersionArgs:        []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "npm", Args: []string{"install", "-g", "eslint"}},
	},
}

const lintTimeout = 120 * time.Second

// findingLine matches the shared shape all four linters can emit:
// file:line[:col]: message
var findingLine = regexp.MustCompile(`^(.+?):(\d+)(?::\d+)?:\s+(.+)$`)

// Files lints the given files with the right tool per ecosystem and returns
// findings in the gitx shape. block sets the verdict the repo chose.
func Files(root string, files []string, block bool) []gitx.Finding {
	byExt := map[string][]string{}
	for _, f := range files {
		if !filepath.IsAbs(f) {
			f = filepath.Join(root, f)
		}
		switch strings.ToLower(filepath.Ext(f)) {
		case ".go":
			if !strings.HasSuffix(f, ".pb.go") {
				byExt["go"] = append(byExt["go"], f)
			}
		case ".py":
			byExt["py"] = append(byExt["py"], f)
		case ".sh", ".bash":
			byExt["sh"] = append(byExt["sh"], f)
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
			byExt["js"] = append(byExt["js"], f)
		}
	}
	var out []gitx.Finding
	if fs := byExt["go"]; len(fs) > 0 {
		out = append(out, lintGo(root, fs, block)...)
	}
	if fs := byExt["py"]; len(fs) > 0 {
		out = append(out, run(root, Ruff, append([]string{"check", "--output-format=concise"}, fs...), fs, block)...)
	}
	if fs := byExt["sh"]; len(fs) > 0 {
		out = append(out, run(root, Shellcheck, append([]string{"--format=gcc"}, fs...), fs, block)...)
	}
	if fs := byExt["js"]; len(fs) > 0 {
		out = append(out, lintJS(root, fs, block)...)
	}
	return out
}

// lintGo runs golangci-lint over the packages holding the files and keeps
// only the findings that land in them.
func lintGo(root string, files []string, block bool) []gitx.Finding {
	bin := tools.Resolve(GolangciLint, root)
	if bin == "" {
		return notChecked(files[0], "golangci-lint")
	}
	dirs := map[string]bool{}
	for _, f := range files {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			continue
		}
		d := filepath.Dir(rel)
		if d == "." {
			dirs["./"] = true
		} else {
			dirs["./"+filepath.ToSlash(d)] = true
		}
	}
	args := []string{"run", "--output.text.path=stdout", "--show-stats=false"}
	for d := range dirs {
		args = append(args, d)
	}
	wanted := map[string]bool{}
	for _, f := range files {
		if rel, err := filepath.Rel(root, f); err == nil {
			wanted[filepath.ToSlash(rel)] = true
		}
	}
	raw, err := execute(root, bin, args)
	findings := finishParse(raw, err, files[0], "golangci-lint", block)
	var out []gitx.Finding
	for _, f := range findings {
		if wanted[filepath.ToSlash(f.File)] || f.File == files[0] {
			out = append(out, f)
		}
	}
	return out
}

// lintJS honours the eslint out-of-scope rule: no project config, no lint.
func lintJS(root string, files []string, block bool) []gitx.Finding {
	if !HasEslintConfig(root) {
		return []gitx.Finding{{File: files[0],
			Message: "eslint: no project config found — JS/TS lint is out of scope until the project carries one (lint)"}}
	}
	bin := tools.Resolve(Eslint, root)
	if bin == "" {
		return notChecked(files[0], "eslint")
	}
	raw, err := execute(root, bin, append([]string{"--format", "unix"}, files...))
	return finishParse(raw, err, files[0], "eslint", block)
}

// HasEslintConfig reports whether the project carries an eslint config —
// the condition for eslint being in scope at all.
func HasEslintConfig(root string) bool {
	for _, name := range []string{"eslint.config.js", "eslint.config.mjs", "eslint.config.cjs",
		"eslint.config.ts", ".eslintrc", ".eslintrc.json", ".eslintrc.js", ".eslintrc.yml", ".eslintrc.yaml"} {
		if fileExists(filepath.Join(root, name)) {
			return true
		}
	}
	return false
}

// run resolves the tool, executes it, and parses the shared finding shape.
func run(root string, tool *tools.Tool, args, files []string, block bool) []gitx.Finding {
	bin := tools.Resolve(tool, root)
	if bin == "" {
		return notChecked(files[0], tool.Name)
	}
	raw, err := execute(root, bin, args)
	return finishParse(raw, err, files[0], tool.Name, block)
}

// execute runs the linter with the hung-tool guard. Linters exit non-zero on
// findings; that is an answer, not an error — a run that produced neither
// findings nor a zero exit is reported as NOT checked by parse's caller
// contract (the raw output simply yields no finding lines and the tool's
// stderr is folded in so failures stay visible).
func execute(root, bin string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), lintTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- resolved from the fixed tool table, never user input
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("%s gave no answer in %s", filepath.Base(bin), lintTimeout)
	}
	return buf.String(), err
}

// finishParse applies the honesty rule to a linter run: findings are the
// answer; a failed run with NO findings must never read as clean.
func finishParse(raw string, runErr error, file string, tool string, block bool) []gitx.Finding {
	out := parse(raw, block)
	if len(out) == 0 && runErr != nil {
		var exit *exec.ExitError
		// exit code 1 with no parseable findings can still be a legitimate
		// "no findings" for some tools, but anything else is a failure
		if !(errors.As(runErr, &exit) && exit.ExitCode() == 1 && raw != "") {
			return []gitx.Finding{{File: file,
				Message: fmt.Sprintf("NOT checked — %s failed: %s (lint)", tool, firstLine(raw+runErr.Error()))}}
		}
	}
	return out
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			if len(t) > 160 {
				t = t[:160]
			}
			return t
		}
	}
	return "no output"
}

func parse(raw string, block bool) []gitx.Finding {
	var out []gitx.Finding
	for _, line := range strings.Split(raw, "\n") {
		m := findingLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		// linters echo source excerpts under findings; real finding paths
		// carry an extension or are our timeout marker
		if !strings.Contains(m[1], ".") {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		out = append(out, gitx.Finding{File: m[1], Line: n, Blocking: block,
			Message: m[3] + " (lint)"})
	}
	return out
}

func notChecked(file, tool string) []gitx.Finding {
	return []gitx.Finding{{File: file,
		Message: fmt.Sprintf("NOT checked — %s is not installed; run `procoder init` (lint)", tool)}}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
