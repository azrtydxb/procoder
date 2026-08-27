package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// checkstyle prefixes every line with a severity tag; stripped, the
// shared parser must yield clean file paths — pinned so a checkstyle
// output-format change fails here, not in a user's repo.
func TestCheckstyleTagsStripBeforeParse(t *testing.T) {
	raw := "[WARN] /abs/path/App.java:5:10: Missing a Javadoc comment. [MissingJavadocMethod]\n" +
		"[ERROR] src/Main.java:3: Utility classes should not have a public constructor. [HideUtilityClassConstructor]\n"
	stripped := checkstyleTagRe.ReplaceAllString(raw, "")
	findings := parse(stripped, false)
	if len(findings) != 2 {
		t.Fatalf("want 2 findings, got %+v", findings)
	}
	for _, f := range findings {
		if strings.Contains(f.File, "[") || strings.Contains(f.File, "WARN") {
			t.Errorf("severity tag leaked into the file path: %q", f.File)
		}
	}
	if findings[0].Line != 5 || findings[1].Line != 3 {
		t.Errorf("lines wrong: %+v", findings)
	}
}

// clippy --message-format short emits file:line:col: severity: msg —
// the shared parser must read it as-is.
func TestClippyShortFormatParses(t *testing.T) {
	raw := "src/main.rs:10:9: warning: unused variable: `x`\n" +
		"src/lib.rs:42:1: error: mismatched types\n"
	findings := parse(raw, false)
	if len(findings) != 2 {
		t.Fatalf("want 2 findings, got %+v", findings)
	}
	if findings[0].File != "src/main.rs" || findings[0].Line != 10 {
		t.Errorf("first finding wrong: %+v", findings[0])
	}
}

func TestParseKeepsFindingsAndDropsExcerpts(t *testing.T) {
	raw := `internal/gitx/gitx_test.go:41:14: Error return value not checked (errcheck)
	os.WriteFile(p, []byte("x"), 0o644)
	            ^
script.sh:3:1: warning: quote this [SC2086]
m.py:7:5: F401 imported but unused
garbage line
`
	got := parse(raw, false)
	if len(got) != 3 {
		t.Fatalf("want 3 findings, got %d: %+v", len(got), got)
	}
	if got[0].File != "internal/gitx/gitx_test.go" || got[0].Line != 41 {
		t.Fatalf("wrong first finding: %+v", got[0])
	}
	for _, f := range got {
		if f.Blocking {
			t.Fatal("report mode must not block")
		}
	}
	if b := parse(raw, true); !b[0].Blocking {
		t.Fatal("block mode must block")
	}
}

// Copilot suggested the lazy file capture drops Windows drive paths; the
// lazy quantifier expands until the rest matches, so it does not — pinned.
func TestParseHandlesWindowsDrivePaths(t *testing.T) {
	got := parse(`C:\dir\file.go:12: something wrong (lint)`+"\n", false)
	if len(got) != 1 || got[0].File != `C:\dir\file.go` || got[0].Line != 12 {
		t.Fatalf("windows path must parse: %+v", got)
	}
}

// A linter that fails without findings must never read as clean.
func TestFailedRunReadsAsNotChecked(t *testing.T) {
	got := finishParse("", fmt.Errorf("exec format error"), "x.py", "ruff", false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("failed run must surface: %+v", got)
	}
	// a timeout arrives as an error with empty output — same rule
	got = finishParse("", fmt.Errorf("ruff gave no answer in 2m0s"), "x.py", "ruff", false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "gave no answer") {
		t.Fatalf("timeout must surface: %+v", got)
	}
}

func TestMissingLinterReadsAsNotCheckedNeverClean(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // keep the conventional-dir fallback empty too
	got := Files("/tmp", []string{"/tmp/x.py"}, false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("missing ruff must read NOT checked: %+v", got)
	}
}

// TypeScript with no eslint config used to be declared out of scope, on
// the reasoning that eslint core carries no TS parser and installing one
// would be imposing. The result was that the most common TypeScript setup
// there is — a tsconfig and no eslint config — got no linting and a green
// gate. The parser is a tool, so it is installed like one, and its absence
// is NOT checked rather than silence.
// proved by: restored the "out of scope" branch for .ts — configless
// TypeScript reports one non-blocking line and the gate passes over code
// nothing read.
func TestConfiglessTSIsLintedOrLoudlyNotChecked(t *testing.T) {
	root := t.TempDir()
	got := Files(root, []string{filepath.Join(root, "a.ts")}, false)
	if len(got) == 0 {
		t.Fatal("configless TypeScript must never pass silently")
	}
	for _, f := range got {
		if strings.Contains(f.Message, "out of scope") {
			t.Errorf("TypeScript is not out of scope any more: %q", f.Message)
		}
	}
	// Whatever could not run has to block: that is the difference between
	// "checked and clean" and "nothing looked at it".
	if !got[0].Blocking && strings.Contains(got[0].Message, "NOT checked") {
		t.Errorf("a check that did not happen must block: %+v", got[0])
	}

	if tools.Resolve(Eslint, "") == "" {
		t.Skip("eslint not installed; baseline leg runs where it is")
	}
	bad := filepath.Join(root, "bad.js")
	os.WriteFile(bad, []byte("var x = 1\nif (y == 2) { console.log(x) }\n"), 0o644)
	got = Files(root, []string{bad}, false)
	joined := ""
	for _, f := range got {
		joined += f.Message + "\n"
	}
	for _, want := range []string{"no-var", "eqeqeq", "no-undef", "procoder baseline"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("baseline must flag %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "console") {
		t.Fatalf("runtime globals must not be no-undef noise:\n%s", joined)
	}
}

// The nearest config wins, ascending from the linted file — a config in a
// subdirectory (web/, packages/app/) counts, not just one at the repo root.
func TestNearestEslintConfigAscendsFromTheFile(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	os.MkdirAll(filepath.Join(webDir, "src"), 0o755)
	os.WriteFile(filepath.Join(webDir, "eslint.config.mjs"), []byte("export default []\n"), 0o644)

	if got := nearestEslintConfigDir(root, filepath.Join(webDir, "src", "app.jsx")); got != webDir {
		t.Fatalf("file under web/ must resolve to web/'s config, got %q", got)
	}
	if got := nearestEslintConfigDir(root, filepath.Join(root, "tool.js")); got != "" {
		t.Fatalf("file outside any config's tree must be uncovered, got %q", got)
	}

	os.WriteFile(filepath.Join(root, ".eslintrc.json"), []byte("{}\n"), 0o644)
	if got := nearestEslintConfigDir(root, filepath.Join(webDir, "src", "app.jsx")); got != webDir {
		t.Fatalf("the NEAREST config must win over the root's, got %q", got)
	}
	if got := nearestEslintConfigDir(root, filepath.Join(root, "tool.js")); got != root {
		t.Fatalf("root config must cover root-level files, got %q", got)
	}
}

// A file covered by a subdirectory config is linted under THAT config, not
// the procoder baseline — the audit path regression from issue #28.
func TestSubdirConfigWinsOverTheBaseline(t *testing.T) {
	if tools.Resolve(Eslint, "") == "" {
		t.Skip("eslint not installed; the resolution logic is tested above")
	}
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	os.MkdirAll(webDir, 0o755)
	// a permissive config: everything the baseline would flag is allowed
	os.WriteFile(filepath.Join(webDir, "eslint.config.mjs"),
		[]byte("export default [{ rules: {} }]\n"), 0o644)
	f := filepath.Join(webDir, "app.js")
	os.WriteFile(f, []byte("var x = 1\nif (x == 2) { }\n"), 0o644)

	for _, g := range Files(root, []string{f}, false) {
		if strings.Contains(g.Message, "procoder baseline") {
			t.Fatalf("the repo's own config must win over the baseline: %+v", g)
		}
	}
}

func TestShellcheckFindsARealProblem(t *testing.T) {
	if tools.Resolve(Shellcheck, "") == "" {
		t.Skip("shellcheck not installed; parser tests carry the logic")
	}
	root := t.TempDir()
	f := filepath.Join(root, "bad.sh")
	os.WriteFile(f, []byte("#!/bin/sh\nrm -rf $1\n"), 0o755)
	got := Files(root, []string{f}, false)
	if len(got) == 0 {
		t.Fatal("shellcheck must flag the unquoted variable")
	}
	found := false
	for _, g := range got {
		if strings.Contains(g.Message, "SC2086") || strings.Contains(strings.ToLower(g.Message), "quote") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want the quoting finding, got %+v", got)
	}
}

func TestRuffFindsARealProblem(t *testing.T) {
	if tools.Resolve(Ruff, "") == "" {
		t.Skip("ruff not installed; parser tests carry the logic")
	}
	root := t.TempDir()
	f := filepath.Join(root, "m.py")
	os.WriteFile(f, []byte("import os\n\nprint('hi')\n"), 0o644)
	got := Files(root, []string{f}, false)
	if len(got) == 0 || !strings.Contains(got[0].Message, "F401") {
		t.Fatalf("ruff must flag the unused import: %+v", got)
	}
}

// The repo's own golangci config always beats the procoder baseline.
func TestHasGolangciConfigDetectsEveryName(t *testing.T) {
	if hasGolangciConfig(t.TempDir()) {
		t.Fatal("empty repo must have no golangci config")
	}
	for _, name := range []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !hasGolangciConfig(root) {
			t.Errorf("%s not detected", name)
		}
	}
}

// The baseline text itself must stay a parseable golangci v2 config shape:
// version pinned, and every curated linter named.
func TestGolangciBaselineNamesTheCuratedSet(t *testing.T) {
	for _, want := range []string{`version: "2"`, "gosec", "gocritic", "errorlint", "unparam", "copyloopvar", "nilerr"} {
		if !strings.Contains(golangciBaseline, want) {
			t.Errorf("baseline lost %q", want)
		}
	}
}

// A language procoder has no linter for at all — not "not installed", but
// genuinely absent from the table — cannot be fixed by installing
// anything. Blocking regardless of [lint] policy the way a missing
// gitleaks does would leave a repository writing that language unable to
// land any commit that touches it, with no escape but disabling the gate
// entirely.
// proved by: hard-coded Blocking: true in lintUnlinted — a repository that
// sets `[lint] policy = "report"` still cannot commit a .cs change.
func TestUnlintedLanguagesHonourLintPolicy(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.cs")
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// csharpier formats .cs, so this file reaches lintUnlinted rather than
	// notChecked — a real installed csharpier is not required, because the
	// finding under test is the one lintUnlinted itself produces.
	t.Setenv("PATH", t.TempDir())

	reported := lintUnlinted([]string{p}, false)
	if len(reported) != 1 || reported[0].Blocking {
		t.Fatalf("policy=report must not block: %+v", reported)
	}
	if !strings.Contains(reported[0].Message, "NOT linted") {
		t.Errorf("the reason must still be named: %q", reported[0].Message)
	}

	blocked := lintUnlinted([]string{p}, true)
	if len(blocked) != 1 || !blocked[0].Blocking {
		t.Fatalf("policy=block must block: %+v", blocked)
	}
}

// golangci-lint caps its own output — 50 issues per linter and 3 with the
// same text — and lints packages concurrently, so which issues survive
// those caps depends on which package finished first. Two runs over an
// unchanged tree reported 48 findings each and disagreed about their
// members (#236).
//
// The caps also hid work rather than merely shuffling it: errcheck's
// messages are near-identical, so max-same-issues kept three of them and
// dropped the rest. Disabling both made the set complete AND stable —
// verified by running the gate twice over the same tree.
//
// proved by: drop either flag from lintGo's args — this names the one that
// went.
func TestGolangciLintIsRunWithoutIssueCaps(t *testing.T) {
	dir := t.TempDir()
	seen := filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + seen + "\nexit 0\n"
	bin := filepath.Join(dir, "golangci-lint")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("HOME", dir)

	root := t.TempDir()
	src := filepath.Join(root, "main.go")
	if err := os.WriteFile(src, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lintGo(root, []string{src}, false)

	raw, err := os.ReadFile(seen)
	if err != nil {
		t.Skipf("the stub was not reached (%v) — this asserts flags, and NOT run is not a pass", err)
	}
	args := string(raw)
	for _, want := range []string{"--max-issues-per-linter=0", "--max-same-issues=0"} {
		if !strings.Contains(args, want) {
			t.Errorf("golangci-lint ran without %s — its own cap then picks which findings to show, and picks differently each run:\n%s", want, args)
		}
	}
}
