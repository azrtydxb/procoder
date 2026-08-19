package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Drift runs on every write the agent makes, and it scans the whole
// Markdown corpus looking for mentions of the changed files — so its cost
// grows with the documentation, not with the change. This repository is
// already past 200 Markdown files; the benchmark fixes a corpus of that
// size so a future rewrite cannot quietly turn a per-write check into a
// per-write crawl.
func BenchmarkDriftOverATypicalCorpus(b *testing.B) {
	root := b.TempDir()
	dir := filepath.Join(root, "docs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatal(err)
	}
	// 200 pages of ordinary prose, a handful naming a source file
	for i := 0; i < 200; i++ {
		body := fmt.Sprintf("# Page %d\n\nProse about the system, several lines of it,\n"+
			"the way a real page reads rather than a stub.\n\nMore prose.\n", i)
		if i%20 == 0 {
			body += "\nSee `internal/service/handler.go` for the entry point.\n"
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("page-%03d.md", i)), []byte(body), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	// Drift only considers changed files that exist — it cannot verify a
	// doc against a file that is gone — so the fixture must create them.
	src := filepath.Join(root, "internal", "service")
	if err := os.MkdirAll(src, 0o755); err != nil {
		b.Fatal(err)
	}
	var changed []string
	for _, name := range []string{"handler.go", "store.go"} {
		p := filepath.Join(src, name)
		if err := os.WriteFile(p, []byte("package service\n"), 0o644); err != nil {
			b.Fatal(err)
		}
		changed = append(changed, p)
	}
	// a benchmark that measures nothing is worse than none: prove the work
	// actually happens before timing it
	if len(Drift(root, changed)) == 0 {
		b.Fatal("fixture produced no drift findings — the benchmark would time an early return")
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Drift(root, changed)
	}
}
