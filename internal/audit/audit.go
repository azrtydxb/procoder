// Package audit is the onboarding sweep: every domain's checks over the
// WHOLE tracked tree of a repository that was not built under procoder,
// aggregated into one scorecard the agent can triage from. Nothing is
// modified — the audit computes, the agent brings the codebase in line
// (P-CONTROL).
package audit

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"procoder/internal/config"
	"procoder/internal/format"
	"procoder/internal/gitcmd"
	"procoder/internal/gitx"
	"procoder/internal/lint"
	"procoder/internal/security"
)

// section caps keep the report readable; totals stay honest.
const maxPerSection = 15

// Run executes the audit and prints the scorecard. Exit 1 when anything
// blocking exists — an audited repo is not "in line" until it would pass
// the gate.
func Run(root string, out func(string)) int {
	files := scopedFiles(root)
	if files == nil {
		out("procoder audit: not a git repository — nothing to audit")
		return 2
	}
	out(fmt.Sprintf("procoder audit over %d file(s) in scope (tracked plus untracked, gitignored excluded)", len(files)))
	out("")

	totalBlocking := 0
	report := func(name string, findings []gitx.Finding) {
		blocking := 0
		for _, f := range findings {
			if f.Blocking {
				blocking++
			}
		}
		totalBlocking += blocking
		out(fmt.Sprintf("## %s — %d finding(s), %d blocking", name, len(findings), blocking))
		shown := orderForDisplay(findings)
		if len(shown) > maxPerSection {
			shown = shown[:maxPerSection]
		}
		for _, f := range shown {
			mark := "info "
			if f.Blocking {
				mark = "BLOCK"
			}
			loc := f.File
			if rel, err := filepath.Rel(root, loc); err == nil && !strings.HasPrefix(rel, "..") {
				loc = rel
			}
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", loc, f.Line)
			}
			if loc != "" {
				loc += "  "
			}
			out(fmt.Sprintf("  %s  %s%s", mark, loc, f.Message))
		}
		if len(findings) > maxPerSection {
			out(fmt.Sprintf("  … %d more", len(findings)-maxPerSection))
		}
		out("")
	}

	// formatting: verdict counts over the whole tree
	clean, unformatted, unchecked, skipped := 0, 0, 0, 0
	var fmtFindings []gitx.Finding
	for _, f := range files {
		switch res := format.Check(f); res.Verdict {
		case format.Clean:
			clean++
		case format.Unformatted:
			unformatted++
			fmtFindings = append(fmtFindings, gitx.Finding{File: f, Blocking: true,
				Message: "unformatted — `procoder format` has the result"})
		case format.Unchecked:
			unchecked++
			fmtFindings = append(fmtFindings, gitx.Finding{File: f, Blocking: true,
				Message: "NOT checked — " + res.Reason})
		case format.OutOfScope:
			skipped++
		}
	}
	out(fmt.Sprintf("## formatting — %d clean, %d unformatted, %d unchecked, %d out of scope",
		clean, unformatted, unchecked, skipped))
	shown := fmtFindings
	if len(shown) > maxPerSection {
		shown = shown[:maxPerSection]
	}
	for _, f := range shown {
		rel := f.File
		if r, err := filepath.Rel(root, rel); err == nil {
			rel = r
		}
		out("  BLOCK  " + rel + "  " + f.Message)
	}
	if len(fmtFindings) > maxPerSection {
		out(fmt.Sprintf("  … %d more", len(fmtFindings)-maxPerSection))
	}
	totalBlocking += len(fmtFindings)
	out("")

	cfg := config.Load(root)
	// Collect is the gate's shared slice: git hygiene, templates, workflow
	// lint, CI hygiene, infrastructure, and documentation in one pass
	report("hygiene (git, workflows, CI, infra, docs)", gitcmd.Collect(root, cfg, files))
	// the file set, not the directory: dir mode would read gitignored
	// trees (node_modules/, target/, local caches) and stamp fingerprints
	// with absolute paths — see SecretsTree
	report("secrets (gitleaks)", security.SecretsTree(root, files))
	report("lint", lint.Files(root, files, cfg.LintBlock))

	out("## scorecard")
	out(fmt.Sprintf("  blocking findings total: %d", totalBlocking))
	if totalBlocking > 0 {
		out("  this repository would NOT pass the procoder gate — triage order:")
		out("  1. secrets (rotate + remove)  2. other blocking  3. judged info findings")
		out("  run `procoder init` first if any section says NOT checked")
		return 1
	}
	out("  this repository would pass the procoder gate")
	return 0
}

// orderForDisplay partitions blocking findings ahead of info ones (stable
// within each group) so the section cap can never hide a BLOCK behind
// info lines.
func orderForDisplay(findings []gitx.Finding) []gitx.Finding {
	ordered := make([]gitx.Finding, 0, len(findings))
	for _, f := range findings {
		if f.Blocking {
			ordered = append(ordered, f)
		}
	}
	for _, f := range findings {
		if !f.Blocking {
			ordered = append(ordered, f)
		}
	}
	return ordered
}

// scopedFiles lists what the gate itself would consider: tracked plus
// untracked-but-not-ignored.
func scopedFiles(root string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "git", "-C", root,
		"ls-files", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line != "" {
			files = append(files, filepath.Join(root, line))
		}
	}
	return files
}
