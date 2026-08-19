// Package adr is the decision memory: architecture decision records live
// as numbered Markdown files under .procoder/adr/, committed with the repo
// so the durable why survives the changelog. The binary PRINTS new records
// and verifies existing ones; it never rewrites an ADR — immutability is
// the point: supersede, never edit history (P-CONTROL).
package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Dir is where decision records live, under D-HOME.
const Dir = ".procoder/adr"

// Template is the shape of a record; `procoder adr new` prints it filled
// with the number, title, and date, and the agent writes the substance.
const Template = `# %s — %s

Status: proposed
Date: %s

## Context

<!-- What forced this decision: the constraint, the failure, the fork in
     the road. A reader six months out needs the situation, not the vibe. -->

## Decision

<!-- What we chose, and why it won over the alternatives — name them.
     A decision without its rejected options reads as arbitrary. -->

## Consequences

<!-- What gets easier and what gets harder. Every decision buys something
     and pays something; record both, or the price gets rediscovered. -->
`

var (
	numberRe = regexp.MustCompile(`^(\d{4})-`)
	statusRe = regexp.MustCompile(`(?m)^Status:\s*(\S+)`)
	dateRe   = regexp.MustCompile(`(?m)^Date:\s*(\S+)`)
	supersRe = regexp.MustCompile(`^superseded-by-(\d{4})$`)
)

// Record is one parsed decision record.
type Record struct {
	Number string // "0001"; "" when the file name carries no number
	Title  string
	Status string
	Date   string
	File   string // path relative to root, forward-slashed
	Err    error  // set when the file is unreadable
}

// Finding is one thing check refuses on.
type Finding struct {
	File    string
	Message string
}

// New prints (never writes) the next-numbered record for the agent to
// write: max existing leading number + 1, 0001 for an empty directory.
func New(root, title string, out func(string)) int {
	slug := slugify(title)
	if slug == "" {
		out("a decision record needs a title")
		return 2
	}
	dir := filepath.Join(root, Dir)
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		out("adr directory unreadable: " + err.Error())
		return 2
	}
	next := 1
	for _, e := range entries {
		if m := numberRe.FindStringSubmatch(e.Name()); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n >= next {
				next = n + 1
			}
		}
	}
	num := fmt.Sprintf("%04d", next)
	rel := filepath.ToSlash(filepath.Join(Dir, num+"-"+slug+".md"))
	out("== write this to " + rel + ":")
	out(fmt.Sprintf(Template, num, title, time.Now().UTC().Format("2006-01-02")))
	out("then fill Context, Decision, and Consequences — and never edit it later: supersede instead.")
	return 0
}

// List prints every record — number, title, status, date — proposed
// first, then by number. An empty directory says how to start.
func List(root string, out func(string)) int {
	records, err := load(root)
	if err != nil {
		out("adr directory unreadable: " + err.Error())
		return 1
	}
	if len(records) == 0 {
		out("no decision records — `procoder adr new <title>` starts one")
		return 0
	}
	sort.Slice(records, func(i, j int) bool {
		pi, pj := records[i].Status == "proposed", records[j].Status == "proposed"
		if pi != pj {
			return pi
		}
		if records[i].Number != records[j].Number {
			return records[i].Number < records[j].Number
		}
		return records[i].File < records[j].File
	})
	for _, r := range records {
		if r.Err != nil {
			out(r.File + "  unreadable: " + r.Err.Error())
			continue
		}
		out(fmt.Sprintf("%s  %s  [%s]  %s", r.Number, r.Title, r.Status, r.Date))
	}
	return 0
}

// Check is the controller: it verifies every record and returns (and
// prints) the findings — empty required sections, unknown statuses,
// dangling supersede references, duplicated numbers, unreadable files.
// The count of records still proposed is informational, not a finding:
// deciding takes a human. No records passes clean.
func Check(root string, out func(string)) []Finding {
	records, err := load(root)
	if err != nil {
		f := Finding{File: Dir, Message: "directory unreadable: " + err.Error()}
		out(f.File + ": " + f.Message)
		return []Finding{f}
	}
	if len(records) == 0 {
		out("no decision records")
		return nil
	}
	var findings []Finding
	numbers := map[string][]string{}
	for _, r := range records {
		if r.Number != "" {
			numbers[r.Number] = append(numbers[r.Number], r.File)
		}
	}
	proposed := 0
	for _, r := range records {
		if r.Err != nil {
			findings = append(findings, Finding{File: r.File, Message: "unreadable: " + r.Err.Error()})
			continue
		}
		if r.Status == "proposed" {
			proposed++
		}
		switch {
		case r.Status == "proposed", r.Status == "accepted":
		case supersRe.MatchString(r.Status):
			target := supersRe.FindStringSubmatch(r.Status)[1]
			if len(numbers[target]) == 0 {
				findings = append(findings, Finding{File: r.File,
					Message: "superseded by " + target + ", but no record carries that number"})
			}
		default:
			findings = append(findings, Finding{File: r.File,
				Message: "status " + strconv.Quote(r.Status) + " — proposed, accepted, or superseded-by-NNNN"})
		}
		for _, name := range []string{"Context", "Decision", "Consequences"} {
			if strings.TrimSpace(stripComments(section(r.text, name))) == "" {
				findings = append(findings, Finding{File: r.File,
					Message: name + " is empty — a record without it decides nothing"})
			}
		}
	}
	for _, num := range sortedKeys(numbers) {
		if files := numbers[num]; len(files) > 1 {
			findings = append(findings, Finding{File: strings.Join(files, ", "),
				Message: "share number " + num + " — numbering is identity; renumber one"})
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Message < findings[j].Message
	})
	for _, f := range findings {
		out(f.File + ": " + f.Message)
	}
	if proposed > 0 {
		out(fmt.Sprintf("%d record(s) still proposed — deciding takes a human", proposed))
	}
	return findings
}

// Run is the shared exit path for the CLI and the audit sweep: it prints
// the findings via Check and returns 1 when any, 0 when clean.
func Run(root string, out func(string)) int {
	if len(Check(root, out)) > 0 {
		return 1
	}
	return 0
}

// record extends Record with the raw text check needs; the parsed body
// stays package-private so the public type carries only facts.
type parsed struct {
	Record
	text string
}

// load reads every record file. A missing directory is an empty set; an
// unreadable directory is an error; an unreadable file is a record with
// Err set — never silently skipped.
func load(root string) ([]parsed, error) {
	dir := filepath.Join(root, Dir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []parsed
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		r := parsed{Record: Record{File: filepath.ToSlash(filepath.Join(Dir, e.Name()))}}
		if m := numberRe.FindStringSubmatch(e.Name()); m != nil {
			r.Number = m[1]
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			r.Err = err
			records = append(records, r)
			continue
		}
		r.text = string(raw)
		if m := statusRe.FindStringSubmatch(r.text); m != nil {
			r.Status = m[1]
		}
		if m := dateRe.FindStringSubmatch(r.text); m != nil {
			r.Date = m[1]
		}
		if i := strings.Index(r.text, "# "); i >= 0 {
			line := r.text[i+2:]
			if j := strings.IndexByte(line, '\n'); j > 0 {
				line = line[:j]
			}
			r.Title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), r.Number+" — "))
		}
		records = append(records, r)
	}
	return records, nil
}

// section returns the body between "## name" and the next "## ".
func section(text, name string) string {
	i := strings.Index(text, "## "+name)
	if i < 0 {
		return ""
	}
	body := text[i+len("## "+name):]
	if j := strings.Index(body, "\n## "); j >= 0 {
		body = body[:j]
	}
	return body
}

func stripComments(s string) string {
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+j+3:]
	}
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func slugify(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	// Windows caps full paths at 260 characters and CI checkouts sit deep
	// in D:\a\...; a slug born from a whole sentence (a seeded acceptance
	// criterion) must not push a file past that. Cut at a word boundary.
	const maxSlug = 60
	if len(slug) > maxSlug {
		cut := slug[:maxSlug]
		if i := strings.LastIndexByte(cut, '-'); i > 40 {
			cut = cut[:i]
		}
		slug = strings.Trim(cut, "-")
	}
	return slug
}
