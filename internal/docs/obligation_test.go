package docs

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixture builds a repository that is emphatically not procoder: a different
// module name, its own documents, its own index. Every obligation test runs
// against it, which is the point — no check may be gated on identity.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/widget\n\ngo 1.22\n")
	write(t, root, "lib.go", "package widget\n\nfunc Existing() {}\n")
	write(t, root, "internal.go", "package widget\n\nfunc helper() {}\n")
	write(t, root, "docs/guide.md", "# guide\n\nExisting is the entry point.\n")
	write(t, root, ".procoder/index/tags.jsonl",
		tagLine("Existing", "lib.go", "func")+tagLine("helper", "internal.go", "func"))
	return root
}

// messages flattens an obligation into text a test can assert on, marking
// what would fail the gate.
func messages(t *testing.T, root string, changed []string, msg string, block bool) string {
	t.Helper()
	var b strings.Builder
	for _, f := range Obligation(root, changed, msg, block) {
		b.WriteString(f.Message)
		if f.Blocking {
			b.WriteString(" [BLOCK]")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestPublicSurfaceChangeWithNoDocRaisesTheObligationNamingTheSymbol(t *testing.T) {
	root := fixture(t)
	// the exported symbol is renamed: the index knew Existing, the file now
	// defines Renamed
	changed := write(t, root, "lib.go", "package widget\n\nfunc Renamed() {}\n")

	got := Obligation(root, []string{changed}, "", false)
	obligation := ""
	for _, f := range got {
		if strings.HasPrefix(f.Message, "documentation obligation:") {
			obligation = f.Message
		}
	}
	if obligation == "" {
		t.Fatalf("a renamed exported symbol with no doc change must raise the obligation: %+v", got)
	}
	if !strings.Contains(obligation, "Renamed") || !strings.Contains(obligation, "Existing") {
		t.Fatalf("the obligation must name both sides of the rename: %s", obligation)
	}
	if !strings.Contains(obligation, "docs: none") {
		t.Fatalf("the obligation must say how it clears: %s", obligation)
	}
}

func TestBlockPolicyIsTheCallersDecision(t *testing.T) {
	root := fixture(t)
	changed := write(t, root, "lib.go", "package widget\n\nfunc Renamed() {}\n")

	if strings.Contains(messages(t, root, []string{changed}, "", false), "[BLOCK]") {
		t.Fatal("the default policy must leave the gate's verdict unchanged")
	}
	if !strings.Contains(messages(t, root, []string{changed}, "", true), "[BLOCK]") {
		t.Fatal("the block policy must make the obligation block")
	}
}

func TestAnyDocumentationChangeClearsTheObligation(t *testing.T) {
	root := fixture(t)
	code := write(t, root, "lib.go", "package widget\n\nfunc Renamed() {}\n")
	doc := write(t, root, "docs/unrelated.md", "# unrelated\n\nedited.\n")

	if got := Obligation(root, []string{code, doc}, "", false); len(got) != 0 {
		t.Fatalf("a documentation change answers the question, even an unrelated one: %+v", got)
	}
}

func TestBacklogMarkdownIsStateNotDocumentation(t *testing.T) {
	root := fixture(t)
	code := write(t, root, "lib.go", "package widget\n\nfunc Renamed() {}\n")
	story := write(t, root, ".procoder/backlog/stories/one.md", "# story\n")

	msgs := messages(t, root, []string{code, story}, "", false)
	if !strings.Contains(msgs, "documentation obligation:") {
		t.Fatalf("planning is not documenting — a backlog edit must not clear: %s", msgs)
	}
}

func TestDocMentionedFileChangedRaisesTheObligationNamingThePage(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/widget\n\ngo 1.22\n")
	write(t, root, "docs/guide.md", "# guide\n\nThe wiring lives in internal/wire.go.\n")
	changed := write(t, root, "internal/wire.go", "package internal\n\nfunc wire() {}\n")
	write(t, root, ".procoder/index/tags.jsonl", tagLine("wire", "internal/wire.go", "func"))

	msgs := messages(t, root, []string{changed}, "", false)
	if !strings.Contains(msgs, "docs/guide.md names changed file internal/wire.go") {
		t.Fatalf("the obligation must name the page and the file: %s", msgs)
	}
}

func TestInternalChangeRaisesNothing(t *testing.T) {
	root := fixture(t)
	changed := write(t, root, "internal.go", "package widget\n\nfunc helper() { _ = 1 }\n")

	if got := Obligation(root, []string{changed}, "", false); len(got) != 0 {
		t.Fatalf("an internal change touching neither surface nor doc-named files raises nothing: %+v", got)
	}
}

func TestAcknowledgmentClearsOnlyWithAReason(t *testing.T) {
	root := fixture(t)
	changed := write(t, root, "lib.go", "package widget\n\nfunc Renamed() {}\n")

	cleared := Obligation(root, []string{changed}, "refactor the widget\n\n"+AckLine("internal refactor"), false)
	if len(cleared) != 0 {
		t.Fatalf("a reasoned acknowledgment clears the obligation: %+v", cleared)
	}
	empty := messages(t, root, []string{changed}, "refactor\n\ndocs: none — \n", false)
	if !strings.Contains(empty, "documentation obligation:") {
		t.Fatalf("an empty reason must not clear — the reason is the point: %s", empty)
	}
}

func TestAckLineIsTheLineTheParserAccepts(t *testing.T) {
	if got := AckLine("internal refactor"); got != "docs: none — internal refactor" {
		t.Fatalf("unexpected acknowledgment line: %q", got)
	}
	if ackReason(AckLine("internal refactor")) != "internal refactor" {
		t.Fatal("what --ack prints must be what the obligation reads back")
	}
	if ackReason(AckLine("   ")) != "" {
		t.Fatal("a blank reason is no reason")
	}
	if ackReason("docs: none -- ascii dashes are typed too") == "" {
		t.Fatal("an agent typing ASCII dashes must still clear")
	}
}

func TestMissingCommitMessageIsReportedAsUnavailable(t *testing.T) {
	root := fixture(t)
	changed := write(t, root, "lib.go", "package widget\n\nfunc Renamed() {}\n")

	msgs := messages(t, root, []string{changed}, "", false)
	if !strings.Contains(msgs, "acknowledgment path unavailable") {
		t.Fatalf("no commit message must be said, not assumed: %s", msgs)
	}
	if strings.Contains(messages(t, root, []string{changed}, "some message", false), "acknowledgment path unavailable") {
		t.Fatal("a message that exists is not unavailable")
	}
}

func TestNoIndexGivesTheMentionTriggerAndAnExplicitNotComputedLine(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/widget\n\ngo 1.22\n")
	write(t, root, "docs/guide.md", "# guide\n\nThe wiring lives in lib.go.\n")
	changed := write(t, root, "lib.go", "package widget\n\nfunc Renamed() {}\n")

	msgs := messages(t, root, []string{changed}, "", false)
	if !strings.Contains(msgs, "public surface NOT computed") {
		t.Fatalf("no index must read as NOT computed, never as clean: %s", msgs)
	}
	if !strings.Contains(msgs, "docs/guide.md names changed file lib.go") {
		t.Fatalf("the file-mention trigger needs no index: %s", msgs)
	}
}

func TestConfigurationFileChangeIsAPublicSurfaceChange(t *testing.T) {
	root := fixture(t)
	changed := write(t, root, ".procoder/config.toml", "[docs]\npolicy = \"block\"\n")

	msgs := messages(t, root, []string{changed}, "", false)
	if !strings.Contains(msgs, "configuration file .procoder/config.toml changed") {
		t.Fatalf("a configuration key is public surface: %s", msgs)
	}
}

func TestDocumentationOnlyChangeRaisesNothing(t *testing.T) {
	root := fixture(t)
	changed := write(t, root, "docs/guide.md", "# guide\n\nrewritten.\n")

	if got := Obligation(root, []string{changed}, "", false); len(got) != 0 {
		t.Fatalf("a documentation-only change is the answer, not the question: %+v", got)
	}
}

func TestAddedFlagStringRaisesTheObligation(t *testing.T) {
	root := gitFixture(t)
	changed := write(t, root, "cli.go", "package widget\n\nfunc run(a string) {\n\tswitch a {\n\tcase \"build\":\n\tcase \"ship\":\n\t}\n\t_ = \"--verbose\"\n\t_ = \"--dry-run\"\n}\n")

	msgs := messages(t, root, []string{changed}, "", false)
	if !strings.Contains(msgs, `CLI string "--dry-run" added`) {
		t.Fatalf("a new flag is surface a reader must be able to discover: %s", msgs)
	}
	if !strings.Contains(msgs, `CLI string "ship" added`) {
		t.Fatalf("a new subcommand is surface too: %s", msgs)
	}
}

// gitFixture is the fixture with one commit behind it, for the checks that
// compare against what the previous commit carried.
func gitFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed — the previous commit cannot be read")
	}
	root := fixture(t)
	write(t, root, "cli.go", "package widget\n\nfunc run(a string) {\n\tswitch a {\n\tcase \"build\":\n\t}\n\t_ = \"--verbose\"\n}\n")
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git fixture unavailable (%v): %s", err, out)
		}
	}
	run("init", "-q")
	run("-c", "user.email=t@example.com", "-c", "user.name=t", "add", ".")
	run("-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-qm", "base")
	return root
}

func TestUnreadableDocumentDoesNotCountAsADocumentationChange(t *testing.T) {
	root := fixture(t)
	code := write(t, root, "lib.go", "package widget\n\nfunc Renamed() {}\n")
	ghost := filepath.Join(root, "docs", "guide.md")
	// a path git reports as changed that no longer reads back: unknown is
	// never done
	msgs := messages(t, root, []string{code, ghost + ".missing"}, "", false)
	if !strings.Contains(msgs, "documentation obligation:") {
		t.Fatalf("a doc that is not there cannot clear the obligation: %s", msgs)
	}
}

// procoder's own store is state, not documentation: it cannot clear an
// obligation, so it must not raise one either. A bug story that names the
// file it fixes would otherwise demand documentation no edit to that story
// could satisfy — the asymmetry that wedged a real close.
func TestStateMarkdownRaisesNoObligationItCannotClear(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/thing\n\ngo 1.22\n")
	write(t, root, "internal/bench/bench.go", "package bench\n")
	write(t, root, ".procoder/backlog/stories/a-bug.md",
		"# a bug\n\nCause traced to internal/bench/bench.go and fixed.\n")

	changed := []string{filepath.Join(root, "internal/bench/bench.go")}
	got := Obligation(root, changed, "", false)
	for _, f := range got {
		if strings.Contains(f.Message, "names changed file") {
			t.Fatalf("state markdown must not raise a mention obligation: %s", f.Message)
		}
	}
}

// proved by: restoring the swallowed root error in markdownFiles
//
// A survey that cannot walk the tree returns a subset, and a subset that
// reads as the whole repository is the lie the honesty rule bans: it would
// let a sweep report a clean documentation corpus it never saw.
func TestUnwalkableTreeIsReportedNotSilentlyEmpty(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod does not deny the walk here")
	}
	root := t.TempDir()
	write(t, root, "docs/guide.md", "# Guide\n")
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(root, 0o755); err != nil {
			t.Error(err)
		}
	})

	var said bool
	for _, f := range CollectSweep(root, nil) {
		if strings.Contains(f.Message, "NOT complete") {
			said = true
		}
	}
	if !said {
		t.Fatal("an unwalkable tree must be reported, never read as a clean corpus")
	}
}

// gitRun runs one git command in the fixture, failing the test if it cannot.
func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	full := append([]string{"-C", root, "-c", "user.email=t@example.com", "-c", "user.name=t"}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Skipf("git %v unavailable (%v): %s", args, err, out)
	}
}

// Writing the documentation in one commit and the code in the next is
// ordinary practice. The obligation is asked of the branch's work, not of
// whichever slice happens to be uncommitted, so a change that IS documented
// must not demand a `docs: none` acknowledgment.
// proved by: looked only at the passed change set — the second commit on a
// branch whose first commit updated the doc is then blocked, which is the
// bug this fixes.
func TestADocChangedEarlierOnTheBranchAnswersTheObligation(t *testing.T) {
	root := gitFixture(t)
	gitRun(t, root, "checkout", "-qb", "feature")
	write(t, root, "docs/guide.md", "# guide\n\nExisting is the entry point, and Added joins it.\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "docs first")

	// the code half, still uncommitted, exporting something new
	changed := write(t, root, "lib.go", "package widget\n\nfunc Existing() {}\n\nfunc Added() {}\n")
	if msgs := messages(t, root, []string{changed}, "", true); strings.Contains(msgs, "documentation obligation") {
		t.Errorf("the doc changed earlier in this branch's work: %s", msgs)
	}
}

// The branch scope widens where the question was answered; it does not stop
// asking. A branch that touched no document still owes one.
// proved by: cleared the obligation whenever a branch existed at all.
func TestABranchWithNoDocChangeStillOwesTheAnswer(t *testing.T) {
	root := gitFixture(t)
	gitRun(t, root, "checkout", "-qb", "feature")
	changed := write(t, root, "lib.go", "package widget\n\nfunc Existing() {}\n\nfunc Added() {}\n")
	msgs := messages(t, root, []string{changed}, "", true)
	if !strings.Contains(msgs, "documentation obligation") || !strings.Contains(msgs, "[BLOCK]") {
		t.Errorf("an undocumented public-surface change must still block: %s", msgs)
	}
}

// The public surface is compared like with like: the previous revision read
// by the same parser as the current one. The index calls every capitalised
// tag exported — Go's rule and nobody else's — so a JavaScript file with a
// capitalised local constant reported that constant removed on every run, a
// phantom trigger dragging a blocking obligation behind it.
// proved by: compared the file's exports against capitalised index tags —
// the untouched ROOT then reads as an exported symbol removed.
func TestACapitalisedLocalIsNotAnExportedSymbol(t *testing.T) {
	root := gitFixture(t)
	gitRun(t, root, "checkout", "-qb", "feature")
	write(t, root, "plugin.js", "const ROOT = \"/tmp\";\n\nexport const Plugin = async () => ({});\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "add the plugin")

	// the body changes; the exported surface does not
	changed := write(t, root, "plugin.js", "const ROOT = \"/var\";\nconst SELF = 1;\n\nexport const Plugin = async () => ({ a: SELF });\n")
	if msgs := messages(t, root, []string{changed}, "", true); strings.Contains(msgs, "exported symbol") {
		t.Errorf("no export changed here: %s", msgs)
	}

	// and a real export still triggers
	changed = write(t, root, "plugin.js", "const ROOT = \"/var\";\n\nexport const Plugin = async () => ({});\nexport function Extra() {}\n")
	if msgs := messages(t, root, []string{changed}, "", true); !strings.Contains(msgs, "exported symbol Extra added") {
		t.Errorf("a real new export must trigger: %s", msgs)
	}
}
