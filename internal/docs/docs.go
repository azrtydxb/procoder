// Package docs is domain 5: documentation treated as a product. Correct (no
// broken references, no silent drift), presentable (diagrams, badges, a
// README that sells), delivered (rendered via CI). The binary computes
// findings in the gitx shape; the agent judges and acts (P-CONTROL). Every
// rule is repo-overridable via .procoder/docs/RULES.md (D-OVERRIDE).
package docs

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"procoder/internal/actions"
	"procoder/internal/gitx"
	"procoder/internal/tools"
)

// RulesPath is the repo's rules file (D-OVERRIDE): what is written there wins
// over the defaults compiled into this package.
const RulesPath = ".procoder/docs/RULES.md"

// MermaidConfigPath is the shared diagram theme, applied when compiling.
const MermaidConfigPath = ".procoder/docs/mermaid.json"

const hungToolTimeout = 30 * time.Second

// mermaidTimeout gives puppeteer's browser launch the headroom a cold CI
// runner needs; 30s was observed flaking while the twin run passed.
const mermaidTimeout = 90 * time.Second

// Registered tools: doctor and init treat them like any formatter.
var Lychee = &tools.Tool{
	Name:        "lychee",
	Install:     "brew install lychee   (or: cargo install lychee)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "lychee"}},
		{Manager: "cargo", Args: []string{"install", "lychee"}},
	},
}

// Mmdc is the Mermaid compiler; a diagram that does not compile is treated
// like a broken link.
var Mmdc = &tools.Tool{
	Name:        "mmdc",
	Install:     "npm i -g @mermaid-js/mermaid-cli",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "npm", Args: []string{"install", "-g", "@mermaid-js/mermaid-cli"}},
	},
}

// Mkdocs builds the docs site the CI job deploys to GitHub Pages.
var Mkdocs = &tools.Tool{
	Name:        "mkdocs",
	Install:     "pipx install mkdocs-material   (mkdocs comes with it)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		// mkdocs-material has no entry point of its own, so pipx must be told
		// the app is mkdocs — plain `pipx install mkdocs-material` fails.
		{Manager: "pipx", Args: []string{"install", "--preinstall", "mkdocs-material", "mkdocs"}},
		{Manager: "pip3", Args: []string{"install", "--user", "mkdocs-material"}},
		{Manager: "brew", Args: []string{"install", "mkdocs"}},
	},
}

// IsMarkdownFile reports whether path is a Markdown document.
func IsMarkdownFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

// Rules is what the repo's RULES.md dictates. Zero value means "use defaults".
type Rules struct {
	RequiredDocs   []string // files that must exist at the repo root
	RequiredBadges []string // substrings that must appear in a README badge image URL or alt text
	ReadmeSections []string // headings/elements required on the README's first screen
}

func defaultRules() Rules {
	return Rules{
		RequiredDocs:   []string{"README.md", "CHANGELOG.md"},
		RequiredBadges: []string{"ci", "license"},
		ReadmeSections: []string{"usp", "badges", "quick start"},
	}
}

// LoadRules reads the repo's RULES.md. The file is prose for the agent with
// three machine-readable list sections; a list that is present replaces the
// default for that section, absent sections keep their defaults.
func LoadRules(root string) Rules {
	r := defaultRules()
	data, err := os.ReadFile(filepath.Join(root, RulesPath))
	if err != nil {
		return r
	}
	section := ""
	var docsL, badgesL, secsL []string
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") {
			section = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(t, "## ")))
			continue
		}
		if !strings.HasPrefix(t, "- ") {
			continue
		}
		item := strings.TrimSpace(strings.Trim(strings.TrimPrefix(t, "- "), "`"))
		switch section {
		case "required docs":
			docsL = append(docsL, item)
		case "required badges":
			badgesL = append(badgesL, strings.ToLower(item))
		case "readme first screen":
			secsL = append(secsL, strings.ToLower(item))
		}
	}
	if docsL != nil {
		r.RequiredDocs = docsL
	}
	if badgesL != nil {
		r.RequiredBadges = badgesL
	}
	if secsL != nil {
		r.ReadmeSections = secsL
	}
	return r
}

// mdLink matches [text](target) and ![alt](target); group 1 is the target.
var mdLink = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)[^)]*\)`)

// CheckFile runs the offline per-file checks on one Markdown file: relative
// references and Mermaid compilation. This is what the write hook uses — it
// must never touch the network.
func CheckFile(root, file string) []gitx.Finding {
	out := RelativeRefs(root, file)
	out = append(out, MermaidBlocks(root, file)...)
	return out
}

// RelativeRefs finds links and images whose relative target does not resolve.
// Objectively broken, so blocking.
func RelativeRefs(root, file string) []gitx.Finding {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var out []gitx.Finding
	inFence := false
	for i, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		for _, m := range mdLink.FindAllStringSubmatch(line, -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") ||
				strings.HasPrefix(target, "#") || strings.HasPrefix(target, "data:") {
				continue
			}
			// strip anchors and query strings from file targets
			if j := strings.IndexAny(target, "#?"); j >= 0 {
				target = target[:j]
			}
			if target == "" {
				continue
			}
			resolved := filepath.Join(filepath.Dir(file), filepath.FromSlash(target))
			if strings.HasPrefix(target, "/") {
				resolved = filepath.Join(root, filepath.FromSlash(target))
			}
			if _, err := os.Stat(resolved); err != nil {
				out = append(out, gitx.Finding{File: file, Line: i + 1, Blocking: true,
					Message: fmt.Sprintf("broken reference: %q does not resolve", m[1])})
			}
		}
	}
	return out
}

// mermaidBlock is one fenced ```mermaid block with its starting line number.
type mermaidBlock struct {
	line int
	body string
}

func mermaidBlocks(data string) []mermaidBlock {
	var blocks []mermaidBlock
	var cur *mermaidBlock
	for i, line := range strings.Split(data, "\n") {
		t := strings.TrimSpace(line)
		if cur == nil && strings.HasPrefix(t, "```mermaid") {
			cur = &mermaidBlock{line: i + 1}
			continue
		}
		if cur != nil {
			if strings.HasPrefix(t, "```") {
				blocks = append(blocks, *cur)
				cur = nil
				continue
			}
			cur.body += line + "\n"
		}
	}
	return blocks
}

// MermaidBlocks compiles every ```mermaid fence with mmdc — a diagram that
// does not compile is a broken link, so blocking. The three-way honesty rule
// applies: a file with diagrams and no compiler reads as NOT checked, never
// as clean.
func MermaidBlocks(root, file string) []gitx.Finding {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	blocks := mermaidBlocks(string(data))
	if len(blocks) == 0 {
		return nil
	}
	bin := tools.Resolve(Mmdc, filepath.Dir(file))
	if bin == "" {
		return []gitx.Finding{{File: file, Line: blocks[0].line, Blocking: true,
			Message: "NOT checked — this file has Mermaid diagrams and mmdc is not installed; run `procoder init`"}}
	}
	var out []gitx.Finding
	for _, b := range blocks {
		if msg := compileMermaid(root, bin, b.body); msg != "" {
			out = append(out, gitx.Finding{File: file, Line: b.line, Blocking: true,
				Message: "Mermaid diagram does not compile: " + msg})
		}
	}
	return out
}

func compileMermaid(root, bin, body string) string {
	dir, err := os.MkdirTemp("", "procoder-mmd")
	if err != nil {
		return err.Error()
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "d.mmd")
	if err := os.WriteFile(in, []byte(body), 0o644); err != nil {
		return err.Error()
	}
	args := []string{"-i", in, "-o", filepath.Join(dir, "d.svg"), "--quiet"}
	if _, err := os.Stat(filepath.Join(root, MermaidConfigPath)); err == nil {
		args = append(args, "--configFile", filepath.Join(root, MermaidConfigPath))
	}
	// CI runners need puppeteer told which browser to use and to skip the
	// sandbox; the environment provides that, never the repo (a committed
	// executablePath would be wrong on every other machine).
	if pc := os.Getenv("PROCODER_PUPPETEER_CONFIG"); pc != "" {
		args = append(args, "--puppeteerConfigFile", pc)
	}
	// mmdc launches a whole browser; a cold start on a busy CI runner can
	// exceed the ordinary tool budget without anything being wrong
	ctx, cancel := context.WithTimeout(context.Background(), mermaidTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("mmdc gave no answer in %s — the diagram was NOT checked", mermaidTimeout)
	}
	if err != nil {
		return firstLine(buf.String())
	}
	return ""
}

// Drift reports docs that mention a changed file, so the agent verifies the
// prose is still true. Only the agent can judge that, so: report, not block.
// ponytail: path-mention matching only; command/symbol matching when this
// proves too coarse.
func Drift(root string, changed []string) []gitx.Finding {
	var code []string
	for _, f := range changed {
		if IsMarkdownFile(f) {
			continue
		}
		// git status lists untracked directories as single entries; a bare
		// directory name matches prose everywhere and means nothing.
		if info, err := os.Stat(f); err != nil || info.IsDir() {
			continue
		}
		if rel, err := filepath.Rel(root, f); err == nil {
			code = append(code, filepath.ToSlash(rel))
		}
	}
	if len(code) == 0 {
		return nil
	}
	var out []gitx.Finding
	for _, md := range MarkdownFiles(root) {
		data, err := os.ReadFile(md)
		if err != nil {
			continue
		}
		text := string(data)
		for _, path := range code {
			if strings.Contains(text, path) {
				out = append(out, gitx.Finding{File: md,
					Message: fmt.Sprintf("mentions changed file %s — verify the doc is still true, update it if not", path)})
			}
		}
	}
	return out
}

// MarkdownFiles lists the repository's Markdown files: what git considers
// part of the repo (tracked, plus untracked-but-not-ignored), so gitignored
// scratch directories are never scanned. Outside a git repo it falls back to
// a directory walk that skips the places no doc lives.
func MarkdownFiles(root string) []string {
	if out, ok := gitMarkdownFiles(root); ok {
		return out
	}
	var out []string
	skip := map[string]bool{".git": true, "node_modules": true, "vendor": true,
		"dist": true, ".claude": true}
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if IsMarkdownFile(path) {
			out = append(out, path)
		}
		return nil
	})
	return out
}

func gitMarkdownFiles(root string) ([]string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root,
		"ls-files", "--cached", "--others", "--exclude-standard")
	raw, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line != "" && IsMarkdownFile(line) {
			out = append(out, filepath.Join(root, filepath.FromSlash(line)))
		}
	}
	return out, true
}

// RequiredDocs reports rule-listed documents missing at the repo root, and a
// missing rules file itself.
func RequiredDocs(root string, r Rules) []gitx.Finding {
	var out []gitx.Finding
	for _, name := range r.RequiredDocs {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			out = append(out, gitx.Finding{Message: name + " is missing — required by the docs rules"})
		}
	}
	if _, err := os.Stat(filepath.Join(root, RulesPath)); err != nil {
		out = append(out, gitx.Finding{Message: RulesPath + " is missing — run `procoder templates` and write it"})
	}
	return out
}

// firstScreen is how much of the README counts as "what a visitor sees first".
const firstScreenLines = 40

// Badges checks the required badge set appears in the README's first screen.
func Badges(root string, r Rules) []gitx.Finding {
	readme := filepath.Join(root, "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		return nil // RequiredDocs already reports a missing README
	}
	head := strings.ToLower(firstN(string(data), firstScreenLines))
	// only what is inside a badge image counts — the keyword appearing in
	// prose is not a badge
	badges := strings.Join(badgeRe.FindAllString(head, -1), "\n")
	var out []gitx.Finding
	for _, want := range r.RequiredBadges {
		if !strings.Contains(badges, strings.ToLower(want)) {
			out = append(out, gitx.Finding{File: readme,
				Message: fmt.Sprintf("README first screen is missing a %q badge (docs rules)", want)})
		}
	}
	return out
}

var badgeRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`)

// ReadmeStructure checks the USP-first first screen: the structure the rules
// demand exists; the agent writes the copy that lures people.
func ReadmeStructure(root string, r Rules) []gitx.Finding {
	readme := filepath.Join(root, "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		return nil
	}
	head := firstN(string(data), firstScreenLines)
	low := strings.ToLower(head)
	var out []gitx.Finding
	for _, want := range r.ReadmeSections {
		ok := false
		switch want {
		case "usp":
			// a non-empty prose line before the first ## heading that is not a
			// badge row: the one-liner that says why this exists
			ok = hasUSP(head)
		case "badges":
			ok = badgeRe.MatchString(low)
		default:
			ok = strings.Contains(low, want)
		}
		if !ok {
			out = append(out, gitx.Finding{File: readme,
				Message: fmt.Sprintf("README first screen is missing %q (docs rules) — the first screen must sell the project", want)})
		}
	}
	return out
}

func hasUSP(head string) bool {
	for _, line := range strings.Split(head, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") {
			return false // the pitch belongs above the first section heading
		}
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "![") ||
			strings.HasPrefix(t, "[!") || strings.HasPrefix(t, "<") {
			continue
		}
		return true
	}
	return false
}

// ExternalLinks verifies http(s) links with lychee. Never skipped — but never
// run from the per-write hook either: this is for `procoder docs --external`,
// the gate's CI run, and the docs CI job.
func ExternalLinks(root string, files []string) []gitx.Finding {
	if len(files) == 0 {
		return nil
	}
	bin := tools.Resolve(Lychee, root)
	if bin == "" {
		return []gitx.Finding{{File: files[0], Blocking: true,
			Message: "NOT checked — external links require lychee; run `procoder init`"}}
	}
	// retries + cache: one flaky server never reds a build twice in a row
	args := []string{"--no-progress", "--format", "markdown",
		"--max-retries", "3", "--cache", "--max-cache-age", "1d",
		"--scheme", "http", "--scheme", "https"}
	args = append(args, files...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return []gitx.Finding{{File: files[0], Blocking: true,
			Message: "lychee gave no answer in 5m — external links were NOT checked"}}
	}
	if err == nil {
		return nil
	}
	var out []gitx.Finding
	for _, line := range strings.Split(buf.String(), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "* [") || strings.Contains(t, "[ERROR]") {
			out = append(out, gitx.Finding{Blocking: true, Message: "dead external link: " + t})
		}
	}
	if len(out) == 0 {
		out = append(out, gitx.Finding{File: files[0], Blocking: true,
			Message: "lychee failed without a parseable report — external links were NOT checked: " + firstLine(buf.String())})
	}
	return out
}

// PagesHealth checks GitHub Pages is enabled and its latest build succeeded.
// Network + gh, so it runs with --external, and only for GitHub repos.
func PagesHealth(root string) []gitx.Finding {
	if tools.Resolve(actions.Gh, root) == "" {
		return []gitx.Finding{{Message: "GitHub Pages NOT checked — gh is not installed; run `procoder init`"}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "api", "repos/{owner}/{repo}/pages", "--jq", ".status")
	cmd.Dir = root
	var buf, errb bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if strings.Contains(errb.String(), "Not Found") {
			return []gitx.Finding{{Message: "GitHub Pages is not enabled for this repository — the docs site is not being served"}}
		}
		return []gitx.Finding{{Message: "GitHub Pages NOT checked: " + firstLine(errb.String())}}
	}
	status := strings.TrimSpace(buf.String())
	if status != "built" {
		return []gitx.Finding{{Message: "GitHub Pages status is " + status + " — the published docs are not the latest"}}
	}
	return nil
}

func firstN(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func firstLine(s string) string {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			if len(t) > 160 {
				t = t[:160]
			}
			return t
		}
	}
	return "no output"
}
