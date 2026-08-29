// Package claims is coordination between agents working at once, not
// sandboxing.
//
// The principles already say two agents never own the same file. That is
// prose, and prose does not notice — a lead fanning out four subagents has
// no way to see that two of them were pointed at the same package until
// the second one's edits land on top of the first's (#199).
//
// A claim is a statement of intent: "I am working on these paths". It does
// not stop a write, and cannot — procoder does not own the editor. What it
// does is make an overlap VISIBLE before the work collides, which is the
// only part a tool can do honestly.
//
// Conservative on purpose. Two globs that COULD match the same path are
// reported as a conflict without proof that they will, because a false
// conflict costs a question and a missed one costs two agents' work.
package claims

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"procoder/internal/store"
)

// File is where claims live: procoder-owned session state, beside the
// handoff note. Ephemeral by nature — a claim outlives nothing but the
// work it describes.
const File = store.ClaimsPath

// Claim is one agent's declared working set.
type Claim struct {
	By     string   `json:"by"`
	Globs  []string `json:"globs"`
	Opened string   `json:"opened"`
}

type ledger struct {
	Claims []Claim `json:"claims"`
}

// Load reads the ledger, and says when it could not.
//
// An absent file is no claims, which is the ordinary case. A file that
// exists and cannot be read or parsed is NOT no claims — reporting it as
// none would say "nobody else is working here" on the strength of not
// having looked, which is the failure this package exists to prevent.
func Load(root string) ([]Claim, error) {
	raw, err := store.LoadClaims(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var l ledger
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("%s is not readable JSON (%v)", File, err)
	}
	return l.Claims, nil
}

// Save writes the ledger. This is procoder's own session state, not
// repository content — the same footing as the handoff note, and outside
// P-CONTROL for the same reason.
func Save(root string, cs []Claim) error {
	raw, err := json.MarshalIndent(ledger{Claims: cs}, "", "  ")
	if err != nil {
		return err
	}
	return store.SaveClaims(root, append(raw, '\n'))
}

// Overlap reports whether two globs could match the same path.
//
// Deliberately generous. It answers "could these collide" rather than
// "will they", because the question being asked is whether two agents
// might be about to write the same file, and a maybe is worth saying.
func Overlap(a, b string) bool {
	if a == b {
		return true
	}
	// A literal path under the other's directory prefix, either way round.
	if covers(a, b) || covers(b, a) {
		return true
	}
	// Both globs: compare the fixed part before the first wildcard.
	pa, pb := literalPrefix(a), literalPrefix(b)
	if pa == "" || pb == "" {
		return true // one of them is wide open
	}
	return strings.HasPrefix(pa, pb) || strings.HasPrefix(pb, pa)
}

func covers(glob, other string) bool {
	if ok, _ := path.Match(glob, other); ok {
		return true
	}
	dir := literalPrefix(glob)
	return dir != "" && strings.HasPrefix(other, dir)
}

// literalPrefix is the part of a glob before its first wildcard, trimmed
// to a path boundary so `internal/ga*` does not read as covering
// `internal/gate/`.
func literalPrefix(glob string) string {
	i := strings.IndexAny(glob, "*?[")
	if i < 0 {
		return glob
	}
	p := glob[:i]
	if j := strings.LastIndex(p, "/"); j >= 0 {
		return p[:j+1]
	}
	return ""
}

// Add records a claim and reports any it overlaps.
func Add(root, by string, globs []string, now time.Time, out func(string)) int {
	if strings.TrimSpace(by) == "" || len(globs) == 0 {
		out("claim <glob>... --by <agent> — who is claiming, and what")
		return 2
	}
	existing, err := Load(root)
	if err != nil {
		out("claims NOT read (" + err.Error() + ") — refusing to claim when procoder cannot see who else holds what")
		return 2
	}
	conflicts := Conflicts(existing, Claim{By: by, Globs: globs})
	existing = append(existing, Claim{By: by, Globs: globs, Opened: now.UTC().Format(time.RFC3339)})
	if err := Save(root, existing); err != nil {
		out("claim NOT recorded (" + err.Error() + ")")
		return 2
	}
	out(fmt.Sprintf("%s claims %s", by, strings.Join(globs, ", ")))
	for _, c := range conflicts {
		out("  CONFLICT " + c)
	}
	if len(conflicts) > 0 {
		out("  advisory: this reports an overlap, it does not prevent a write — procoder does not own the editor. Two agents on one file is a decision for whoever is leading them")
	}
	return 0
}

// Conflicts is every existing claim the new one could collide with.
func Conflicts(existing []Claim, want Claim) []string {
	var out []string
	for _, e := range existing {
		if strings.EqualFold(e.By, want.By) {
			continue // an agent does not conflict with itself
		}
		for _, eg := range e.Globs {
			for _, wg := range want.Globs {
				if Overlap(eg, wg) {
					out = append(out, fmt.Sprintf("%s already claims %s, which overlaps %s", e.By, eg, wg))
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// Release drops an agent's claims.
func Release(root, by string, out func(string)) int {
	existing, err := Load(root)
	if err != nil {
		out("claims NOT read (" + err.Error() + ")")
		return 2
	}
	var kept []Claim
	dropped := 0
	for _, c := range existing {
		if strings.EqualFold(c.By, by) {
			dropped++
			continue
		}
		kept = append(kept, c)
	}
	if err := Save(root, kept); err != nil {
		out("claims NOT written (" + err.Error() + ")")
		return 2
	}
	out(fmt.Sprintf("released %d claim(s) held by %s", dropped, by))
	return 0
}

// List prints who holds what, and every overlap between them.
func List(root string, out func(string)) int {
	cs, err := Load(root)
	if err != nil {
		out("claims NOT read (" + err.Error() + ") — this is not the same as nobody holding anything")
		return 2
	}
	if len(cs) == 0 {
		out("no active claims")
		return 0
	}
	for _, c := range cs {
		out(fmt.Sprintf("  %-16s %s", c.By, strings.Join(c.Globs, ", ")))
	}
	seen := map[string]bool{}
	for i, c := range cs {
		for _, other := range Conflicts(cs[:i], c) {
			if !seen[other] {
				out("  CONFLICT " + other)
				seen[other] = true
			}
		}
	}
	out(fmt.Sprintf("%d claim(s), %d overlap(s) — advisory: an overlap is reported, never prevented", len(cs), len(seen)))
	return 0
}
