package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// stubTypescriptEslint writes a module at the exact path the resolver looks
// for, exporting the shape the generated config uses: config() and
// configs.recommended. A stub rather than the real package because what is
// under test is procoder's half — that it finds the entry, writes a config
// importing it by absolute path, and reads back what eslint says. The real
// typescript-eslint would test typescript-eslint, and would make this skip
// on every machine that had not npm-installed it, which is how a guard ends
// up never running anywhere.
func stubTypescriptEslint(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "node_modules", "typescript-eslint", "dist")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// One rule, and one that fires on the fixture below without needing a
	// TypeScript parser: the wiring is the subject, not the rule set.
	// The files patterns are not decoration: eslint's flat config lints
	// .js and nothing else unless a config block names other extensions,
	// and the real tseslint.configs.recommended carries exactly these. A
	// stub without them would report "File ignored because no matching
	// configuration was supplied" and this test would be measuring the
	// stub's infidelity rather than procoder's wiring.
	const mod = `const recommended = [{
  files: ["**/*.ts", "**/*.tsx", "**/*.mts", "**/*.cts"],
  rules: { "no-debugger": "error" },
}];
export default { config: (...parts) => parts.flat(), configs: { recommended } };
`
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The generated config must import the entry by ABSOLUTE path. It is
// written to a temp directory, and node resolves a bare specifier relative
// to the importing file — so a bare "typescript-eslint" would resolve
// against /tmp, find nothing, and every TypeScript lint would fail with a
// module-not-found that looks like a broken tool rather than a wrong path.
// proved by: changed tsBaselineConfig to import the bare package name —
// this test names it, and the run below fails to load its config.
func TestTheTypescriptBaselineImportsAnAbsolutePath(t *testing.T) {
	got := tsBaselineConfig("/abs/node_modules/typescript-eslint/dist/index.js")
	if !strings.Contains(got, `"/abs/node_modules/typescript-eslint/dist/index.js"`) {
		t.Errorf("the entry must be imported by absolute path:\n%s", got)
	}
	if !strings.Contains(got, "configs.recommended") {
		t.Errorf("the baseline must be the recommended set:\n%s", got)
	}
}

// The success path, end to end: a .ts file in a project with no eslint
// config is linted against the baseline and a real finding comes back with
// its file and line. Without this the whole feature could be broken and the
// suite would stay green through the blocking-refusal path alone.
// proved by: made lintTSBaseline return notChecked unconditionally — the
// refusal path still passes its own test and this one names the missing
// finding.
func TestConfiglessTypescriptGetsARealBaselineFinding(t *testing.T) {
	if tools.Resolve(Eslint, "") == "" {
		t.Skip("eslint not installed; the baseline leg runs where it is")
	}
	root := t.TempDir()
	stubTypescriptEslint(t, root)
	// Deliberately free of TypeScript-only syntax: the stub carries no
	// parser, and what this test is for is procoder's wiring — that the
	// entry is found, the config generated, eslint invoked with it, and
	// its output read back. Real TypeScript syntax is covered by
	// TestTheRealParserReadsTypescriptSyntax below, against the real
	// package.
	src := filepath.Join(root, "a.ts")
	if err := os.WriteFile(src, []byte("export function f() {\n  debugger;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Files(root, []string{src}, false)
	if len(got) == 0 {
		t.Fatal("configless TypeScript must be linted against the baseline, got nothing")
	}
	var found bool
	for _, f := range got {
		if strings.Contains(f.Message, "NOT checked") {
			t.Fatalf("the baseline was available and did not run: %q", f.Message)
		}
		if strings.Contains(f.Message, "no-debugger") {
			found = true
			if f.Line != 2 {
				t.Errorf("the finding must carry its line, got %d", f.Line)
			}
			if !strings.Contains(f.Message, "procoder baseline") {
				t.Errorf("the finding must say it came from the baseline: %q", f.Message)
			}
		}
	}
	if !found {
		t.Errorf("the baseline rule must fire on the fixture: %+v", got)
	}
}

// And when the parser is absent the refusal names it, so `procoder init`
// has something to install — a blocking refusal without a remedy is a wall.
// proved by: returned nil instead of notChecked — configless TypeScript
// goes back to passing silently on a machine without the parser.
func TestWithoutTheParserTypescriptSaysSo(t *testing.T) {
	if tools.Resolve(Eslint, "") == "" {
		t.Skip("eslint not installed")
	}
	root := t.TempDir()
	src := filepath.Join(root, "a.ts")
	if err := os.WriteFile(src, []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Files(root, []string{src}, false)
	if len(got) != 1 {
		t.Fatalf("want one refusal, got %+v", got)
	}
	if !strings.Contains(got[0].Message, TypescriptEslint.Name) {
		t.Errorf("the refusal must name the parser to install: %q", got[0].Message)
	}
	if !got[0].Blocking {
		t.Error("a check that did not happen must block")
	}
}

// The stub above cannot parse TypeScript, so one thing it cannot prove is
// the thing typescript-eslint exists for: that a type annotation is not a
// syntax error. This runs against the real package where it is installed.
// It skips in CI, which is why it is not the only test of this feature —
// the wiring test above runs everywhere.
// proved by: pointed tsBaselineConfig at eslint's default parser — the
// annotation below becomes "Parsing error: Unexpected token :" and this
// test names it.
func TestTheRealParserReadsTypescriptSyntax(t *testing.T) {
	if tools.Resolve(Eslint, "") == "" {
		t.Skip("eslint not installed")
	}
	// The fixture lives in a temp directory, so the real package has to be
	// brought to it: resolving from the working directory finds the copy
	// this repository has, and a symlink puts it where the walk-up looks.
	// Without this the test could only ever skip, which is a guard that
	// proves nothing while looking like one that does.
	real := tsESLintEntry("", filepath.Join(mustWD(t), "x"))
	if real == "" {
		t.Skip("the real typescript-eslint is not installed here")
	}
	root := t.TempDir()
	pkg := filepath.Dir(filepath.Dir(real)) // .../node_modules/typescript-eslint
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(pkg, filepath.Join(root, "node_modules", "typescript-eslint")); err != nil {
		t.Skip("cannot link the real package here: ", err)
	}
	// eslint resolves the plugin's own dependencies from where the package
	// really lives, so the rest of that node_modules has to be reachable.
	if err := os.Symlink(filepath.Dir(pkg), filepath.Join(root, "nm")); err != nil {
		t.Skip("cannot link: ", err)
	}
	src := filepath.Join(root, "a.ts")
	// A return type annotation: valid TypeScript, a syntax error to any
	// JavaScript parser.
	if err := os.WriteFile(src, []byte("export function f(): void {\n  debugger;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range Files(root, []string{src}, false) {
		if strings.Contains(f.Message, "Parsing error") {
			t.Errorf("TypeScript syntax must parse: %q", f.Message)
		}
	}
}

func mustWD(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}
