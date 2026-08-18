// Package gate is the commit-time formatting check: `procoder check`. It runs
// over the changed files (or the paths it is given), exits non-zero when
// anything is unformatted or unchecked, and counts what it skipped — because a
// count of skipped files is the difference between "clean" and "not looked at".
package gate

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"procoder/internal/format"
)

// Run checks the given paths, or the repository's changed files when none are
// given. Returns the exit code.
func Run(paths []string, root string, stdout io.Writer) int {
	if len(paths) == 0 {
		var err error
		paths, err = changedFiles(root)
		if err != nil {
			fmt.Fprintf(stdout, "procoder: cannot list changed files (%v) — pass paths explicitly\n", err)
			return 2
		}
		if len(paths) == 0 {
			fmt.Fprintln(stdout, "procoder: no changed files")
			return 0
		}
	}

	var unformatted, unchecked []format.Result
	clean, skipped := 0, 0
	for _, p := range paths {
		res := format.Check(p)
		switch res.Verdict {
		case format.Clean:
			clean++
		case format.OutOfScope:
			skipped++
		case format.Unformatted:
			unformatted = append(unformatted, res)
		case format.Unchecked:
			unchecked = append(unchecked, res)
		}
	}

	for _, r := range unformatted {
		fmt.Fprintf(stdout, "unformatted  %s  (run `procoder format %q` for the result)\n", r.File, r.File)
	}
	for _, r := range unchecked {
		fmt.Fprintf(stdout, "UNCHECKED    %s — %s\n", r.File, r.Reason)
	}
	fmt.Fprintf(stdout, "procoder format gate: %d clean, %d unformatted, %d unchecked, %d out of scope\n",
		clean, len(unformatted), len(unchecked), skipped)

	// Unchecked fails the gate exactly like unformatted does: a file the gate
	// could not look at is not a passing file.
	if len(unformatted) > 0 || len(unchecked) > 0 {
		return 1
	}
	return 0
}

// changedFiles: everything modified, added, renamed or untracked per git —
// the set a commit is about to contain. Deleted files are not checkable.
func changedFiles(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		status, name := line[:2], strings.TrimSpace(line[3:])
		if strings.Contains(status, "D") {
			continue
		}
		// a rename is "old -> new"; the new side is what the commit contains
		if i := strings.Index(name, " -> "); i >= 0 {
			name = name[i+4:]
		}
		files = append(files, filepath.Join(root, name))
	}
	return files, nil
}
