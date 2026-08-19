// The --types extension of domain 2: the type-checker, where the canonical
// linter does not already compile the code. Go and Rust arrive compiled
// (golangci-lint and clippy build what they lint); TypeScript and Python do
// not, so tsc --noEmit and pyright close that hole on demand. Same contract
// as lint itself: the project's config wins, findings are reported, and a
// checker that could not run is NOT checked, never clean.
package lint

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"procoder/internal/gitx"
	"procoder/internal/tools"
)

// Tsc is the TypeScript compiler in check-only mode. It only runs under a
// project tsconfig.json — without one procoder would be imposing compiler
// options, which it never does.
var Tsc = &tools.Tool{
	Name:        "tsc",
	Install:     "npm i -D typescript   (project-local preferred)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "npm", Args: []string{"install", "-g", "typescript"}},
	},
}

// Pyright type-checks Python; it reads the project's own pyproject/
// pyrightconfig settings when present and has usable defaults otherwise.
var Pyright = &tools.Tool{
	Name:        "pyright",
	Install:     "npm i -g pyright   (or: pipx install pyright)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "npm", Args: []string{"install", "-g", "pyright"}},
		{Manager: "pipx", Args: []string{"install", "pyright"}},
	},
}

// Types type-checks the given files: tsc for TypeScript, pyright for Python.
// Other extensions are already compiled by their linters and add nothing.
func Types(root string, files []string, block bool) []gitx.Finding {
	var ts, py []string
	for _, f := range files {
		if !filepath.IsAbs(f) {
			f = filepath.Join(root, f)
		}
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx", ".mts", ".cts":
			ts = append(ts, f)
		case ".py", ".pyi":
			py = append(py, f)
		}
	}
	var out []gitx.Finding
	if len(ts) > 0 {
		out = append(out, typeCheckTS(root, ts, block)...)
	}
	if len(py) > 0 {
		out = append(out, typeCheckPy(root, py, block)...)
	}
	return out
}

// typeCheckTS groups the files under their nearest tsconfig.json (the way
// tsc itself resolves a project) and runs `tsc --noEmit` per project.
func typeCheckTS(root string, files []string, block bool) []gitx.Finding {
	byCfg := map[string][]string{}
	var out []gitx.Finding
	for _, f := range files {
		dir := nearestFileDir(root, f, "tsconfig.json")
		if dir == "" {
			out = append(out, gitx.Finding{File: f,
				Message: "type check needs a tsconfig.json — out of scope until the project carries one (lint --types)"})
			continue
		}
		byCfg[dir] = append(byCfg[dir], f)
	}
	dirs := make([]string, 0, len(byCfg))
	for d := range byCfg {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		fs := byCfg[dir]
		// a project-local typescript may live beside the config, not the root
		bin := tools.Resolve(Tsc, dir)
		if bin == "" {
			bin = tools.Resolve(Tsc, root)
		}
		if bin == "" {
			out = append(out, notChecked(fs[0], "tsc (typescript)")...)
			continue
		}
		raw, err := execute(dir, bin, []string{"--noEmit", "--pretty", "false"})
		findings := parseTsc(raw, dir, root, block)
		if len(findings) == 0 && err != nil {
			out = append(out, gitx.Finding{File: fs[0],
				Message: fmt.Sprintf("NOT checked — tsc failed: %s (lint --types)", firstLine(raw+err.Error()))})
			continue
		}
		// debt: tsc checks the whole project but only findings in the asked
		// files are kept (the clippy precedent) — cross-file fallout hides;
		// revisit if a rename or refactor ships broken siblings past this.
		wanted := map[string]bool{}
		for _, f := range fs {
			if rel, rerr := filepath.Rel(root, f); rerr == nil {
				wanted[filepath.ToSlash(rel)] = true
			}
		}
		for _, f := range findings {
			if wanted[filepath.ToSlash(f.File)] {
				out = append(out, f)
			}
		}
	}
	return out
}

// tscLine matches `path(line,col): error TS1234: message` (--pretty false).
var tscLine = regexp.MustCompile(`^(.+?)\((\d+),\d+\): (error|warning) (TS\d+): (.+)$`)

// parseTsc reads tsc's plain output; paths arrive relative to the run
// directory and are rebased onto the repo root.
func parseTsc(raw, dir, root string, block bool) []gitx.Finding {
	var out []gitx.Finding
	for _, line := range strings.Split(raw, "\n") {
		m := tscLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		path := m[1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
			path = filepath.ToSlash(rel)
		}
		out = append(out, gitx.Finding{File: path, Line: n, Blocking: block,
			Message: fmt.Sprintf("%s [%s] (lint --types)", m[5], m[4])})
	}
	return out
}

// typeCheckPy runs pyright over exactly the asked files; pyright resolves
// the project's own pyproject/pyrightconfig settings from the paths.
func typeCheckPy(root string, files []string, block bool) []gitx.Finding {
	bin := tools.Resolve(Pyright, root)
	if bin == "" {
		return notChecked(files[0], "pyright")
	}
	raw, err := execute(root, bin, append([]string{"--outputjson"}, files...))
	findings := parsePyright(raw, root, block)
	if len(findings) == 0 && err != nil {
		var exit *exec.ExitError
		// pyright exits 1 when it reported errors; anything else with no
		// parsed diagnostics is a failed run
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || raw == "" {
			return []gitx.Finding{{File: files[0],
				Message: fmt.Sprintf("NOT checked — pyright failed: %s (lint --types)", firstLine(raw+err.Error()))}}
		}
	}
	return findings
}

// parsePyright reads pyright's --outputjson diagnostics (0-based lines).
func parsePyright(raw, root string, block bool) []gitx.Finding {
	var report struct {
		GeneralDiagnostics []struct {
			File     string `json:"file"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
			Rule     string `json:"rule"`
			Range    struct {
				Start struct {
					Line int `json:"line"`
				} `json:"start"`
			} `json:"range"`
		} `json:"generalDiagnostics"`
	}
	if json.Unmarshal([]byte(raw), &report) != nil {
		return nil
	}
	var out []gitx.Finding
	for _, d := range report.GeneralDiagnostics {
		if d.Severity != "error" && d.Severity != "warning" {
			continue
		}
		path := d.File
		if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
			path = filepath.ToSlash(rel)
		}
		msg := strings.SplitN(d.Message, "\n", 2)[0]
		rule := d.Rule
		if rule == "" {
			rule = d.Severity
		}
		out = append(out, gitx.Finding{File: path, Line: d.Range.Start.Line + 1, Blocking: block,
			Message: fmt.Sprintf("%s [%s] (lint --types)", msg, rule)})
	}
	return out
}

// nearestFileDir ascends from the file's directory to the repo root looking
// for name; empty means it is absent all the way up.
func nearestFileDir(root, file, name string) string {
	dir := filepath.Dir(file)
	for {
		if fileExists(filepath.Join(dir, name)) {
			return dir
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
