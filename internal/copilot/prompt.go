package copilot

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// CanAsk reports whether there is a human at the other end to answer. It is
// exported because the caller must say WHICH silence it hit — a command that
// exits non-zero with no output at all reads as a crash — and duplicating the
// character-device test there would let the two drift apart.
func CanAsk(in *os.File) bool {
	if in == nil {
		return false
	}
	info, err := in.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	// /dev/null is a character device too, so the mode test alone answers
	// "there is a terminal" for the most common way of saying there is not:
	// `procoder version --check < /dev/null` printed a prompt and read the
	// EOF as a no. The outcome was safe; the reasoning was wrong, and the
	// next caller to trust CanAsk would not be as lucky.
	null, err := os.Stat(os.DevNull)
	if err != nil {
		return true // cannot rule it out; a char device is the best answer left
	}
	return !os.SameFile(info, null)
}

// Prompt asks once whether to publish and record the findings, and answers no
// unless a human typed yes.
//
// It answers no without asking when in is not a character device — a pipe, a
// redirect, a CI runner, a hook with no terminal behind it. There is nobody
// there to consent, and a prompt read from a pipe would consume input meant
// for something else and could be answered by accident. Checking the file mode
// rather than a terminal library keeps this stdlib-only, and is exactly the
// question being asked: is there a terminal on the other end?
//
// out receives the prompt text so the caller decides where it lands; the
// answer is always read from in.
func Prompt(in *os.File, out func(string), count int, since time.Duration) bool {
	if !CanAsk(in) {
		return false // no terminal, no consent — and no question asked
	}
	out(fmt.Sprintf("copilot-leak: %d finding(s) from Copilot auto-reviews since %s.", count, since))
	out("Create anonymised issue(s) under the `auto-copilot` label and record")
	out("lesson(s)? [y/N]")

	// a read error leaves line empty, which isYes rejects — input that ended
	// before an answer is not an answer
	line, _ := bufio.NewReader(in).ReadString('\n')
	return isYes(line)
}

// ReadYes reads one line of consent from an already-checked terminal. It is
// exported for the other places that must ask before acting — the upgrade
// among them — so the definition of a yes lives in one place: anything but a
// bare y or yes is a no.
func ReadYes(in *os.File) bool {
	line, _ := bufio.NewReader(in).ReadString('\n')
	return isYes(line)
}

// isYes is the whole definition of consent: the two words a human types to
// mean yes, and nothing else. Split out because the terminal check above makes
// Prompt itself unreachable in a test, and an unread rule is an unpinned rule.
func isYes(line string) bool {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
