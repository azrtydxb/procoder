package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"procoder/internal/ask"
)

// blockedFile remembers the message this hook last refused to end the turn
// on, so the same one cannot block twice. A block the agent cannot satisfy
// would loop the session, which is worse than the failure being prevented.
const blockedFile = "last-unasked-decision"

// Two shapes of ask, because they are recognised differently.
//
// interrogative phrases are only an ask when they are actually asked. "Do
// you want me to keep it?" is a decision; "I asked whether you want me to
// keep it, and you said yes" is narration about one already taken. The
// difference is the question mark in the same sentence, and without that
// test this fired on the second — found by a fixture taken from a real
// message in the session that built this.
var interrogative = regexp.MustCompile(`(?i)\b(` + strings.Join([]string{
	`should i\b`,
	`shall i\b`,
	`do you want\b`,
	`would you (like|prefer|rather)\b`,
	`which (one )?(would|do) you\b`,
	`want me to\b`,
	`do you want me to\b`,
}, "|") + `)`)

// deferring phrases hand the choice over outright. They are an ask with or
// without a question mark — "say the word and I'll tag it" is a decision
// put to somebody just as much as "shall I tag it?".
var deferring = regexp.MustCompile(`(?i)\b(` + strings.Join([]string{
	`let me know (if|which|whether|what)\b`,
	`your call\b`,
	`say the word\b`,
	`up to you\b`,
	`tell me (which|whether|if|what) you\b`,
}, "|") + `)`)

// sentences splits on terminators, keeping each terminator with its
// sentence so a question mark can be tested against the clause it belongs
// to rather than against the whole message.
var sentences = regexp.MustCompile(`[^.!?\n]*[.!?\n]|[^.!?\n]+$`)

// decisionInProse reports whether a message puts a decision to the reader
// rather than reporting work.
//
// The tail, not the whole message. A decision buried at the end of a long
// report is the failure this exists for, and an ask-shaped phrase in the
// middle of narration is usually prose about a decision already taken.
//
// The asymmetry is the whole design. A missed burial costs one prose
// question, which is what happens today anyway. A false block fires on
// ordinary reports at the end of every single turn, and a check that
// interrupts correct work is one people route around — the failure behind
// #172 and #185. So this errs toward silence.
func decisionInProse(message string) bool {
	lines := strings.Split(strings.TrimRight(message, "\n"), "\n")
	from := 0
	if len(lines) > 12 {
		from = len(lines) - 12
	}
	tail := strings.Join(lines[from:], "\n")
	for _, s := range sentences.FindAllString(tail, -1) {
		if deferring.MatchString(s) {
			return true
		}
		if interrogative.MatchString(s) && strings.Contains(s, "?") {
			return true
		}
	}
	return false
}

// unaskedDecision reports whether the turn is ending with a decision put
// to the reader in prose and none recorded, and the reason to say so.
//
// Every uncertain path returns false. procoder not being able to tell
// whether a decision was recorded is not grounds for refusing to let a
// session end — that would be a block nobody can act on, which is the
// thing this is meant to prevent, not cause.
func unaskedDecision(root, message string) (bool, string) {
	if strings.TrimSpace(message) == "" {
		return false, "" // an absent field is not evidence of anything
	}
	if !decisionInProse(message) {
		return false, ""
	}
	pending, notes, err := ask.PendingDecisions(root)
	if err != nil || len(notes) > 0 {
		// A note means the decisions file exists and could not be read, or
		// holds something in a shape procoder does not recognise. Whether a
		// decision was recorded is then UNKNOWN, and blocking a turn on not
		// knowing is a block nobody can act on — the thing this hook exists
		// to prevent, not to cause. Raised in review on #215, where the
		// notes were being discarded and unknown read as none.
		return false, ""
	}
	// A recorded decision silences this hook only when it was recorded THIS
	// TURN. `len(pending) > 0` used to be the whole test, and it meant the
	// hook switched itself off permanently: the first decision written to
	// the file satisfied it, and every later turn that buried a DIFFERENT
	// decision in prose went unchallenged. Measured on this repository
	// mid-session — six pending, the hook silent since the first one.
	//
	// An enforcement that goes quiet exactly when decisions are piling up
	// unanswered has it backwards. What "the agent already recorded one"
	// really means is that the file CHANGED while this turn ran.
	if len(pending) > 0 && decisionsChanged(root) {
		return false, ""
	}
	if alreadyBlocked(root, message) {
		return false, ""
	}
	// Only block if the guard can be recorded. A block this hook cannot
	// remember is one it will repeat on the next stop, and a session that
	// cannot end is worse than a decision that went unasked.
	if !rememberBlocked(root, message) {
		return false, ""
	}
	return true, "This turn ends with a decision put to the user in prose, and " +
		"`.procoder/ask/decisions.md` records none.\n\n" +
		"A question at the end of a report has not been asked — it has been " +
		"mentioned. The user has to notice it, scroll back, and reconstruct " +
		"what is being decided.\n\n" +
		"Write the decision to `.procoder/ask/decisions.md` — one `## ` heading, " +
		"its options as a list beneath — then put it to the user with the host's " +
		"structured question tool, on its own, before continuing.\n\n" +
		"If what you wrote is not a decision, record it anyway or reword it: this " +
		"fires on an explicit ask, and it will not fire twice on the same message."
}

// decisionsFile is the digest of .procoder/ask/decisions.md as of the last
// stop, so the next one can tell a decision recorded during this turn from
// a backlog that was already there.
const decisionsFile = "last-decisions-digest"

// decisionsChanged reports whether the decisions file differs from what it
// held at the previous stop, and records the new digest either way.
//
// Unknown is not "changed": a digest that cannot be read or written leaves
// the hook unable to tell a fresh decision from an old one, and the safe
// direction there is to let the turn end. A hook that blocks on its own
// broken bookkeeping is one people disable.
func decisionsChanged(root string) bool {
	path := filepath.Join(root, filepath.FromSlash(ask.Dir), ask.DecisionsFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		return true // no file, or unreadable: not evidence of a backlog
	}
	sum := sha256.Sum256(raw)
	now := hex.EncodeToString(sum[:])[:16]

	digestPath := filepath.Join(root, filepath.FromSlash(StateDir), decisionsFile)
	before, readErr := os.ReadFile(digestPath)
	if os.MkdirAll(filepath.Dir(digestPath), 0o755) == nil {
		_ = os.WriteFile(digestPath, []byte(now), 0o644)
	}
	if readErr != nil {
		// First stop in this tree: nothing to compare against, so the
		// question "was this recorded just now" has no answer yet.
		return true
	}
	return strings.TrimSpace(string(before)) != now
}

func blockedPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(StateDir), blockedFile)
}

func fingerprint(message string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(message)))
	return hex.EncodeToString(sum[:])[:16]
}

func alreadyBlocked(root, message string) bool {
	raw, err := os.ReadFile(blockedPath(root))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(raw)) == fingerprint(message)
}

// rememberBlocked records the fingerprint and reports whether it stuck.
// The caller blocks only when it did — see unaskedDecision.
func rememberBlocked(root, message string) bool {
	dir := filepath.Dir(blockedPath(root))
	if os.MkdirAll(dir, 0o755) != nil {
		return false
	}
	return os.WriteFile(blockedPath(root), []byte(fingerprint(message)), 0o644) == nil
}
