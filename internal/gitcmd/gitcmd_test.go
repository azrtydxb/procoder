package gitcmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/actions"
	"procoder/internal/config"
	"procoder/internal/gitx"
	"procoder/internal/tools"
)

// gitRepo is a fixture repository: real git, so the hygiene checks answer
// from the same place they do in anger. No git, no verdict — skip rather
// than pretend.
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed — the hygiene checks have nothing to read")
	}
	root := t.TempDir()
	run(t, root, "init", "-q")
	return root
}

func run(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// commit writes the whole tree with the fixture identity, so the message
// checks have something unpushed to read.
func commit(t *testing.T, root, message string) {
	t.Helper()
	run(t, root, "add", "-A")
	run(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", message)
}

func writeAt(t *testing.T, root, name, body string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// find returns the first finding whose message carries substr.
func find(findings []gitx.Finding, substr string) *gitx.Finding {
	for i, f := range findings {
		if strings.Contains(f.Message, substr) {
			return &findings[i]
		}
	}
	return nil
}

func messages(findings []gitx.Finding) string {
	var b strings.Builder
	for _, f := range findings {
		b.WriteString(f.File + " | " + f.Message + "\n")
	}
	return b.String()
}

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
		{"REVIEW.md", "my rubric\n"},
		{"LESSONS.md", "my ledger\n"},
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
	secDir := filepath.Join(root, ".procoder", "security")
	if err := os.MkdirAll(secDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secDir, "RULES.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
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

// GitHub reads the PR template only from .github/; the repo's master lives
// under .procoder/github/. A drifted mirror means someone edited the wrong
// copy silently — that blocks; a matching mirror is silent; a missing mirror
// is reported with the fix.
func TestPRTemplateMirrorMustMatchTheMaster(t *testing.T) {
	root := t.TempDir()
	master := filepath.Join(root, ".procoder", "github", "PULL_REQUEST_TEMPLATE.md")
	mirror := filepath.Join(root, ".github", "PULL_REQUEST_TEMPLATE.md")
	os.MkdirAll(filepath.Dir(master), 0o755)
	os.MkdirAll(filepath.Dir(mirror), 0o755)
	os.WriteFile(master, []byte("## What\n"), 0o644)

	got := mirrorSync(root)
	if len(got) != 1 || got[0].Blocking || !strings.Contains(got[0].Message, "missing") {
		t.Fatalf("missing mirror is reported, not blocked: %+v", got)
	}

	os.WriteFile(mirror, []byte("## Something else\n"), 0o644)
	got = mirrorSync(root)
	if len(got) != 1 || !got[0].Blocking {
		t.Fatalf("drifted mirror must block: %+v", got)
	}

	os.WriteFile(mirror, []byte("## What\n"), 0o644)
	if got = mirrorSync(root); len(got) != 0 {
		t.Fatalf("matching mirror is silent: %+v", got)
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

// Every hygiene class the gate owns must actually reach the caller: a
// conflict marker, staged junk, an oversized blob, and an ecosystem whose
// garbage nothing ignores. One dropped line in collectHygiene and a repo
// goes out the door with a conflict marker in it.
// proved by: deleting the gitx.JunkFiles line from collectHygiene — the
// .DS_Store went unreported and the test failed.
func TestCollectHygieneReportsEveryClassOverTheChangedSet(t *testing.T) {
	root := gitRepo(t)
	conflict := writeAt(t, root, "merged.txt", "one\n<<<<<<< HEAD\ntwo\n>>>>>>> theirs\n")
	junk := writeAt(t, root, ".DS_Store", "finder droppings")
	big := writeAt(t, root, "blob.bin", strings.Repeat("x", 2<<20))
	mod := writeAt(t, root, "go.mod", "module example.com/thing\n\ngo 1.22\n")

	got := collectHygiene(root, config.Config{MaxFileMB: 1}, []string{conflict, junk, big, mod})

	marker := find(got, "merge conflict marker")
	if marker == nil || !marker.Blocking || marker.File != conflict || marker.Line != 2 {
		t.Fatalf("conflict marker must block at merged.txt:2:\n%s", messages(got))
	}
	if f := find(got, "junk file staged"); f == nil || !f.Blocking || f.File != junk {
		t.Fatalf("staged junk must block:\n%s", messages(got))
	}
	if f := find(got, "over the 1 MB limit"); f == nil || !f.Blocking || f.File != big {
		t.Fatalf("the 2 MB blob must block against a 1 MB limit:\n%s", messages(got))
	}
	if f := find(got, ".gitignore is missing"); f == nil || f.Blocking {
		t.Fatalf("a go.mod with no .gitignore is reported, never blocked:\n%s", messages(got))
	}
}

// The oversized limit is the repository's to set: the same blob is fine
// under the default and a block under a 1 MB ceiling. A hardcoded threshold
// would make .procoder/config.toml decorative.
// proved by: passing a literal 5 instead of cfg.MaxFileMB to gitx.Oversized
// — the 1 MB ceiling stopped being honoured and the test failed.
func TestOversizedHonoursTheConfiguredCeiling(t *testing.T) {
	root := gitRepo(t)
	big := writeAt(t, root, "blob.bin", strings.Repeat("x", 2<<20))

	if got := collectHygiene(root, config.Config{MaxFileMB: 5}, []string{big}); find(got, "MB limit") != nil {
		t.Fatalf("2 MB is under a 5 MB ceiling and must pass:\n%s", messages(got))
	}
	got := collectHygiene(root, config.Config{MaxFileMB: 1}, []string{big})
	if f := find(got, "over the 1 MB limit"); f == nil || !f.Blocking {
		t.Fatalf("2 MB is over a 1 MB ceiling and must block:\n%s", messages(got))
	}
}

// Working on the default branch is reported by default and blocks only
// where the repository asked for it — the config decides, not the harness.
// proved by: passing a literal false for cfg.BlockDefaultBranch — the
// opted-in repository stopped blocking and the test failed.
func TestDefaultBranchPolicyFollowsTheConfig(t *testing.T) {
	root := gitRepo(t)
	writeAt(t, root, "notes.txt", "hello\n")
	commit(t, root, "first")

	reported := collectHygiene(root, config.Config{MaxFileMB: 5}, nil)
	f := find(reported, "working directly on the default branch")
	if f == nil || f.Blocking {
		t.Fatalf("the default policy reports, it does not block:\n%s", messages(reported))
	}
	blocked := collectHygiene(root, config.Config{MaxFileMB: 5, BlockDefaultBranch: true}, nil)
	if f := find(blocked, "working directly on the default branch"); f == nil || !f.Blocking {
		t.Fatalf("default_branch_policy = block must block:\n%s", messages(blocked))
	}
}

// The unpushed commits are the ones whose wording the gate can still save:
// an AI-attribution line blocks, a bloated subject and a missing blank line
// are reported. All three come from the same message, so a gate that reads
// none of them looks exactly as clean as one that reads all three.
// proved by: deleting the gitx.Attribution(msgs) line from collectHygiene —
// the Co-Authored-By trailer went unreported and the test failed.
func TestUnpushedCommitMessagesAreJudged(t *testing.T) {
	root := gitRepo(t)
	writeAt(t, root, "notes.txt", "hello\n")
	subject := "feat: " + strings.Repeat("a", 74) // 80 chars, over the 72 aim
	commit(t, root, subject+"\nbody starts with no blank line\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n")

	got := collectHygiene(root, config.Config{MaxFileMB: 5}, nil)

	if f := find(got, "AI-attribution line"); f == nil || !f.Blocking {
		t.Fatalf("an attribution trailer must block:\n%s", messages(got))
	}
	if f := find(got, "commit subject is 80 chars"); f == nil || f.Blocking {
		t.Fatalf("an 80-char subject is reported, never blocked:\n%s", messages(got))
	}
	if f := find(got, "no blank line between subject and body"); f == nil || f.Blocking {
		t.Fatalf("a body glued to the subject is reported:\n%s", messages(got))
	}
}

// actionlint is given workflow files and nothing else. A YAML file that
// merely lives elsewhere is not a workflow, and handing it over would mean
// a stream of nonsense findings about ordinary config.
// proved by: passing `changed` straight to actions.Lint instead of the
// filtered `workflows` slice — config/other.yml drew a finding and the test
// failed.
func TestWorkflowLintOnlySeesWorkflowFiles(t *testing.T) {
	root := gitRepo(t)
	other := writeAt(t, root, "config/other.yml", "database:\n  host: localhost\n")
	flow := writeAt(t, root, ".github/workflows/ci.yml",
		"name: ci\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go test ./...\n")

	got := collectHygiene(root, config.Config{MaxFileMB: 5}, []string{other, flow})

	// actionlint reports paths relative to the working directory, so the
	// file is recognised by its tail, not by the string we handed over.
	for _, f := range got {
		if strings.HasSuffix(filepath.ToSlash(f.File), "config/other.yml") {
			t.Fatalf("an ordinary YAML file is not a workflow: %s", f.Message)
		}
	}
	// and the workflow half is genuinely live: without actionlint installed
	// the file is honestly reported as NOT checked, with it the file is
	// linted — either way the workflow, and only the workflow, was looked at.
	if tools.Resolve(actions.Actionlint, "") == "" {
		if f := find(got, "NOT checked — actionlint is not installed"); f == nil || f.File != flow {
			t.Fatalf("a missing actionlint is reported against the workflow, never silent:\n%s", messages(got))
		}
	}
}

// Each template the repo has not written yet is named, with the command
// that prints it. Information, never a block: a repo adopting procoder
// must not be stopped at the gate for files it has not met.
// proved by: deleting the workflowPath stanza from templateFindings — the
// third template went unreported and the test failed.
func TestMissingTemplatesAreNamedAndNeverBlock(t *testing.T) {
	root := t.TempDir()

	got := templateFindings(root)

	if len(got) != 3 {
		t.Fatalf("a bare repo is missing exactly the three templates, got %d:\n%s", len(got), messages(got))
	}
	for _, path := range []string{prTemplatePath, commitTemplatePath, workflowPath} {
		f := find(got, path)
		if f == nil {
			t.Fatalf("%s is not reported missing:\n%s", path, messages(got))
		}
		if f.Blocking {
			t.Fatalf("%s must be information, not a block", path)
		}
		if !strings.Contains(f.Message, "procoder templates") {
			t.Fatalf("the finding must name the fix: %s", f.Message)
		}
	}
}

// The line that shipped yesterday: a whole-tree sweep passes every file, so
// the diff-scoped documentation questions — "a doc mentions the file you
// changed", "this diff moved public surface" — would answer about the whole
// repository at once. CollectTree must not ask them; CollectFor, which has a
// real diff, must. Both keep the shared hygiene half.
// proved by: making CollectTree call docs.CollectOfflineFor (the diff path)
// instead of docs.CollectSweep — the sweep started reporting drift about
// every file and the test failed.
func TestCollectTreeDropsTheDiffScopedDocsQuestionsAndCollectForKeepsThem(t *testing.T) {
	root := gitRepo(t)
	writeAt(t, root, "go.mod", "module example.com/thing\n\ngo 1.22\n")
	writeAt(t, root, "internal/service/handler.go", "package service\n\n// Serve serves.\nfunc Serve() {}\n")
	writeAt(t, root, "docs/guide.md", "# Guide\n\nSee internal/service/handler.go for the entry point.\n")
	files := []string{
		filepath.Join(root, "go.mod"),
		filepath.Join(root, "internal/service/handler.go"),
		filepath.Join(root, "docs/guide.md"),
	}
	cfg := config.Config{MaxFileMB: 5}

	diff := CollectFor(root, cfg, files, "")
	if f := find(diff, "mentions changed file internal/service/handler.go"); f == nil {
		t.Fatalf("the diff path must still ask about drift:\n%s", messages(diff))
	}

	sweep := CollectTree(root, cfg, files)
	for _, f := range sweep {
		if strings.Contains(f.Message, "mentions changed file") ||
			strings.Contains(f.Message, "documentation obligation") {
			t.Fatalf("a sweep must not ask a diff-scoped question: %s", f.Message)
		}
	}
	// the sweep keeps everything that does not depend on a diff
	if find(sweep, ".gitignore is missing") == nil || find(sweep, prTemplatePath) == nil {
		t.Fatalf("the sweep still owes the hygiene and template halves:\n%s", messages(sweep))
	}
	if find(diff, ".gitignore is missing") == nil || find(diff, prTemplatePath) == nil {
		t.Fatalf("the diff path still owes the hygiene and template halves:\n%s", messages(diff))
	}
}

// The status a developer reads before calling work finished: the counts,
// the template state, and an exit code that is 1 exactly when something
// blocks. A status that printed BLOCK and exited 0 would gate nothing.
// proved by: returning 0 instead of 1 from the blocking branch of Status —
// the conflict-marker repo reported success and the test failed.
func TestStatusPrintsTheCountsAndExitsOnBlockingFindings(t *testing.T) {
	root := gitRepo(t)
	writeAt(t, root, "merged.txt", "one\n<<<<<<< HEAD\ntwo\n")

	var out bytes.Buffer
	code := Status(root, &out)
	got := out.String()

	if code != 1 {
		t.Fatalf("a conflict marker blocks: exit %d\n%s", code, got)
	}
	for _, want := range []string{
		"changed files   1",
		"pr template     missing",
		"commit template missing, not registered",
		"workflow rules  missing",
		"  BLOCK merged.txt:2  merge conflict marker left in the file",
		"1 blocking finding(s)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status is missing %q:\n%s", want, got)
		}
	}

	// nothing blocking: the same repo once the marker is gone and everything
	// is committed — findings remain (the missing templates), and none of
	// them stops the developer.
	writeAt(t, root, "merged.txt", "one\ntwo\n")
	commit(t, root, "resolve the conflict")
	out.Reset()
	if code := Status(root, &out); code != 0 {
		t.Fatalf("only blocking findings gate: exit %d\n%s", code, out.String())
	}
	if got := out.String(); !strings.Contains(got, "changed files   0") || strings.Contains(got, "BLOCK") {
		t.Fatalf("a clean tree has no blocks:\n%s", got)
	}
}

// Raised in review on #187: the commit message being written was never
// checked for an attribution trailer. The unpushed range catches one that
// already landed; nothing caught one on the way in.
//
// The two halves matter differently. Before the commit, the line can be
// removed by editing the message. After it, only by rewriting history —
// and until that happens it blocks every subsequent commit, which is what
// made #185 as bad as it was.
//
// proved by: the gitx.AttributionInMessage call removed from
// CollectUniversal — the trailer passes on the way in.
func TestTheCommitMessageBeingWrittenIsCheckedForAttribution(t *testing.T) {
	root := t.TempDir()
	msg := "feat: a thing\n\nCo-Authored-By: Claude <noreply@anthropic.com>"

	for _, tc := range []struct {
		name string
		got  []gitx.Finding
	}{
		{"universal", CollectUniversal(root, config.Config{}, nil, msg)},
		{"adopted", CollectFor(root, config.Config{}, nil, msg)},
	} {
		found := false
		for _, f := range tc.got {
			if strings.Contains(f.Message, "AI-attribution") && f.Blocking {
				found = true
				// The fix must be the one that applies at this moment.
				if !strings.Contains(f.Message, "before committing") {
					t.Errorf("%s: the finding tells you to rewrite history for a commit that does not exist yet: %s", tc.name, f.Message)
				}
			}
		}
		if !found {
			t.Errorf("%s: a trailer in the message being written was not caught: %+v", tc.name, tc.got)
		}
	}
}

// And a clean message is silent, so the check above cannot pass by
// flagging everything.
//
// The empty-message guard in AttributionInMessage is an early return, not
// the protection — matchAIIdentity finds nothing in an empty string
// anyway. Verified: removing it does not fail this test.
//
// proved by: AttributionInMessage made to return a finding
// unconditionally — every message is flagged and the test names it.
func TestACleanCommitMessageIsSilent(t *testing.T) {
	root := t.TempDir()
	for _, msg := range []string{"", "feat: an ordinary message\n\nWith a body."} {
		for _, f := range CollectUniversal(root, config.Config{}, nil, msg) {
			if strings.Contains(f.Message, "AI-attribution") {
				t.Errorf("a clean message was flagged (%q): %s", msg, f.Message)
			}
		}
	}
}
