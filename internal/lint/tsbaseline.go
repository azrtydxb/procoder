package lint

import (
	"fmt"
	"os"
	"path/filepath"
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
	Resolved: func(root string) string {
		if root == "" {
			return ""
		}
		return tsESLintEntry(root, filepath.Join(root, "x"))
	},
	// Project-local, not global: the baseline config imports the entry
	// module by absolute path, resolved by walking up from the file being
	// linted. A global install lands somewhere that walk never reaches, so
	// `init` would report success and the lint would still say NOT checked.
	InstallVia: []tools.InstallCandidate{
		{Manager: "npm", Args: []string{"install", "-D", "typescript-eslint"}},
	},
}

// tsESLintEntry finds the typescript-eslint entry module by walking up from
// the file, the same way node resolves a package. The absolute path is what
// the generated config imports: the config lives in a temp directory, so a
// bare package name would resolve against that directory and find nothing.
func tsESLintEntry(root string, file string) string {
	dir := filepath.Dir(file)
	for {
		entry := filepath.Join(dir, "node_modules", "typescript-eslint", "dist", "index.js")
		if info, err := os.Stat(entry); err == nil && !info.IsDir() {
			return entry
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
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
