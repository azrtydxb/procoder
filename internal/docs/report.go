package docs

import (
	"fmt"
	"io"
	"path/filepath"

	"procoder/internal/gitx"
)

// CollectOfflineFor is CollectOffline with the two things only the caller
// knows: the commit message the acknowledgment line would live in (empty when
// there is none at this moment, reported as unavailable) and whether the
// repository opted the documentation obligation into blocking
// ([docs] policy = "block").
func CollectOfflineFor(root string, changed []string, commitMessage string, block bool) []gitx.Finding {
	var out []gitx.Finding
	for _, f := range changed {
		if IsMarkdownFile(f) {
			out = append(out, CheckFile(root, f)...)
		}
	}
	out = append(out, Drift(root, changed)...)
	out = append(out, MissingAPIDocs(changed)...)
	// Before the obligation, and independent of it: the obligation asks
	// whether a doc CHANGED, and emptying one is a change. Without this,
	// destroying a page satisfies the very check meant to protect it.
	out = append(out, EmptyDocs(root, changed)...)
	out = append(out, VersionSync(root)...)
	out = append(out, Obligation(root, changed, commitMessage, block)...)
	// SurfaceCoverage is deliberately NOT here: it answers "what is
	// undocumented in this repository", which is a standing report, not a
	// verdict on the change in hand. Twenty informational lines on every
	// gate run would train the reader to skim the gate. It lives in
	// `procoder docs`, where the reader asked the question.
	return out
}

// RunFor is Run with the commit message the acknowledgment line would live in
// and the repository's docs policy, for the caller that has read config.
// CollectSweep is the diff-independent half of the offline slice, for a
// whole-tree sweep like `procoder audit`. Drift and the documentation
// obligation both ask a question about a CHANGE — "does a doc mention the
// file you just touched", "did this diff move public surface" — and a
// sweep passes every file, so both answer about everything at once and
// bury the real findings. A survey asks what is true, not what just moved.
func CollectSweep(root string, files []string) []gitx.Finding {
	var out []gitx.Finding
	if _, err := markdownFiles(root); err != nil {
		out = append(out, gitx.Finding{Blocking: true,
			Message: "documentation survey NOT complete — the tree could not be walked: " + err.Error()})
	}
	for _, f := range files {
		if IsMarkdownFile(f) {
			out = append(out, CheckFile(root, f)...)
		}
	}
	out = append(out, MissingAPIDocs(files)...)
	out = append(out, EmptyDocs(root, files)...)
	return append(out, VersionSync(root)...)
}

// RunFor is `procoder docs`: the full documentation report over the whole
// repository, plus the diff-scoped questions the caller has the answers
// for — the commit message an acknowledgment would live in, and whether
// the repository opted the obligation into blocking. external adds the
// link and Pages checks, which are the only ones that touch the network.
func RunFor(root string, changed []string, commitMessage string, external, block bool, stdout io.Writer) int {
	rules := LoadRules(root)
	md := MarkdownFiles(root)

	var findings []gitx.Finding
	for _, f := range md {
		findings = append(findings, CheckFile(root, f)...)
	}
	findings = append(findings, Drift(root, changed)...)
	findings = append(findings, MissingAPIDocs(changed)...)
	findings = append(findings, RequiredDocs(root, rules)...)
	findings = append(findings, VersionSync(root)...)
	findings = append(findings, Badges(root, rules)...)
	findings = append(findings, ReadmeStructure(root, rules)...)
	findings = append(findings, ReadmeMentions(root, rules)...)
	findings = append(findings, Obligation(root, changed, commitMessage, block)...)
	findings = append(findings, SurfaceCoverage(root)...)
	if external {
		findings = append(findings, ExternalLinks(root, md)...)
		findings = append(findings, PagesHealth(root)...)
	}

	blocking := 0
	for _, f := range findings {
		mark := "  info "
		if f.Blocking {
			mark = "  BLOCK"
			blocking++
		}
		loc := ""
		if f.File != "" {
			loc = rel(root, f.File)
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", loc, f.Line)
			}
			loc += "  "
		}
		fmt.Fprintf(stdout, "%s %s%s\n", mark, loc, f.Message)
	}
	scope := "offline checks only — run with --external for links and Pages"
	if external {
		scope = "external links and Pages included"
	}
	fmt.Fprintf(stdout, "procoder docs: %d markdown file(s), %d finding(s) (%d blocking) — %s\n",
		len(md), len(findings), blocking, scope)
	if blocking > 0 {
		return 1
	}
	return 0
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(r)
	}
	return path
}
