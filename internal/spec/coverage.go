package spec

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"procoder/internal/textutil"
)

// scopeID matches the identifier a scope bullet carries and a criterion
// cites: `- [S-3] …`. Case-insensitive on the letter so a spec written
// `[s-3]` is not silently a different item.
var scopeID = regexp.MustCompile(`\[([Ss]-\d+)\]`)

// ScopeCoverage reports the scope this spec promises and does not test.
//
// It exists because a spec can promise five things, write criteria for
// three, and pass every check procoder had. `backlog seed` generates one
// story per criterion, so the two untested promises become no stories,
// get no sprint, and are never missed — the epic closes at "all stories
// done" while the feature ships half-built. That happened in this
// repository, to this spec, and nothing anywhere noticed until somebody
// asked a direct question.
//
// Coverage is declared, never inferred. Matching a scope bullet to a
// criterion by keyword or similarity would fail OPEN — a bullet wrongly
// judged covered is exactly the silence this exists to break — so the
// link is an id the author writes on both ends and this function only
// checks the two sets agree.
func ScopeCoverage(text string) (uncovered []string, declared bool) {
	scope := textutil.Section(text, "In scope")
	// Comments stripped before the criteria are scanned: a citation
	// inside an HTML comment is not a criterion, and counting it would
	// let a spec satisfy coverage with work nobody committed to. The
	// same divergence the zero-criteria guard in backlog seed exists
	// for, arriving one check earlier.
	criteria := textutil.StripComments(textutil.Section(text, "Acceptance criteria"))

	inScope := idsIn(scope)
	if len(inScope) == 0 {
		return nil, false
	}
	tested := map[string]bool{}
	for _, id := range idsIn(criteria) {
		tested[id] = true
	}
	for _, id := range inScope {
		if !tested[id] {
			uncovered = append(uncovered, id)
		}
	}
	return uncovered, true
}

// ScopePromises reports whether the In scope section says anything at
// all, so a spec that promises without labelling can be told it is
// unchecked rather than assumed covered.
//
// Any content counts, not only bullets. Counting bullets alone left a
// bypass: scope written as prose would carry no ids, satisfy nothing,
// and be reported as fully covered — the same failing-open this check
// exists to prevent, reachable by writing a paragraph.
func ScopePromises(text string) bool {
	return strings.TrimSpace(textutil.StripComments(textutil.Section(text, "In scope"))) != ""
}

// idsIn returns the scope ids a section cites, deduplicated and ordered
// so a report reads the same way twice.
func idsIn(section string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range scopeID.FindAllStringSubmatch(section, -1) {
		id := strings.ToUpper(m[1])
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return idLess(out[i], out[j]) })
	return out
}

// idLess orders S-2 before S-10, which string order does not.
func idLess(a, b string) bool {
	na, nb := 0, 0
	fmt.Sscanf(a, "S-%d", &na)
	fmt.Sscanf(b, "S-%d", &nb)
	return na < nb
}

// StripScopeIDs removes the `[S-n]` citations from a criterion, leaving
// the requirement. The ids link scope to test; they are not part of what
// the criterion asks for, so anything rendering a criterion for a reader
// — a story title, a file name — takes this form.
func StripScopeIDs(s string) string {
	return strings.TrimSpace(scopeID.ReplaceAllString(s, ""))
}
