package lint

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"procoder/internal/gomod"
	"procoder/internal/tools"
)

// goCall is one golangci-lint invocation the stub recorded.
type goCall struct {
	dir  string
	args []string
}

// stubGolangci puts a fake golangci-lint on PATH that records its working
// directory and arguments and answers every package pattern with one finding
// in that package's x.go, with the path relative to the directory it ran in
// — which is what the real binary prints. It returns a reader for the calls.
func stubGolangci(t *testing.T) func() []goCall {
	return stubGolangciMode(t, stubModern)
}

// stubMode says how the fake binary behaves.
type stubMode int

const (
	// stubModern knows --path-mode and, given it, prints absolute paths.
	stubModern stubMode = iota
	// stubOld lacks --path-mode: its help does not list it and it errors
	// if given the flag; it prints paths relative to its working directory.
	stubOld
)

func stubGolangciMode(t *testing.T, mode stubMode) func() []goCall {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a /bin/sh script; the logic it checks is not platform-specific")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.txt")
	help := "Usage:\n  --allow-serial-runners\n  --path-mode string\n"
	if mode == stubOld {
		help = "Usage:\n  --allow-serial-runners\n"
	}
	script := `#!/bin/sh
for a in "$@"; do
  if [ "$a" = "--help" ]; then printf '` + help + `'; exit 0; fi
done
printf 'CALL\n' >> ` + log + `
printf 'pwd=%s\n' "$(pwd -P)" >> ` + log + `
for a in "$@"; do printf 'arg=%s\n' "$a" >> ` + log + `; done
abs=0
for a in "$@"; do
  case "$a" in
    --path-mode=abs) abs=1
` + map[bool]string{true: "      printf 'Error: unknown flag: --path-mode\n'; exit 3\n", false: ""}[mode == stubOld] + `;;
  esac
done
rc=0
for a in "$@"; do
  case "$a" in
    ./*) p=${a#./}; p=${p%/}
         if [ -z "$p" ]; then f=x.go; else f=$p/x.go; fi
         if [ $abs = 1 ]; then f="$(pwd -P)/$f"; fi
         printf '%s:3:1: boom (stub)\n' "$f"; rc=1 ;;
  esac
done
exit $rc
`
	if err := os.WriteFile(filepath.Join(dir, "golangci-lint"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("HOME", dir)
	return func() []goCall {
		raw, err := os.ReadFile(log)
		if err != nil {
			return nil
		}
		var calls []goCall
		for _, chunk := range strings.Split(string(raw), "CALL\n")[1:] {
			var c goCall
			for _, line := range strings.Split(strings.TrimSpace(chunk), "\n") {
				switch {
				case strings.HasPrefix(line, "pwd="):
					c.dir = strings.TrimPrefix(line, "pwd=")
				case strings.HasPrefix(line, "arg="):
					c.args = append(c.args, strings.TrimPrefix(line, "arg="))
				}
			}
			calls = append(calls, c)
		}
		return calls
	}
}

// moduleRepo lays out a root module and a nested one (momo's bench/ shape):
// its own go.mod, `replace => ../`, files that the root module does not own.
func moduleRepo(t *testing.T, workFile bool) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	put := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("go.mod", "module example.com/root\n\ngo 1.22\n")
	put("pkg/x.go", "package pkg\n")
	put("sub/go.mod", "module example.com/root/sub\n\ngo 1.22\n\nrequire example.com/root v0.0.0\n\nreplace example.com/root => ../\n")
	put("sub/x/x.go", "package x\n")
	if workFile {
		put("go.work", "go 1.22\n\nuse (\n\t.\n\t./sub\n)\n")
	}
	return root
}

func hasArg(c goCall, want string) bool {
	for _, a := range c.args {
		if a == want {
			return true
		}
	}
	return false
}

func argAfter(c goCall, flag string) string {
	for i, a := range c.args {
		if a == flag && i+1 < len(c.args) {
			return c.args[i+1]
		}
	}
	return ""
}

// A file in a nested module must be linted FROM that module, with patterns
// relative to it, and its findings must come back repo-root-relative.
// Run from the repo root golangci-lint fails with "main module (...) does
// not contain package ...", and the gate reports the file NOT checked — so
// every commit touching a nested module was blocked while correct.
//
// proved by: leave cmd.Dir at the repo root in lintGo, or drop
// --path-mode=abs — the call's dir is
// the root and the pattern is ./sub/x, so this fails on both.
func TestLintGoRunsANestedModuleFromItsOwnDirectory(t *testing.T) {
	calls := stubGolangci(t)
	root := moduleRepo(t, false)
	file := filepath.Join(root, "sub", "x", "x.go")

	got := lintGo(root, []string{file}, false)

	cs := calls()
	if len(cs) != 1 {
		t.Fatalf("want exactly one golangci-lint run, got %d: %+v", len(cs), cs)
	}
	if want := filepath.Join(root, "sub"); cs[0].dir != want {
		t.Errorf("ran in %s, want the nested module directory %s", cs[0].dir, want)
	}
	// golangci-lint v2 prints paths relative to the CONFIG file's directory
	// by default; with the baseline in the temp dir every path came back
	// as ../../tmp/... and was dropped by the wanted filter.
	if !hasArg(cs[0], "--path-mode=abs") {
		t.Errorf("paths must be absolute, independent of the config file's directory: %v", cs[0].args)
	}
	if !hasArg(cs[0], "./x") || hasArg(cs[0], "./sub/x") {
		t.Errorf("patterns must be relative to the module dir (./x, not ./sub/x): %v", cs[0].args)
	}
	if len(got) != 1 || got[0].File != "sub/x/x.go" || got[0].Line != 3 {
		t.Errorf("the finding must map back to the repo-root path sub/x/x.go, got %+v", got)
	}
}

// Files from two modules in one commit get one run each, none from the
// wrong directory, and each module's findings keep their own prefix.
//
// proved by: collapse the groups into one run — one call, not two.
func TestLintGoRunsOncePerModule(t *testing.T) {
	calls := stubGolangci(t)
	root := moduleRepo(t, false)
	files := []string{filepath.Join(root, "pkg", "x.go"), filepath.Join(root, "sub", "x", "x.go")}

	got := lintGo(root, files, false)

	cs := calls()
	if len(cs) != 2 {
		t.Fatalf("want one run per module (2), got %d: %+v", len(cs), cs)
	}
	dirs := map[string]goCall{}
	for _, c := range cs {
		dirs[c.dir] = c
	}
	if c, ok := dirs[root]; !ok || !hasArg(c, "./pkg") {
		t.Errorf("root module run missing or wrong: %+v", dirs)
	}
	if c, ok := dirs[filepath.Join(root, "sub")]; !ok || !hasArg(c, "./x") {
		t.Errorf("nested module run missing or wrong: %+v", dirs)
	}
	seen := map[string]bool{}
	for _, f := range got {
		seen[f.File] = true
	}
	if !seen["pkg/x.go"] || !seen["sub/x/x.go"] {
		t.Errorf("want findings in pkg/x.go and sub/x/x.go, got %+v", got)
	}
}

// A go.work at the root makes golangci-lint resolve the nested module
// itself, so today's single run from the root must stay exactly that.
// This is the regression guard for the fix, not a test of the bug.
//
// proved by: group by go.mod unconditionally — the go.work repo then gets
// a run from sub/ and this fails on the call count and directory.
func TestLintGoKeepsOneRootRunUnderGoWork(t *testing.T) {
	calls := stubGolangci(t)
	root := moduleRepo(t, true)
	files := []string{filepath.Join(root, "pkg", "x.go"), filepath.Join(root, "sub", "x", "x.go")}

	got := lintGo(root, files, false)

	cs := calls()
	if len(cs) != 1 || cs[0].dir != root {
		t.Fatalf("a go.work repo keeps one run from the root, got %+v", cs)
	}
	if !hasArg(cs[0], "./pkg") || !hasArg(cs[0], "./sub/x") {
		t.Errorf("root-relative patterns expected under go.work: %v", cs[0].args)
	}
	seen := map[string]bool{}
	for _, f := range got {
		seen[f.File] = true
	}
	if !seen["pkg/x.go"] || !seen["sub/x/x.go"] {
		t.Errorf("findings must stay root-relative, got %+v", got)
	}
}

// The project's golangci config wins: a nested module without its own file
// inherits the ROOT config explicitly (it now runs from a directory the
// root file is not in), one with its own is left to find it, and a repo
// with neither gets the procoder baseline as before.
//
// proved by: drop the --config pass-through for the root file — the first
// case then runs without it and silently loses the project's linters.
func TestLintGoConfigSelectionPerModule(t *testing.T) {
	t.Run("root config reaches a nested module that has none", func(t *testing.T) {
		calls := stubGolangci(t)
		root := moduleRepo(t, false)
		cfg := filepath.Join(root, ".golangci.yml")
		if err := os.WriteFile(cfg, []byte("version: \"2\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		lintGo(root, []string{filepath.Join(root, "sub", "x", "x.go")}, false)
		cs := calls()
		if len(cs) != 1 || argAfter(cs[0], "--config") != cfg {
			t.Errorf("want --config %s, got %+v", cfg, cs)
		}
	})
	t.Run("the nested module's own config is not overridden", func(t *testing.T) {
		calls := stubGolangci(t)
		root := moduleRepo(t, false)
		for _, p := range []string{filepath.Join(root, ".golangci.yml"), filepath.Join(root, "sub", ".golangci.yml")} {
			if err := os.WriteFile(p, []byte("version: \"2\"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		lintGo(root, []string{filepath.Join(root, "sub", "x", "x.go")}, false)
		cs := calls()
		if len(cs) != 1 || hasArg(cs[0], "--config") {
			t.Errorf("the module's own config must be found by golangci-lint, not overridden: %+v", cs)
		}
	})
	t.Run("no project config means the baseline", func(t *testing.T) {
		calls := stubGolangci(t)
		root := moduleRepo(t, false)
		lintGo(root, []string{filepath.Join(root, "sub", "x", "x.go")}, false)
		cs := calls()
		if len(cs) != 1 || !strings.Contains(argAfter(cs[0], "--config"), "procoder-golangci-") {
			t.Errorf("want the procoder baseline config, got %+v", cs)
		}
	})
}

// The real binary, where one is installed: a nested module's file used to
// come back NOT checked ("main module ... does not contain package"). Here
// it must be checked and the finding must carry the repo-root path.
//
// proved by: revert the fix — the finding is the NOT-checked refusal, whose
// message names golangci-lint failing, not an errcheck on sub/x/x.go.
func TestGolangciFindsARealProblemInANestedModule(t *testing.T) {
	if tools.Resolve(GolangciLint, "") == "" {
		// where CI promises the binary, skipping is a failure: otherwise a
		// broken install restores the silent skip over the guard
		if os.Getenv("PROCODER_GOLANGCI_REQUIRED") != "" {
			t.Fatal("golangci-lint is required here and was not found")
		}
		t.Skip("golangci-lint not installed; the stub tests carry the logic")
	}
	root := moduleRepo(t, false)
	src := "package x\n\nimport \"os\"\n\nfunc F() { os.Remove(\"nope\") }\n"
	file := filepath.Join(root, "sub", "x", "x.go")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got := lintGo(root, []string{file}, true)
	if len(got) == 0 {
		t.Fatal("an unchecked error in the nested module produced no finding")
	}
	for _, f := range got {
		if strings.Contains(f.Message, "NOT checked") {
			t.Fatalf("the nested module's file was not checked: %+v", f)
		}
		if f.File != "sub/x/x.go" {
			t.Errorf("finding path %q, want sub/x/x.go", f.File)
		}
	}
}

// The nearest go.mod decides the module, ascending from the file and
// stopping at the root; a file under none belongs to the root.
//
// proved by: make gomod.ModuleDir return root unconditionally — sub/x/x.go
// groups under the root module and the nested-module bug is back.
func TestModuleDirIsTheNearestGoMod(t *testing.T) {
	root := moduleRepo(t, false)
	deep := filepath.Join(root, "sub", "x", "y")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{
		filepath.Join(root, "pkg", "x.go"):        root,
		filepath.Join(root, "main.go"):            root,
		filepath.Join(root, "sub", "x", "x.go"):   filepath.Join(root, "sub"),
		filepath.Join(deep, "y.go"):               filepath.Join(root, "sub"),
		filepath.Join(root, "nomod", "z", "z.go"): root,
	} {
		if got := gomod.ModuleDir(root, file); got != want {
			t.Errorf("ModuleDir(%s) = %s, want %s", file, got, want)
		}
	}
}

// proved by: key groups by the root instead of the module — one group, so
// the nested file would again be named from the wrong module; and drop the
// go.work branch — a workspace repo then splits into two groups.
func TestGroupSplitsByModuleUnlessAWorkspace(t *testing.T) {
	root := moduleRepo(t, false)
	a, b := filepath.Join(root, "pkg", "x.go"), filepath.Join(root, "sub", "x", "x.go")
	g := gomod.Group(root, []string{a, b})
	if len(g) != 2 || len(g[root]) != 1 || len(g[filepath.Join(root, "sub")]) != 1 {
		t.Errorf("want one group per module, got %v", g)
	}
	ws := moduleRepo(t, true)
	g = gomod.Group(ws, []string{filepath.Join(ws, "pkg", "x.go"), filepath.Join(ws, "sub", "x", "x.go")})
	if len(g) != 1 || len(g[ws]) != 2 {
		t.Errorf("a go.work repo is one group at the root, got %v", g)
	}
}

// proved by: compute patterns relative to the root — ./sub/x leaks into the
// nested module's run; drop the dedupe — ./x appears twice.
func TestGoPatternsAreRelativeToTheModule(t *testing.T) {
	mod := filepath.Join(t.TempDir(), "r", "sub")
	got := goPatterns(mod, []string{
		filepath.Join(mod, "main.go"),
		filepath.Join(mod, "x", "a.go"),
		filepath.Join(mod, "x", "b.go"),
		filepath.Join(mod, "x", "y", "c.go"),
	})
	want := []string{"./", "./x", "./x/y"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("goPatterns = %v, want %v", got, want)
	}
}

// proved by: return the path unchanged — a finding printed as x/x.go in the
// sub module no longer matches the wanted sub/x/x.go and is silently dropped.
func TestRepoRelPathMapsModuleRelativeFindingsToTheRoot(t *testing.T) {
	root := t.TempDir() // absolute on every OS, drive letter included
	sub := filepath.Join(root, "sub")
	for _, c := range []struct{ modDir, in, want string }{
		{sub, "x/x.go", "sub/x/x.go"},
		{sub, "x.go", "sub/x.go"},
		{root, "pkg/x.go", "pkg/x.go"},
		{sub, filepath.Join(sub, "x", "x.go"), "sub/x/x.go"},
		{sub, "../other/x.go", "other/x.go"},
	} {
		if got := repoRelPath(root, c.modDir, c.in); got != c.want {
			t.Errorf("repoRelPath(%s, %s, %q) = %q, want %q", root, c.modDir, c.in, got, c.want)
		}
	}
}

// A file directly in the nested module's root (sub/x.go) is the module's
// "./" package: the run is from sub with the pattern "./", never ./sub.
//
// proved by: resolve the module from the repo root — the run happens in
// root with ./sub, and this fails on the directory.
func TestLintGoFileAtTheNestedModuleRoot(t *testing.T) {
	calls := stubGolangci(t)
	root := moduleRepo(t, false)
	file := filepath.Join(root, "sub", "x.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := lintGo(root, []string{file}, false)
	cs := calls()
	if len(cs) != 1 || cs[0].dir != filepath.Join(root, "sub") || !hasArg(cs[0], "./") {
		t.Fatalf("want one run in sub with ./, got %+v", cs)
	}
	if len(got) != 1 || got[0].File != "sub/x.go" {
		t.Errorf("finding must map to sub/x.go (the stub's file for ./), got %+v", got)
	}
}

// golangci-lint v2 prints absolute paths under --path-mode=abs. A finding
// in the FIRST linted file arrives as that file's absolute path — exactly
// what the old `File == files[0]` shortcut mistook for the run-failed
// refusal and passed through unmapped, so the gate printed an absolute path
// and the wanted filter was never applied.
//
// proved by: restore `if f.File == files[0]` in lintGoModule in place of the
// NOT checked prefix test — the finding keeps its absolute path.
func TestLintGoMapsAbsoluteFindingInTheFirstFile(t *testing.T) {
	calls := stubGolangci(t)
	root := moduleRepo(t, false)
	file := filepath.Join(root, "sub", "x", "x.go")
	got := lintGo(root, []string{file}, false)
	cs := calls()
	if len(cs) != 1 || !hasArg(cs[0], "--path-mode=abs") {
		t.Fatalf("a binary that knows --path-mode must be given it: %+v", cs)
	}
	if len(got) != 1 || got[0].File != "sub/x/x.go" {
		t.Errorf("an absolute finding in files[0] must be mapped to sub/x/x.go, got %+v", got)
	}
}

// A golangci-lint v2 too old to know --path-mode rejects the flag, which
// would turn every Go file into NOT checked. Support is read from the
// binary's own help, the flag is withheld, and its working-directory
// relative output is still mapped and kept.
//
// proved by: pass --path-mode=abs unconditionally — the stub exits 3 and
// the file reads NOT checked.
func TestLintGoWithholdsPathModeFromABinaryThatLacksIt(t *testing.T) {
	calls := stubGolangciMode(t, stubOld)
	root := moduleRepo(t, false)
	got := lintGo(root, []string{filepath.Join(root, "sub", "x", "x.go")}, false)
	cs := calls()
	if len(cs) != 1 || hasArg(cs[0], "--path-mode=abs") {
		t.Fatalf("the unknown flag must not be passed: %+v", cs)
	}
	if len(got) != 1 || got[0].File != "sub/x/x.go" || strings.Contains(got[0].Message, "NOT checked") {
		t.Errorf("the file must be linted and mapped, got %+v", got)
	}
}

// proved by: drop the symlink fallback in repoRelPath — the resolved path
// the tool prints is outside the symlinked root and comes back unmapped.
func TestRepoRelPathMapsAResolvedPathUnderASymlinkedRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(resolved, link); err != nil {
		t.Skipf("cannot symlink: %v", err)
	}
	got := repoRelPath(link, filepath.Join(link, "sub"), filepath.Join(resolved, "sub", "x", "x.go"))
	if got != "sub/x/x.go" {
		t.Errorf("repoRelPath = %q, want sub/x/x.go", got)
	}
}
