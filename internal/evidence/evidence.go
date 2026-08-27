// Package evidence tells a measurement from a claim.
//
// `todo close` and `backlog close story` require an `## Evidence` section
// and accept any non-empty one. So "I ran the suite and it passed" and the
// suite actually having run are indistinguishable to every check
// procoder makes — a ledger that looks measured and is really somebody's
// word for it (#208).
//
// The fix is not to ban prose. Plenty of real evidence is a sentence, and
// a rule demanding machine output for everything would be satisfied by
// pasting something irrelevant. What is missing is the DIFFERENCE being
// visible: a fingerprint can be checked, a sentence has to be trusted, and
// a reader deserves to know which one they are reading.
package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// recordTimeout bounds a command somebody asked to be measured. Generous,
// because a test suite is the obvious thing to record and suites are slow.
const recordTimeout = 10 * time.Minute

// Kind is what an evidence section is made of.
type Kind int

const (
	// Claim is prose: a person's account of what happened. Not worthless —
	// most evidence is this, and a sentence explaining why a check was not
	// needed is exactly right — but it is testimony, not measurement.
	Claim Kind = iota
	// Measured carries a fingerprint: something ran, and this is what it
	// produced. Checkable by anybody with the same command.
	Measured
)

func (k Kind) String() string {
	if k == Measured {
		return "measured"
	}
	return "manual claim"
}

// fingerprintLine is the shape Record prints and Classify recognises.
var fingerprintLine = regexp.MustCompile(`(?m)^\s*Fingerprint:\s*sha256:[0-9a-f]{8,}`)

// Classify reports whether an evidence section carries a measurement.
//
// Deliberately one signal, not a heuristic that guesses. A section either
// has a fingerprint procoder wrote or it does not; anything cleverer would
// be procoder judging prose, which it does not do anywhere else and would
// do badly here.
func Classify(section string) Kind {
	if fingerprintLine.MatchString(section) {
		return Measured
	}
	return Claim
}

// Record runs a command and prints evidence of what it produced, without
// printing what it produced.
//
// The output is never echoed and never written. A test suite's output can
// carry a token, a customer name, a path that identifies somebody's
// machine — and evidence lives in a file that gets committed. The
// fingerprint proves a specific command produced a specific result, which
// is strictly more falsifiable than prose, without putting the result on
// disk.
//
// This executes what a PERSON typed as an argument. It never executes
// something read from a repository file — that boundary is the subject of
// its own rule, and this command is on the right side of it because the
// command arrives from the command line, not from state an agent wrote.
func Record(root string, argv []string, out func(string)) int {
	if len(argv) == 0 {
		out("evidence record <command> — the command to run and fingerprint")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), recordTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) // nosemgrep -- typed by a person as an argument, never read from a file
	cmd.Dir = root
	raw, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		out(fmt.Sprintf("evidence NOT recorded — %s could not run (%v)", argv[0], err))
		return 2
	}
	if ctx.Err() != nil {
		out(fmt.Sprintf("evidence NOT recorded — %s did not finish within %s", argv[0], recordTimeout))
		return 2
	}
	sum := sha256.Sum256(raw)
	out("== paste this under `## Evidence`:")
	out("")
	out(fmt.Sprintf("Fingerprint: sha256:%s", hex.EncodeToString(sum[:])))
	out(fmt.Sprintf("Produced: %d bytes, exit %d", len(raw), code))
	// The command is echoed, because evidence that does not say what ran
	// proves nothing. That means a secret passed as an ARGUMENT is
	// printed — the protection here is against a secret in the command's
	// OUTPUT, which is the case a person cannot control.
	out(fmt.Sprintf("Command: %s", strings.Join(argv, " ")))
	out("")
	out("The output itself is deliberately not printed: evidence gets committed, and")
	out("a suite's output can carry a token, a customer name or somebody's home path.")
	return 0
}
