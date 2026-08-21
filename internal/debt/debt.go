// Package debt harvests deliberate-simplification markers into a ledger.
// The convention: when a corner is cut on purpose with a known ceiling, the
// code carries a comment — `// debt: global lock, per-account locks when
// throughput matters` — naming the ceiling and the trigger to revisit. This
// command greps them out so "later" has a list instead of a memory. Markers
// naming no trigger are flagged: those are the ones that silently rot.
// Read-only; the binary reports, the agent (or the user) decides.
package debt

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"procoder/internal/config"
)

// markerRe requires a comment leader right before the marker so prose that
// merely mentions the convention stays out of the ledger.
func markerRe(marker string) *regexp.Regexp {
	return regexp.MustCompile(`(?:^|\s)(?://|#|--|<!--|;|/\*)\s*` + regexp.QuoteMeta(marker) + `\s*(.+)`)
}

// triggerRe: a marker has an upgrade trigger when it says when to revisit.
var triggerRe = regexp.MustCompile(`(?i)\b(when|until|if|once|upgrade|after)\b|->`)

// Entry is one harvested marker.
type Entry struct {
	File      string
	Line      int
	Text      string
	NoTrigger bool
}

// Scan walks the gate's file scope (tracked plus untracked-but-not-ignored)
// for marker comments.
func Scan(root string) ([]Entry, error) {
	cfg := config.Load(root)
	re := markerRe(cfg.DebtMarker)
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not a git repository (or git missing): %v", err)
	}
	var entries []Entry
	for _, rel := range strings.Split(string(raw), "\x00") {
		if rel == "" {
			continue
		}
		entries = append(entries, scanFile(root, rel, re)...)
	}
	return entries, nil
}

// scanFile scans one text file line by line; binaries (null byte in the
// first block) are skipped.
func scanFile(root, rel string, re *regexp.Regexp) []Entry {
	f, err := os.Open(filepath.Join(root, rel))
	if err != nil {
		return nil
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	if bytes.IndexByte(head[:n], 0) >= 0 {
		return nil
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil
	}
	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	// a marker's block is consumed while looking for its trigger, so the
	// line that ended it must still be offered to the next iteration
	pending, pendingLine := "", 0
	for {
		var cur string
		if pendingLine != 0 {
			cur, line = pending, pendingLine
			pending, pendingLine = "", 0
		} else {
			if !sc.Scan() {
				break
			}
			line++
			cur = sc.Text()
		}
		m := re.FindStringSubmatch(cur)
		if m == nil {
			continue
		}
		markerLine := line
		text := trimMarkerTail(strings.TrimSpace(m[1]))
		// The revisit condition routinely lands on a continuation line: the
		// marker line is already full of what the ceiling IS. Judging the
		// trigger on the first line alone cries rot over debt recorded
		// exactly as the principles ask, so the whole comment block counts —
		// while the ledger still SHOWS the first line, which is the summary.
		block := text
		for sc.Scan() {
			line++
			cont, ok := continuationOf(sc.Text())
			if !ok {
				pending, pendingLine = sc.Text(), line
				break
			}
			block += " " + cont
		}
		entries = append(entries, Entry{File: rel, Line: markerLine,
			Text: text, NoTrigger: !triggerRe.MatchString(block)})
	}
	return entries
}

// trimMarkerTail drops a comment terminator the marker text swallowed.
func trimMarkerTail(text string) string {
	text = strings.TrimSpace(strings.TrimSuffix(text, "-->"))
	return strings.TrimSpace(strings.TrimSuffix(text, "*/"))
}

// continuationOf returns the prose of a line that continues the marker's
// comment block, and false for anything that ends it.
func continuationOf(raw string) (string, bool) {
	t := strings.TrimSpace(raw)
	for _, p := range []string{"//", "#", "--", ";", "*"} {
		if rest, ok := strings.CutPrefix(t, p); ok {
			return trimMarkerTail(strings.TrimSpace(rest)), true
		}
	}
	return "", false
}

// Run prints the ledger.
func Run(root string, out func(string)) int {
	entries, err := Scan(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	if len(entries) == 0 {
		out("no debt markers — clean ledger")
		return 0
	}
	noTrigger := 0
	lastFile := ""
	for _, e := range entries {
		if e.File != lastFile {
			out(e.File)
			lastFile = e.File
		}
		flag := ""
		if e.NoTrigger {
			flag = "  [no-trigger]"
			noTrigger++
		}
		out(fmt.Sprintf("  %d: %s%s", e.Line, e.Text, flag))
	}
	out(fmt.Sprintf("procoder debt: %d marker(s), %d with no upgrade trigger — no-trigger debt silently rots; give each a revisit condition", len(entries), noTrigger))
	// Non-zero on rot, so the CI step that runs this can fail on it.
	// Printing the count and exiting 0 made the ledger a thing that had to
	// be read by a person who already knew to look, which is the shape of
	// every check this sprint went looking for.
	if noTrigger > 0 {
		return 1
	}
	return 0
}
