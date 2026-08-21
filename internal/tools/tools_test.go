package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stub writes an executable file that stands in for a real binary. Resolve
// only ever stats these — nothing is executed — so the contents are a marker
// for the human reading a failure, not a program.
func stub(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// skipOnWindows guards the legs that build a PATH out of extension-less stub
// files; candidateNames covers Windows separately.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PATH stubs need an executable extension on Windows")
	}
}

// The language matrix: every popular language procoder claims maps to its
// canonical formatter. A registry edit that drops a language fails here
// before the docs can overclaim.
func TestFormatterMatrixCoversTheClaimedLanguages(t *testing.T) {
	want := map[string]string{
		".go":    "gofmt",
		".py":    "ruff",
		".js":    "prettier",
		".ts":    "prettier",
		".json":  "prettier",
		".md":    "prettier",
		".html":  "prettier",
		".css":   "prettier",
		".yaml":  "prettier",
		".rs":    "rustfmt",
		".c":     "clang-format",
		".cpp":   "clang-format",
		".h":     "clang-format",
		".sh":    "shfmt",
		".java":  "google-java-format",
		".kt":    "ktfmt",
		".kts":   "ktfmt",
		".swift": "swiftformat",
		".rb":    "rubocop",
		".dart":  "dart",
		".cs":    "csharpier",
	}
	for ext, name := range want {
		tool := ByExtension[ext]
		if tool == nil {
			t.Errorf("%s has no formatter — the matrix regressed", ext)
			continue
		}
		if tool.Name != name {
			t.Errorf("%s -> %s, want %s", ext, tool.Name, name)
		}
	}
}

// Every registered tool can be installed or explained: a human fallback
// line always exists, and stdin tools produce argv without a file.
func TestEveryFormatterHasAnInstallStory(t *testing.T) {
	seen := map[string]bool{}
	for ext, tool := range ByExtension {
		if seen[tool.Name] {
			continue
		}
		seen[tool.Name] = true
		if tool.Install == "" {
			t.Errorf("%s (%s) has no human install line", tool.Name, ext)
		}
		if tool.Args == nil {
			t.Errorf("%s (%s) builds no argv", tool.Name, ext)
		}
	}
}

// A JS project pins its own prettier; the version its .prettierrc was written
// against is the one that must answer, even when a different prettier sits on
// PATH ahead of it.
// proved by: deleting the whole `if repoRoot != ""` block in Resolve, so the
// PATH copy wins and the test sees the wrong path.
func TestResolvePrefersTheProjectsOwnBinaryOverPath(t *testing.T) {
	skipOnWindows(t)
	root := t.TempDir()
	local := stub(t, filepath.Join(root, "node_modules", ".bin"), "prettier")
	pathDir := t.TempDir()
	stub(t, pathDir, "prettier")
	t.Setenv("PATH", pathDir)

	if got := Resolve(prettier, root); got != local {
		t.Errorf("Resolve = %q, want the project's own %q", got, local)
	}
}

// With nothing pinned in the project, PATH answers.
// proved by: making Resolve return "" instead of p on the LookPath success
// path.
func TestResolveFallsBackToPathWhenTheProjectPinsNothing(t *testing.T) {
	skipOnWindows(t)
	pathDir := t.TempDir()
	want := stub(t, pathDir, "prettier")
	t.Setenv("PATH", pathDir)

	if got := Resolve(prettier, t.TempDir()); got != want {
		t.Errorf("Resolve = %q, want %q", got, want)
	}
}

// `go install`, pipx and `dotnet tool install -g` all drop binaries into dirs
// a login shell often forgets to export. A tool sitting there IS installed,
// and doctor must not tell the user to install it twice.
// proved by: dropping ".dotnet/tools" (then "go/bin", then ".local/bin") from
// the fallback dir list in Resolve — each case fails on its own line.
func TestResolveFindsToolsInTheUnexportedInstallDirs(t *testing.T) {
	skipOnWindows(t)
	for _, dir := range [][]string{{"go", "bin"}, {".local", "bin"}, {".dotnet", "tools"}} {
		name := filepath.Join(dir...)
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			want := stub(t, filepath.Join(home, name), "shfmt")
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("PATH", t.TempDir()) // empty: LookPath must fail first

			if got := Resolve(shfmt, ""); got != want {
				t.Errorf("Resolve = %q, want %q", got, want)
			}
		})
	}
}

// Nothing anywhere means not installed — the empty string doctor turns into
// an install line. Never a guess at a path.
// proved by: making Resolve return t.Name instead of "" when LookPath fails.
func TestResolveReturnsEmptyWhenTheToolIsAbsent(t *testing.T) {
	skipOnWindows(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", t.TempDir())

	if got := Resolve(shfmt, t.TempDir()); got != "" {
		t.Errorf("Resolve = %q, want \"\" for an uninstalled tool", got)
	}
}

// macOS ships a BSD ctags answering to the same name as universal-ctags. The
// first PATH hit failing its Probe must not end the search — the real tool may
// sit further down the PATH, and reporting "not installed" would be a lie.
// proved by: replacing the impostor-recovery loop in Resolve with `return ""`.
func TestResolveWalksPastAnImpostorToTheRealTool(t *testing.T) {
	skipOnWindows(t)
	impostorDir, realDir := t.TempDir(), t.TempDir()
	impostor := stub(t, impostorDir, "ctags")
	real := stub(t, realDir, "ctags")
	t.Setenv("PATH", impostorDir+string(os.PathListSeparator)+realDir)

	tool := &Tool{Name: "ctags", Probe: func(bin string) bool { return bin == real }}
	got := Resolve(tool, "")
	if got == impostor {
		t.Fatalf("Resolve accepted the impostor at %q", impostor)
	}
	if got != real {
		t.Errorf("Resolve = %q, want the probed binary %q", got, real)
	}
}

// A PATH holding nothing but impostors is not an installation — the recovery
// walk must keep probing every later candidate, not settle for the next one
// that merely bears the name.
// proved by: dropping the `t.Probe(cand)` term from the recovery loop's
// condition, which makes the second impostor pass as the answer.
func TestResolveRejectsAPathOfNothingButImpostors(t *testing.T) {
	skipOnWindows(t)
	first, second := t.TempDir(), t.TempDir()
	stub(t, first, "ctags")
	stub(t, second, "ctags")
	dir := first + string(os.PathListSeparator) + second
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", dir)

	tool := &Tool{Name: "ctags", Probe: func(string) bool { return false }}
	if got := Resolve(tool, ""); got != "" {
		t.Errorf("Resolve = %q, want \"\" when every candidate fails its probe", got)
	}
}

// On anything but Windows the binary is the bare name; the .exe/.cmd/.bat
// spread exists for Windows alone.
// proved by: returning []string{name + ".exe", name} from the non-Windows
// branch of candidateNames.
func TestCandidateNamesIsBareOffWindows(t *testing.T) {
	skipOnWindows(t)
	got := candidateNames("shfmt")
	if len(got) != 1 || got[0] != "shfmt" {
		t.Errorf("candidateNames = %v, want [shfmt]", got)
	}
}

// clang-format's default is LLVM style. Running it without a project
// .clang-format would impose a style the repository never chose, so the
// config decides whether C files are in scope at all — and the config may sit
// far above the file, at the repository root.
// proved by: replacing the upward walk in HasProjectConfig with a single
// check of the file's own directory.
func TestHasProjectConfigWalksUpToAnAncestor(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "src", "vendor", "lib")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".clang-format"), []byte("BasedOnStyle: LLVM\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasProjectConfig(clangFormat, filepath.Join(deep, "a.c")) {
		t.Error("a .clang-format at the repository root must bring src/vendor/lib/a.c into scope")
	}
}

// No config anywhere up to the filesystem root: out of scope, said and
// counted — never silently reported clean.
// proved by: returning true instead of false when the walk reaches the root.
// HasProjectConfig still answers for any tool that names a required config.
// clang-format no longer does — it takes a fallback style instead — so the
// mechanism is exercised against a tool declared here rather than against
// whichever real tool happens to need a config this month.
func TestHasProjectConfigIsFalseWhenNothingAboveTheFileHasIt(t *testing.T) {
	needy := &Tool{Name: "needy", NeedsProjectConfig: ".needyrc"}
	dir := t.TempDir()
	if HasProjectConfig(needy, filepath.Join(dir, "a.x")) {
		t.Error("no .needyrc anywhere above the file, yet the file was called in scope")
	}
	if err := os.WriteFile(filepath.Join(dir, ".needyrc"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasProjectConfig(needy, filepath.Join(dir, "a.x")) {
		t.Error("the config is right there and the file was called out of scope")
	}
}

// clang-format must NOT require a project config: requiring one is what
// made every unconfigured C and C++ file skip the gate.
// proved by: put NeedsProjectConfig back on clang-format — this fails, and
// so does the formatting test that expects the file to be formatted.
func TestClangFormatNeedsNoProjectConfig(t *testing.T) {
	if clangFormat.NeedsProjectConfig != "" {
		t.Errorf("clang-format must format without a config, needs %q", clangFormat.NeedsProjectConfig)
	}
	args := clangFormat.Args("/x/a.c")
	var fallback bool
	for _, a := range args {
		if strings.HasPrefix(a, "--fallback-style=") {
			fallback = true
		}
	}
	if !fallback {
		t.Errorf("without a config the baseline style must be named: %v", args)
	}
}

// gofmt's own defaults ARE the canonical Go style, so a tool that requires no
// config is always in scope.
// proved by: deleting the `if t.NeedsProjectConfig == ""` early return, which
// sends gofmt into a walk looking for a file named "".
func TestHasProjectConfigAlwaysPassesForToolsThatNeedNone(t *testing.T) {
	if !HasProjectConfig(gofmt, filepath.Join(t.TempDir(), "main.go")) {
		t.Error("gofmt requires no project config and must always be in scope")
	}
}

// Extensions are matched case-insensitively: a Windows-authored MAIN.GO is a
// Go file.
// proved by: dropping strings.ToLower from ForFile.
func TestForFileNormalisesExtensionCasing(t *testing.T) {
	for _, path := range []string{"MAIN.GO", "dir/Main.Go", "a.go"} {
		tool := ForFile(path)
		if tool == nil || tool.Name != "gofmt" {
			t.Errorf("ForFile(%q) = %v, want gofmt", path, tool)
		}
	}
}

// An unknown extension is out of scope, and nil is how the domain says so —
// distinct from "checked and clean".
// proved by: making ForFile return gofmt as a fallback when the map misses.
func TestForFileIsNilOutsideTheDomain(t *testing.T) {
	for _, path := range []string{"a.bin", "LICENSE", "Makefile", "a.tar.gz", ""} {
		if tool := ForFile(path); tool != nil {
			t.Errorf("ForFile(%q) = %s, want nil — that file type is out of scope", path, tool.Name)
		}
	}
}

// RepoRoot must find .git whether it is a directory (normal checkout) or a
// file (worktree, submodule) — a worktree is a repository too.
// proved by: replacing os.Stat's success test in RepoRoot with a check that
// the entry is a directory, which breaks the "file" case alone.
func TestRepoRootFindsGitAsDirectoryAndAsFile(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		root := t.TempDir()
		deep := filepath.Join(root, "internal", "tools")
		if err := os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(deep, 0o755); err != nil {
			t.Fatal(err)
		}
		if got := RepoRoot(deep); got != root {
			t.Errorf("RepoRoot = %q, want %q", got, root)
		}
	})
	t.Run("file", func(t *testing.T) {
		root := t.TempDir()
		deep := filepath.Join(root, "sub")
		if err := os.MkdirAll(deep, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /elsewhere/.git/worktrees/w\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := RepoRoot(deep); got != root {
			t.Errorf("RepoRoot = %q, want %q for a worktree's .git file", got, root)
		}
	})
}

// Outside any repository RepoRoot hands the input straight back — callers
// degrade gracefully instead of being handed "/".
// proved by: returning d (the walk's last value, "/") instead of dir when the
// walk reaches the filesystem root.
func TestRepoRootReturnsTheInputWhenThereIsNoRepository(t *testing.T) {
	dir := t.TempDir()
	if got := RepoRoot(dir); got != dir {
		t.Errorf("RepoRoot = %q, want the input %q unchanged", got, dir)
	}
}
