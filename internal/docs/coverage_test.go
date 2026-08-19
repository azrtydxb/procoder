package docs

import (
	"strings"
	"testing"
)

func TestCommandCoverageSkipsRepositoriesThatAreNotProcoder(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/some/library\n\ngo 1.22\n")
	write(t, root, "README.md", "# lib\n")
	write(t, root, "docs/index.md", "docs site with no procoder mentions\n")

	if got := CommandCoverage(root); len(got) != 0 {
		t.Fatalf("a repo that does not ship the commands must not be gated on documenting them, got %+v", got)
	}
}

func TestCommandCoverageReportsMissingCommandsInTheProcoderRepo(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module procoder\n\ngo 1.22\n")
	var mentions strings.Builder
	for _, cmd := range Commands {
		if cmd == "audit" {
			continue // the one we leave undocumented
		}
		mentions.WriteString("`procoder " + cmd + "`\n")
	}
	write(t, root, "docs/reference.md", mentions.String())
	write(t, root, "README.md", "# procoder\n")

	got := CommandCoverage(root)
	if len(got) != 1 {
		t.Fatalf("want exactly the one undocumented command, got %d: %+v", len(got), got)
	}
	if !got[0].Blocking || !strings.Contains(got[0].Message, "procoder audit") {
		t.Fatalf("finding must block and name the command: %+v", got[0])
	}
}

func TestCommandCoverageStillNeedsADocsSite(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module procoder\n\ngo 1.22\n")
	write(t, root, "README.md", "# procoder\n")

	if got := CommandCoverage(root); len(got) != 0 {
		t.Fatalf("no docs/ directory means no comprehensive-docs claim, got %+v", got)
	}
}
