package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

func skipConfig(t *testing.T, root, list string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "[lint]\npolicy = \"block\"\nskip_tools = " + list + "\n"
	if err := os.WriteFile(filepath.Join(root, ".procoder", "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A linter the repository scoped out does not run and does not block, and
// its files are still said to be unlinted — even with [lint] policy at
// block and the linter absent, which would otherwise be NOT checked.
// proved by: dropped the skipped() call from Files — golangci-lint is not
// on this PATH, so the Go file comes back as a blocking NOT checked.
func TestSkipToolsScopesALinterOutLoudlyNotSilently(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	skipConfig(t, root, `["golangci-lint"]`)
	src := filepath.Join(root, "main.go")
	if err := os.WriteFile(src, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Files(root, []string{src}, true)
	if len(got) != 1 {
		t.Fatalf("want one skip notice, got %+v", got)
	}
	if got[0].Blocking {
		t.Error("a skip the repository chose must not block")
	}
	if !strings.Contains(got[0].Message, "NOT linted") || !strings.Contains(got[0].Message, "golangci-lint") {
		t.Errorf("the skip must say what did not run: %q", got[0].Message)
	}
}

// typescript-eslint is a route inside the JavaScript group: skipping it
// skips configless TypeScript and nothing else.
// proved by: removed the skipTool check in lintJS — the .ts file reaches
// lintTSBaseline and comes back NOT checked.
func TestSkippingTypescriptEslintLeavesJavascriptLinted(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	isolateCache(t)
	root := t.TempDir()
	skipConfig(t, root, `["typescript-eslint"]`)
	ts := filepath.Join(root, "a.ts")
	js := filepath.Join(root, "b.js")
	for _, f := range []string{ts, js} {
		if err := os.WriteFile(f, []byte("export const x = 1;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var skippedTS, refusedJS bool
	for _, f := range Files(root, []string{ts, js}, true) {
		switch {
		case f.File == ts && strings.Contains(f.Message, "skipped by [lint] skip_tools") && !f.Blocking:
			skippedTS = true
		case f.File == js && f.Blocking && strings.Contains(f.Message, "eslint"):
			refusedJS = true // eslint absent: JavaScript is still NOT checked
		default:
			t.Errorf("unexpected finding %+v", f)
		}
	}
	if !skippedTS || !refusedJS {
		t.Errorf("want the .ts skipped and the .js still checked; skipped=%v js-refused=%v", skippedTS, refusedJS)
	}
}

// A name procoder runs no linter by skips nothing, and says so: a writer
// who misspells a tool believes it is off.
// proved by: dropped the unknown-name finding — the typo passes unnoticed.
func TestSkipToolsNamesAnUnknownLinterOutLoud(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	skipConfig(t, root, `["golangci-lnt", "golangci-lint"]`)
	src := filepath.Join(root, "main.go")
	if err := os.WriteFile(src, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var named bool
	for _, f := range Files(root, []string{src}, true) {
		if strings.Contains(f.Message, `"golangci-lnt"`) {
			named = true
		}
	}
	if !named {
		t.Error("an unknown skip_tools name must be reported")
	}
}

// Every name skip_tools accepts is a linter Files dispatches to, so a
// renamed tool cannot leave a skip that silently matches nothing.
// proved by: a skipGroups entry pointing at a group Files never fills.
func TestEverySkipNameScopesARealGroup(t *testing.T) {
	groups := map[string]bool{"go": true, "py": true, "sh": true, "js": true, "kt": true,
		"swift": true, "rb": true, "rs": true, "java": true, "php": true, "c": true}
	for name, g := range skipGroups {
		if !groups[g] {
			t.Errorf("skip_tools %q scopes out group %q, which Files never fills", name, g)
		}
	}
}

// eslint 9.18 and later load eslint.config.mts and .cts. A project
// configured that way must have its own config used, not overruled by the
// baseline's --no-config-lookup.
// proved by: removed the .mts name — the config is not found.
func TestTypescriptFlatConfigsCountAsProjectConfig(t *testing.T) {
	for _, name := range []string{"eslint.config.mts", "eslint.config.cts"} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, name), []byte("export default [];\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := nearestEslintConfigDir(root, filepath.Join(root, "src", "a.ts")); got != root {
			t.Errorf("%s not recognised as the project's eslint config", name)
		}
	}
}

// eslint 9 and 10 do not read .eslintrc, and refuse with a first line of
// "Oops! Something went wrong!". The finding must name the actual cause.
// proved by: dropped the noFlatConfig branch — the finding reads "eslint
// failed: Oops! Something went wrong! :(".
func TestLegacyEslintrcUnderFlatOnlyEslintNamesTheMigration(t *testing.T) {
	bin := tools.Resolve(Eslint, "")
	if bin == "" {
		t.Skip("eslint not installed")
	}
	raw, _ := execute(t.TempDir(), bin, []string{"--version"})
	if strings.HasPrefix(strings.TrimSpace(raw), "v8.") {
		t.Skip("eslint 8 still reads .eslintrc")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".eslintrc.json"), []byte(`{"rules":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "a.js")
	if err := os.WriteFile(src, []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := lintJS(root, []string{src}, false)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "no longer reads .eslintrc") {
		t.Fatalf("want one blocking finding naming the .eslintrc migration, got %+v", got)
	}
}
