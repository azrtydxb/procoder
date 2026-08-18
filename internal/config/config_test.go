package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".procoder", "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestMissingFileMeansDefaults(t *testing.T) {
	cfg := Load(t.TempDir())
	if cfg.BlockDefaultBranch || cfg.MaxFileMB != 5 {
		t.Fatalf("defaults wrong: %+v", cfg)
	}
}

func TestValuesAreRead(t *testing.T) {
	dir := write(t, t.TempDir(), `
# comment
[git]
default_branch_policy = "block"
max_file_mb = 25   # generous
`)
	cfg := Load(dir)
	if !cfg.BlockDefaultBranch || cfg.MaxFileMB != 25 {
		t.Fatalf("got %+v", cfg)
	}
}

func TestGarbageLinesAreSkippedNotGuessed(t *testing.T) {
	dir := write(t, t.TempDir(), `
[git]
max_file_mb = not-a-number
default_branch_policy = "report"
`)
	cfg := Load(dir)
	if cfg.MaxFileMB != 5 || cfg.BlockDefaultBranch {
		t.Fatalf("got %+v", cfg)
	}
}
