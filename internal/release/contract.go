package release

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SkillPath is the governance contract agents read. Its body is generated
// from AGENTS.md; its frontmatter carries the contract version.
const SkillPath = "skills/procoder/SKILL.md"

// ContractDrift reports a skill contract that changed without saying so.
//
// The body of that file is what an agent is told to do. When it changes
// and the version in its frontmatter does not, an adopter who upgrades is
// governed by different rules with nothing to tell them — and nothing in
// the repository would have noticed, because the version is only a string
// until something compares it (#196).
//
// Reported, not refused. A wording fix changes the body too, and blocking
// every typo behind a version bump would make the bump meaningless within
// a month — which is how a version stops tracking anything. What this
// buys is that the change is stated at release time, where somebody is
// already deciding what shipped.
func ContractDrift(root, previousTag string) []string {
	if previousTag == "" {
		return nil // nothing to compare against: the first release
	}
	was, err := fileAtTag(root, previousTag, SkillPath)
	if err != nil {
		// The file did not exist at that tag, or git could not answer.
		// Neither is evidence the contract changed.
		return nil
	}
	now, err := fileAtTag(root, "HEAD", SkillPath)
	if err != nil {
		return nil
	}
	if bodyOf(was) == bodyOf(now) {
		return nil
	}
	oldV, newV := contractVersionOf(was), contractVersionOf(now)
	if oldV != newV {
		return nil // it changed and said so
	}
	return []string{fmt.Sprintf(
		"%s changed since %s and its contract version is still %q — an adopter upgrading would be governed by different rules with nothing to tell them. Bump `ContractVersion` in internal/portability/portability.go and regenerate, or say in the changelog why the change does not alter the contract",
		SkillPath, previousTag, newV)}
}

// bodyOf drops the frontmatter, so a version bump alone is not read as a
// contract change — the version IS the announcement, not the thing
// announced.
func bodyOf(file string) string {
	if !strings.HasPrefix(file, "---\n") {
		return file
	}
	if i := strings.Index(file[4:], "\n---\n"); i >= 0 {
		return file[4+i+5:]
	}
	return file
}

func contractVersionOf(file string) string {
	for _, line := range strings.Split(file, "\n") {
		t := strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(t, "contract:"); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
		if strings.HasPrefix(t, "# ") {
			break // past the frontmatter
		}
	}
	return ""
}

func fileAtTag(root, ref, path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "show", ref+":"+path)
	raw, err := cmd.Output()
	return string(raw), err
}

// previousTag is the newest v-tag reachable from HEAD, or "" when there is
// none — the first release, where there is nothing to compare against.
func previousTag(root string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "describe", "--tags", "--abbrev=0", "--match", "v*")
	raw, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
