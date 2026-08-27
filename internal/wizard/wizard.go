// Package wizard walks a human through a procedure procoder cannot perform
// for them: create an account, generate a token, click submit in somebody
// else's dashboard. Those steps are not shell commands and never were, so
// this package executes nothing at all — it displays steps and, under
// `run`, advances through them one at a time so none is skipped.
//
// A wizard is `.procoder/wizards/<name>.md`, written by the agent. procoder
// reads it; `wizard new` PRINTS a scaffold rather than writing one, the
// same P-CONTROL every other authoring command follows.
//
// A captured value is validated and never echoed. Wizards exist to walk
// somebody through generating credentials, and a token printed back to the
// terminal is a token in the scrollback.
package wizard

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Dir is where wizards live, relative to the repository root.
const Dir = ".procoder/wizards"

// Step is one thing a human does, in order.
type Step struct {
	Title string
	Body  []string
	// Capture is the name of a value this step asks back, empty when the
	// step only needs a confirmation.
	Capture string
	// Shape is the pattern Capture must match. Empty means any non-empty
	// value is accepted — a shape nobody wrote is not a shape that failed.
	Shape string
}

// captureRe reads `Capture: NAME matching <pattern>` or `Capture: NAME`.
var captureRe = regexp.MustCompile(`^Capture:\s*(\w+)(?:\s+matching\s+(.+))?$`)

// Parse reads a wizard file into ordered steps. A `## ` heading starts a
// step; everything under it is that step's body.
func Parse(text string) []Step {
	var steps []Step
	for _, line := range strings.Split(normaliseEOL(text), "\n") {
		if strings.HasPrefix(line, "## ") {
			steps = append(steps, Step{Title: strings.TrimSpace(line[3:])})
			continue
		}
		if len(steps) == 0 {
			continue // preamble before the first step is the title block
		}
		cur := &steps[len(steps)-1]
		if m := captureRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			cur.Capture, cur.Shape = m[1], strings.TrimSpace(m[2])
			continue
		}
		cur.Body = append(cur.Body, line)
	}
	return steps
}

func normaliseEOL(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// read returns a wizard's text, or a message naming why it could not.
func read(root, name string) (string, string) {
	p := filepath.Join(root, Dir, name+".md")
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "no wizard named " + name + " — expected " + filepath.ToSlash(filepath.Join(Dir, name+".md"))
		}
		// Unknown is never none: an unreadable wizard is a named failure.
		return "", filepath.ToSlash(filepath.Join(Dir, name+".md")) + " cannot be read (" + err.Error() + ")"
	}
	return string(raw), ""
}

// List names the wizards this repository carries.
func List(root string, out func(string)) int {
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if err != nil {
		if os.IsNotExist(err) {
			out("no wizards — `procoder wizard new <name>` prints one to write")
			return 0
		}
		out(Dir + " cannot be read (" + err.Error() + ") — the wizards are NOT listed")
		return 1
	}
	var names []string
	for _, e := range entries {
		if n := strings.TrimSuffix(e.Name(), ".md"); !e.IsDir() && n != e.Name() {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		out("no wizards — `procoder wizard new <name>` prints one to write")
		return 0
	}
	sort.Strings(names)
	for _, n := range names {
		text, _ := read(root, n)
		out("  " + n + " — " + strconv.Itoa(len(Parse(text))) + " step(s)")
	}
	return 0
}

// Show prints every step without prompting. This is the default shape:
// display it, and let a human decide to walk it.
func Show(root, name string, out func(string)) int {
	text, bad := read(root, name)
	if bad != "" {
		out(bad)
		return 1
	}
	steps := Parse(text)
	if len(steps) == 0 {
		out(name + " has no `## ` step headings — nothing to walk")
		return 1
	}
	out(name + " — " + strconv.Itoa(len(steps)) + " step(s), nothing runs:")
	for i, s := range steps {
		out("")
		out("  " + strconv.Itoa(i+1) + ". " + s.Title)
		for _, b := range s.Body {
			if strings.TrimSpace(b) != "" {
				out("     " + strings.TrimSpace(b))
			}
		}
		if s.Capture != "" {
			out("     asks for: " + s.Capture + shapeSuffix(s.Shape))
		}
	}
	return 0
}

func shapeSuffix(shape string) string {
	if shape == "" {
		return ""
	}
	return " (matching " + shape + ")"
}

// Run walks the steps one at a time, advancing only on a confirmation, so
// a step cannot be skipped by reading past it. It executes nothing.
//
// A captured value is checked against its shape and never echoed: what the
// human is told is that it was accepted, which is all they need to know.
func Run(root, name string, in io.Reader, out func(string)) int {
	text, bad := read(root, name)
	if bad != "" {
		out(bad)
		return 1
	}
	steps := Parse(text)
	if len(steps) == 0 {
		out(name + " has no `## ` step headings — nothing to walk")
		return 1
	}
	r := bufio.NewScanner(in)
	captured := make([]string, 0, len(steps))
	out(name + " — " + strconv.Itoa(len(steps)) + " step(s). procoder runs none of them; you do.")
	for i, s := range steps {
		out("")
		out("step " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(steps)) + ": " + s.Title)
		for _, b := range s.Body {
			if strings.TrimSpace(b) != "" {
				out("  " + strings.TrimSpace(b))
			}
		}
		if s.Capture == "" {
			out("  press enter when this is done (or type 'stop')")
			if !r.Scan() {
				out("stopped at step " + strconv.Itoa(i+1) + " — input ended before the wizard did")
				return 1
			}
			if strings.EqualFold(strings.TrimSpace(r.Text()), "stop") {
				out("stopped at step " + strconv.Itoa(i+1) + " of " + strconv.Itoa(len(steps)))
				return 1
			}
			continue
		}
		ok, code := capture(r, s, out)
		if !ok {
			return code
		}
		captured = append(captured, s.Capture)
	}
	out("")
	out("every step confirmed (" + strconv.Itoa(len(steps)) + "/" + strconv.Itoa(len(steps)) + ")")
	if len(captured) > 0 {
		// Named, never valued. The point of capturing was to check the
		// shape while the human still had the value in front of them.
		out("values checked and NOT stored or printed: " + strings.Join(captured, ", "))
	}
	return 0
}

// capture prompts for one value and validates its shape. It returns false
// with the exit code when the wizard should stop.
func capture(r *bufio.Scanner, s Step, out func(string)) (bool, int) {
	var re *regexp.Regexp
	if s.Shape != "" {
		// Compile is deliberate over MustCompile: the pattern comes from a
		// file somebody wrote, and a bad pattern is a finding, not a panic.
		c, err := regexp.Compile(s.Shape)
		if err != nil {
			out("  the shape for " + s.Capture + " is not a valid pattern (" + err.Error() + ") — NOT checked")
			return false, 1
		}
		re = c
	}
	for {
		out("  enter " + s.Capture + shapeSuffix(s.Shape) + " (or 'stop'); it is not echoed or stored")
		if !r.Scan() {
			out("stopped — input ended before " + s.Capture + " was given")
			return false, 1
		}
		v := strings.TrimSpace(r.Text())
		if strings.EqualFold(v, "stop") {
			out("stopped before " + s.Capture)
			return false, 1
		}
		if v == "" {
			out("  " + s.Capture + " cannot be empty")
			continue
		}
		if re != nil && !re.MatchString(v) {
			// The value is never quoted back, not even when it is wrong.
			out("  that does not match " + s.Shape + " — check it and enter it again")
			continue
		}
		out("  " + s.Capture + " accepted")
		return true, 0
	}
}

// Scaffold PRINTS a wizard to write. It does not create the file: the
// binary prints and the agent writes (P-CONTROL).
func Scaffold(name string, out func(string)) int {
	if strings.TrimSpace(name) == "" {
		out("wizard new <name> — the name becomes " + Dir + "/<name>.md")
		return 2
	}
	out("write this to " + filepath.ToSlash(filepath.Join(Dir, name+".md")) + ":")
	out("")
	out("# " + name)
	out("")
	out("One sentence: what a person has when this is finished.")
	out("")
	out("## Create the account")
	out("")
	out("Go to https://example.com/signup and register. Procoder cannot do")
	out("this for you: it needs an email nobody else can read.")
	out("")
	out("## Generate a token")
	out("")
	out("Settings -> Developer -> New token. Give it the smallest scope that")
	out("works.")
	out("")
	out("Capture: TOKEN matching ^[A-Za-z0-9_-]{20,}$")
	out("")
	out("## Paste it where it belongs")
	out("")
	out("Put the token in your password manager, then into the CI secret.")
	out("Procoder never sees it.")
	return 0
}
