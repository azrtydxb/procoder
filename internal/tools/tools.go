// Package tools is the formatter registry: which tool answers for which file
// type, how to ask it for a formatted result, and what its absence means.
//
// One shape for every tool, and it is the shape that keeps P-CONTROL honest:
// the tool is always invoked so that it PRINTS the formatted result instead of
// touching the file. procoder compares that output with the file's bytes; a
// difference means "unformatted", and the formatted bytes are already in hand
// to give to the agent. No temp files, no in-place writes, one code path.
package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallCandidate is one way to install a tool: a package manager and the
// argv it needs. init picks the first candidate whose manager exists on this
// machine — order in the list IS the preference order.
type InstallCandidate struct {
	Manager string
	Args    []string
}

// Tool is one formatter.
type Tool struct {
	// Name is the binary looked up on PATH (or node_modules/.bin, see Resolve).
	Name string
	// Args builds the argv (after the binary) that makes the tool print the
	// formatted content of file to stdout without modifying anything.
	Args func(file string) []string
	// Stdin is true when the tool reads the source from stdin instead of
	// opening the file itself.
	Stdin bool
	// NeedsProjectConfig names a config file that must exist somewhere above
	// the target for this tool to run. Empty means the tool's defaults are the
	// canonical style (gofmt) or the tool falls back to sane defaults the
	// project can override (prettier, ruff). clang-format is the exception:
	// without a project .clang-format procoder would be imposing LLVM style,
	// which violates "the project's own config always wins" — so no config
	// means the file is out of scope, counted and said, never silently clean.
	NeedsProjectConfig string
	// Install is the human fallback when no known package manager is present —
	// the one line doctor and init print for a person to act on.
	Install string
	// InstallVia lists the machine-executable ways to install, best first.
	InstallVia []InstallCandidate
	// VersionArgs asks the tool its version, for doctor.
	VersionArgs []string
	// Probe, when set, must confirm a resolved binary really is this tool.
	// macOS ships a BSD ctags that answers to the same name; presence on
	// PATH is not the same as the right tool being installed.
	Probe func(bin string) bool
}

// ByExtension maps a file extension (lowercase, with dot) to its formatter.
// A file whose extension is absent here is OUT OF SCOPE for the formatting
// domain — skipped and counted, which is different from checked-and-clean.
var ByExtension = map[string]*Tool{}

var (
	gofmt = &Tool{
		Name:        "gofmt",
		Args:        func(f string) []string { return []string{f} },
		Install:     "gofmt ships with Go: https://go.dev/dl",
		VersionArgs: nil, // gofmt has no --version; presence is the check
		// gofmt is not separately installable — it arrives with the Go
		// toolchain, so the only candidate is installing Go itself.
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "go"}},
			{Manager: "apt-get", Args: []string{"install", "-y", "golang"}},
			{Manager: "dnf", Args: []string{"install", "-y", "golang"}},
			{Manager: "winget", Args: []string{"install", "--id", "GoLang.Go", "-e"}},
		},
	}
	ruff = &Tool{
		Name: "ruff",
		// stdin keeps ruff from needing write access anywhere; --stdin-filename
		// lets it resolve the project's own pyproject/ruff config for the file.
		Args:        func(f string) []string { return []string{"format", "--stdin-filename", f, "-"} },
		Stdin:       true,
		Install:     "brew install ruff   (or: pipx install ruff)",
		VersionArgs: []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "ruff"}},
			{Manager: "pipx", Args: []string{"install", "ruff"}},
			{Manager: "pip3", Args: []string{"install", "--user", "ruff"}},
			{Manager: "winget", Args: []string{"install", "--id", "astral-sh.ruff", "-e"}},
		},
	}
	prettier = &Tool{
		Name:        "prettier",
		Args:        func(f string) []string { return []string{f} },
		Install:     "npm i -D prettier   (or: brew install prettier)",
		VersionArgs: []string{"--version"},
		// npm -g rather than -D: init installs MACHINE tooling. A project that
		// wants prettier pinned adds it to its own devDependencies, and Resolve
		// already prefers node_modules/.bin when it exists.
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "prettier"}},
			{Manager: "npm", Args: []string{"install", "-g", "prettier"}},
		},
	}
	rustfmt = &Tool{
		Name:        "rustfmt",
		Args:        func(f string) []string { return nil }, // stdin -> stdout
		Stdin:       true,
		Install:     "rustup component add rustfmt",
		VersionArgs: []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "rustup", Args: []string{"component", "add", "rustfmt"}},
			{Manager: "brew", Args: []string{"install", "rustup"}},
		},
	}
	clangFormat = &Tool{
		Name:               "clang-format",
		Args:               func(f string) []string { return []string{"--style=file", f} },
		NeedsProjectConfig: ".clang-format",
		Install:            "brew install clang-format   (or: apt install clang-format)",
		VersionArgs:        []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "clang-format"}},
			{Manager: "apt-get", Args: []string{"install", "-y", "clang-format"}},
			{Manager: "dnf", Args: []string{"install", "-y", "clang-tools-extra"}},
			{Manager: "winget", Args: []string{"install", "--id", "LLVM.LLVM", "-e"}},
		},
	}
	shfmt = &Tool{
		Name:        "shfmt",
		Args:        func(f string) []string { return []string{f} },
		Install:     "brew install shfmt",
		VersionArgs: []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "shfmt"}},
			{Manager: "go", Args: []string{"install", "mvdan.cc/sh/v3/cmd/shfmt@latest"}},
			{Manager: "winget", Args: []string{"install", "--id", "mvdan.shfmt", "-e"}},
		},
	}
)

func init() {
	reg := func(t *Tool, exts ...string) {
		for _, e := range exts {
			ByExtension[e] = t
		}
	}
	reg(gofmt, ".go")
	reg(ruff, ".py", ".pyi")
	reg(prettier, ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts",
		".json", ".css", ".scss", ".md", ".yaml", ".yml", ".html")
	reg(rustfmt, ".rs")
	reg(clangFormat, ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp")
	reg(shfmt, ".sh", ".bash")
}

// ForFile returns the formatter for a path, or nil when the file type is out
// of the domain's scope.
func ForFile(path string) *Tool {
	return ByExtension[strings.ToLower(filepath.Ext(path))]
}

// Resolve finds the tool's binary: the project's own node_modules/.bin first
// (the version a JS project pinned is the version its config was written
// against), then PATH. Empty string means not installed.
func Resolve(t *Tool, repoRoot string) string {
	if repoRoot != "" {
		local := filepath.Join(repoRoot, "node_modules", ".bin", t.Name)
		if runnable(local) {
			return local
		}
	}
	p, err := exec.LookPath(t.Name)
	if err != nil {
		// the conventional install dirs `go install` and tarball installs
		// use are often missing from PATH; a tool sitting there is installed
		if home, herr := os.UserHomeDir(); herr == nil {
			for _, dir := range []string{filepath.Join(home, "go", "bin"), filepath.Join(home, ".local", "bin")} {
				for _, name := range candidateNames(t.Name) {
					cand := filepath.Join(dir, name)
					if runnable(cand) && (t.Probe == nil || t.Probe(cand)) {
						return cand
					}
				}
			}
		}
		return ""
	}
	if t.Probe == nil || t.Probe(p) {
		return p
	}
	// The first PATH hit is an impostor (macOS's BSD ctags); the real tool
	// may still sit later on the PATH.
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		for _, name := range candidateNames(t.Name) {
			cand := filepath.Join(dir, name)
			if cand != p && runnable(cand) && t.Probe(cand) {
				return cand
			}
		}
	}
	return ""
}

// candidateNames covers Windows, where the executable carries an extension
// the bare tool name lacks.
func candidateNames(name string) []string {
	if runtime.GOOS == "windows" {
		return []string{name + ".exe", name + ".cmd", name + ".bat", name}
	}
	return []string{name}
}

func runnable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// HasProjectConfig walks from the file's directory to the filesystem root
// looking for the config file the tool requires. Tools with no requirement
// always pass.
func HasProjectConfig(t *Tool, file string) bool {
	if t.NeedsProjectConfig == "" {
		return true
	}
	dir := filepath.Dir(file)
	for {
		if runnable(filepath.Join(dir, t.NeedsProjectConfig)) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// RepoRoot walks upward from dir to the directory holding .git — a directory
// in a normal checkout, a file in a worktree or submodule — or returns dir
// unchanged when there is none; every caller degrades gracefully.
func RepoRoot(dir string) string {
	d := dir
	for {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return dir
		}
		d = parent
	}
}
