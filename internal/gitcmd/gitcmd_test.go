package gitcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// The workflow rules are a template like the other two: printed when missing
// so the agent writes them, silent once the repo has its own copy — the
// repo's file is what the skills obey, so it must be creatable and editable.
func TestWorkflowRulesArePrintedWhenMissingAndRespectedWhenPresent(t *testing.T) {
	root := t.TempDir()

	var out bytes.Buffer
	Templates(root, &out)
	if !strings.Contains(out.String(), workflowPath) || !strings.Contains(out.String(), "## Worktrees") {
		t.Fatalf("missing workflow template not offered:\n%s", out.String())
	}

	dir := filepath.Join(root, ".procoder", "github")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ name, body string }{
		{"WORKFLOW.md", "my own rules\n"},
		{"PULL_REQUEST_TEMPLATE.md", PRTemplate},
		{"COMMIT_TEMPLATE.md", CommitTemplate},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	docsDir := filepath.Join(root, ".procoder", "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"RULES.md", "mermaid.json"} {
		if err := os.WriteFile(filepath.Join(docsDir, name), []byte("mine\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out.Reset()
	Templates(root, &out)
	if strings.Contains(out.String(), "## Worktrees") {
		t.Fatalf("existing WORKFLOW.md must never be overwritten with the default:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "all templates exist") {
		t.Fatalf("want 'all templates exist', got:\n%s", out.String())
	}
}

// A git worktree marks its root with a .git FILE, not a directory. Since the
// workflow rules make worktrees the default place work happens, the harness
// must resolve the worktree as the root — not climb past it to the parent
// checkout and report against the wrong tree.
func TestWorktreeRootIsTheWorktreeNotTheParentCheckout(t *testing.T) {
	parent := t.TempDir()
	if err := os.MkdirAll(filepath.Join(parent, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(parent, ".claude", "worktrees", "x")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tools.RepoRoot(wt); got != wt {
		t.Fatalf("RepoRoot(%s) = %s; the worktree itself is the root", wt, got)
	}
}
