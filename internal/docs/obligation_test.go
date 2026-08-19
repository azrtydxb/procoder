package docs

import (
	"os/exec"
	"path/filepath"
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
