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

	"procoder/internal/codeindex"
	"procoder/internal/config"
	"procoder/internal/debt"
	"procoder/internal/format"
	"procoder/internal/gitcmd"
	"procoder/internal/lint"
	"procoder/internal/maintain"
	"procoder/internal/security"
	"procoder/internal/testrun"
)

// Run checks the given paths, or the repository's changed files when none are
// given. Returns the exit code.
func Run(paths []string, root string, stdout io.Writer) int {
	return RunWith(paths, root, "", stdout)
}

// RunWith is Run plus the commit message being prepared, so the
// documentation acknowledgment can clear its obligation at the moment of
// the commit. Everything else is identical.
func RunWith(paths []string, root string, commitMessage string, stdout io.Writer) int {
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

	// a stale code index refreshes at this lifecycle moment — the per-write
	// hook cannot see editor edits or reach the precise tier
	if note := codeindex.RebuildIfStale(root); note != "" {
		fmt.Fprintln(stdout, note)
	}
	// the change's blast radius, put in front of the agent at the moment it
	// matters — informational, the agent judges
	codeindex.ImpactIfIndexed(root, paths, func(line string) { fmt.Fprintln(stdout, "  info  "+line) })

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

	cfg := config.Load(root)
	// Domain 9: git hygiene, workflow lint, message checks — same rules as
	// `procoder git`, via the shared Collect, so the two cannot drift apart.
	hygiene := gitcmd.CollectFor(root, cfg, paths, commitMessage)
	// Domain 2: the canonical linters over the changed set — report by
	// default, blocking when the repo opted in ([lint] policy = "block").
	hygiene = append(hygiene, lint.Files(root, paths, cfg.LintBlock)...)
	// Domain 1: a secret in a changed file blocks, always
	hygiene = append(hygiene, security.SecretsChangedFiles(root, paths)...)
	// Domain 1, the SAST leg: findings on the files this commit carries.
	// It costs seconds rather than milliseconds — semgrep's rule loading
	// is a fixed cost that scoping cannot remove — and it is here because
	// a commit is not a keystroke, and a finding caught now is caught
	// before it leaves the machine.
	hygiene = append(hygiene, security.SastChanged(root, paths)...)
	// Complexity on the files this commit carries. Reported unless the
	// repository asked for block: these are judgement calls, and a
	// threshold that blocks by surprise stops work on exactly the files
	// that need the refactor.
	hygiene = append(hygiene, maintain.ComplexityChanged(root, paths, cfg.MaintainBlock)...)
	// Known vulnerabilities, when the commit touches a dependency
	// manifest. Only then: the scan answers about the manifests, so a
	// commit that edits a comment would pay nearly a second to be told the
	// same thing it was told last time.
	hygiene = append(hygiene, security.DepsChanged(root, paths)...)
	// Debt markers with no revisit condition, in the files this commit
	// carries. The whole ledger is the tree's and belongs to CI; what
	// belongs here is the shortcut being taken right now, while the reason
	// for it is still in the author's head.
	hygiene = append(hygiene, debt.GateCheck(root, paths)...)
	// The suite, narrowed to the packages this commit touches: the whole
	// suite cold is a minute on this repository and one package is a
	// second. A failing test blocks where the repository asked; a suite
	// that could not run blocks regardless, because the policy governs
	// whether a FAILING test stops a commit and "no answer" is not a
	// verdict it has an opinion about.
	hygiene = append(hygiene, testrun.GateCheck(root, paths, cfg.TestBlock)...)
	blockingHygiene := 0
	for _, f := range hygiene {
		mark := "info        "
		if f.Blocking {
			mark = "BLOCKING    "
			blockingHygiene++
		}
		loc := ""
		if f.File != "" {
			loc = f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", loc, f.Line)
			}
			loc = loc + "  "
		}
		fmt.Fprintf(stdout, "%s %s%s\n", mark, loc, f.Message)
	}
	fmt.Fprintf(stdout, "procoder gate: %d clean, %d unformatted, %d unchecked, %d out of scope, %d hygiene finding(s) (%d blocking)\n",
		clean, len(unformatted), len(unchecked), skipped, len(hygiene), blockingHygiene)

	// Unchecked fails the gate exactly like unformatted does: a file the gate
	// could not look at is not a passing file. Blocking hygiene findings fail
	// it for the same reason a human reviewer would — this condition is pinned
	// by a test, because the first release printed the findings and exited 0.
	if len(unformatted) > 0 || len(unchecked) > 0 || blockingHygiene > 0 {
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
