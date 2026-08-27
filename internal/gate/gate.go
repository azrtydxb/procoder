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
	"procoder/internal/parallel"
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
	// Each leg shells out to a different tool and none reads another's
	// answer, so they wait on each other for no reason. Measured over this
	// repository's 787 tracked files: gitleaks 41.2s, the suite 35.0s,
	// semgrep 25.3s, then lint 2.8s, osv 2.6s, complexity 1.1s, debt 0.2s
	// — 108s in a row, against 41s if the longest one sets the pace.
	//
	// The ORDER OF THE RESULTS is fixed regardless. Findings are printed
	// straight out of this slice, and a gate whose report reordered itself
	// between runs would make "did my change alter this?" unanswerable —
	// see #236 for what that costs when it happens by accident. Each leg
	// writes its own slot; the concatenation below is in declaration
	// order, whatever order they finished in.
	legs := []func() []gitx.Finding{
		// Domain 2: the canonical linters over the changed set — report by
		// default, blocking when the repo opted in ([lint] policy = "block").
		// A house rule by definition: it is procoder's choice of linter, and a
		// project that picked another one did not ask to be told about this.
		// Lint and complexity share golangci-lint, and golangci-lint
		// refuses to run twice at once — it takes a lock and the second
		// instance dies with "parallel golangci-lint is running", which
		// this gate then reports as a blocking complexity failure. They
		// are one leg for that reason, run in order inside it. CI caught
		// this on the change that made these concurrent; locally the two
		// happened never to overlap.
		//
		// The wording above is deliberate: the no-silent-green audit reads
		// this file as text, and the phrase it looks for inside a comment
		// here reads as an unblocked finding.
		func() []gitx.Finding {
			out := lint.Files(root, paths, cfg.LintBlock)
			// Complexity on the files this commit carries. Reported unless
			// the repository asked for block: these are judgement calls,
			// and a threshold that blocks by surprise stops work on
			// exactly the files that need the refactor.
			return append(out, maintain.ComplexityChanged(root, paths, cfg.MaintainBlock)...)
		},
		// Domain 1, the SAST leg: findings on the files this commit carries.
		// It costs seconds rather than milliseconds — semgrep's rule loading
		// is a fixed cost that scoping cannot remove — and it is here because
		// a commit is not a keystroke, and a finding caught now is caught
		// before it leaves the machine.
		func() []gitx.Finding { return security.SastChanged(root, paths) },
		// Known vulnerabilities, when the commit touches a dependency
		// manifest. Only then: the scan answers about the manifests, so a
		// commit that edits a comment would pay nearly a second to be told the
		// same thing it was told last time.
		func() []gitx.Finding { return security.DepsChanged(root, paths) },
		// Debt markers with no revisit condition, in the files this commit
		// carries. The whole ledger is the tree's and belongs to CI; what
		// belongs here is the shortcut being taken right now, while the reason
		// for it is still in the author's head.
		func() []gitx.Finding { return debt.GateCheck(root, paths) },
		// The suite, narrowed to the packages this commit touches: the whole
		// suite cold is a minute on this repository and one package is a
		// second. A failing test blocks where the repository asked; a suite
		// that could not run blocks regardless, because the policy governs
		// whether a FAILING test stops a commit and "no answer" is not a
		// verdict it has an opinion about.
		func() []gitx.Finding { return testrun.GateCheck(root, paths, cfg.TestBlock) },
	}
	return concurrently(legs)
}

// concurrently runs every leg at once and returns their findings
// concatenated IN DECLARATION ORDER, not completion order.
//
// Unbounded on purpose, unlike the per-file fan-out: this is a handful of
// legs fixed at compile time, not one unit of work per file in the tree.
func concurrently(legs []func() []gitx.Finding) []gitx.Finding {
	results := make([][]gitx.Finding, len(legs))
	parallel.Do(len(legs), func(i int) { results[i] = legs[i]() })
	var out []gitx.Finding
	for _, r := range results {
		out = append(out, r...)
	}
	return out
}

// hygieneFor is every non-formatting check the gate runs, and the one
// place the two scopes diverge. Extracted from RunWith so the divergence
// reads as three paired branches rather than being spread through a
// function that is also doing formatting and reporting.

// checkAll runs the formatter check over every path and returns the
// results IN THE ORDER GIVEN, whatever order they finished in. Callers
// print findings straight from this slice, so a run that reordered them
// would make the gate's output depend on which subprocess won a race.
//
// Concurrent because the cost here is process startup, not computation.
// Measured on this repository: 787 tracked files, 510 of them markdown, so
// the tracked-tree pass was 510 serial prettier cold starts at ~0.2s each
// — 6m47s in CI, against a job budget of 10 minutes. Nothing was slow; it
// was just one at a time.
//
// Bounded by NumCPU: every unit of work is an external process, and an
// unbounded fan-out over a large tree would fork a thousand of them.
func checkAll(paths []string) []format.Result {
	results := make([]format.Result, len(paths))
	// One budget for the whole binary, not one per fan-out: this pass, the
	// secret scan and the gate's legs all draw on it. See internal/parallel
	// for what happened when each sized itself independently.
	parallel.Do(len(paths), func(i int) {
		// Each unit writes a distinct index and reads none, so the slice
		// needs no lock.
		results[i] = format.Check(paths[i])
	})
	return results
}

func hygieneFor(root string, cfg config.Config, paths []string, commitMessage string, scope Scope) []gitx.Finding {
	// Three independent blocks, run at once. Measured over 787 tracked
	// files: the secret scan is ~41s and the house rules ~46s, and neither
	// reads the other's answer, so waiting was costing the sum of them.
	// Order of the findings is fixed by declaration, not by which finished.
	return concurrently([]func() []gitx.Finding{
		// Domain 9: git hygiene, workflow lint, message checks — same rules as
		// `procoder git`, via the shared Collect, so the two cannot drift apart.
		//
		// In a non-adopting repository only its universal half runs: conflict
		// markers, junk, oversized files and the attribution trailer. The rest
		// of it — the agent layer, release and documentation hygiene,
		// procoder's own templates, the planning chain — is procoder's house
		// style, and a project that never asked for it is not answerable to it.
		func() []gitx.Finding {
			if scope == Adopted {
				return gitcmd.CollectFor(root, cfg, paths, commitMessage)
			}
			return gitcmd.CollectUniversal(root, cfg, paths, commitMessage)
		},
		// Domain 1: a secret in a changed file blocks, always — in any
		// repository. What narrows outside an adopting one is WHERE it looks:
		// only the lines this commit wrote, because four thousand lines
		// somebody else wrote are not this commit's to answer for.
		func() []gitx.Finding {
			if scope == Adopted {
				// Conflict markers are not added here: CollectFor already ran
				// them whole-file, which is what an adopting repository asked for.
				return security.SecretsChangedFiles(root, paths)
			}
			out := security.SecretsInDiff(root, paths)
			// The same narrowing, for the same reason: a marker four thousand
			// lines away in somebody else's file is not this commit's doing.
			return append(out, gitx.NarrowToDiff(root, paths, gitx.ConflictMarkers(paths))...)
		},
		// Everything here is procoder's opinion about how code should be
		// written, and runs only where that was asked for. One block rather
		// than a flag on each: a repository either wanted these or it did
		// not, and a per-domain matrix would be configuration for
		// repositories that by definition carry no configuration.
		func() []gitx.Finding {
			if scope != Adopted {
				return nil
			}
			return houseRules(root, cfg, paths)
		},
	})
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

	// The formatting pass and the hygiene pass are the two halves of the
	// gate and neither reads the other, so they run at once. They were the
	// last two big blocks still waiting in line: ~45s of formatter startup
	// and ~46s of hygiene on this repository's 787 files.
	//
	// Started here and collected below, where its findings are first
	// needed — the formatting results are printed before them.
	hygieneDone := make(chan []gitx.Finding, 1)
	go func() { hygieneDone <- hygieneFor(root, cfg, paths, commitMessage, scope) }()

	var unformatted, unchecked []format.Result
	clean, skipped := 0, 0
	// Not "clean" and not "out of scope" — neither is true of a file
	// nobody looked at, and the summary line says so rather than filing it
	// under a verdict it never got. This was the first statement of the
	// loop below; it never depended on the file, so it is a guard.
	if scope == Adopted {
		for _, res := range checkAll(paths) {
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
	}

	for _, r := range unformatted {
		fmt.Fprintf(stdout, "unformatted  %s  (run `procoder format %q` for the result)\n", r.File, r.File)
	}
	for _, r := range unchecked {
		fmt.Fprintf(stdout, "UNCHECKED    %s — %s\n", r.File, r.Reason)
	}

	hygiene := <-hygieneDone

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
