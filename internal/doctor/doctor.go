// Package doctor answers one question: for the languages actually present in
// this repository, are the formatters installed — and where they are not, what
// installs them. Missing tooling exits non-zero, because a gate that cannot
// look is a gate with a hole in it, and a hole should fail loudly somewhere a
// human is watching.
package doctor

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"procoder/internal/tools"
)

// ExtraTools are domain tools that are not extension-driven: actionlint is
// needed when the repo has GitHub Actions workflows, gh when it talks to a
// GitHub remote. Injected as a function so doctor stays free of an import
// cycle with the packages that define them.
var ExtraTools func(root string) []*tools.Tool

// Directories never walked: not source, and node_modules alone can make the
// walk take longer than every check combined.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "vendor": true,
	"target": true, "__pycache__": true,
}

// Run inspects root and reports. Returns the exit code.
func Run(root string, stdout io.Writer) int {
	present := ExtensionsIn(root)

	// One row per distinct tool that the present extensions call for.
	type row struct {
		tool *tools.Tool
		exts []string
	}
	byTool := map[string]*row{}
	for ext := range present {
		t := tools.ByExtension[ext]
		if t == nil {
			continue
		}
		r := byTool[t.Name]
		if r == nil {
			r = &row{tool: t}
			byTool[t.Name] = r
		}
		r.exts = append(r.exts, ext)
	}

	if ExtraTools != nil {
		for _, t := range ExtraTools(root) {
			if byTool[t.Name] == nil {
				byTool[t.Name] = &row{tool: t, exts: []string{"(repo)"}}
			}
		}
	}

	names := make([]string, 0, len(byTool))
	for n := range byTool {
		names = append(names, n)
	}
	sort.Strings(names)

	missing := 0
	for _, n := range names {
		r := byTool[n]
		sort.Strings(r.exts)
		bin := tools.Resolve(r.tool, root)
		if bin == "" {
			missing++
			fmt.Fprintf(stdout, " GAP  %-14s %-28s %s\n", n, strings.Join(r.exts, " "), "missing")
			fmt.Fprintf(stdout, "      run `procoder init` to install it (manual: %s)\n", r.tool.Install)
			continue
		}
		fmt.Fprintf(stdout, "  ok  %-14s %-28s %s\n", n, strings.Join(r.exts, " "), version(bin, r.tool))
	}

	if len(byTool) == 0 {
		fmt.Fprintln(stdout, "no files in this tree have a formatter-covered type")
		return 0
	}
	if missing == 0 {
		fmt.Fprintln(stdout, "\nevery formatter this repository needs is installed")
		return 0
	}
	fmt.Fprintf(stdout, "\n%d formatter(s) missing — files of those types are reported as unchecked, never as clean\n", missing)
	return 1
}

func version(bin string, t *tools.Tool) string {
	if len(t.VersionArgs) == 0 {
		return "present"
	}
	out, err := exec.Command(bin, t.VersionArgs...).Output() // nosemgrep -- resolved from the fixed tool table, never user input
	if err != nil {
		return "present (version unknown)"
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if len(line) > 60 {
		line = line[:60]
	}
	return line
}

// ExtensionsIn surveys which file extensions exist under root.
func ExtensionsIn(root string) map[string]bool {
	found := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable entry must not abort the survey
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || (strings.HasPrefix(d.Name(), ".") && d.Name() != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
			found[ext] = true
		}
		return nil
	})
	return found
}

// RequiredTools returns the distinct formatters the extensions under root call
// for, sorted by tool name — the shared survey doctor and init both start from.
func RequiredTools(root string) []*tools.Tool {
	seen := map[string]*tools.Tool{}
	for ext := range ExtensionsIn(root) {
		if t := tools.ByExtension[ext]; t != nil {
			seen[t.Name] = t
		}
	}
	if ExtraTools != nil {
		for _, t := range ExtraTools(root) {
			seen[t.Name] = t
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]*tools.Tool, 0, len(names))
	for _, n := range names {
		out = append(out, seen[n])
	}
	return out
}

// Root returns the repo root for the working directory.
func Root() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return tools.RepoRoot(wd)
}
