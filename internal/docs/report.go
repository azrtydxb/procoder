package docs

import (
	"fmt"
	"io"
	"path/filepath"

	"procoder/internal/gitx"
)

// CollectOffline is the docs slice shared by the gate and `procoder git`:
// per-file checks on the changed Markdown files, drift and API-doc reports
// for the changed code. Offline by construction.
func CollectOffline(root string, changed []string) []gitx.Finding {
	var out []gitx.Finding
	for _, f := range changed {
		if IsMarkdownFile(f) {
			out = append(out, CheckFile(root, f)...)
		}
	}
	out = append(out, Drift(root, changed)...)
	out = append(out, MissingAPIDocs(changed)...)
	return out
}

// Run is `procoder docs [--external]`: the full documentation report. The
// agent reads it, fixes what is real, and explains what is not.
func Run(root string, changed []string, external bool, stdout io.Writer) int {
	rules := LoadRules(root)
	md := MarkdownFiles(root)

	var findings []gitx.Finding
	for _, f := range md {
		findings = append(findings, CheckFile(root, f)...)
	}
	findings = append(findings, Drift(root, changed)...)
	findings = append(findings, MissingAPIDocs(changed)...)
	findings = append(findings, RequiredDocs(root, rules)...)
	findings = append(findings, Badges(root, rules)...)
	findings = append(findings, ReadmeStructure(root, rules)...)
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
