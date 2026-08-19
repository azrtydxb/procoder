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
	for sc.Scan() {
		line++
		m := re.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		text := strings.TrimSpace(m[1])
		text = strings.TrimSpace(strings.TrimSuffix(text, "-->"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		entries = append(entries, Entry{File: rel, Line: line,
			Text: text, NoTrigger: !triggerRe.MatchString(text)})
	}
	return entries
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
	return 0
}
