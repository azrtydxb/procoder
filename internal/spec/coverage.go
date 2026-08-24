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
	criteria := textutil.Section(text, "Acceptance criteria")

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

// ScopeBullets counts the bullets in the In scope section, so a spec that
// promises things without giving any of them an id can be told what it
// would have to label.
func ScopeBullets(text string) int {
	n := 0
	for _, line := range strings.Split(textutil.Section(text, "In scope"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- ") {
			n++
		}
	}
	return n
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
