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

// ModuleDir is the directory of the go.mod nearest to file, ascending from
// the file's directory and stopping at root. A file under no go.mod belongs
// to root: the repository is then taken to be one module, as it was before
// nested modules were understood.
func ModuleDir(root, file string) string {
	dir := filepath.Dir(file)
	for {
		if exists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if dir == root || parent == dir {
			return root
		}
		dir = parent
	}
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
