// Package lint is domain 2: best practices, delivered by each ecosystem's
// canonical linter. The project's own linter config always wins; procoder
// adds no rules of its own. Findings are REPORTED by default — lint is
// judgment, formatting was not — and a repo opts into blocking via
// `[lint] policy = "block"` in .procoder/config.toml (D-OVERRIDE).
package lint

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"procoder/internal/gitx"
	"procoder/internal/textutil"
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

// Ktlint lints Kotlin; its default plain output is file:line:col: message.
var Ktlint = &tools.Tool{
	Name:        "ktlint",
	Install:     "brew install ktlint",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "ktlint"}},
	},
}

// Swiftlint lints Swift; the xcode reporter emits file:line:col: severity: msg.
var Swiftlint = &tools.Tool{
	Name:        "swiftlint",
	Install:     "brew install swiftlint",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "swiftlint"}},
	},
}

// RubocopLint is rubocop in lint mode (the formatter registry holds its
// autocorrect mode); emacs format emits file:line:col: severity: msg.
var RubocopLint = &tools.Tool{
	Name:        "rubocop",
	Install:     "brew install rubocop   (or: gem install rubocop)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "rubocop"}},
		{Manager: "gem", Args: []string{"install", "rubocop"}},
	},
}

// Cargo carries clippy for Rust; `--message-format short` emits
// file:line:col: severity: msg lines.
var Cargo = &tools.Tool{
	Name:        "cargo",
	Install:     "rustup: https://rustup.rs (clippy: rustup component add clippy)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "rustup"}},
	},
}

// Checkstyle lints Java; the bundled google_checks is the baseline when
// the repo carries no checkstyle.xml of its own (D-OVERRIDE as usual).
var Checkstyle = &tools.Tool{
	Name:        "checkstyle",
	Install:     "brew install checkstyle",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "checkstyle"}},
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
		case ".py", ".pyi":
			// .pyi is a stub file and ruff lints it; it was formatted and
			// never linted, which is the same silence by omission as the
			// TypeScript module extensions below.
			byExt["py"] = append(byExt["py"], f)
		case ".sh", ".bash":
			byExt["sh"] = append(byExt["sh"], f)
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
			// .mts and .cts are TypeScript's module variants. They were
			// formatted and never linted while .mjs and .cjs were — not a
			// decision anybody made, just an extension list written once
			// and never revisited.
			byExt["js"] = append(byExt["js"], f)
		case ".kt", ".kts":
			byExt["kt"] = append(byExt["kt"], f)
		case ".swift":
			byExt["swift"] = append(byExt["swift"], f)
		case ".rb", ".rake":
			byExt["rb"] = append(byExt["rb"], f)
		case ".rs":
			byExt["rs"] = append(byExt["rs"], f)
		case ".java":
			byExt["java"] = append(byExt["java"], f)
		case ".php":
			byExt["php"] = append(byExt["php"], f)
		case ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp":
			byExt["c"] = append(byExt["c"], f)
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
	if fs := byExt["kt"]; len(fs) > 0 {
		out = append(out, run(root, Ktlint, fs, fs, block)...)
	}
	if fs := byExt["swift"]; len(fs) > 0 {
		out = append(out, run(root, Swiftlint, append([]string{"lint", "--quiet", "--reporter", "xcode"}, fs...), fs, block)...)
	}
	if fs := byExt["rb"]; len(fs) > 0 {
		out = append(out, run(root, RubocopLint, append([]string{"--format", "emacs", "--no-color"}, fs...), fs, block)...)
	}
	if fs := byExt["rs"]; len(fs) > 0 {
		out = append(out, lintRust(root, fs, block)...)
	}
	if fs := byExt["java"]; len(fs) > 0 {
		out = append(out, lintJava(root, fs, block)...)
	}
	if fs := byExt["php"]; len(fs) > 0 {
		out = append(out, lintPHP(root, fs, block)...)
	}
	if fs := byExt["c"]; len(fs) > 0 {
		out = append(out, lintCFamily(root, fs, block)...)
	}
	// Languages procoder formats and has no linter for say so, once per
	// language, rather than passing silently.
	out = append(out, lintUnlinted(files, block)...)
	return out
}

// lintRust runs clippy over the whole cargo workspace (clippy has no
// per-file mode) and keeps the findings landing in the changed files.
func lintRust(root string, files []string, block bool) []gitx.Finding {
	if !fileExists(filepath.Join(root, "Cargo.toml")) {
		return []gitx.Finding{{Blocking: true, File: files[0],
			Message: "NOT checked — clippy needs Cargo.toml at the repository root (a crate in a subdirectory is a known ceiling for now) (lint)"}}
	}
	bin := tools.Resolve(Cargo, root)
	if bin == "" {
		return notChecked(files[0], "cargo (clippy)")
	}
	raw, err := execute(root, bin, []string{"clippy", "--quiet", "--message-format", "short"})
	findings := finishParse(raw, err, files[0], "clippy", block)
	wanted := map[string]bool{}
	for _, f := range files {
		if rel, rerr := filepath.Rel(root, f); rerr == nil {
			wanted[filepath.ToSlash(rel)] = true
		}
	}
	var out []gitx.Finding
	for _, f := range findings {
		if wanted[filepath.ToSlash(f.File)] || f.File == files[0] {
			out = append(out, f)
		}
	}
	return out
}

// lintJava runs checkstyle; the repo's own checkstyle.xml wins, the
// bundled google_checks is the baseline otherwise. checkstyle prefixes
// every line with a [WARN]/[ERROR] tag the shared parser would fold into
// the file path, so it is stripped first.
func lintJava(root string, files []string, block bool) []gitx.Finding {
	cfg := "/google_checks.xml" // bundled inside the checkstyle jar
	if fileExists(filepath.Join(root, "checkstyle.xml")) {
		cfg = "checkstyle.xml"
	}
	bin := tools.Resolve(Checkstyle, root)
	if bin == "" {
		return notChecked(files[0], "checkstyle")
	}
	raw, err := execute(root, bin, append([]string{"-c", cfg}, files...))
	raw = checkstyleTagRe.ReplaceAllString(raw, "")
	return finishParse(raw, err, files[0], "checkstyle", block)
}

var checkstyleTagRe = regexp.MustCompile(`(?m)^\[(WARN|ERROR|INFO)\] `)

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
	// No issue caps. golangci-lint defaults to --max-issues-per-linter 50
	// and --max-same-issues 3, and it lints packages concurrently — so
	// WHICH issues survive those caps depends on which package finished
	// first. Two runs over an unchanged tree reported forty-eight findings
	// each and disagreed about their members, which made "did my change
	// alter this?" unanswerable (#236).
	//
	// The caps were also hiding work: max-same-issues 3 keeps three of any
	// near-identical message, and errcheck emits almost nothing else, so
	// most of them never reached the report at all.
	args := []string{"run", "--output.text.path=stdout", "--show-stats=false",
		"--max-issues-per-linter=0", "--max-same-issues=0"}
	// the project's own config always wins; without one, procoder supplies
	// a curated baseline strong enough to catch the bug classes reviewers
	// keep finding (security edges, error handling, loop allocations) —
	// the file lives in the temp dir, nothing is imposed on the repo
	// (requires golangci-lint v2, the version doctor installs)
	var baselineNote []gitx.Finding
	if !hasGolangciConfig(root) {
		cfgFile, err := os.CreateTemp("", "procoder-golangci-*.yml")
		if err == nil {
			_, err = cfgFile.WriteString(golangciBaseline)
			// a failed Close can mean the write never hit disk — treat it
			// as a write failure, not a success
			if cerr := cfgFile.Close(); err == nil {
				err = cerr
			}
			defer os.Remove(cfgFile.Name())
		}
		if err == nil {
			args = append(args, "--config", cfgFile.Name())
		} else {
			// the run still happens on stock defaults, but silently losing
			// the curated set would be dishonest — say so
			baselineNote = append(baselineNote, gitx.Finding{File: files[0],
				Message: "go lint ran WITHOUT the procoder baseline (cannot write temp config: " + err.Error() + ") (lint)"})
		}
	}
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
	out := baselineNote
	for _, f := range findings {
		if wanted[filepath.ToSlash(f.File)] || f.File == files[0] {
			out = append(out, f)
		}
	}
	return out
}

// golangciBaseline is the curated default when a repo carries no golangci
// config: the standard set plus the linters that catch the classes code
// review keeps finding after the fact — security edges (gosec), API misuse
// and per-iteration work (gocritic), error-wrapping mistakes (errorlint),
// dead parameters (unparam), and nil-on-error returns (nilerr).
const golangciBaseline = `version: "2"
linters:
  default: standard
  enable:
    - gosec
    - gocritic
    - errorlint
    - unparam
    - copyloopvar
    - nilerr
`

// hasGolangciConfig reports whether the project carries its own golangci
// config — the repo's file always wins over the baseline (D-OVERRIDE).
func hasGolangciConfig(root string) bool {
	for _, name := range []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}
	return false
}

// lintJS runs eslint. The project's config always wins and is resolved the
// way eslint itself resolves flat config — the nearest eslint config
// ascending from each linted file — so a config in a subdirectory (web/,
// packages/app/) counts, not just one at the repo root. Files under a
// config are linted from that config's directory; only files no config
// covers get the baseline of eslint's BUILT-IN core rules — procoder still
// imposes nothing on the repo (the file lives in the temp dir), and
// baseline findings are labeled as such. eslint v10 dropped the unix
// formatter from core, so both paths parse JSON.
func lintJS(root string, files []string, block bool) []gitx.Finding {
	byCfg := map[string][]string{}
	var uncovered []string
	for _, f := range files {
		if dir := nearestEslintConfigDir(root, f); dir != "" {
			byCfg[dir] = append(byCfg[dir], f)
		} else {
			uncovered = append(uncovered, f)
		}
	}
	var out []gitx.Finding
	cfgDirs := make([]string, 0, len(byCfg))
	for d := range byCfg {
		cfgDirs = append(cfgDirs, d)
	}
	sort.Strings(cfgDirs)
	for _, dir := range cfgDirs {
		fs := byCfg[dir]
		// a project-local eslint may live beside the config, not the root
		bin := tools.Resolve(Eslint, dir)
		if bin == "" {
			bin = tools.Resolve(Eslint, root)
		}
		if bin == "" {
			out = append(out, notChecked(fs[0], "eslint")...)
			continue
		}
		// run from the config's directory: eslint's own config search starts
		// there, so the nearest config is the one that gets used
		raw, err := execute(dir, bin, append([]string{"--format", "json"}, fs...))
		out = append(out, parseEslintJSON(raw, err, fs[0], "lint", block)...)
	}
	if len(uncovered) == 0 {
		return out
	}
	bin := tools.Resolve(Eslint, root)
	// no config covers these files: plain JS gets the built-in-rules
	// baseline; TypeScript needs a parser eslint core does not carry, and
	// installing one would be imposing — TS is out of scope until the
	// project carries a config, whether or not eslint itself is installed.
	var jsFiles, tsFiles []string
	for _, f := range uncovered {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx", ".mts", ".cts":
			tsFiles = append(tsFiles, f)
		default:
			jsFiles = append(jsFiles, f)
		}
	}
	if len(tsFiles) > 0 {
		out = append(out, lintTSBaseline(root, tsFiles, block)...)
	}
	if len(jsFiles) == 0 {
		return out
	}
	if bin == "" {
		return append(out, notChecked(jsFiles[0], "eslint")...)
	}
	cfgFile, err := os.CreateTemp("", "procoder-eslint-*.mjs")
	if err != nil {
		return append(out, notChecked(jsFiles[0], "eslint (baseline config unwritable)")...)
	}
	cfg := cfgFile.Name()
	defer os.Remove(cfg)
	// a failed Close can mean the write never hit disk — treat it as a
	// write failure, exactly as lintGo does; the rubric names this case.
	//
	// debt: this branch is unprotected by tests, revisit if the temp-config
	// write grows an interface — os.File.Close cannot be made to fail on
	// demand without a seam, and an interface would make the class testable.
	_, werr := cfgFile.WriteString(baselineEslintConfig)
	if cerr := cfgFile.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return append(out, notChecked(jsFiles[0], "eslint (baseline config unwritable)")...)
	}
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
		return []gitx.Finding{{Blocking: true, File: file,
			Message: fmt.Sprintf("NOT checked — eslint output unreadable: %s (lint)", textutil.FirstLine(raw+errStr(runErr)))}}
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
			return []gitx.Finding{{Blocking: true, File: file,
				Message: fmt.Sprintf("NOT checked — eslint failed: %s (lint)", textutil.FirstLine(errStr(runErr)))}}
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

// eslintConfigNames are the config files eslint recognises, flat first.
var eslintConfigNames = []string{"eslint.config.js", "eslint.config.mjs", "eslint.config.cjs",
	"eslint.config.ts", ".eslintrc", ".eslintrc.json", ".eslintrc.js", ".eslintrc.yml", ".eslintrc.yaml"}

// HasEslintConfig reports whether the project carries an eslint config at
// its root — the condition for eslint being in scope at all.
func HasEslintConfig(root string) bool {
	for _, name := range eslintConfigNames {
		if fileExists(filepath.Join(root, name)) {
			return true
		}
	}
	return false
}

// nearestEslintConfigDir ascends from the file's directory to the repo root
// looking for an eslint config, the way eslint resolves flat config. Empty
// means no config covers the file.
func nearestEslintConfigDir(root, file string) string {
	dir := filepath.Dir(file)
	for {
		for _, name := range eslintConfigNames {
			if fileExists(filepath.Join(dir, name)) {
				return dir
			}
		}
		if dir == root {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
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
			return []gitx.Finding{{Blocking: true, File: file,
				Message: fmt.Sprintf("NOT checked — %s failed: %s (lint)", tool, textutil.FirstLine(raw+runErr.Error()))}}
		}
	}
	return out
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

// CacheDir gives each repository root its own tool-cache directory.
func CacheDir(root string) string {
	// A directory name derived from a path, so each repository gets its own
	// tool cache. No security property rests on it.
	sum := sha1.Sum([]byte(root)) // nosemgrep: use-of-sha1
	return filepath.Join(os.TempDir(), "procoder-cache-"+hex.EncodeToString(sum[:6]))
}

// notChecked is a check that did NOT happen. It blocks, and the blocking is
// not governed by [lint] policy: that setting decides whether a linter's
// JUDGEMENTS stop a commit, and "the linter never ran" is not a judgement.
// Domain 1 has always blocked on a missing gitleaks; domain 2 printed the
// same sentence as info and let the gate pass, which made an empty machine
// indistinguishable from clean code.
func notChecked(file, tool string) []gitx.Finding {
	return []gitx.Finding{{Blocking: true, File: file,
		Message: fmt.Sprintf("NOT checked — %s is not installed; run `procoder init` (lint)", tool)}}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
