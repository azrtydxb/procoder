package learn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"procoder/internal/evidence"
)

// class labels a number with where it came from, reusing the evidence
// classification rather than inventing a second vocabulary for the same
// distinction (S-4).
//
// A total over recorded runs is measured. Anything this package infers
// from those totals — a projection, a proposal's expected saving — is a
// claim, and says so in the same words `backlog close` uses.
func class(measured bool) string {
	if measured {
		return evidence.Measured.String()
	}
	return evidence.Claim.String()
}

// Measure prints what procoder costs in this repository (S-2).
func Measure(root string, recording bool, out func(string)) int {
	reading, err := Read(root)
	if err != nil {
		if os.IsNotExist(err) {
			if !recording {
				// Not "no cost". Nothing was ever recorded, and the report
				// names the setting rather than printing an empty ranking
				// that reads like a measurement of zero.
				out("no timing records — recording is off; set [learn] record = true in .procoder/config.toml")
				return 0
			}
			out("no timing records yet — recording is on, so the next `procoder check` starts one")
			return 0
		}
		// Unknown is never none.
		out("NOT measured — " + filepath.ToSlash(filepath.Join(Dir, File)) + " cannot be read (" + err.Error() + ")")
		return 1
	}
	if len(reading.Records) == 0 {
		out("no usable timing records" + skipped(reading))
		return 0
	}
	ranked := rank(reading.Records)
	out(fmt.Sprintf("procoder learn: %d run(s) recorded, ranked by total time [%s]%s",
		len(reading.Records), class(true), skipped(reading)))
	for _, c := range ranked {
		line := fmt.Sprintf("  %-28s %8s over %3d run(s) [%s]",
			c.Name, dur(c.TotalMs), c.Runs, class(true))
		if c.Blocked > 0 {
			line += fmt.Sprintf("  blocked %d", c.Blocked)
		}
		out(line)
	}
	return 0
}

// skipped reports what the reading could not use. Never silent: a report
// that discarded input without saying so is not a measurement.
func skipped(r Reading) string {
	if r.Corrupt == 0 && r.Negative == 0 {
		return ""
	}
	return fmt.Sprintf(" — %d unreadable line(s), %d negative duration(s) skipped", r.Corrupt, r.Negative)
}

func dur(ms int64) string {
	if ms < 1000 {
		return strconv.FormatInt(ms, 10) + "ms"
	}
	return strconv.FormatFloat(float64(ms)/1000, 'f', 1, 64) + "s"
}

// Propose prints configuration changes, and writes nothing (S-3).
func Propose(root string, recording bool, minSamples int, out func(string)) int {
	reading, err := Read(root)
	if err != nil {
		return Measure(root, recording, out)
	}
	if len(reading.Records) < minSamples {
		// A ranking from four runs is not evidence, and saying how many
		// more are needed is more useful than a proposal nobody should act
		// on.
		out(fmt.Sprintf("no proposal: %d run(s) recorded, %d needed — %d more ([learn] min_samples)",
			len(reading.Records), minSamples, minSamples-len(reading.Records)))
		return 0
	}
	ranked := rank(reading.Records)
	out(fmt.Sprintf("procoder learn: %d run(s) recorded [%s]%s", len(reading.Records), class(true), skipped(reading)))
	printed := 0
	for _, c := range ranked {
		p := proposalFor(c, len(reading.Records))
		if p == "" {
			continue
		}
		printed++
		out("")
		out(fmt.Sprintf("  %s cost %s over %d run(s) [%s]", c.Name, dur(c.TotalMs), c.Runs, class(true)))
		out("  " + p)
		out("  expected saving is a projection from those runs, not a measurement [" + class(false) + "]")
		if c.Blocked == 0 {
			// S-7. The measurement can see what a check COST and cannot
			// see what it PREVENTED, and a proposal that showed one side
			// of that trade as though it were both is the failure this
			// command exists to avoid.
			out("  it cannot see the defects this check prevented — that cost is not in these records")
		}
	}
	if printed == 0 {
		out("  nothing worth changing: no single command dominates these runs")
	}
	out("")
	out("procoder writes none of this. Apply what you agree with, then record it:")
	out("  " + filepath.ToSlash(filepath.Join(Dir, AppliedFile)))
	return 0
}

// proposalFor is the one heuristic here, and it is deliberately blunt.
//
// debt: a single threshold — a command that is more than half of all
// recorded time. It proposes nothing subtle and will miss a repository
// whose cost is spread evenly. Revisit when there are real records from
// more than one repository to reason about, which is the thing this
// command exists to produce.
func proposalFor(c Cost, totalRuns int) string {
	if c.Runs*2 < totalRuns {
		return ""
	}
	if c.Blocked == 0 {
		return "consider [lint] policy = \"report\" for this domain if it has never blocked here"
	}
	return "it blocked " + strconv.Itoa(c.Blocked) + " time(s); the cost is buying something"
}

// Applied is the marker an agent writes when it applies a proposal.
type Applied struct {
	Target   string `json:"target"`
	At       string `json:"at"`
	BeforeMs int64  `json:"before_ms"`
}

// Verify reports whether an applied proposal actually reduced what it
// targeted, and prints the revert when it did not (S-5).
func Verify(root string, out func(string)) int {
	path := filepath.Join(root, filepath.FromSlash(Dir), AppliedFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		// No anchor is not "it worked". Without a marker there is nothing
		// to measure against, and guessing from the git history of
		// config.toml cannot tell which proposal an edit corresponds to.
		out("no proposal marker at " + filepath.ToSlash(filepath.Join(Dir, AppliedFile)) +
			" — NOT verified; record one when you apply a proposal")
		return 1
	}
	var a Applied
	if json.Unmarshal(raw, &a) != nil {
		out(filepath.ToSlash(filepath.Join(Dir, AppliedFile)) + " is unreadable — NOT verified")
		return 1
	}
	reading, rerr := Read(root)
	if rerr != nil {
		out("NOT verified — the records cannot be read (" + rerr.Error() + ")")
		return 1
	}
	since, err := time.Parse(time.RFC3339, a.At)
	if err != nil {
		out(filepath.ToSlash(filepath.Join(Dir, AppliedFile)) + " has no readable timestamp — NOT verified")
		return 1
	}
	var after Cost
	for _, r := range reading.Records {
		t, terr := time.Parse(time.RFC3339, r.At)
		if terr != nil || t.Before(since) {
			continue
		}
		name := r.Cmd
		if r.Domain != "" {
			name = r.Cmd + " (" + r.Domain + ")"
		}
		if name != a.Target {
			continue
		}
		after.Runs++
		after.TotalMs += r.Ms
	}
	if after.Runs == 0 {
		out(a.Target + ": no runs recorded since the proposal was applied — NOT verified")
		return 1
	}
	out(fmt.Sprintf("%s: %s before, %s over %d run(s) since [%s]",
		a.Target, dur(a.BeforeMs), dur(after.TotalMs), after.Runs, class(true)))
	if after.TotalMs < a.BeforeMs {
		out("  the cost it targeted fell")
		return 0
	}
	out("  the cost it targeted did NOT fall — revert it:")
	out("  git diff HEAD -- .procoder/config.toml   # then undo that change")
	return 1
}
