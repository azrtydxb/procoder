// Package gomod answers which Go module owns a file. golangci-lint and
// `go test` both resolve packages against the module of their working
// directory, so a file in a nested module (its own go.mod, e.g. a bench/
// with `replace => ../`) cannot be named from the repository root: the run
// fails with "main module ... does not contain package". Callers group
// files by module and run once per group from the module's directory.
package gomod

import (
	"os"
	"path/filepath"
	"strings"
)

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Workspace reports whether root carries a go.work. A workspace resolves
// every member module from the root, so such a repository needs no split.
func Workspace(root string) bool {
	return exists(filepath.Join(root, "go.work"))
}

// ModuleDir is the directory of the go.mod nearest to p, ascending from p
// when it is a directory (the gate passes package directories) or from its
// parent when it is a file, and stopping at root. A path under no go.mod, or
// outside root, belongs to root: the repository is then taken to be one
// module, as it was before nested modules were understood. A go.mod under a
// testdata or vendor directory is ignored, as the go command ignores it, so
// a file there belongs to the enclosing real module.
func ModuleDir(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return root
	}
	dir := p
	if info, err := os.Stat(p); err != nil || !info.IsDir() {
		dir = filepath.Dir(p)
	}
	for {
		if exists(filepath.Join(dir, "go.mod")) && !ignoredDir(root, dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if dir == root || parent == dir {
			return root
		}
		dir = parent
	}
}

// ignoredDir reports whether dir sits at or under a testdata or vendor
// directory below root.
func ignoredDir(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == "testdata" || part == "vendor" {
			return true
		}
	}
	return false
}

// Group splits absolute paths by owning module directory. Under a go.work
// at root everything stays one group at root.
func Group(root string, paths []string) map[string][]string {
	out := map[string][]string{}
	work := Workspace(root)
	for _, p := range paths {
		dir := root
		if !work {
			dir = ModuleDir(root, p)
		}
		out[dir] = append(out[dir], p)
	}
	return out
}
