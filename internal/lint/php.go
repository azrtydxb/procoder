package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"procoder/internal/gitx"
	"procoder/internal/textutil"
	"procoder/internal/tools"
)

// PHP has several linters in common use and no single winner, so procoder
// runs the ones the project configured and imposes none of its own. This is
// the eslint and clang-format precedent (D-OVERRIDE: the project's config
// always wins) applied to a language where the choice is genuinely
// contested — a repository that picked phpstan gets phpstan, one that
// picked phpcs gets phpcs, one that picked both gets both.
var (
	Phpstan = &tools.Tool{
		Name:        "phpstan",
		Install:     "composer require --dev phpstan/phpstan",
		VersionArgs: []string{"--version"},
		InstallVia: []tools.InstallCandidate{
			{Manager: "composer", Args: []string{"global", "require", "phpstan/phpstan"}},
			{Manager: "brew", Args: []string{"install", "phpstan"}},
		},
	}
	Phpcs = &tools.Tool{
		Name:        "phpcs",
		Install:     "composer require --dev squizlabs/php_codesniffer",
		VersionArgs: []string{"--version"},
		InstallVia: []tools.InstallCandidate{
			{Manager: "composer", Args: []string{"global", "require", "squizlabs/php_codesniffer"}},
			{Manager: "brew", Args: []string{"install", "php-code-sniffer"}},
		},
	}
	// PHP is the floor, and it is the language's own binary: a repository
	// with no linter configured still gets its syntax errors caught, and a
	// syntax error is never a matter of taste.
	PHP = &tools.Tool{
		Name:        "php",
		Install:     "https://www.php.net/downloads",
		VersionArgs: []string{"--version"},
		InstallVia: []tools.InstallCandidate{
			{Manager: "brew", Args: []string{"install", "php"}},
			{Manager: "apt-get", Args: []string{"install", "-y", "php-cli"}},
			{Manager: "dnf", Args: []string{"install", "-y", "php-cli"}},
		},
	}
)

// phpConfigs names the config files that select a linter. The .dist variant
// is how PHP projects ship a committed default a developer may override
// locally, so both spellings count as configured.
var (
	phpstanConfigs = []string{"phpstan.neon", "phpstan.neon.dist", "phpstan.dist.neon"}
	phpcsConfigs   = []string{"phpcs.xml", "phpcs.xml.dist", ".phpcs.xml", ".phpcs.xml.dist"}
)

// lintPHP runs whichever linters the project configured, falling back to
// `php -l` when it configured none.
func lintPHP(root string, files []string, block bool) []gitx.Finding {
	var out []gitx.Finding
	stan := hasAny(root, files[0], phpstanConfigs)
	cs := hasAny(root, files[0], phpcsConfigs)

	if stan != "" {
		out = append(out, runPhpstan(root, stan, files, block)...)
	}
	if cs != "" {
		out = append(out, run(root, Phpcs, append([]string{"--report=emacs", "--no-colors"}, files...), files, block)...)
	}
	if stan == "" && cs == "" {
		out = append(out, lintPHPBaseline(root, files, block)...)
	}
	return out
}

// phpstanBaseline is procoder's curated default for a PHP project that
// carries no linter config of its own — the same bargain Go gets from the
// golangci baseline: a real linter rather than nothing, written to a temp
// file so the repository is never touched, and overridden the moment the
// project adds its own config.
//
// Level 5 is the level, chosen by measuring rather than by taste. Against
// ordinary untyped legacy PHP — no declare, no typehints, associative
// arrays everywhere — levels 0 through 5 report nothing, while level 6
// demands a typehint on every parameter and produced four findings on a
// fourteen-line file. A default that shouts at every existing PHP codebase
// on the day it is installed is a default people turn off. Level 5 still
// catches the things that are bugs in any style: a function that returns
// the wrong type, a call to something that does not exist.
const phpstanBaseline = `parameters:
    level: 5
`

// lintPHPBaseline lints a project that configured nothing, with procoder's
// own default. When phpstan is absent it says so — the tool is named so
// `procoder init` installs it — and still runs `php -l`, because a syntax
// error caught by the binary already on the machine is worth more than a
// silence explained by a footnote.
func lintPHPBaseline(root string, files []string, block bool) []gitx.Finding {
	if tools.Resolve(Phpstan, root) == "" {
		out := notChecked(files[0], Phpstan.Name)
		return append(out, lintPHPSyntax(root, files, block)...)
	}
	cfg, err := os.CreateTemp("", "procoder-phpstan-*.neon")
	if err == nil {
		_, err = cfg.WriteString(phpstanBaseline)
		// a failed Close can mean the write never reached disk; treat it as
		// a write failure rather than a success
		if cerr := cfg.Close(); err == nil {
			err = cerr
		}
		defer func() { _ = os.Remove(cfg.Name()) }()
	}
	if err != nil {
		// The baseline could not be written, so the analysis procoder
		// promised did not happen. Falling silently back to the syntax
		// floor would report a thinner check as if it were the full one.
		out := []gitx.Finding{{Blocking: true, File: files[0],
			Message: fmt.Sprintf("NOT checked — could not write the phpstan baseline: %v (lint)", err)}}
		return append(out, lintPHPSyntax(root, files, block)...)
	}
	return runPhpstan(root, cfg.Name(), files, block)
}

// hasAny walks up from the file to the repository root looking for any of
// the named config files, and returns the one it found. Walking rather than
// checking the root only: a monorepo puts one package's phpstan.neon beside
// that package, not at the top.
func hasAny(root, file string, names []string) string {
	dir := filepath.Dir(file)
	for {
		for _, n := range names {
			p := filepath.Join(dir, n)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir || dir == root {
			return ""
		}
		dir = parent
	}
}

// runPhpstan analyses the given files with the project's own configuration.
// -c names the config explicitly because phpstan resolves its config from
// the working directory, and procoder's working directory is not the
// project's.
func runPhpstan(root, config string, files []string, block bool) []gitx.Finding {
	bin := tools.Resolve(Phpstan, root)
	if bin == "" {
		return notChecked(files[0], Phpstan.Name)
	}
	args := append([]string{"analyse", "--no-progress", "--error-format=raw", "-c", config}, files...)
	raw, err := execute(root, bin, args)
	out := parsePhpstan(raw, block)
	if len(out) == 0 && err != nil && !strings.Contains(raw, "No errors") {
		return []gitx.Finding{{Blocking: true, File: files[0],
			Message: fmt.Sprintf("NOT checked — phpstan failed: %s (lint)", textutil.FirstLine(raw+err.Error()))}}
	}
	return out
}

// phpstanLine matches phpstan's raw format: `path:line:message`, with NO
// space after the line number. The shared finding parser requires that
// space, so reusing it here would silently discard every phpstan finding
// and report a clean lint over a file full of errors.
var phpstanLine = regexp.MustCompile(`^(.+?):(\d+):(.+)$`)

func parsePhpstan(raw string, block bool) []gitx.Finding {
	var out []gitx.Finding
	for _, line := range strings.Split(raw, "\n") {
		m := phpstanLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || !strings.Contains(m[1], ".") {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		// phpstan appends `[identifier=return.type]`, which names the rule
		// rather than the problem; the reader gets the sentence.
		msg := strings.TrimSpace(identifierSuffix.ReplaceAllString(m[3], ""))
		out = append(out, gitx.Finding{File: m[1], Line: n, Blocking: block,
			Message: msg + " (lint)"})
	}
	return out
}

var identifierSuffix = regexp.MustCompile(`\s*\[identifier=[^\]]+\]\s*$`)

// lintPHPSyntax is the floor: `php -l` per file. It reports only what
// cannot be argued with — a file that does not parse — and says nothing
// about style, because nobody chose a style here.
func lintPHPSyntax(root string, files []string, block bool) []gitx.Finding {
	bin := tools.Resolve(PHP, root)
	if bin == "" {
		return notChecked(files[0], PHP.Name)
	}
	var out []gitx.Finding
	for _, f := range files {
		// One file per run: `php -l` takes a single file, and passing
		// several silently checks only the first.
		raw, err := execute(root, bin, []string{"-l", "-n", f})
		if err == nil {
			continue // parses cleanly; there is nothing else php -l can say
		}
		out = append(out, parsePHPSyntax(raw, f, block)...)
	}
	return out
}

// phpParseLine matches `Parse error: <message> in <file> on line <n>`.
var phpParseLine = regexp.MustCompile(`^(?:PHP )?(?:Parse|Fatal) error:\s*(.+?) in (.+) on line (\d+)`)

func parsePHPSyntax(raw, file string, block bool) []gitx.Finding {
	for _, line := range strings.Split(raw, "\n") {
		m := phpParseLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[3])
		return []gitx.Finding{{File: m[2], Line: n, Blocking: block,
			Message: strings.TrimSpace(m[1]) + " (lint)"}}
	}
	// php exited non-zero and said something this does not recognise.
	// Unknown is not clean.
	return []gitx.Finding{{Blocking: true, File: file,
		Message: fmt.Sprintf("NOT checked — php -l failed: %s (lint)", textutil.FirstLine(raw))}}
}

// HasPhpstanConfig and HasPhpcsConfig report whether the repository
// selected that linter. doctor asks so it can name the tools this
// repository will actually run, rather than every linter PHP has.
func HasPhpstanConfig(root string) bool {
	return hasAny(root, filepath.Join(root, "x"), phpstanConfigs) != ""
}
func HasPhpcsConfig(root string) bool {
	return hasAny(root, filepath.Join(root, "x"), phpcsConfigs) != ""
}
