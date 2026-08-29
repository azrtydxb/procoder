package store

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The content owners are the other half of .procoder/: the files a
// repository commits and reviews, as opposed to the gitignored session
// state in state.go. They get the same lock and the same atomic write —
// two sessions editing two stories must not lose one of them either.
//
// Two shapes cover all of them. A directory owner holds many files under
// one directory (specs, plans, stories, todos, ADRs); a document owner is
// a single known file (config.toml, PRINCIPLES.md, the github templates).

// ListDir names the files directly under relDir, sorted.
//
// An absent directory is an empty list and a nil error, not a fault: every
// caller already treats "no specs yet" as an ordinary state, and making
// them each unwrap an IsNotExist would be twenty chances to get it wrong.
func ListDir(root, relDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(relDir)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// LoadIn reads one file from a directory owner.
func LoadIn(root, relDir, name string) ([]byte, error) {
	p, err := inDir(relDir, name)
	if err != nil {
		return nil, err
	}
	return ReadFile(root, p)
}

// SaveIn replaces one file in a directory owner.
func SaveIn(root, relDir, name string, data []byte) error {
	p, err := inDir(relDir, name)
	if err != nil {
		return err
	}
	return save(root, p, data)
}

// LoadDoc reads a single-file owner.
func LoadDoc(root, relPath string) ([]byte, error) { return ReadFile(root, relPath) }

// SaveDoc replaces a single-file owner.
func SaveDoc(root, relPath string, data []byte) error { return save(root, relPath, data) }

// inDir refuses a name that is a path, for the reason markerPath does:
// joining an unchecked name lets ".." walk out of the directory the caller
// named and overwrite whatever it finds there.
func inDir(relDir, name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return "", errors.New("procoder: a file in " + relDir + " is named by a file name, not a path: " + name)
	}
	return strings.TrimRight(relDir, "/") + "/" + name, nil
}

// Rel turns an absolute path inside root into the repo-relative slash form
// every other function here speaks.
//
// It exists because several packages list .procoder/ files as absolute
// paths and hand them around — spec.Files, plan.Files, analysis.Files —
// and their readers would otherwise have to go around the store to open
// what they were given. It refuses a path outside root, so "read what I
// was handed" cannot become "read anything".
func Rel(root, abs string) (string, error) {
	r, err := resolve(root)
	if err != nil {
		return "", err
	}
	a, err := resolve(abs)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(r, a)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", errors.New("procoder: " + filepath.ToSlash(abs) + " is not a file inside " + filepath.ToSlash(root))
	}
	return rel, nil
}

// resolve makes a path absolute AND follows symlinks, which matters in both
// directions.
//
// Without it a legitimate path is refused: on macOS a temp root is
// /var/... while anything that has called EvalSymlinks holds
// /private/var/..., and comparing the two says "outside". And an escape is
// permitted: a symlink inside the root pointing out of it would otherwise
// produce a clean-looking relative path that reads and writes elsewhere.
//
// A path that does not exist yet cannot be resolved; its nearest existing
// ancestor is resolved instead, so a first write is not refused for not
// having happened yet.
func resolve(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		return r, nil
	}
	dir, base := filepath.Split(abs)
	// The fixed point, or this recurses forever: Clean("/") is "/", so a
	// filesystem root that EvalSymlinks cannot resolve — a Windows drive
	// that is not ready — would call this with the same argument until the
	// stack ran out.
	clean := filepath.Clean(dir)
	if clean == abs || base == "" {
		return abs, nil
	}
	parent, err := resolve(clean)
	if err != nil {
		return abs, nil
	}
	return filepath.Join(parent, base), nil
}

// LoadUnder and SaveUnder reach a .procoder/ file the caller was handed as
// an ABSOLUTE path.
//
// Several packages list .procoder/ files as absolute paths and hand them
// around — spec.Files, plan.Files, analysis.Files, Item.Path, Task.Path —
// and their readers would otherwise have to go around the store to open
// what they were given. Rel refuses a path outside the root, so "read what
// I was handed" cannot become "read anything".
func LoadUnder(root, abs string) ([]byte, error) {
	rel, err := Rel(root, abs)
	if err != nil {
		return nil, err
	}
	return LoadDoc(root, rel)
}

func SaveUnder(root, abs string, data []byte) error {
	rel, err := Rel(root, abs)
	if err != nil {
		return err
	}
	return SaveDoc(root, rel, data)
}

// OpenIn opens a file in a directory owner for streaming.
//
// The index's tag file runs to megabytes and two readers scan it line by
// line rather than loading it whole; handing them the bytes would undo
// that. Reads take no lock in any case, so a handle is no weaker than a
// ReadFile — and routing them here keeps the rule "nothing outside this
// package opens a .procoder/ file" true, which is what makes the rule
// checkable.
func OpenIn(root, relDir, name string) (*os.File, error) {
	p, err := inDir(relDir, name)
	if err != nil {
		return nil, err
	}
	return os.Open(filepath.Join(root, filepath.FromSlash(p)))
}
