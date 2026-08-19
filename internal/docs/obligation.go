package docs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"procoder/internal/gitx"
)

// AckPrefix is the acknowledgment line's fixed head; what follows the dash is
// the reason, and the reason is the point — see AckLine.
const AckPrefix = "docs: none"

// AckLine is the line `procoder docs --ack "<reason>"` prints for the agent to
// place in the commit message. The record lives in history where a reviewer
// sees it, never in a state file.
func AckLine(reason string) string {
	return AckPrefix + " — " + strings.TrimSpace(reason)
}

// maxTriggersNamed caps how much of the trigger set one obligation spells out;
// naming three is enough to act on, naming forty is noise.
const maxTriggersNamed = 5

// Obligation is the change-driven documentation obligation: did this change
// invalidate a document? It fires when the changed files carry a public-surface
// change (an exported symbol the index does not know, one it knows and the file
// no longer defines, a CLI flag or subcommand string added or removed, a
// configuration file touched) OR when a document names one of the changed
// files — and no documentation file changed in the same diff.
//
// It clears two ways, both explicit: change a documentation file, or record the
// decision as a `docs: none — <reason>` line in the commit message. Silence
// never clears it.
//
// Pure by construction: the caller decides whether the obligation blocks
// (config's [docs] policy) and supplies the commit message; nothing here reads
// config, git state it was not given, or the clock. commitMessage may be empty,
// which is reported as the acknowledgment path being unavailable rather than
// silently treated as "no acknowledgment".
func Obligation(root string, changed []string, commitMessage string, block bool) []gitx.Finding {
	corpus := docCorpus(root)
	var out []gitx.Finding
	var code []string
	docChanged := false
	for _, f := range changed {
		rel, ok := relInRoot(root, f)
		if !ok {
			continue
		}
		if !IsMarkdownFile(rel) {
			code = append(code, rel)
			continue
		}
		if !corpus[rel] || strings.HasPrefix(rel, stateDir) {
			// site output, vendored docs, procoder's own state store:
			// regenerating or planning never clears the obligation
			continue
		}
		if _, err := os.ReadFile(f); err != nil {
			// unknown is never done: a doc we cannot read is not a doc we can
			// say was updated
			out = append(out, gitx.Finding{File: f,
				Message: rel + " is unreadable — it does not count as a documentation change"})
			continue
		}
		docChanged = true
	}
	if docChanged {
		return nil // the question was asked and answered by editing a document
	}
	if reason := ackReason(commitMessage); reason != "" {
		return nil // answered in the commit message, where a reviewer sees it
	}

	triggers, notes := surfaceTriggers(root, code)
	triggers = append(triggers, mentionTriggers(root, changed)...)
	out = append(out, notes...)
	if len(triggers) == 0 {
		return out
	}
	named := triggers
	more := ""
	if len(named) > maxTriggersNamed {
		more = fmt.Sprintf(" (and %d more)", len(named)-maxTriggersNamed)
		named = named[:maxTriggersNamed]
	}
	out = append(out, gitx.Finding{Blocking: block,
		Message: fmt.Sprintf("documentation obligation: %s%s — no documentation file changed in this diff; update a doc, or record the decision with a `%s — <reason>` line in the commit message (`procoder docs --ack \"<reason>\"` prints it)",
			strings.Join(named, "; "), more, AckPrefix)})
	if strings.TrimSpace(commitMessage) == "" {
		out = append(out, gitx.Finding{
			Message: "acknowledgment path unavailable — no commit message at check time; the obligation stands until a doc changes or the check runs where the message exists"})
	}
	return out
}

// ackReason returns the reason from a `docs: none — <reason>` line, or "" when
// the message carries no such line or the reason is empty. An empty reason does
// not clear: the reason is the point.
func ackReason(message string) string {
	for _, line := range strings.Split(message, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(t), AckPrefix) {
			continue
		}
		rest := t[len(AckPrefix):]
		// accept the em dash we print and the ASCII dashes an agent may type
		rest = strings.TrimLeft(rest, " \t")
		for _, dash := range []string{"—", "–", "--", "-", ":"} {
			if r, ok := strings.CutPrefix(rest, dash); ok {
				rest = r
				break
			}
		}
		if reason := strings.TrimSpace(rest); reason != "" {
			return reason
		}
	}
	return ""
}

// mentionTriggers reuses Drift — the doc-mention rule lives there and must not
// be written twice — and states each hit as a trigger.
func mentionTriggers(root string, changed []string) []string {
	var out []string
	for _, f := range Drift(root, changed) {
		path, ok := strings.CutPrefix(f.Message, driftPrefix)
		if !ok {
			continue
		}
		path, _, _ = strings.Cut(path, " ")
		page := f.File
		if rel, ok := relInRoot(root, f.File); ok {
			page = rel
		}
		// procoder's own store cannot raise an obligation it is also
		// forbidden to clear. A bug story naming the file it fixes would
		// otherwise demand documentation that no edit to that story could
		// ever satisfy — the tool contradicting itself.
		if strings.HasPrefix(page, stateDir) {
			continue
		}
		out = append(out, fmt.Sprintf("%s names changed file %s", page, path))
	}
	return out
}

// driftPrefix is the head of Drift's message; mentionTriggers reads the path
// back out of it rather than re-deriving the match.
const driftPrefix = "mentions changed file "

// surfaceTriggers computes the public-surface half: what the index knew the
// changed files defined against what they define now. Returns the triggers and
// the honesty notes — a repository with no index cannot have its public surface
// computed, and says so instead of reading clean.
func surfaceTriggers(root string, code []string) (triggers []string, notes []gitx.Finding) {
	var source []string
	for _, rel := range code {
		if trig := configTrigger(rel); trig != "" {
			triggers = append(triggers, trig)
		}
		if languageOf(rel) != "" {
			source = append(source, rel)
		}
	}
	triggers = append(triggers, literalTriggers(root, source)...)
	if len(source) == 0 {
		return triggers, notes
	}
	known, err := indexedSymbols(root)
	if err != nil {
		notes = append(notes, gitx.Finding{
			Message: "public surface NOT computed — no index; run `procoder index build` (the doc-mention trigger still applies): " + err.Error()})
		return triggers, notes
	}
	indexedExt := map[string]bool{}
	for path := range known {
		indexedExt[strings.ToLower(filepath.Ext(path))] = true
	}
	var unindexed []string
	for _, rel := range source {
		now := exportedSymbols(filepath.Join(root, filepath.FromSlash(rel)))
		was, inIndex := known[rel]
		if !inIndex && !indexedExt[strings.ToLower(filepath.Ext(rel))] {
			// the indexer does not cover this language: saying nothing here
			// would read as "no public-surface change", which we do not know
			unindexed = append(unindexed, rel)
			continue
		}
		for _, name := range sortedDiff(now, was) {
			triggers = append(triggers, fmt.Sprintf("exported symbol %s added in %s", name, rel))
		}
		for _, name := range sortedDiff(was, now) {
			triggers = append(triggers, fmt.Sprintf("exported symbol %s removed or renamed in %s", name, rel))
		}
	}
	if len(unindexed) > 0 {
		notes = append(notes, gitx.Finding{
			Message: fmt.Sprintf("public surface NOT computed for %d changed file(s) the index does not cover (%s) — run `procoder index build`",
				len(unindexed), strings.Join(firstFew(unindexed), ", "))})
	}
	return triggers, notes
}

func firstFew(in []string) []string {
	if len(in) > 3 {
		return append(append([]string{}, in[:3]...), "…")
	}
	return in
}

// sortedDiff is the names in a that b does not have, in a stable order.
func sortedDiff(a, b map[string]bool) []string {
	var out []string
	for name := range a {
		if !b[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// configTrigger names a changed configuration file. A config key added or
// removed is a public-surface change; the file's own text is the honest
// granularity we have without parsing every dialect. Workflow and manifest
// YAML under .github or a chart directory is not a configuration surface, so
// only root-level and config-named YAML counts.
func configTrigger(rel string) string {
	ext := strings.ToLower(filepath.Ext(rel))
	base := strings.ToLower(filepath.Base(rel))
	switch ext {
	case ".toml":
		return "configuration file " + rel + " changed"
	case ".yaml", ".yml":
		if !strings.Contains(rel, "/") || strings.Contains(base, "config") ||
			strings.HasPrefix(rel, ".procoder/") || strings.HasPrefix(rel, "config/") {
			return "configuration file " + rel + " changed"
		}
	}
	return ""
}

// flagOrCommand matches the two CLI surfaces a reader discovers a tool
// through: a long flag string and a `case "sub"` dispatch arm.
var (
	longFlagRe = regexp.MustCompile(`"(--[a-zA-Z][\w-]*)"`)
	caseArmRe  = regexp.MustCompile(`\bcase\s+((?:"[^"]*"\s*,\s*)*"[^"]*")\s*:`)
	quotedRe   = regexp.MustCompile(`"([^"]*)"`)
)

// literalTriggers compares the CLI strings a file carries now against the ones
// the previous commit carried. This needs git; without it the comparison is
// simply not made — the symbol trigger above still runs.
func literalTriggers(root string, source []string) []string {
	var out []string
	for _, rel := range source {
		old, ok := gitShow(root, rel)
		if !ok {
			continue // new file, or no git: the symbol trigger covers it
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		now := cliStrings(string(data))
		was := cliStrings(old)
		for _, s := range sortedDiff(now, was) {
			out = append(out, fmt.Sprintf("CLI string %q added in %s", s, rel))
		}
		for _, s := range sortedDiff(was, now) {
			out = append(out, fmt.Sprintf("CLI string %q removed in %s", s, rel))
		}
	}
	return out
}

func cliStrings(body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range longFlagRe.FindAllStringSubmatch(body, -1) {
		out[m[1]] = true
	}
	for _, m := range caseArmRe.FindAllStringSubmatch(body, -1) {
		for _, q := range quotedRe.FindAllStringSubmatch(m[1], -1) {
			if q[1] != "" {
				out[q[1]] = true
			}
		}
	}
	return out
}

func gitShow(root, rel string) (string, bool) {
	out, err := exec.Command("git", "-C", root, "show", "HEAD:"+rel).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// relInRoot turns a changed path into a repo-relative forward-slash path,
// dropping directories (git lists untracked directories as one entry) and
// anything outside the tree.
func relInRoot(root, path string) (string, bool) {
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// isInternalPath reports whether a path lives where the project says its
// surface does not: an internal or private directory segment, a convention
// Go enforces and every other language uses by hand.
func isInternalPath(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == "internal" || seg == "private" || seg == "_internal" {
			return true
		}
	}
	return false
}

// stateDir is procoder's own store — backlog, plans, specs, lessons. Its
// Markdown is project state, not the repository's documentation: editing a
// backlog story must never clear a documentation obligation.
const stateDir = ".procoder/"

// docCorpus is the documentation set as repo-relative paths: what MarkdownFiles
// considers part of the repository, which is what a changed document must be a
// member of to count.
func docCorpus(root string) map[string]bool {
	out := map[string]bool{}
	for _, md := range MarkdownFiles(root) {
		if rel, err := filepath.Rel(root, md); err == nil {
			out[filepath.ToSlash(rel)] = true
		}
	}
	return out
}

// languageOf names the language a path's public surface can be read from, or
// "" when this package cannot read it.
func languageOf(rel string) string {
	if strings.Contains(rel, "/vendor/") || strings.HasPrefix(rel, "vendor/") ||
		strings.Contains(rel, "node_modules/") {
		return ""
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go":
		if strings.HasSuffix(rel, "_test.go") {
			return ""
		}
		return "go"
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx":
		return "js"
	case ".rs":
		return "rust"
	case ".py":
		return "python"
	}
	return ""
}

var (
	goFuncRe   = regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Z]\w*)`)
	goDeclRe   = regexp.MustCompile(`^(?:type|var|const)\s+([A-Z]\w*)`)
	goGroupRe  = regexp.MustCompile(`^(?:type|var|const)\s*\($`)
	goMemberRe = regexp.MustCompile(`^\s+([A-Z]\w*)\b`)
	jsExportRe = regexp.MustCompile(`^export\s+(?:default\s+)?(?:async\s+)?(?:function\*?|class|const|let|var)\s+(\w+)`)
	jsListRe   = regexp.MustCompile(`^export\s*\{([^}]*)\}`)
	rustRe     = regexp.MustCompile(`^\s*pub(?:\([^)]*\))?\s+(?:async\s+|unsafe\s+|extern\s+"[^"]*"\s+)*(?:fn|struct|enum|trait|const|static|type|mod|union)\s+(\w+)`)
	pyRe       = regexp.MustCompile(`^(?:async\s+)?(?:def|class)\s+([A-Za-z]\w*)`)
)

// exportedSymbols reads a file's public surface: the names another package,
// module, or crate can reach. Deliberately syntactic — no toolchain, no
// network, the same answer on every machine.
func exportedSymbols(path string) map[string]bool {
	out := map[string]bool{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	lang := languageOf(filepath.ToSlash(path))
	inGroup := false
	for _, line := range strings.Split(string(data), "\n") {
		switch lang {
		case "go":
			if inGroup {
				if strings.HasPrefix(strings.TrimSpace(line), ")") {
					inGroup = false
					continue
				}
				if m := goMemberRe.FindStringSubmatch(line); m != nil {
					out[m[1]] = true
				}
				continue
			}
			if goGroupRe.MatchString(strings.TrimRight(line, " \t")) {
				inGroup = true
				continue
			}
			if m := goFuncRe.FindStringSubmatch(line); m != nil {
				out[m[1]] = true
			}
			if m := goDeclRe.FindStringSubmatch(line); m != nil {
				out[m[1]] = true
			}
		case "js":
			if m := jsExportRe.FindStringSubmatch(line); m != nil {
				out[m[1]] = true
			}
			if m := jsListRe.FindStringSubmatch(line); m != nil {
				for _, part := range strings.Split(m[1], ",") {
					name := strings.TrimSpace(part)
					if i := strings.Index(name, " as "); i >= 0 {
						name = strings.TrimSpace(name[i+4:])
					}
					if name != "" {
						out[name] = true
					}
				}
			}
		case "rust":
			if m := rustRe.FindStringSubmatch(line); m != nil {
				out[m[1]] = true
			}
		case "python":
			if m := pyRe.FindStringSubmatch(line); m != nil && !strings.HasPrefix(m[1], "_") {
				out[m[1]] = true
			}
		}
	}
	return out
}

// isExportedSymbol is the universal proxy the index itself uses: a name whose
// first rune is upper case is reachable surface. Applied to index tags, whose
// language the tag names but whose visibility it does not.
func isExportedSymbol(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}
