package format

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// chdir moves the test process somewhere else and back: the hook that
// gates a linked worktree runs with the session's root checkout as its cwd.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// repo makes a directory that RepoRoot stops at. A linked worktree's .git
// is a file, which is exactly what is written here.
func repo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// The formatter runs in the file's repository, whatever procoder's own
// working directory is. A stub that prints its cwd is the formatter here:
// what is under test is where procoder starts it, not what prettier does.
// proved by: removed cmd.Dir from run — the stub prints the other
// checkout, and this names it.
func TestTheFormatterRunsInTheFilesRepositoryNotProcodersCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub is a shell script")
	}
	wt := repo(t)
	elsewhere := repo(t)
	file := write(t, wt, "a.ts", "x\n")
	bin := filepath.Join(t.TempDir(), "fmt")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\npwd -P\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, elsewhere)

	got, err := run(bin, &tools.Tool{Name: "fmt", Args: func(f string) []string { return []string{f} }}, file, []byte("x\n"), tools.RepoRoot(filepath.Dir(file)))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(wt)
	if strings.TrimSpace(string(got)) != want {
		t.Errorf("the formatter ran in %q, want the file's repository %q", strings.TrimSpace(string(got)), want)
	}
}

// The reported case, against the real prettier: a file the worktree's
// .prettierignore covers is not unformatted, even when procoder was
// started in a checkout that has no .prettierignore (#326).
// proved by: removed cmd.Dir from run — gen.ts reads Unformatted.
func TestAWorktreesPrettierignoreHoldsFromAnotherCheckout(t *testing.T) {
	needs(t, "prettier")
	wt := repo(t)
	write(t, wt, ".prettierignore", "gen.ts\n")
	gen := write(t, wt, "gen.ts", "export   const   g={a:1}\n")
	chdir(t, repo(t)) // the root checkout: no .prettierignore

	if got := Check(gen); got.Verdict == Unformatted {
		t.Errorf("an ignored file was judged by the wrong checkout's ignore list: %+v", got)
	}
}

// A relative path still means relative to where procoder was asked: the
// directory change under the formatter must not move what the path names.
// proved by: dropped the filepath.Abs before exec — the stub is handed a
// path that no longer exists from its new cwd.
func TestARelativePathSurvivesTheMoveToTheRepositoryRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub is a shell script")
	}
	wt := repo(t)
	sub := filepath.Join(wt, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, sub, "s.ts", "relative\n")
	bin := filepath.Join(t.TempDir(), "fmt")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ncat \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, sub)

	got, err := run(bin, &tools.Tool{Name: "fmt", Args: func(f string) []string { return []string{f} }}, "s.ts", []byte("relative\n"), wt)
	if err != nil || string(got) != "relative\n" {
		t.Errorf("the relative path was lost in the move: %q, %v", got, err)
	}
}
