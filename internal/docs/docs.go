// Package docs is domain 5: documentation treated as a product. Correct (no
// broken references, no silent drift), presentable (diagrams, badges, a
// README that sells), delivered (rendered via CI). The binary computes
// findings in the gitx shape; the agent judges and acts (P-CONTROL). Every
// rule is repo-overridable via .procoder/docs/RULES.md (D-OVERRIDE).
package docs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"procoder/internal/actions"
	"procoder/internal/gitx"
	"procoder/internal/textutil"
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
	VersionedDocs  []string // docs whose first screen must carry the current version
	ReadmeMentions []string // phrases the README's narrative must carry — feature families, not commands
	VersionSources []string // `path` or `path:key` for JSON files declaring the project's version; `none` switches the check off
}

func defaultRules() Rules {
	return Rules{
		RequiredDocs:   []string{"README.md", "CHANGELOG.md"},
		RequiredBadges: []string{"ci", "license"},
		ReadmeSections: []string{"usp", "badges", "quick start"},
		VersionedDocs:  []string{"README.md", "docs/index.md"},
		ReadmeMentions: nil, // opt-in via the rules file: a repo lists its own feature families
		// Two JSON files, in order. Deliberately not every manifest a
		// repository might carry: a polyglot tree declares a version in
		// several places and picking one for it would hold the README to
		// a number the project never meant as its own. A repository whose
		// version lives elsewhere names it under `## version source`, and
		// one that does not want the check says `none` there.
		VersionSources: []string{".claude-plugin/plugin.json:version", "package.json:version"},
	}
}

// LoadRules reads the repo's RULES.md. The file is prose for the agent with
// machine-readable list sections; a list that is present replaces the
// default for that section, absent sections keep their defaults.
func LoadRules(root string) Rules {
	r := defaultRules()
	data, err := os.ReadFile(filepath.Join(root, RulesPath))
	if err != nil {
		return r
	}
	section := ""
	var docsL, badgesL, secsL, verL, mentionsL, srcL []string
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
		case "version-tracked docs":
			verL = append(verL, item)
		case "readme must mention":
			mentionsL = append(mentionsL, strings.ToLower(item))
		case "version source":
			srcL = append(srcL, item)
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
	if srcL != nil {
		r.VersionSources = srcL
	}
	if verL != nil {
		r.VersionedDocs = verL
	}
	if mentionsL != nil {
		r.ReadmeMentions = mentionsL
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
	out = append(out, UnclosedSpans(root, file)...)
	return out
}

// RelativeRefs finds links and images whose relative target does not resolve.
// Objectively broken, so blocking.
func RelativeRefs(root, file string) []gitx.Finding {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	// OKF bundles (a directory whose index.md declares okf_version) resolve
	// absolute links from the bundle root, not the repo root — "/log.md"
	// inside .okf/ means .okf/log.md. Empty when the file is in no bundle.
	bundle := okfBundleRoot(root, file)
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
			anchor := ""
			if j := strings.IndexAny(target, "#?"); j >= 0 {
				if target[j] == '#' {
					anchor = target[j+1:]
				}
				target = target[:j]
			}
			if target == "" {
				continue
			}
			resolved := filepath.Join(filepath.Dir(file), filepath.FromSlash(target))
			if strings.HasPrefix(target, "/") {
				base := root
				if bundle != "" {
					base = bundle
				}
				resolved = filepath.Join(base, filepath.FromSlash(target))
			}
			if _, err := os.Stat(resolved); err != nil {
				out = append(out, gitx.Finding{File: file, Line: i + 1, Blocking: true,
					Message: fmt.Sprintf("broken reference: %q does not resolve", m[1])})
				continue
			}
			// A link may name a heading, and a heading that no longer
			// exists drops the reader at the top of the page. mkdocs
			// reports that at INFO, so --strict stays green and it ships.
			if anchor != "" && strings.EqualFold(filepath.Ext(resolved), ".md") {
				ids, ok := anchorIDs(resolved)
				if ok && !ids[strings.ToLower(anchor)] {
					out = append(out, gitx.Finding{File: file, Line: i + 1, Blocking: true,
						Message: fmt.Sprintf("broken reference: %q resolves but no heading in it generates the anchor %q", target, anchor)})
				}
			}
		}
	}
	return out
}

// okfBundleRoot walks from the file's directory up to the repo root and
// returns the nearest directory whose index.md declares okf_version in its
// frontmatter — the OKF bundle root. Empty when no ancestor is one.
func okfBundleRoot(root, file string) string {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	dir, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		return ""
	}
	for {
		if hasOKFVersion(filepath.Join(dir, "index.md")) {
			return dir
		}
		if dir == absRoot {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir || len(parent) < len(absRoot) {
			return ""
		}
		dir = parent
	}
}

// hasOKFVersion reports whether the file opens with a YAML frontmatter block
// that declares okf_version.
func hasOKFVersion(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	for _, l := range lines[1:] {
		t := strings.TrimSpace(l)
		if t == "---" {
			return false
		}
		if strings.HasPrefix(t, "okf_version:") {
			return true
		}
	}
	return false
}

var (
	headingLine = regexp.MustCompile(`(?m)^#{1,6}\s+(.+?)\s*$`)
	explicitID  = regexp.MustCompile(`\{#([^}\s]+)\}`)
	htmlID      = regexp.MustCompile(`(?i)\bid\s*=\s*["']([^"']+)["']`)
)

// anchorIDs collects every anchor a Markdown page offers: the slug each
// heading generates, plus ids written by hand — attr_list's `{#custom}`
// and raw HTML `id="…"`. The second return is whether the file could be
// read at all; a file that could not be read yields no verdict rather
// than a wrong one.
func anchorIDs(path string) (map[string]bool, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	text := string(data)
	ids := map[string]bool{}
	for _, m := range explicitID.FindAllStringSubmatch(text, -1) {
		ids[strings.ToLower(m[1])] = true
	}
	for _, m := range htmlID.FindAllStringSubmatch(text, -1) {
		ids[strings.ToLower(m[1])] = true
	}
	// duplicate headings get -1, -2… appended, the way the toc extension
	// disambiguates them
	seen := map[string]int{}
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		m := headingLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, slug := range headingAnchors(explicitID.ReplaceAllString(m[1], "")) {
			if n := seen[slug]; n > 0 {
				ids[fmt.Sprintf("%s-%d", slug, n)] = true
			} else {
				ids[slug] = true
			}
			seen[slug]++
		}
	}
	return ids, true
}

// headingAnchors returns every anchor one heading offers, because the two
// renderers a repository's Markdown meets disagree. Python-Markdown's toc
// slugify — what mkdocs uses — collapses runs of hyphens; GitHub does not,
// so `## Files & skills` is `files-skills` on a site and `files--skills` on
// github.com. Which renderer the reader used is not knowable from here, and
// the same file is routinely read through both, so a heading is credited
// with both spellings: the check exists to catch an anchor no heading
// generates at all, not to enforce a dialect. Deduplicated, because the two
// agree for most headings and the caller counts these for the `-1`, `-2`
// suffixes duplicate headings get.
func headingAnchors(title string) []string {
	github := headingSlug(title)
	mkdocs := github
	for strings.Contains(mkdocs, "--") {
		mkdocs = strings.ReplaceAll(mkdocs, "--", "-")
	}
	switch github {
	case "":
		return nil
	case mkdocs:
		return []string{github}
	}
	return []string{github, mkdocs}
}

// headingSlug maps a heading's text to GitHub's anchor: lower case, drop
// everything that is not a word character, whitespace, or a hyphen, and turn
// each remaining separator into one hyphen. An em dash disappears rather than
// becoming a separator — it leaves the two hyphens its surrounding spaces
// made, which is precisely the case that is easy to get wrong by hand.
func headingSlug(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(stripInlineMarkup(title)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '\t' || r == '-':
			b.WriteByte('-')
		case r > 127 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r) // \w matches unicode letters and digits too
		}
	}
	return strings.Trim(b.String(), "-")
}

// stripInlineMarkup removes the emphasis, code, and link syntax that a
// heading may carry — the slug is built from the rendered text. Code spans
// are taken out first and their contents kept whole, because that is where
// the two kinds of underscore part company: `reasoning_content` keeps its
// underscore in the anchor, while the ones around _word_ are emphasis that
// renders away and must not reach the slug.
func stripInlineMarkup(s string) string {
	s = mdLink.ReplaceAllString(s, "")
	var b strings.Builder
	for i, part := range strings.Split(s, "`") {
		if i%2 == 1 {
			b.WriteString(part) // inside a code span: the text is the name
			continue
		}
		for _, mark := range []string{"**", "*", "__", "_"} {
			part = strings.ReplaceAll(part, mark, "")
		}
		b.WriteString(part)
	}
	return strings.ReplaceAll(strings.ReplaceAll(b.String(), "[", ""), "]", "")
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
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- resolved from the fixed tool table, never user input
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("mmdc gave no answer in %s — the diagram was NOT checked", mermaidTimeout)
	}
	if err != nil {
		return textutil.FirstLine(buf.String())
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
// a directory walk that skips the places no doc lives — generated site output
// and vendored trees, so regenerating a site never counts as documenting.
// This is the whole documentation corpus, AGENTS.md and every root-level
// Markdown file included: what the non-Claude hosts read is documentation too.
func MarkdownFiles(root string) []string {
	out, _ := markdownFiles(root)
	return out
}

// markdownFiles is MarkdownFiles with the survey's own honesty: when the
// walk itself fails, the corpus is a subset, and a subset that reads as
// the whole repository is the lie the honesty rule bans. Callers that
// produce findings report it; the rest keep the simple form.
func markdownFiles(root string) ([]string, error) {
	if out, ok := gitMarkdownFiles(root); ok {
		return out, nil
	}
	var out []string
	skip := map[string]bool{".git": true, "node_modules": true, "vendor": true,
		"dist": true, ".claude": true, "site": true, "_site": true}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		// one unreadable directory deeper in is skipped, not fatal: a
		// survey that stops at the first bad entry answers less than one
		// that continues. An unreadable ROOT is no survey at all.
		if err != nil {
			if path == root {
				return err
			}
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
	return out, err
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

// VersionSync checks every version-tracked doc's first screen carries the
// version. Drift here shipped three releases in a row unnoticed: prose claims
// aren't file paths, so the drift check never fires on them — this is the
// mechanical tripwire that forces the README to be touched, and therefore
// read, at every release. Objectively wrong, so blocking.
func VersionSync(root string) []gitx.Finding {
	rules := LoadRules(root)
	version := ""
	source := ""
	for _, spec := range rules.VersionSources {
		if strings.EqualFold(strings.TrimSpace(spec), "none") {
			return nil // the repository said its version is not procoder's business
		}
		path, key := spec, "version"
		if i := strings.LastIndex(spec, ":"); i > 0 {
			path, key = spec[:i], spec[i+1:]
		}
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if v, ok := m[key].(string); ok && v != "" {
			version, source = v, path
			break
		}
	}
	if version == "" {
		return nil // no declared version, nothing to hold the README to
	}
	var out []gitx.Finding
	for _, doc := range rules.VersionedDocs {
		path := filepath.Join(root, doc)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // absent docs are RequiredDocs' business, not this check's
		}
		if !strings.Contains(firstN(string(data), firstScreenLines), version) {
			out = append(out, gitx.Finding{File: path, Blocking: true,
				Message: fmt.Sprintf("%s first screen does not carry the current version %s (%s) — a release without a reviewed page is how docs go stale", doc, version, source)})
		}
	}
	// the changelog must COVER the current version, not merely exist —
	// a version bump without a written entry is a silent release
	if data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md")); err == nil {
		if !strings.Contains(string(data), "## "+version) {
			out = append(out, gitx.Finding{File: filepath.Join(root, "CHANGELOG.md"), Blocking: true,
				Message: fmt.Sprintf("CHANGELOG.md has no `## %s` entry — the current version (%s) shipped without release notes", version, source)})
		}
	}
	return out
}

// ReadmeMentions holds the README's NARRATIVE to the repo's declared
// feature families — presence checks pass while the story goes stale, so
// a repo lists what its README must actually talk about (## README must
// mention in the docs rules) and a missing family blocks. This is the
// adaptation for the lesson where eleven releases shipped against a
// README still describing release one.
func ReadmeMentions(root string, r Rules) []gitx.Finding {
	if len(r.ReadmeMentions) == 0 {
		return nil // opt-in: no declared families, nothing to hold the README to
	}
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return nil // a missing README is RequiredDocs' finding
	}
	// only the NARRATIVE counts: badge images and link targets are
	// stripped first (a ci.yml badge URL is not the README telling the
	// reader about CI), then families match as whole words so "spec" is
	// not satisfied by "specific"
	text := readmeImageRe.ReplaceAllString(string(data), " ")
	text = readmeLinkTargetRe.ReplaceAllString(text, "]")
	text = strings.ToLower(text)
	var out []gitx.Finding
	for _, m := range r.ReadmeMentions {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(m) + `\b`)
		if !re.MatchString(text) {
			out = append(out, gitx.Finding{File: filepath.Join(root, "README.md"), Blocking: true,
				Message: fmt.Sprintf("README never mentions %q — a declared feature family the front page does not tell (docs)", m)})
		}
	}
	return out
}

var (
	readmeImageRe      = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	readmeLinkTargetRe = regexp.MustCompile(`\]\([^)]*\)`)
)

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
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- resolved from the fixed tool table, never user input
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
	site := siteURL(root)
	var out []gitx.Finding
	for _, line := range strings.Split(buf.String(), "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "* [") && !strings.Contains(t, "[ERROR]") {
			continue
		}
		// a dead link to OUR OWN site whose page exists locally is not
		// dead — it is pending this change's deploy (a PR adding a page
		// and linking it would otherwise never pass CI)
		if u := extractURL(t); site != "" && strings.HasPrefix(u, site) && localPageExists(root, strings.TrimPrefix(u, site)) {
			out = append(out, gitx.Finding{
				Message: "own-site link not deployed yet (page exists locally, resolves after the docs deploy): " + u})
			continue
		}
		out = append(out, gitx.Finding{Blocking: true, Message: "dead external link: " + t})
	}
	if len(out) == 0 {
		out = append(out, gitx.Finding{File: files[0], Blocking: true,
			Message: "lychee failed without a parseable report — external links were NOT checked: " + textutil.FirstLine(buf.String())})
	}
	return out
}

// siteURL reads the docs site's base URL from mkdocs.yml; empty when the
// repo has no site.
func siteURL(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, "mkdocs.yml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "site_url:"); ok {
			u := strings.TrimSpace(rest)
			if u != "" && !strings.HasSuffix(u, "/") {
				u += "/"
			}
			return u
		}
	}
	return ""
}

var urlInLycheeLine = regexp.MustCompile(`<(https?://[^>]+)>`)

func extractURL(line string) string {
	if m := urlInLycheeLine.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// localPageExists maps a site path to its mkdocs source page.
func localPageExists(root, rel string) bool {
	rel = strings.Trim(rel, "/")
	if i := strings.IndexAny(rel, "#?"); i >= 0 {
		rel = strings.Trim(rel[:i], "/")
	}
	if strings.Contains(rel, "..") {
		return false
	}
	if rel == "" {
		rel = "index"
	}
	for _, candidate := range []string{rel + ".md", rel + "/index.md"} {
		if _, err := os.Stat(filepath.Join(root, "docs", candidate)); err == nil {
			return true
		}
	}
	return false
}

// PagesHealth checks GitHub Pages is enabled and its latest build succeeded.
// Network + gh, so it runs with --external, and only for GitHub repos.
func PagesHealth(root string) []gitx.Finding {
	if tools.Resolve(actions.Gh, root) == "" {
		return []gitx.Finding{{Blocking: true,
			Message: "GitHub Pages NOT checked — gh is not installed; run `procoder init`"}}
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
		return []gitx.Finding{{Blocking: true,
			Message: "GitHub Pages NOT checked: " + textutil.FirstLine(errb.String())}}
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
