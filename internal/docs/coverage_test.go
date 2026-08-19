package docs

import (
	"strings"
	"testing"
)

// tagLine writes one broad-tier index line the way `procoder index build` does.
func tagLine(name, path, kind string) string {
	return `{"name":"` + name + `","path":"` + path + `","line":1,"kind":"` + kind + `","language":"Go"}` + "\n"
}

func TestSurfaceCoverageRunsInAnyRepositoryAndNamesUndocumentedSurface(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/some/library\n\ngo 1.22\n")
	write(t, root, "lib.go", "package lib\n\nfunc Documented() {}\nfunc Orphan() {}\n")
	write(t, root, "README.md", "# lib\n\nCall `Documented` to start.\n")
	write(t, root, ".procoder/index/tags.jsonl",
		tagLine("Documented", "lib.go", "func")+tagLine("Orphan", "lib.go", "func")+
			tagLine("hidden", "lib.go", "func"))

	got := SurfaceCoverage(root)
	if len(got) != 1 {
		t.Fatalf("want exactly the one undocumented exported symbol, got %d: %+v", len(got), got)
	}
	if got[0].Blocking {
		t.Fatalf("coverage is information, never a wall: %+v", got[0])
	}
	if !strings.Contains(got[0].Message, "Orphan") {
		t.Fatalf("finding must name the symbol: %+v", got[0])
	}
}

func TestSurfaceCoverageCountsAgentsMarkdownAsDocumentation(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/other\n\ngo 1.22\n")
	write(t, root, "lib.go", "package lib\n\nfunc Orphan() {}\n")
	write(t, root, "AGENTS.md", "# rules\n\nOrphan is the entry point every host calls.\n")
	write(t, root, ".procoder/index/tags.jsonl", tagLine("Orphan", "lib.go", "func"))

	if got := SurfaceCoverage(root); len(got) != 0 {
		t.Fatalf("AGENTS.md is documentation — a mention there must count, got %+v", got)
	}
}

func TestSurfaceCoverageWithoutAnIndexSaysNotComputed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/other\n\ngo 1.22\n")
	write(t, root, "README.md", "# lib\n")

	got := SurfaceCoverage(root)
	if len(got) != 1 || !strings.Contains(got[0].Message, "NOT computed") {
		t.Fatalf("no index must read as NOT computed, never as clean: %+v", got)
	}
	if got[0].Blocking {
		t.Fatalf("the honesty line is information: %+v", got[0])
	}
}

func TestSurfaceCoverageRanksEntryPointsFirstAndCaps(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/wide\n\ngo 1.22\n")
	write(t, root, "README.md", "# wide\n")
	var tags strings.Builder
	tags.WriteString(tagLine("Ztype", "lib.go", "type"))
	for i := 'A'; i <= 'Z'; i++ {
		tags.WriteString(tagLine("Fn"+string(i), "lib.go", "func"))
	}
	write(t, root, ".procoder/index/tags.jsonl", tags.String())

	got := SurfaceCoverage(root)
	if len(got) != surfaceCoverageCap+1 {
		t.Fatalf("want the cap plus the honest tail line, got %d", len(got))
	}
	if !strings.Contains(got[0].Message, "func FnA") {
		t.Fatalf("entry points rank first, alphabetically: %+v", got[0])
	}
	if !strings.Contains(got[len(got)-1].Message, "more exported symbol") {
		t.Fatalf("the tail must say how much was not listed: %+v", got[len(got)-1])
	}
}

func TestSurfaceCoverageRanksPublicSurfaceAboveInternal(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/ranked\n\ngo 1.22\n")
	write(t, root, "README.md", "# ranked\n")
	write(t, root, ".procoder/index/tags.jsonl",
		tagLine("Aaa", "internal/deep.go", "func")+tagLine("Zzz", "api.go", "func"))

	got := SurfaceCoverage(root)
	if len(got) != 2 || !strings.Contains(got[0].Message, "Zzz") {
		t.Fatalf("what a caller can reach outranks what only the project can: %+v", got)
	}
}

// Commands is procoder's own inventory, pinned against the usage text by
// cmd/procoder's test; no check may hold another repository to it.
func TestCommandsListSurvivesForTheUsagePin(t *testing.T) {
	if len(Commands) == 0 {
		t.Fatal("Commands is the list cmd/procoder pins its usage against")
	}
}
