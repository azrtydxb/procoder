package codeindex

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"procoder/internal/tools"
)

// Refresh runs on every write the agent makes and rewrites the whole
// broad-tier index to update one file's symbols, so its cost grows with
// the repository rather than with the change. This repository's index is
// already past 1500 entries; the benchmark fixes a corpus of that size so
// a future change cannot quietly make each keystroke pay for the whole
// index twice.
func BenchmarkRefreshRewritesTheIndex(b *testing.B) {
	if tools.Resolve(Ctags, "") == "" {
		b.Skip("universal-ctags not installed; the benchmark runs where it is")
	}
	root := b.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		b.Skipf("no usable git: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "demo.go"), []byte(goSrc), 0o644); err != nil {
		b.Fatal(err)
	}
	if err := Build(root, func(string) {}); err != nil {
		b.Fatal(err)
	}
	// pad the index to a realistic size: the cost is the rewrite, so the
	// entry count is what the benchmark must hold fixed
	tags := filepath.Join(root, Dir, tagsFile)
	raw, err := os.ReadFile(tags)
	if err != nil {
		b.Fatal(err)
	}
	f, err := os.OpenFile(tags, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 1500; i++ {
		fmt.Fprintf(f, `{"name":"Sym%d","path":"pkg/file%d.go","line":%d,"kind":"func"}`+"\n", i, i%80, i)
	}
	if err := f.Close(); err != nil {
		b.Fatal(err)
	}
	if len(raw) == 0 {
		b.Fatal("fixture index is empty — the benchmark would time an early return")
	}

	target := filepath.Join(root, "demo.go")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Refresh(root, target)
	}
}
