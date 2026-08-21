package debt

import (
	"fmt"

	"procoder/internal/gitx"
)

// GateCheck reports debt markers with no revisit condition in the files
// this commit carries.
//
// The whole ledger is a property of the tree and belongs to CI: printing
// eight markers on every commit is how a list becomes wallpaper. What
// belongs here is the one a developer is adding right now, while the
// reason for it is still in their head — a marker with no revisit
// condition is a shortcut with no way back, and the moment to say so is
// before it is committed rather than in a report nobody opens.
//
// Reported, not blocking. A deliberate shortcut is a judgement the author
// is entitled to make; what they are not entitled to is making it
// silently.
func GateCheck(root string, files []string) []gitx.Finding {
	changed := map[string]bool{}
	for _, f := range files {
		if rel, ok := gitx.RepoRel(root, f); ok {
			changed[rel] = true
		}
	}
	if len(changed) == 0 {
		return nil
	}
	entries, err := Scan(root)
	if err != nil {
		// Scan fails when git cannot list the tree, which is the same
		// condition the rest of the gate is already refusing over. Saying
		// it again here would be noise, but reporting clean would be a
		// check that did not run wearing the face of one that passed.
		return []gitx.Finding{{Message: fmt.Sprintf("debt ledger NOT read — %v (debt)", err)}}
	}
	var out []gitx.Finding
	for _, e := range entries {
		if !e.NoTrigger || !changed[e.File] {
			continue
		}
		out = append(out, gitx.Finding{Message: fmt.Sprintf(
			"%s:%d debt marker with no revisit condition — name what would make this worth doing properly (debt)",
			e.File, e.Line)})
	}
	return out
}
