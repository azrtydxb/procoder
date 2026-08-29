// Package learn measures what procoder's own governance costs in THIS
// repository, and proposes configuration changes against those
// measurements (#190).
//
// Nothing here is applied. P-CONTROL: the binary prints and the agent
// writes, so `learn propose` prints a change and `learn verify` prints a
// revert. The issue asked for a loop that applies and reverts on its own;
// that loop cannot exist in this binary, so the loop closes with a person
// in it and `verify` is what keeps that honest rather than optional.
//
// Recording is off unless the repository asks for it, and a failed record
// write is dropped in silence — the one place in procoder where a failure
// is not reported, because measurement must never be able to fail the
// thing it measures.
package learn

import (
	"encoding/json"
	"sort"
	"strings"

	"procoder/internal/store"
)

// Dir is where the records live, under the gitignored state directory:
// timing data is not repository content and is never committed.
const Dir = store.StateDir

// File is the append-only record, one JSON object per line.
const File = "learn.jsonl"

// AppliedFile is the marker an agent writes when it applies a proposal,
// and the anchor `verify` measures against. Inferring the moment from the
// git history of config.toml was considered and rejected in the spec:
// history shows THAT the file changed, never which proposal a change
// corresponds to, nor whether an edit was a proposal at all.
const AppliedFile = "learn-applied.json"

// maxRecords bounds the file so an old repository does not carry an
// unbounded one. Oldest dropped.
//
// debt: a flat line count, not a size or an age. It is the cheapest thing
// that bounds the file, and a repository whose commands vary wildly in
// number per day gets an uneven window. Revisit when somebody reports a
// window that is too short to be useful.
const maxRecords = 5000

// Record is one command run.
type Record struct {
	Cmd      string `json:"cmd"`
	Domain   string `json:"domain,omitempty"`
	Ms       int64  `json:"ms"`
	Exit     int    `json:"exit"`
	Blocking bool   `json:"blocking"`
	At       string `json:"at"`
}

// Append writes one record, and reports nothing when it cannot.
//
// Deliberately silent on failure, and the only such place in procoder. The
// gate runs on every commit; a measurement that could turn a clean commit
// into a failed one would be a governance cost of its own, and worse than
// the ignorance it was meant to remove.
func Append(root string, r Record, on bool) {
	if !on {
		return
	}
	line, err := json.Marshal(r)
	if err != nil {
		return
	}
	// Silence is the contract, per the doc comment above: a measurement
	// able to fail the run it measures is a governance cost of its own.
	_ = store.AppendLearn(root, append(line, '\n'))
}

// Reading is what a report knows about the records: the ones it could
// parse, and honestly, the ones it could not.
type Reading struct {
	Records  []Record
	Corrupt  int
	Negative int
}

// Read parses the record file. A line it cannot read is counted, never
// dropped in silence: a report that quietly discarded half its input would
// be a measurement nobody could check.
func Read(root string) (Reading, error) {
	raw, err := store.LoadLearn(root)
	if err != nil {
		return Reading{}, err
	}
	var out Reading
	for _, line := range strings.Split(normaliseEOL(string(raw)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r Record
		if json.Unmarshal([]byte(line), &r) != nil {
			// A truncated tail is what a killed process leaves; it is
			// expected, and it is still counted.
			out.Corrupt++
			continue
		}
		if r.Ms < 0 {
			// Dropped rather than clamped. A clock that moved backwards
			// produced a duration nobody can interpret, and clamping it to
			// zero would put a fiction into an average.
			out.Negative++
			continue
		}
		out.Records = append(out.Records, r)
	}
	return out, nil
}

func normaliseEOL(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// Cost is one command's or domain's share of the total.
type Cost struct {
	Name    string
	Runs    int
	TotalMs int64
	Blocked int
}

// rank groups the records by command, slowest first. Ties break on the
// name so two runs over the same records print the same report — the
// property #236 cost a session to establish the value of.
func rank(rs []Record) []Cost {
	by := map[string]*Cost{}
	for _, r := range rs {
		k := r.Cmd
		if r.Domain != "" {
			k = r.Cmd + " (" + r.Domain + ")"
		}
		c, ok := by[k]
		if !ok {
			c = &Cost{Name: k}
			by[k] = c
		}
		c.Runs++
		c.TotalMs += r.Ms
		if r.Blocking {
			c.Blocked++
		}
	}
	out := make([]Cost, 0, len(by))
	for _, c := range by {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalMs != out[j].TotalMs {
			return out[i].TotalMs > out[j].TotalMs
		}
		return out[i].Name < out[j].Name
	})
	return out
}
