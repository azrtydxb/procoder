package lint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"procoder/internal/gitx"
	"procoder/internal/tools"
)

// TypescriptEslint is the parser that lets eslint read TypeScript. eslint
// core cannot: it parses JavaScript, and a `.ts` file is a syntax error to
// it. procoder used to treat that as a reason to check nothing and call
// TypeScript out of scope until the project wrote its own eslint config —
// which meant the most common TypeScript setup there is, a repo with a
// tsconfig and no eslint config, got no linting and a green gate.
//
// It is a tool, so it is installed like a tool: doctor names it, init
// installs it, and its absence is NOT checked rather than silence.
var TypescriptEslint = &tools.Tool{
	Name:    "typescript-eslint",
	Install: "npm i -D typescript-eslint",
	// A library, not a binary: presence is the entry module being
	// resolvable, not a name on PATH.
	Resolved: resolvedTSESLint,
	// Project-local, not global: the baseline config imports the entry
	// module by absolute path, resolved by walking up from the file being
	// linted. A global install lands somewhere that walk never reaches, so
	// `init` would report success and the lint would still say NOT checked.
	InstallVia: []tools.InstallCandidate{
		{Manager: "npm", Args: []string{"install", "-D", "typescript-eslint"}},
	},
}

// tsESLintMaxMajor is the newest TypeScript major typescript-eslint
// installs against. Its peer range ended at <6.1.0 when TypeScript 7
// shipped, so a TS 7 project cannot install it at all: npm refuses with
// ERESOLVE, and --legacy-peer-deps is an unsupported install (#324).
//
// debt: a hand-kept copy of typescript-eslint's peer range — ceiling: a
// project past it is parsed by procoder's own TypeScript 6 copy even after
// typescript-eslint learns its version, so syntax new in that major reads as
// a parse error; revisit when typescript-eslint publishes a release whose
// peer range covers TypeScript 7, and raise this.
const tsESLintMaxMajor = 6

// TypescriptEslintTS7 is typescript-eslint for a project whose TypeScript
// is past tsESLintMaxMajor. The project cannot install it, so procoder
// keeps its own copy — typescript-eslint beside a TypeScript 6 — outside
// the repository, and the baseline parses with that. TypeScript 7 is the
// compiler ported to Go, not a new language: the syntax a TypeScript 6
// parser reads is the syntax the project writes, and the recommended set
// the baseline runs needs no type information from the project's compiler.
// The type check stays the project's own: `procoder lint --types` runs the
// project's tsc, TypeScript 7 included.
var TypescriptEslintTS7 = &tools.Tool{
	Name:     "typescript-eslint",
	Install:  "npm install --prefix " + ownedTSESLintDir() + " --no-save --no-package-lock typescript-eslint typescript@~6.0",
	Resolved: resolvedTSESLint,
	InstallVia: []tools.InstallCandidate{
		{Manager: "npm", Args: []string{"install", "--prefix", ownedTSESLintDir(),
			"--no-save", "--no-package-lock", "typescript-eslint", "typescript@~6.0"}},
	},
}

// TypescriptEslintFor is the typescript-eslint the project under root
// installs: its own, or procoder's copy when its TypeScript is past the
// range typescript-eslint supports.
func TypescriptEslintFor(root string) *tools.Tool {
	if tsMajor(filepath.Join(root, "x")) > tsESLintMaxMajor {
		return TypescriptEslintTS7
	}
	return TypescriptEslint
}

func resolvedTSESLint(root string) string {
	if root == "" {
		return ""
	}
	return tsESLintEntry(root, filepath.Join(root, "x"))
}

// ownedTSESLintDir is where procoder keeps its own typescript-eslint for
// projects past tsESLintMaxMajor: the user cache, so it survives a reboot
// and is shared by every such repository on the machine.
func ownedTSESLintDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "procoder", "typescript-eslint-ts6")
}

// tsMajor is the major version of the installed TypeScript nearest the
// file — the one node would load for it — or 0 when none is installed or
// its manifest cannot be read. 0 is "not known to be past the range": the
// project-local install is then the one worth trying.
func tsMajor(file string) int {
	dir := filepath.Dir(file)
	for {
		raw, err := os.ReadFile(filepath.Join(dir, "node_modules", "typescript", "package.json"))
		if err == nil {
			var manifest struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(raw, &manifest) != nil {
				return 0
			}
			major, _, _ := strings.Cut(manifest.Version, ".")
			n, err := strconv.Atoi(major)
			if err != nil {
				return 0
			}
			return n
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return 0
		}
		dir = parent
	}
}

// tsESLintEntry finds the typescript-eslint entry module for the file. The
// absolute path is what the generated config imports: the config lives in
// a temp directory, so a bare package name would resolve against that
// directory and find nothing.
//
// A project past tsESLintMaxMajor gets procoder's copy first. A copy the
// project forced in with --legacy-peer-deps loads the project's TypeScript,
// which typescript-eslint does not support; procoder's copy loads its own
// TypeScript 6, which it does.
func tsESLintEntry(root string, file string) string {
	if tsMajor(file) > tsESLintMaxMajor {
		if entry := entryIn(ownedTSESLintDir()); entry != "" {
			return entry
		}
	}
	dir := filepath.Dir(file)
	for {
		if entry := entryIn(dir); entry != "" {
			return entry
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// entryIn is typescript-eslint's entry module under dir/node_modules, or "".
func entryIn(dir string) string {
	entry := filepath.Join(dir, "node_modules", "typescript-eslint", "dist", "index.js")
	if info, err := os.Stat(entry); err == nil && !info.IsDir() {
		return entry
	}
	return ""
}

// tsBaselineConfig is procoder's default for TypeScript that the project
// has not configured: typescript-eslint's own recommended set. Recommended
// rather than strict or type-checked — the type-checked rules need a
// tsconfig wired into the lint run and report on patterns that are matters
// of house style, and a default that fires everywhere on the day it is
// installed is a default people switch off.
func tsBaselineConfig(entry string) string {
	return fmt.Sprintf(`import tseslint from %q;
export default tseslint.config(
  { basePath: process.cwd() },
  ...tseslint.configs.recommended,
);
`, filepath.ToSlash(entry))
}

// lintTSBaseline lints TypeScript that no project config covers.
func lintTSBaseline(root string, files []string, block bool) []gitx.Finding {
	bin := tools.Resolve(Eslint, root)
	if bin == "" {
		return notChecked(files[0], "eslint")
	}
	entry := tsESLintEntry(root, files[0])
	if entry == "" {
		return notChecked(files[0], TypescriptEslint.Name)
	}
	cfgFile, err := os.CreateTemp("", "procoder-tseslint-*.mjs")
	if err == nil {
		_, err = cfgFile.WriteString(tsBaselineConfig(entry))
		// a failed Close can mean the write never reached disk
		if cerr := cfgFile.Close(); err == nil {
			err = cerr
		}
		defer func() { _ = os.Remove(cfgFile.Name()) }()
	}
	if err != nil {
		return []gitx.Finding{{Blocking: true, File: files[0],
			Message: fmt.Sprintf("NOT checked — could not write the TypeScript baseline: %v (lint)", err)}}
	}
	// paths relative to the repo root: node resolves the cwd through
	// symlinks, and absolute args read as outside the base path
	rel := make([]string, 0, len(files))
	for _, f := range files {
		if r, err := filepath.Rel(root, f); err == nil && !strings.HasPrefix(r, "..") {
			rel = append(rel, r)
		} else {
			rel = append(rel, f)
		}
	}
	raw, runErr := execute(root, bin, append([]string{"--format", "json",
		"--no-config-lookup", "--config", cfgFile.Name()}, rel...))
	return parseEslintJSON(raw, runErr, files[0], "lint, procoder baseline", block)
}
