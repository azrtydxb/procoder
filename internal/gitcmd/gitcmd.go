// Package gitcmd is `procoder git` — the one status a senior developer reads
// before calling work finished — and `procoder templates`, which prints the
// default template contents for the agent to write (P-CONTROL: the binary
// creates no files).
package gitcmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"procoder/internal/actions"
	"procoder/internal/ciops"
	"procoder/internal/config"
	"procoder/internal/docs"
	"procoder/internal/gitx"
	"procoder/internal/infra"
	"procoder/internal/security"
)

// Paths under D-HOME.
const (
	prTemplatePath     = ".procoder/github/PULL_REQUEST_TEMPLATE.md"
	commitTemplatePath = ".procoder/github/COMMIT_TEMPLATE.md"
	workflowPath       = ".procoder/github/WORKFLOW.md"
)

// Status prints the full picture and returns an exit code: non-zero when
// anything blocking is present, so the skill can gate on it.
func Status(root string, stdout io.Writer) int {
	cfg := config.Load(root)

	branch := gitx.CurrentBranch(root)
	def := gitx.DefaultBranch(root)
	fmt.Fprintf(stdout, "branch          %s (default: %s)\n", orDetached(branch), orUnknown(def))

	changed, err := gitx.ChangedFiles(root)
	if err != nil {
		fmt.Fprintf(stdout, "changed files   cannot list (%v)\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "changed files   %d\n", len(changed))

	fmt.Fprintf(stdout, "pr template     %s\n", presence(filepath.Join(root, prTemplatePath)))
	fmt.Fprintf(stdout, "commit template %s, %s\n",
		presence(filepath.Join(root, commitTemplatePath)), registered(root))
	fmt.Fprintf(stdout, "workflow rules  %s\n", presence(filepath.Join(root, workflowPath)))

	findings := Collect(root, cfg, changed)
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
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "hygiene         clean")
	}
	if blocking > 0 {
		fmt.Fprintf(stdout, "\n%d blocking finding(s)\n", blocking)
		return 1
	}
	return 0
}

// Collect runs every domain-9 check over the changed set. Shared by Status and
// the commit gate so the two can never disagree about what the rules are.
func Collect(root string, cfg config.Config, changed []string) []gitx.Finding {
	var out []gitx.Finding
	out = append(out, gitx.ConflictMarkers(changed)...)
	out = append(out, gitx.JunkFiles(changed)...)
	out = append(out, gitx.Oversized(changed, cfg.MaxFileMB)...)
	out = append(out, gitx.OnDefaultBranch(root, cfg.BlockDefaultBranch)...)
	out = append(out, gitx.IgnoreCoverage(root)...)

	msgs := gitx.UnpushedMessages(root)
	out = append(out, gitx.Attribution(msgs)...)
	out = append(out, gitx.SubjectShape(msgs)...)

	var workflows []string
	for _, f := range changed {
		if actions.IsWorkflowFile(f) {
			workflows = append(workflows, f)
		}
	}
	out = append(out, actions.Lint(workflows)...)

	// Domain 7: workflow hygiene — pinned actions, timeouts, concurrency,
	// the existence of tests. Scans ALL workflows, not just the changed set:
	// pipeline discipline is a property of the repo, so findings can appear
	// even when no workflow changed.
	out = append(out, ciops.Check(root, cfg.PinActions)...)

	// Domain 8: infrastructure hygiene, only when infra files exist
	out = append(out, infra.Check(root)...)

	// Domain 5: the offline docs slice rides the same gate so `git`, `check`,
	// and CI can never disagree about documentation health either.
	out = append(out, docs.CollectOffline(root, changed)...)

	// Missing templates are information, not a block: the fix is one
	// `procoder templates` away and the finding says so.
	if _, err := os.Stat(filepath.Join(root, prTemplatePath)); err != nil {
		out = append(out, gitx.Finding{Message: prTemplatePath + " is missing — run `procoder templates` and write it"})
	}
	if _, err := os.Stat(filepath.Join(root, commitTemplatePath)); err != nil {
		out = append(out, gitx.Finding{Message: commitTemplatePath + " is missing — run `procoder templates` and write it"})
	}
	if _, err := os.Stat(filepath.Join(root, workflowPath)); err != nil {
		out = append(out, gitx.Finding{Message: workflowPath + " is missing — run `procoder templates` and write it"})
	}
	out = append(out, mirrorSync(root)...)
	return out
}

// GitHub only reads the PR template at its own hardcoded path, so the repo
// carries a mirror of the .procoder master there. mirrorSync enforces it:
// drift means someone edited one copy silently, and that BLOCKS — the master
// under .procoder/github/ is the one to edit, then copy over the mirror.
const githubPRTemplatePath = ".github/PULL_REQUEST_TEMPLATE.md"

func mirrorSync(root string) []gitx.Finding {
	master, err := os.ReadFile(filepath.Join(root, prTemplatePath))
	if err != nil {
		return nil // absence of the master is already its own finding
	}
	mirror, err := os.ReadFile(filepath.Join(root, githubPRTemplatePath))
	if err != nil {
		return []gitx.Finding{{Message: githubPRTemplatePath + " is missing — GitHub only auto-fills PRs from that path; copy the .procoder/github/ master there"}}
	}
	if !bytes.Equal(master, mirror) {
		return []gitx.Finding{{File: filepath.Join(root, githubPRTemplatePath), Blocking: true,
			Message: "out of sync with " + prTemplatePath + " — edit the .procoder/github/ master, then copy it over this mirror"}}
	}
	return nil
}

// Templates prints the default content for each template that is missing, so
// the agent can review and write it, then register the commit template.
func Templates(root string, stdout io.Writer) int {
	printed := false
	if _, err := os.Stat(filepath.Join(root, prTemplatePath)); err != nil {
		fmt.Fprintf(stdout, "== write this to %s:\n%s\n", prTemplatePath, PRTemplate)
		printed = true
	}
	if _, err := os.Stat(filepath.Join(root, commitTemplatePath)); err != nil {
		fmt.Fprintf(stdout, "== write this to %s:\n%s\n", commitTemplatePath, CommitTemplate)
		printed = true
	}
	if _, err := os.Stat(filepath.Join(root, workflowPath)); err != nil {
		fmt.Fprintf(stdout, "== write this to %s:\n%s\n", workflowPath, WorkflowTemplate)
		printed = true
	}
	if _, err := os.Stat(filepath.Join(root, docs.RulesPath)); err != nil {
		fmt.Fprintf(stdout, "== write this to %s:\n%s\n", docs.RulesPath, docs.DefaultRules)
		printed = true
	}
	if _, err := os.Stat(filepath.Join(root, docs.MermaidConfigPath)); err != nil {
		fmt.Fprintf(stdout, "== write this to %s:\n%s\n", docs.MermaidConfigPath, docs.DefaultMermaidConfig)
		printed = true
	}
	if _, err := os.Stat(filepath.Join(root, security.RulesPath)); err != nil {
		fmt.Fprintf(stdout, "== write this to %s:\n%s\n", security.RulesPath, security.DefaultRules)
		printed = true
	}
	if !printed {
		fmt.Fprintln(stdout, "all templates exist")
	}
	fmt.Fprintf(stdout, "== then register the commit template (once per clone):\ngit config commit.template %s\n", commitTemplatePath)
	return 0
}

// Scrub checks text (a drafted PR body, a commit message) for attribution
// lines. Reads the named file, or stdin for "-".
func Scrub(path string, stdin io.Reader, stdout io.Writer) int {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		fmt.Fprintf(stdout, "cannot read %s: %v\n", path, err)
		return 2
	}
	findings := gitx.ScrubText(string(data))
	for _, f := range findings {
		fmt.Fprintln(stdout, f.Message)
	}
	if len(findings) > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "clean — no attribution lines")
	return 0
}

func registered(root string) string {
	out, err := exec.Command("git", "-C", root, "config", "commit.template").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "not registered (git config commit.template)"
	}
	return "registered: " + strings.TrimSpace(string(out))
}

func presence(path string) string {
	if _, err := os.Stat(path); err != nil {
		return "missing"
	}
	return "present"
}

func rel(root, f string) string {
	if r, err := filepath.Rel(root, f); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return f
}

func orDetached(s string) string {
	if s == "" {
		return "(detached)"
	}
	return s
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
