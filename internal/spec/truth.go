package spec

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// This file is the half of spec checking that asks whether a document is
// TRUE, rather than whether it is complete.
//
// Everything else in the chain is structural: sections present, questions
// closed, [S-n] ids covered. All of it can be satisfied by a document that
// asserts things the code does not do and promises criteria no fixture can
// produce — and sprint 021 was exactly that, five times over, in a spec
// that was long and careful (#186).
//
// Nothing here judges prose. "Is this sentence true" is not mechanical,
// and a checker that guessed would lie more confidently than the gap it
// replaced. What is mechanical is narrower and still catches most of it:
// a claim that CITES something can have the citation resolved, and a
// criterion can be required to name how it is observed.

// citationRe matches a backticked token that looks like a Go symbol
// (pkg.Symbol) or a repository path (internal/x/y.go).
//
// Deliberately narrow. A tool name, a command, a config key and an English
// word in backticks all stay out of it, because a false refusal here
// teaches people to route around the checker — the failure this project
// keeps relearning (#172, #185).
var citationRe = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*\\.[A-Za-z_][A-Za-z0-9_]*|(?:internal|cmd|docs)/[A-Za-z0-9_./-]+\\.(?:go|md))`")

// fenceRe finds fenced code blocks, which are excluded: a spec showing
// example output or a symbol that deliberately does not exist is not
// making a claim about this repository.
var fenceRe = regexp.MustCompile("(?s)```.*?```")

// claimSections are the parts of a spec that assert things about the
// world. Edge cases and Failure modes describe what WOULD happen and
// routinely name things hypothetically, so they are left alone.
var claimSections = []string{"In scope", "Constraints", "Interfaces", "Decisions"}

// commandRe matches a cited procoder command: `procoder spec check`,
// `procoder prune --apply`.
//
// Known limitation, stated rather than discovered later: only the
// top-level command is resolved. `procoder backlog check` passes because
// `backlog` exists, though `check` is not one of its subcommands — and
// that exact citation was in this checker's own spec. Resolving
// subcommands needs a table of them, which is a second thing to keep in
// step with the code and a new way for a true citation to read as false.
var commandRe = regexp.MustCompile("`procoder ([a-z-]+)")

// Citation is one reference a spec makes to something in the repository.
type Citation struct {
	Text string
	Line int
}

// UnresolvedCitations returns the citations a document makes that do not
// exist in the tree.
//
// This is the mechanical stand-in for "is this claim true". A spec that
// says the gate runs a thing, and names the thing, can be checked; one
// that says it without naming anything cannot, and requiring the name is
// what turns the second into the first.
func UnresolvedCitations(root, text string) []Citation {
	symbols := repoSymbols(root)
	var out []Citation
	for _, section := range claimSections {
		body := sectionOf(text, section)
		if body == "" {
			continue
		}
		body = fenceRe.ReplaceAllString(body, "")
		for i, line := range strings.Split(body, "\n") {
			for _, m := range citationRe.FindAllStringSubmatch(line, -1) {
				cite := m[1]
				if resolves(root, cite, symbols) {
					continue
				}
				out = append(out, Citation{Text: cite, Line: lineOf(text, section, i)})
			}
			for _, m := range commandRe.FindAllStringSubmatch(line, -1) {
				if Commands[m[1]] {
					continue
				}
				out = append(out, Citation{Text: "procoder " + m[1], Line: lineOf(text, section, i)})
			}
		}
	}
	return out
}

// fileExtensions are the suffixes that make a citation a FILE rather than
// a symbol. Without this, `AGENTS.md` parses as the pkg.Symbol shape, the
// symbol `md` is looked up, nothing has it, and a spec citing a file that
// sits in the repository root is refused — the false refusal this check is
// most likely to produce, found by running it against a real spec.
var fileExtensions = map[string]bool{
	"md": true, "go": true, "toml": true, "json": true,
	"yaml": true, "yml": true, "sh": true, "txt": true, "mdc": true,
}

func resolves(root, cite string, symbols map[string]bool) bool {
	if _, ext, ok := strings.Cut(cite, "."); ok && fileExtensions[ext] {
		return fileExists(root, cite)
	}
	if strings.Contains(cite, "/") {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(cite)))
		return err == nil
	}
	// pkg.Symbol — the package half is not verified, because a spec may
	// name a package by a shorter alias than its import path. What is
	// checked is that a symbol by that name exists somewhere in the tree,
	// which is what catches a citation to something never written.
	_, sym, _ := strings.Cut(cite, ".")
	return symbols[sym]
}

// fileExists looks for a cited file at the repository root and, failing
// that, anywhere in the tree — a spec naming `AGENTS.md` or `SKILL.md`
// means the file, wherever it happens to live.
func fileExists(root, cite string) bool {
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(cite))); err == nil {
		return true
	}
	base := filepath.Base(filepath.FromSlash(cite))
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || found {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules", "dist", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == base {
			found = true
		}
		return nil
	})
	return found
}

// repoSymbols is every top-level Go identifier declared in the tree.
//
// Parsed rather than grepped: a grep for the name would match it in a
// comment or a string, and a citation that resolves only to prose about
// itself is exactly the false confidence this is meant to remove.
func repoSymbols(root string) map[string]bool {
	out := map[string]bool{}
	fset := token.NewFileSet()
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "node_modules" || base == "dist" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil || f == nil {
			return nil
		}
		// Declarations, not f.Scope: SkipObjectResolution leaves Scope nil,
		// and walking the decls is the more direct question anyway — what
		// this file declares at the top level.
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name != nil {
					out[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spc := range d.Specs {
					switch v := spc.(type) {
					case *ast.TypeSpec:
						if v.Name != nil {
							out[v.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, n := range v.Names {
							out[n.Name] = true
						}
					}
				}
			}
		}
		return nil
	})
	return out
}

// observableRe recognises a criterion that says how it is observed: a
// procoder command, a Go test name, or a named file.
var observableRe = regexp.MustCompile("`[^`]*procoder [a-z]|`Test[A-Za-z0-9_]+|`[A-Za-z0-9_./-]+\\.(?:go|md|toml|json|ya?ml)`|\\bexits? [0-9]|\\bexit code\\b")

// prerequisite is a domain whose findings need setup before they can
// appear at all, and the words that name that setup.
//
// This is the table that catches the sprint-021 criteria which were green
// whatever the code did: the docs domain prints "public surface NOT
// computed" without an index and never reaches its finding, so a criterion
// about a documentation obligation on an unindexed fixture cannot fail.
type prerequisite struct {
	domain  string   // what the criterion is about
	needs   string   // what a person must do about it
	trigger []string // words that mean the criterion is about that domain
	names   []string // words that show the criterion accounted for it
}

var prerequisites = []prerequisite{
	{
		domain: "the documentation domain", needs: "a built index (`procoder index build`) — without one the docs domain reports `public surface NOT computed` and never reaches a finding, so the criterion passes whatever the code does",
		trigger: []string{"documentation obligation", "docs domain", "documentation gap", "no documentation file"},
		names:   []string{"index build", "built index", "indexed", "index"},
	},
	{
		domain: "the suite leg", needs: "a fixture that actually has tests — with no suite the leg reports nothing and the criterion cannot fail",
		trigger: []string{"failing suite", "the suite reports", "suite verdict", "failing test suite"},
		names:   []string{"test suite", "a suite", "tests exist", "go test", "pytest"},
	},
	{
		domain: "the dependency scan", needs: "a lockfile in the fixture — osv-scanner has nothing to read without one",
		trigger: []string{"vulnerable dependency", "dependency scan", "vulnerable dependencies"},
		names:   []string{"lockfile", "package-lock", "go.sum", "pnpm-lock"},
	},
}

// CriterionFault is one acceptance criterion that cannot be checked as
// written, and why.
type CriterionFault struct {
	Text string
	Line int
	Why  string
}

// UncheckableCriteria returns the acceptance criteria that name no
// observable, or that name one their own fixture cannot produce.
//
// A criterion nobody can run is an agreement, not a test. It passes
// review, becomes a story, gets ticked, and the thing it promised was
// never checked by anything.
func UncheckableCriteria(text string) []CriterionFault {
	body := sectionOf(text, "Acceptance criteria")
	if body == "" {
		return nil
	}
	var out []CriterionFault
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "- [") {
			continue
		}
		// A criterion wraps. Reading only its first line was this
		// checker's own first bug: it refused three criteria in its own
		// spec whose observable sat on the continuation line, which is
		// where a wrapped sentence usually puts it.
		criterion := joinWrapped(lines, i)
		lower := strings.ToLower(criterion)
		if !observableRe.MatchString(criterion) {
			out = append(out, CriterionFault{Text: criterion, Line: lineOf(text, "Acceptance criteria", i),
				Why: "names no observable — say the command that runs it (`procoder check`), the test that asserts it (`TestSomething`), or the file it inspects; a criterion nobody can run is an agreement, not a test"})
			continue
		}
		for _, p := range prerequisites {
			if !containsAny(lower, p.trigger) || containsAny(lower, p.names) {
				continue
			}
			out = append(out, CriterionFault{Text: criterion, Line: lineOf(text, "Acceptance criteria", i),
				Why: "is about " + p.domain + " but does not name " + p.needs})
			break
		}
	}
	return out
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// sectionOf returns a `## `-delimited section's body.
//
// Line endings are normalised first. A Windows checkout has CRLF, the
// marker is then "## Name\r\n", the lookup for "## Name\n" finds nothing,
// and every check above reports a clean document — a silent green that
// passed on three platforms and failed on the fourth. Found by CI, which
// is what CI is for.
func sectionOf(text, name string) string {
	text = normaliseEOL(text)
	marker := "## " + name + "\n"
	i := strings.Index(text, marker)
	if i < 0 {
		return ""
	}
	rest := text[i+len(marker):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		return rest[:j]
	}
	return rest
}

// lineOf maps an offset within a section back to a line number in the
// whole document, so a refusal can point at the line rather than at the
// section.
func lineOf(text, section string, withinSection int) int {
	text = normaliseEOL(text)
	marker := "## " + section + "\n"
	i := strings.Index(text, marker)
	if i < 0 {
		return 0
	}
	return strings.Count(text[:i+len(marker)], "\n") + withinSection + 1
}

// TruthGaps is both checks, rendered as the chain's other controllers
// render theirs: each one names what to do about it, because a refusal
// without a route is a gate people learn to route around.
func TruthGaps(root, text string) []string {
	var gaps []string
	for _, c := range UnresolvedCitations(root, text) {
		gaps = append(gaps, fmt.Sprintf("line %d cites `%s`, which is not in this repository — cite something that exists, or drop the citation and say it in prose; a claim that names nothing cannot be checked by anyone",
			c.Line, c.Text))
	}
	for _, f := range UncheckableCriteria(text) {
		gaps = append(gaps, fmt.Sprintf("line %d: the criterion %s", f.Line, f.Why))
	}
	for _, f := range UncitedClaims(text) {
		gaps = append(gaps, fmt.Sprintf("line %d: the promise %s", f.Line, f.Why))
	}
	for _, f := range CriteriaWithoutFalsifiers(text) {
		gaps = append(gaps, fmt.Sprintf("line %d: the criterion %s", f.Line, f.Why))
	}
	for _, f := range WeakOracles(text) {
		gaps = append(gaps, fmt.Sprintf("line %d: the criterion %s", f.Line, f.Why))
	}
	return gaps
}

// joinWrapped returns a bullet and its continuation lines as one string.
// A continuation is an indented line that does not itself start a bullet;
// a blank line or the next bullet ends it.
func joinWrapped(lines []string, start int) string {
	parts := []string{strings.TrimSpace(lines[start])}
	for _, next := range lines[start+1:] {
		if strings.TrimSpace(next) == "" {
			break
		}
		if !strings.HasPrefix(next, " ") && !strings.HasPrefix(next, "\t") {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(next), "- [") {
			break
		}
		parts = append(parts, strings.TrimSpace(next))
	}
	return strings.Join(parts, " ")
}

// domainWords name a procoder domain. A claim that a domain does or does
// not do something is a claim about code that exists, and the author can
// check it — but only if they are made to look.
//
// This is the mechanical form of the deviation that cost sprint 021 most.
// Its S-3 listed nine domains, formatting among them, and cited nothing.
// Nobody looked at where the code decides formatting, so nobody noticed
// the format loop ran BEFORE the scope decision; honouring that one word
// meant restructuring `RunWith` and repairing four fixtures, discovered
// mid-sprint. Requiring the citation does not verify the claim — it puts
// the author in the file, which is where the discovery happens.
var domainWords = []string{
	"formatting", "linting", "the linter", "documentation", "docs domain",
	"the planning chain", "planning", "the agent layer", "agents drift",
	"maintainability", "complexity", "debt", "the suite", "the test suite",
	"templates", "release hygiene", "secrets", "the secret scan",
	"conflict markers", "junk files", "oversized files",
}

// UncitedClaims returns the In-scope promises that name a domain and cite
// nothing, so nobody had to look at the code to write them.
func UncitedClaims(text string) []CriterionFault {
	body := sectionOf(text, "In scope")
	if body == "" {
		return nil
	}
	body = fenceRe.ReplaceAllString(body, "")
	var out []CriterionFault
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "- [S-") {
			continue
		}
		bullet := joinWrapped(lines, i)
		named := domainsNamedIn(strings.ToLower(bullet))
		if len(named) == 0 {
			continue
		}
		if citationRe.MatchString(bullet) || commandRe.MatchString(bullet) {
			continue
		}
		out = append(out, CriterionFault{Text: bullet, Line: lineOf(text, "In scope", i),
			Why: fmt.Sprintf("names %s and cites nothing — cite where that lives (`pkg.Symbol`, a path, or the command), so the claim is one somebody had to open the file to write",
				strings.Join(named, ", "))})
	}
	return out
}

func domainsNamedIn(lower string) []string {
	var found []string
	for _, d := range domainWords {
		if strings.Contains(lower, d) {
			found = append(found, d)
		}
	}
	if len(found) > 3 {
		found = found[:3]
	}
	return found
}

// falsifierRe recognises a criterion that says what would make it fail.
//
// This is the project's own mutation discipline, applied to the criterion
// rather than to the test. A criterion nobody has asked "what would break
// this?" about is one that may not be able to fail at all, and two of
// sprint 021's five deviations were exactly that: a criterion about
// narrowing junk findings to the diff, describing a failure that cannot
// happen because those findings carry no line number; and one about a
// typo falling back to detection, on a fixture where an accepted typo and
// a correct fallthrough are indistinguishable because `parseScope`
// returns Adopted as its zero value.
//
// Writing the falsifier is what surfaces both. You cannot state what
// change would make the criterion fail without constructing the case that
// distinguishes pass from fail — and when you cannot, that is the answer.
var falsifierRe = regexp.MustCompile(`(?i)\b(fails? if|falsified by|proved by|breaks? if|breaks? when|regresses? if|would pass if|cannot fail)\b`)

// namesATest matches a cited Go test function.
var namesATest = regexp.MustCompile("`Test[A-Za-z0-9_]+")

// CriteriaWithoutFalsifiers returns the criteria that never say what would
// make them fail.
func CriteriaWithoutFalsifiers(text string) []CriterionFault {
	body := sectionOf(text, "Acceptance criteria")
	if body == "" {
		return nil
	}
	var out []CriterionFault
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "- [") {
			continue
		}
		criterion := joinWrapped(lines, i)
		if falsifierRe.MatchString(criterion) {
			continue
		}
		// A criterion that names a test already carries the discipline:
		// the test itself must name the mutation that makes it fail, and
		// `procoder test` is what asks for that. Demanding the clause here
		// as well would be the same question twice, and a rule that asks
		// twice is the one people learn to satisfy by pasting.
		//
		// This is what keeps the rule affordable. Name the test you would
		// write — which a good criterion does anyway — and there is
		// nothing further to add. It costs nothing where the work is
		// already done, and bites exactly where nobody has asked what
		// would break the promise: sprint 021's junk-narrowing and
		// scope-override criteria named no test, and are still caught.
		if namesATest.MatchString(criterion) {
			continue
		}
		out = append(out, CriterionFault{Text: criterion, Line: lineOf(text, "Acceptance criteria", i),
			Why: "never says what would make it fail — name the test that asserts it (`TestSomething`, which carries its own `proved by:`) or add `fails if <the change that breaks it>`; a criterion nobody has asked that about may not be able to fail at all, which is how a promise gets ticked without ever being tested"})
	}
	return out
}

// Commands is the set a spec may cite. It mirrors the list the
// docs-coverage check keeps, duplicated rather than imported because
// internal/docs imports this package's neighbours.
//
// A command missing here makes a true citation read as false, which is the
// failure direction that teaches people to ignore the checker — so
// TestCommandsMatchTheDocsCoverageList pins the two together.
var Commands = map[string]bool{
	"adr": true, "agents": true, "analyze": true, "ask": true, "audit": true,
	"backlog": true, "bench": true, "check": true, "ci": true, "config": true,
	"context": true, "copilot-leak": true, "debt": true, "deps": true, "docs": true,
	"doctor": true, "env": true, "evidence": true, "format": true, "git": true, "hook": true,
	"index": true, "infra": true, "init": true, "lessons": true, "lint": true,
	"maintain": true, "plan": true, "principles": true, "prune": true,
	"release": true, "review": true, "run": true, "scrub": true,
	"security": true, "self-upgrade": true, "spec": true, "sprint": true,
	"status": true, "templates": true, "test": true, "todo": true,
	"version": true,
}

// normaliseEOL makes CRLF and CR read as LF, so a document parses the same
// on every platform. Cheap, and the alternative is every reader here
// carrying its own \r handling and one of them forgetting.
func normaliseEOL(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// The remaining half of #198, after 3.2.0 took the unfalsifiable case.
//
// Three ways a criterion can look measured and not be. Each fires only on
// an unambiguous shape, for the reason every rule in this file errs the
// same way: a criterion refused wrongly gets deleted rather than fixed.

// hedged is vocabulary that removes the possibility of being wrong.
// "Mostly works" has no observation that contradicts it, so a criterion
// written this way passes whatever the code does.
var hedged = regexp.MustCompile(`(?i)\b(mostly|generally|as appropriate|reasonably|roughly|more or less|where sensible|if needed|should be fine|works well)\b`)

// fixedOutput is a command whose result cannot differ. `echo`, a `--help`,
// a `--version`: they print the same thing on a working system and a
// broken one, so a criterion checking them has no failing branch at all.
var fixedOutput = regexp.MustCompile("`[^`]*\\b(echo|true|cat\\s+[^`]*README|--help|--version|-h\\b)[^`]*`")

// unmeasured is a bar nobody can hold a result against. "Fast enough" and
// "not too many" name a threshold without giving one, so two people
// reading the same result disagree about whether it passed.
var unmeasured = regexp.MustCompile(`(?i)\b(fast enough|quick enough|not too (many|slow|long|big)|small enough|large enough|acceptable performance|reasonable time)\b`)

// WeakOracles returns the criteria that look like measurements and are
// not.
//
// Separate from UncheckableCriteria because the failure differs: that one
// is a criterion with no observation at all, this one is a criterion whose
// observation cannot fail. Both end the same way — a promise ticked
// without ever being tested — and reporting them separately tells the
// author which mistake they made.
func WeakOracles(text string) []CriterionFault {
	body := sectionOf(text, "Acceptance criteria")
	if body == "" {
		return nil
	}
	var out []CriterionFault
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "- [") {
			continue
		}
		criterion := joinWrapped(lines, i)
		at := lineOf(text, "Acceptance criteria", i)
		switch {
		case fixedOutput.MatchString(criterion):
			out = append(out, CriterionFault{Text: criterion, Line: at,
				Why: "checks a command whose output cannot differ — it prints the same thing on a working system and a broken one, so this criterion has no failing branch; check something the change actually affects"})
		case hedged.MatchString(criterion):
			out = append(out, CriterionFault{Text: criterion, Line: at,
				Why: "is hedged — there is no observation that contradicts \"mostly\" or \"generally\", so it passes whatever the code does; say what must be true, exactly"})
		case unmeasured.MatchString(criterion):
			out = append(out, CriterionFault{Text: criterion, Line: at,
				Why: "names a bar without giving one — two people reading the same result would disagree about whether it passed; put the number in"})
		}
	}
	return out
}
