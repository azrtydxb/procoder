package copilot

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

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
	if in == nil {
		return false
	}
	info, err := in.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false // no terminal, no consent — silent pass
	}
	out(fmt.Sprintf("copilot-leak: %d finding(s) from Copilot auto-reviews since %s.", count, since))
	out("Create anonymised issue(s) under the `auto-copilot` label and record")
	out("lesson(s)? [y/N]")

	// a read error leaves line empty, which isYes rejects — input that ended
	// before an answer is not an answer
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
