// Package initcmd computes — and on request executes — the install plan for
// the formatters this repository needs.
//
// The split follows P-CONTROL, the same way formatting does: by default the
// binary COMPUTES the plan and prints the exact commands, and the agent (or
// the human) executes them where they can be seen. `--yes` exists for the
// contexts with no agent in the loop — CI, a bare terminal — and even there
// every command is printed before it runs and the survey is re-run afterwards,
// because "the installer exited 0" and "the tool now answers" are different
// claims.
package initcmd

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"procoder/internal/doctor"
	"procoder/internal/tools"
)

// An installer that has produced no result in this long is wedged or waiting
// on a prompt nobody will answer. Package managers are slow; ten minutes is
// far beyond any healthy install, and reaching it is reported, not absorbed.
const installTimeout = 10 * time.Minute

// Step is one missing tool and what to do about it.
type Step struct {
	Tool *tools.Tool
	// Manager and Args are the chosen install command; Manager is empty when
	// no known package manager exists on this machine, in which case Fallback
	// carries the human instruction.
	Manager  string
	Args     []string
	Fallback string
}

// Plan surveys root and returns one Step per missing formatter. Tools already
// installed produce no step — init never reinstalls.
func Plan(root string) []Step {
	var steps []Step
	for _, t := range doctor.RequiredTools(root) {
		if tools.Resolve(t, root) != "" {
			continue
		}
		steps = append(steps, choose(t))
	}
	return steps
}

func choose(t *tools.Tool) Step {
	for _, c := range t.InstallVia {
		if _, err := exec.LookPath(c.Manager); err == nil {
			return Step{Tool: t, Manager: c.Manager, Args: c.Args}
		}
	}
	return Step{Tool: t, Fallback: t.Install}
}

// Run prints the plan; with execute it also carries it out. Returns the exit
// code: zero only when nothing is missing at the END — a plan that was merely
// printed leaves gaps and says so with its exit.
func Run(root string, execute bool, stdout io.Writer) int {
	steps := Plan(root)
	if len(steps) == 0 {
		fmt.Fprintln(stdout, "procoder init: every formatter this repository needs is installed")
		return 0
	}

	fmt.Fprintf(stdout, "procoder init: %d formatter(s) missing\n\n", len(steps))
	for _, s := range steps {
		if s.Manager == "" {
			fmt.Fprintf(stdout, "  %-14s no known package manager found — install by hand: %s\n",
				s.Tool.Name, s.Fallback)
			continue
		}
		fmt.Fprintf(stdout, "  %-14s %s %s\n", s.Tool.Name, s.Manager, strings.Join(s.Args, " "))
	}

	if !execute {
		fmt.Fprintln(stdout, "\nrun these commands (or `procoder init --yes` to have procoder run them), then `procoder doctor` to verify")
		return 1
	}

	failed := 0
	for _, s := range steps {
		if s.Manager == "" {
			failed++
			continue
		}
		fmt.Fprintf(stdout, "\n== %s %s\n", s.Manager, strings.Join(s.Args, " "))
		if err := runInstall(s, stdout); err != nil {
			fmt.Fprintf(stdout, "procoder init: %s failed: %v\n", s.Tool.Name, err)
			failed++
		}
	}

	// The re-survey is the actual answer. An installer's exit code is a claim;
	// the tool resolving on PATH is the fact.
	remaining := Plan(root)
	if len(remaining) == 0 && failed == 0 {
		fmt.Fprintln(stdout, "\nprocoder init: done — every formatter now answers")
		return 0
	}
	fmt.Fprintf(stdout, "\nprocoder init: %d formatter(s) still missing — run `procoder doctor` for details\n", len(remaining))
	return 1
}

func runInstall(s Step, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.Manager, s.Args...)
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("no result after %s — the installer was killed", installTimeout)
	}
	return err
}
