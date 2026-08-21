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
	"sort"
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
	// ExitOneIsAnswer: this tool exits 1 even on success when it detected
	// something (rubocop detects offenses it corrected) — exit 1 WITH
	// stdout is the answer, not a failure.
	ExitOneIsAnswer bool
	// Resolved overrides how presence is decided, for a "tool" that is a
	// library rather than a binary. typescript-eslint ships no executable —
	// it is imported by an eslint config — so looking for it on PATH would
	// report it missing on every machine that has it.
	Resolved func(root string) string
	// Missing reports why this tool cannot run for a given file, or "" when
	// it can. It exists for a tool whose availability is not answered by a
	// binary on PATH: prettier can be installed and still be unable to read
	// PHP, because the plugin that parses PHP is a separate package. That
	// is a MISSING TOOL and reports as unchecked, which fails the gate —
	// not a missing style config, which would be out of scope and green.
	Missing func(file string) string
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
		Name: "clang-format",
		// --style=file uses the project's .clang-format wherever one is
		// found; --fallback-style names what happens when none is. Together
		// they are D-OVERRIDE in one flag pair: the repo's style wins, and
		// a repo without one is still formatted.
		//
		// This used to require a .clang-format and report the file out of
		// scope without one, so that procoder would not impose LLVM's
		// taste. That was right about not writing a config into someone's
		// repository and wrong about the alternative being to check
		// nothing: out of scope passes the gate, so every C and C++ project
		// without a style file was formatted by nothing and told it was
		// fine. A named fallback puts nothing on disk.
		Args: func(f string) []string {
			return []string{"--style=file", "--fallback-style=LLVM", f}
		},
		Install:     "brew install clang-format   (or: apt install clang-format)",
		VersionArgs: []string{"--version"},
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
	googleJavaFormat = &Tool{
		Name:        "google-java-format",
		Args:        func(f string) []string { return []string{f} }, // prints to stdout by default
		Install:     "brew install google-java-format",
		VersionArgs: []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "google-java-format"}},
		},
	}
	ktfmt = &Tool{
		Name: "ktfmt",
		// kotlinlang style is what ktlint's default enforces — the
		// formatter and the linter must not fight over indentation
		Args:        func(f string) []string { return []string{"--kotlinlang-style", "-"} }, // stdin -> stdout
		Stdin:       true,
		Install:     "brew install ktfmt",
		VersionArgs: []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "ktfmt"}},
		},
	}
	swiftformat = &Tool{
		Name: "swiftformat",
		// stdinpath lets it find the project's own .swiftformat config
		Args:        func(f string) []string { return []string{"stdin", "--stdinpath", f} },
		Stdin:       true,
		Install:     "brew install swiftformat",
		VersionArgs: []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "swiftformat"}},
		},
	}
	// prettierPHP is prettier plus the PHP plugin. The plugin is named by
	// absolute path because the formatter runs with procoder's working
	// directory, not the project's, and prettier resolves a bare plugin
	// name against the former — so `--plugin=@prettier/plugin-php` works
	// when procoder happens to be run from the project root and fails
	// everywhere else, which is the worst kind of works.
	//
	// prettier is the formatter for PHP because it is the only one that can
	// be: P-CONTROL requires printing the formatted source without touching
	// the file, and php-cs-fixer's stdin mode reports which files can be
	// fixed rather than emitting the fix, while phpcbf and pint write in
	// place. This was tested against each, not assumed.
	prettierPHP = &Tool{
		Name: "prettier",
		Args: func(f string) []string {
			return []string{"--plugin=" + phpPluginPath(f), f}
		},
		Missing: func(f string) string {
			if phpPluginPath(f) == "" {
				return "the prettier PHP plugin is not installed — npm i -D prettier @prettier/plugin-php"
			}
			return ""
		},
		Install:     "npm i -D prettier @prettier/plugin-php",
		VersionArgs: []string{"--version"},
		// Project-local, not global. phpPluginPath resolves the plugin by
		// walking up from the file into the project's node_modules, which
		// a global install never reaches — so `init --yes` would install
		// prettier, report success, and every PHP file would still be
		// UNCHECKED with the same install line it had just run.
		InstallVia: []InstallCandidate{
			{Manager: "npm", Args: []string{"install", "-D", "prettier", "@prettier/plugin-php"}},
		},
	}
	// biome formats JS and TS, and reaches the menu because it can PRINT:
	// --stdin-file-path makes it emit the formatted source on stdout and
	// leave the file alone. That is the whole entry requirement — a tool
	// that can only write in place cannot be offered here whatever its
	// merits, which is why pint and phpcbf are absent.
	biome = &Tool{
		Name: "biome",
		Args: func(f string) []string {
			return []string{"format", "--stdin-file-path=" + f}
		},
		Stdin:       true,
		Install:     "npm i -D @biomejs/biome",
		VersionArgs: []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "npm", Args: []string{"install", "-D", "@biomejs/biome"}},
		},
	}
	rubocopFmt = &Tool{
		Name: "rubocop",
		// --stdin + -x (layout-only autocorrect: formatting, not semantics)
		// prints the corrected source to stdout, diagnostics to stderr; the
		// project's own .rubocop.yml is resolved from the path. rubocop
		// exits 1 whenever offenses were DETECTED, corrected or not — with
		// output on stdout that is an answer, not a failure.
		Args:            func(f string) []string { return []string{"--stdin", f, "-x", "--stderr", "--format", "quiet"} },
		Stdin:           true,
		ExitOneIsAnswer: true,
		Install:         "brew install rubocop   (or: gem install rubocop)",
		VersionArgs:     []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "rubocop"}},
			{Manager: "gem", Args: []string{"install", "rubocop"}},
		},
	}
	dartFmt = &Tool{
		Name: "dart",
		// --summary none: dart_style otherwise appends "Formatted 1 file…"
		// to stdout, which would corrupt the handed-back source
		Args:        func(f string) []string { return []string{"format", "-o", "show", "--summary", "none", f} },
		Install:     "brew install dart-sdk   (or: https://dart.dev/get-dart)",
		VersionArgs: []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "brew", Args: []string{"install", "dart-sdk"}},
		},
	}
	csharpier = &Tool{
		Name:        "csharpier",
		Args:        func(f string) []string { return []string{"format", "--write-stdout", f} },
		Install:     "dotnet tool install -g csharpier",
		VersionArgs: []string{"--version"},
		InstallVia: []InstallCandidate{
			{Manager: "dotnet", Args: []string{"tool", "install", "-g", "csharpier"}},
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
	reg(googleJavaFormat, ".java")
	reg(ktfmt, ".kt", ".kts")
	reg(swiftformat, ".swift")
	reg(rubocopFmt, ".rb", ".rake")
	reg(dartFmt, ".dart")
	reg(csharpier, ".cs")
	reg(prettierPHP, ".php")
}

// Alternatives are the tools a repository may choose INSTEAD of the
// default for a language, by name, in [tools]. A tool is here only if it
// can print the formatted source without touching the file: procoder owns
// the invocation, and that is what keeps the print-don't-write contract a
// guarantee rather than a hope.
//
// The language key is the name a person would write, not an extension —
// `js = "biome"` covers every extension biome formats.
var Alternatives = map[string]map[string]*Tool{
	"js": {"biome": biome, "prettier": prettier},
}

// languageOf maps an extension to the language key [tools] uses.
var languageOf = map[string]string{
	".js": "js", ".jsx": "js", ".mjs": "js", ".cjs": "js",
	".ts": "js", ".tsx": "js", ".mts": "js", ".cts": "js",
}

// ForFile returns the formatter for a path, or nil when the file type is out
// of the domain's scope. It does not consult the repository's choice —
// ForFileIn does, and callers that have a root should use that.
func ForFile(path string) *Tool {
	return ByExtension[strings.ToLower(filepath.Ext(path))]
}

// ForFileIn is ForFile with the repository's [tools] choice honoured.
// An unknown name is not this function's business to report — Config
// already reported it and blocked — so the default is used and the file is
// still checked, because a mistyped tool name must not leave code unread.
func ForFileIn(path string, choice map[string]string) *Tool {
	ext := strings.ToLower(filepath.Ext(path))
	lang, ok := languageOf[ext]
	if !ok {
		return ByExtension[ext]
	}
	name, ok := choice[lang]
	if !ok {
		return ByExtension[ext]
	}
	if t, ok := Alternatives[lang][name]; ok {
		return t
	}
	return ByExtension[ext]
}

// KnownFor lists the tool names a language accepts, for the message a
// person reads when they name one procoder does not ship.
func KnownFor(lang string) []string {
	var out []string
	for name := range Alternatives[lang] {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// IsLanguage reports whether [tools] recognises this key.
func IsLanguage(lang string) bool { _, ok := Alternatives[lang]; return ok }

// Resolve finds the tool's binary: the project's own node_modules/.bin first
// (the version a JS project pinned is the version its config was written
// against), then PATH. Empty string means not installed.
func Resolve(t *Tool, repoRoot string) string {
	if t.Resolved != nil {
		return t.Resolved(repoRoot)
	}
	if repoRoot != "" {
		// A project-local tool is the right one: it is the version the
		// project pinned, and it is what the project's own scripts run.
		// node_modules/.bin is npm's; vendor/bin is composer's, and a PHP
		// project installs phpstan and phpcs there rather than globally —
		// so without it procoder would report every PHP linter missing on
		// exactly the repositories that had carefully installed one.
		for _, local := range []string{
			filepath.Join(repoRoot, "node_modules", ".bin", t.Name),
			filepath.Join(repoRoot, "vendor", "bin", t.Name),
		} {
			if runnable(local) {
				return local
			}
		}
	}
	p, err := exec.LookPath(t.Name)
	if err != nil {
		// the conventional install dirs `go install` and tarball installs
		// use are often missing from PATH; a tool sitting there is installed
		dirs := kegDirs()
		if home, herr := os.UserHomeDir(); herr == nil {
			dirs = append(dirs, filepath.Join(home, "go", "bin"), filepath.Join(home, ".local", "bin"),
				filepath.Join(home, ".dotnet", "tools"))
		}
		{
			for _, dir := range dirs {
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

// kegDirs are install locations a package manager deliberately keeps OFF
// PATH. Homebrew's llvm formula is keg-only — `brew install llvm` succeeds
// and clang-tidy is still not runnable by name — so without these, `init`
// would install the tool, the resurvey would still report it missing, and
// C/C++ would stay blocked with a remedy that had already been carried
// out. That is worse than the original silence: a wall with no door.
func kegDirs() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	return []string{
		"/opt/homebrew/opt/llvm/bin", // Homebrew on Apple silicon
		"/usr/local/opt/llvm/bin",    // Homebrew on Intel macOS
		"/usr/lib/llvm/bin",          // some Linux distributions
	}
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

// phpPluginPath walks up from a file to the node_modules that carries the
// prettier PHP plugin and returns its absolute path. Empty when there is
// none — HasProjectConfig has already refused that case, so a caller only
// reaches the formatter when this finds something.
func phpPluginPath(file string) string {
	dir := filepath.Dir(file)
	for {
		cand := filepath.Join(dir, "node_modules", "@prettier", "plugin-php")
		// The plugin's entry module, not the package directory: prettier
		// refuses a directory ("Directory import is not supported"), and the
		// package declares its entry through "exports" rather than "main",
		// so there is nothing for a resolver to fall back to.
		if entry := filepath.Join(cand, "src", "index.mjs"); runnable(entry) {
			return entry
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
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
