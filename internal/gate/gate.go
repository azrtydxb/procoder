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
	"procoder/internal/gitx"
	"procoder/internal/lint"
	"procoder/internal/maintain"
	"procoder/internal/security"
	"procoder/internal/testrun"
)

// houseRules are the domains that encode procoder's own conventions:
// linting, SAST, complexity, dependency vulnerabilities, debt markers and
// the suite. Every one of them is an opinion about how code should be
// written, and every one is wrong to impose on a project that chose
// differently — Biome instead of prettier, its own changelog process, its
// own test command.
//
// They are grouped so the gate has one place to ask "did this repository
// ask for procoder's opinions", rather than nine.
func houseRules(root string, cfg config.Config, paths []string) []gitx.Finding {
	var out []gitx.Finding
	// Domain 2: the canonical linters over the changed set — report by
	// default, blocking when the repo opted in ([lint] policy = "block").
	// A house rule by definition: it is procoder's choice of linter, and a
	// project that picked another one did not ask to be told about this.
	out = append(out, lint.Files(root, paths, cfg.LintBlock)...)
	// Domain 1, the SAST leg: findings on the files this commit carries.
	// It costs seconds rather than milliseconds — semgrep's rule loading
	// is a fixed cost that scoping cannot remove — and it is here because
	// a commit is not a keystroke, and a finding caught now is caught
	// before it leaves the machine.
	out = append(out, security.SastChanged(root, paths)...)
	// Complexity on the files this commit carries. Reported unless the
	// repository asked for block: these are judgement calls, and a
	// threshold that blocks by surprise stops work on exactly the files
	// that need the refactor.
	out = append(out, maintain.ComplexityChanged(root, paths, cfg.MaintainBlock)...)
	// Known vulnerabilities, when the commit touches a dependency
	// manifest. Only then: the scan answers about the manifests, so a
	// commit that edits a comment would pay nearly a second to be told the
	// same thing it was told last time.
	out = append(out, security.DepsChanged(root, paths)...)
	// Debt markers with no revisit condition, in the files this commit
	// carries. The whole ledger is the tree's and belongs to CI; what
	// belongs here is the shortcut being taken right now, while the reason
	// for it is still in the author's head.
	out = append(out, debt.GateCheck(root, paths)...)
	// The suite, narrowed to the packages this commit touches: the whole
	// suite cold is a minute on this repository and one package is a
	// second. A failing test blocks where the repository asked; a suite
	// that could not run blocks regardless, because the policy governs
	// whether a FAILING test stops a commit and "no answer" is not a
	// verdict it has an opinion about.
	out = append(out, testrun.GateCheck(root, paths, cfg.TestBlock)...)
	return out
}

// hygieneFor is every non-formatting check the gate runs, and the one
// place the two scopes diverge. Extracted from RunWith so the divergence
// reads as three paired branches rather than being spread through a
// function that is also doing formatting and reporting.
func hygieneFor(root string, cfg config.Config, paths []string, commitMessage string, scope Scope) []gitx.Finding {
	// Domain 9: git hygiene, workflow lint, message checks — same rules as
	// `procoder git`, via the shared Collect, so the two cannot drift apart.
	//
	// In a non-adopting repository only its universal half runs: conflict
	// markers, junk, oversized files and the attribution trailer. The rest
	// of it — the agent layer, release and documentation hygiene,
	// procoder's own templates, the planning chain — is procoder's house
	// style, and a project that never asked for it is not answerable to it.
	var hygiene []gitx.Finding
	if scope == Adopted {
		hygiene = gitcmd.CollectFor(root, cfg, paths, commitMessage)
	} else {
		hygiene = gitcmd.CollectUniversal(root, cfg, paths, commitMessage)
	}
	// Domain 1: a secret in a changed file blocks, always — in any
	// repository. What narrows outside an adopting one is WHERE it looks:
	// only the lines this commit wrote, because four thousand lines
	// somebody else wrote are not this commit's to answer for.
	if scope == Adopted {
		// Conflict markers are not added here: CollectFor already ran
		// them whole-file, which is what an adopting repository asked for.
		hygiene = append(hygiene, security.SecretsChangedFiles(root, paths)...)
	} else {
		hygiene = append(hygiene, security.SecretsInDiff(root, paths)...)
		// The same narrowing, for the same reason: a marker four thousand
		// lines away in somebody else's file is not this commit's doing.
		hygiene = append(hygiene, gitx.NarrowToDiff(root, paths, gitx.ConflictMarkers(paths))...)
	}
	// Everything from here to the suite is procoder's opinion about how
	// code should be written, and runs only where that was asked for. One
	// block rather than a flag on each: a repository either wanted these
	// or it did not, and a per-domain matrix would be configuration for
	// repositories that by definition carry no configuration.
	if scope == Adopted {
		hygiene = append(hygiene, houseRules(root, cfg, paths)...)
	}

	return hygiene
}

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

	cfg := config.Load(root)

	// How much of procoder this repository is subject to. A repository
	// that never adopted it is somebody else's: it gets the checks that
	// are true anywhere and none of procoder's conventions. See ADR 0005.
	//
	// Decided before anything is checked, because formatting is scoped by
	// it too. A formatter is a house rule like any other — this repository
	// may run Biome, or gofumpt, or nothing, and rewriting its files to
	// procoder's taste is exactly the overreach #172 reported.
	scope, why := ScopeFor(root, cfg.GateScope)

	var unformatted, unchecked []format.Result
	clean, skipped := 0, 0
	for _, p := range paths {
		if scope != Adopted {
			// Not "clean" and not "out of scope" — neither is true. The
			// file was not looked at, and the summary line says so rather
			// than filing it under a verdict it never got.
			break
		}
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

	hygiene := hygieneFor(root, cfg, paths, commitMessage, scope)

	fmt.Fprintf(stdout, "gate scope: %s (%s)\n", scope, why)
	if scope == Universal {
		fmt.Fprintln(stdout, "  procoder's own conventions are NOT checked here — this repository has not adopted it.")
		fmt.Fprintf(stdout, "  For the full gate: %s=adopted, or adopt procoder in this repository.\n", ScopeEnv)
	}

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
	if scope == Adopted {
		fmt.Fprintf(stdout, "procoder gate: %d clean, %d unformatted, %d unchecked, %d out of scope, %d hygiene finding(s) (%d blocking)\n",
			clean, len(unformatted), len(unchecked), skipped, len(hygiene), blockingHygiene)
	} else {
		// "0 clean" would be a lie in the one direction that matters: it
		// reads as a formatting pass over nothing rather than a formatting
		// check that never ran. The count that IS true here is how many
		// files went unlooked-at.
		fmt.Fprintf(stdout, "procoder gate (universal): %d file(s) not formatting-checked, %d hygiene finding(s) (%d blocking)\n",
			len(paths), len(hygiene), blockingHygiene)
	}

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
