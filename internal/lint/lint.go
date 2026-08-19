// Package lint is domain 2: best practices, delivered by each ecosystem's
// canonical linter. The project's own linter config always wins; procoder
// adds no rules of its own. Findings are REPORTED by default — lint is
// judgment, formatting was not — and a repo opts into blocking via
// `[lint] policy = "block"` in .procoder/config.toml (D-OVERRIDE).
package lint

import (
	"bytes"
	"context"
	"encoding/json"
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

// lintJS runs eslint. The project's config always wins; where none exists,
// a generated baseline of eslint's BUILT-IN core rules fills the void —
// procoder still imposes nothing on the repo (the file lives in the temp
// dir), and baseline findings are labeled as such. eslint v10 dropped the
// unix formatter from core, so both paths parse JSON.
func lintJS(root string, files []string, block bool) []gitx.Finding {
	bin := tools.Resolve(Eslint, root)
	if HasEslintConfig(root) {
		if bin == "" {
			return notChecked(files[0], "eslint")
		}
		raw, err := execute(root, bin, append([]string{"--format", "json"}, files...))
		return parseEslintJSON(raw, err, files[0], "lint", block)
	}
	// no project config: plain JS gets the built-in-rules baseline;
	// TypeScript needs a parser eslint core does not carry, and installing
	// one would be imposing — TS is out of scope until the project carries
	// a config, whether or not eslint itself is installed.
	var jsFiles []string
	var out []gitx.Finding
	for _, f := range files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx":
			out = append(out, gitx.Finding{File: f,
				Message: "eslint: TypeScript needs a project eslint config (a TS parser is not built in) — out of scope until the project carries one (lint)"})
		default:
			jsFiles = append(jsFiles, f)
		}
	}
	if len(jsFiles) == 0 {
		return out
	}
	if bin == "" {
		return append(out, notChecked(jsFiles[0], "eslint")...)
	}
	cfg := filepath.Join(os.TempDir(), fmt.Sprintf("procoder-eslint-%d.mjs", os.Getpid()))
	if err := os.WriteFile(cfg, []byte(baselineEslintConfig), 0o644); err != nil {
		return append(out, notChecked(jsFiles[0], "eslint (baseline config unwritable)")...)
	}
	defer os.Remove(cfg)
	// paths go relative to the repo root: node resolves the cwd through
	// symlinks (macOS /var vs /private/var) and absolute args would read
	// as outside the base path
	relFiles := make([]string, 0, len(jsFiles))
	for _, f := range jsFiles {
		if rel, err := filepath.Rel(root, f); err == nil && !strings.HasPrefix(rel, "..") {
			relFiles = append(relFiles, rel)
		} else {
			relFiles = append(relFiles, f)
		}
	}
	raw, err := execute(root, bin, append([]string{"--format", "json", "--no-config-lookup", "--config", cfg}, relFiles...))
	return append(out, parseEslintJSON(raw, err, jsFiles[0], "lint, procoder baseline", block)...)
}

// baselineEslintConfig uses only rules built into eslint core — no imports,
// no npm packages — with common runtime globals so no-undef is signal.
const baselineEslintConfig = `export default [{
  basePath: process.cwd(),
  rules: {
    "no-unused-vars": "warn",
    "no-undef": "error",
    "eqeqeq": "warn",
    "no-var": "warn",
    "no-debugger": "error",
    "no-dupe-keys": "error",
    "no-unreachable": "error",
    "no-constant-condition": "warn",
    "no-self-assign": "warn",
    "no-fallthrough": "warn"
  },
  languageOptions: { globals: {
    console: "readonly", process: "readonly", require: "readonly",
    module: "writable", exports: "writable", window: "readonly",
    document: "readonly", setTimeout: "readonly", setInterval: "readonly",
    clearTimeout: "readonly", clearInterval: "readonly", fetch: "readonly",
    Buffer: "readonly", __dirname: "readonly", __filename: "readonly",
    globalThis: "readonly", URL: "readonly", Promise: "readonly"
  } }
}]
`

// parseEslintJSON reads eslint's --format json output; the honesty rule
// applies — a failed run with nothing parsed never reads clean.
func parseEslintJSON(raw string, runErr error, file, label string, block bool) []gitx.Finding {
	var report []struct {
		FilePath string `json:"filePath"`
		Messages []struct {
			Line    int    `json:"line"`
			Message string `json:"message"`
			RuleID  string `json:"ruleId"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return []gitx.Finding{{File: file,
			Message: fmt.Sprintf("NOT checked — eslint output unreadable: %s (lint)", firstLine(raw+errStr(runErr)))}}
	}
	var out []gitx.Finding
	for _, f := range report {
		for _, m := range f.Messages {
			rule := m.RuleID
			if rule == "" {
				rule = "parse"
			}
			out = append(out, gitx.Finding{File: f.FilePath, Line: m.Line, Blocking: block,
				Message: fmt.Sprintf("%s [%s] (%s)", m.Message, rule, label)})
		}
	}
	if len(out) == 0 && runErr != nil {
		var exit *exec.ExitError
		if !(errors.As(runErr, &exit) && exit.ExitCode() == 1) {
			return []gitx.Finding{{File: file,
				Message: fmt.Sprintf("NOT checked — eslint failed: %s (lint)", firstLine(errStr(runErr)))}}
		}
	}
	return out
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
