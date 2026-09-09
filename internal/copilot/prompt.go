package copilot

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CanAsk reports whether there is a human at the other end to answer. It is
// exported because the caller must say WHICH silence it hit — a command that
// exits non-zero with no output at all reads as a crash — and duplicating the
// character-device test there would let the two drift apart.
func CanAsk(in *os.File) bool { return CanAskWith(in, nil) }

// CanAskWith is CanAsk plus an answer the caller already has.
//
// A request over the socket carries no terminal and never will, so the
// character-device test below is the right answer to the wrong question
// there: what a caller needs to say is not "is there a terminal" but "is
// there an answer". A supplied answer is one, whatever it says — a
// supplied "no" is a person declining, which is a different thing from
// nobody being there, and both are different from a terminal.
//
// nil supplied means no person, which is exactly the non-interactive path
// a command takes today when nothing is attached to its stdin.
func CanAskWith(in *os.File, supplied *string) bool {
	if supplied != nil {
		return true
	}
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
	if null, err := os.Stat(os.DevNull); err == nil {
		return !os.SameFile(info, null)
	}
	// Windows: os.Stat("NUL") fails, so SameFile cannot answer. The name is
	// the only thing left to go on, and it is a reserved device name there —
	// no real file can be called it.
	return !strings.EqualFold(filepath.Base(in.Name()), os.DevNull)
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
	return PromptWith(in, nil, out, count, since)
}

// PromptWith is Prompt with an answer the caller already has. A supplied
// answer is used verbatim and nothing is printed: the question was already
// put, wherever the answer came from, and asking it again into a socket
// would be writing a prompt nobody will read.
func PromptWith(in *os.File, supplied *string, out func(string), count int, since time.Duration) bool {
	if supplied != nil {
		return isYes(*supplied)
	}
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
func ReadYes(in *os.File) bool { return ReadYesFrom(in, nil) }

// ReadYesFrom is ReadYes with an answer the caller already has. The
// definition of a yes stays in one place either way.
func ReadYesFrom(in *os.File, supplied *string) bool {
	if supplied != nil {
		return isYes(*supplied)
	}
	if in == nil {
		return false
	}
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
