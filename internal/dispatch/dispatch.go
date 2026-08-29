// Package dispatch records whether a fan-out was actually parallel.
//
// The principles tell an agent to launch independent work together rather
// than grinding through it serially. Nothing checked, and the difference
// is invisible afterwards: an agent that ran three subagents one at a
// time and narrated it as parallel produces the same transcript as one
// that did it properly (#202).
//
// The states are a barrier. Every task in a wave is STARTED, then the wave
// is SEALED, and only then may anything RETURN. A return recorded before
// the seal is the signature of serial work — the first task finished
// before the last was launched, which is precisely what parallel means it
// should not have done.
//
// Advisory, and the issue says so itself: procoder cannot stop an agent
// calling this dishonestly, or at all. What it can do is make the claim
// checkable, which turns "I ran them in parallel" from a narration into
// something with a record behind it. It catches the honest mistake
// — genuinely serial work, honestly recorded — and that is the common one.
package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"procoder/internal/store"
)

// File is where waves live: procoder's own session state, beside the
// handoff note and the claims ledger.
const File = store.DispatchPath

// Wave is one fan-out.
type Wave struct {
	ID string `json:"id"`
	// Started, in the order the tasks were launched.
	Started []string `json:"started"`
	// Sealed is when no more tasks may join. Empty means still open.
	Sealed string `json:"sealed,omitempty"`
	// Returned, in the order they came back.
	Returned []string `json:"returned"`
	// EarlyReturn is a task that came back before the wave was sealed.
	// Recorded when it happens, because by the time status is asked the
	// seal has usually been called and the evidence would be gone.
	EarlyReturn []string `json:"early_return,omitempty"`
}

type ledger struct {
	Waves []Wave `json:"waves"`
}

// Load reads the ledger, and says when it could not. An absent file is no
// waves; a file that exists and will not parse is not the same thing, and
// reporting it as none would be claiming every wave was clean on the
// strength of not having looked.
func Load(root string) ([]Wave, error) {
	raw, err := store.LoadDispatch(root)
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
	return l.Waves, nil
}

// Save writes the ledger — session state, not repository content.
func Save(root string, ws []Wave) error {
	raw, err := json.MarshalIndent(ledger{Waves: ws}, "", "  ")
	if err != nil {
		return err
	}
	return store.SaveDispatch(root, append(raw, '\n'))
}

func find(ws []Wave, id string) int {
	for i, w := range ws {
		if w.ID == id {
			return i
		}
	}
	return -1
}

// Apply performs one transition and returns the ledger to save.
//
// Split out from the command so the barrier itself is testable without a
// filesystem: the rule is the interesting part, not the plumbing.
func Apply(ws []Wave, verb, id, task string, now time.Time) ([]Wave, string, int) {
	i := find(ws, id)
	switch verb {
	case "open":
		if i >= 0 {
			return ws, fmt.Sprintf("wave %q already exists — pick another id rather than reopening one whose record you would be overwriting", id), 2
		}
		return append(ws, Wave{ID: id}), fmt.Sprintf("wave %s open", id), 0
	case "start":
		if i < 0 {
			return ws, fmt.Sprintf("no wave %q — open it first", id), 2
		}
		if ws[i].Sealed != "" {
			return ws, fmt.Sprintf("wave %s is sealed — a task starting after the seal was not part of the wave, and recording it as one would make the barrier meaningless", id), 2
		}
		ws[i].Started = append(ws[i].Started, task)
		return ws, fmt.Sprintf("wave %s started %s (%d launched)", id, task, len(ws[i].Started)), 0
	case "seal":
		if i < 0 {
			return ws, fmt.Sprintf("no wave %q — open it first", id), 2
		}
		if ws[i].Sealed != "" {
			return ws, fmt.Sprintf("wave %s is already sealed", id), 0
		}
		ws[i].Sealed = now.UTC().Format(time.RFC3339)
		return ws, fmt.Sprintf("wave %s sealed — %d task(s) launched before anything was awaited", id, len(ws[i].Started)), 0
	case "return":
		if i < 0 {
			return ws, fmt.Sprintf("no wave %q — open it first", id), 2
		}
		ws[i].Returned = append(ws[i].Returned, task)
		if ws[i].Sealed == "" {
			// The signature of serial work: this one finished before the
			// last was launched. Recorded now, because by the time anybody
			// asks, the seal will have been called and the evidence gone.
			ws[i].EarlyReturn = append(ws[i].EarlyReturn, task)
			return ws, fmt.Sprintf("wave %s returned %s BEFORE the seal — this task finished before the wave stopped accepting launches, which is what serial work looks like", id, task), 0
		}
		return ws, fmt.Sprintf("wave %s returned %s (%d of %d)", id, task, len(ws[i].Returned), len(ws[i].Started)), 0
	}
	return ws, "unknown dispatch verb " + verb, 2
}

// Status reports the verdict for one wave, or all of them.
func Status(root, id string, out func(string)) int {
	ws, err := Load(root)
	if err != nil {
		out("dispatch NOT read (" + err.Error() + ") — which is not the same as no waves having run")
		return 2
	}
	if len(ws) == 0 {
		out("no waves recorded")
		return 0
	}
	sort.Slice(ws, func(i, j int) bool { return ws[i].ID < ws[j].ID })
	for _, w := range ws {
		if id != "" && w.ID != id {
			continue
		}
		out(Verdict(w))
	}
	return 0
}

// Verdict is the one-line answer for a wave.
//
// Three outcomes, and the middle one is the point: not verified is not the
// same as verified serial. A wave still open, or one nobody sealed, has
// not been shown to be parallel — and saying so is different from saying
// it was not.
func Verdict(w Wave) string {
	switch {
	case len(w.EarlyReturn) > 0:
		return fmt.Sprintf("  %s: NOT parallel — %s returned before the wave was sealed, so the first finished before the last was launched",
			w.ID, strings.Join(w.EarlyReturn, ", "))
	case w.Sealed == "":
		return fmt.Sprintf("  %s: NOT verified — %d started, never sealed. Nothing here says the wave was serial; it says nobody recorded the barrier",
			w.ID, len(w.Started))
	default:
		return fmt.Sprintf("  %s: parallel — %d task(s) launched before the first was awaited, %d returned",
			w.ID, len(w.Started), len(w.Returned))
	}
}

// Run is the command surface.
func Run(root, verb, id, task string, now time.Time, out func(string)) int {
	if verb == "status" {
		return Status(root, id, out)
	}
	if strings.TrimSpace(id) == "" {
		out("dispatch " + verb + " <wave-id> — which wave")
		return 2
	}
	if (verb == "start" || verb == "return") && strings.TrimSpace(task) == "" {
		out("dispatch " + verb + " <wave-id> --task <id> — which task")
		return 2
	}
	ws, err := Load(root)
	if err != nil {
		out("dispatch NOT read (" + err.Error() + ") — refusing to record against a ledger procoder cannot see")
		return 2
	}
	ws, msg, code := Apply(ws, verb, id, task, now)
	if code == 0 {
		if serr := Save(root, ws); serr != nil {
			out("dispatch NOT recorded (" + serr.Error() + ")")
			return 2
		}
	}
	out(msg)
	return code
}
