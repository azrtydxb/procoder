package release

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Credit is one contributor named in a changelog paragraph, alongside the
// issues and pull requests that paragraph cites.
type Credit struct {
	Handle string
	Cites  []int
}

var (
	linkedHandle = regexp.MustCompile(`\[@([A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)\]\(https://github\.com/`)
	citedNumber  = regexp.MustCompile(`\[#(\d+)\]\(https://github\.com/azrtydxb/procoder/(?:pull|issues)/\d+\)`)
)

// CreditsIn pairs each contributor named in the entry with the issues and
// pull requests cited in the same paragraph.
//
// Paragraph scope is the whole rule: a changelog entry credits somebody
// FOR something, and the something is whatever that paragraph links. A
// handle in one paragraph and a number in another are unrelated, and
// pairing them would invent a claim the entry never made.
func CreditsIn(entry string) []Credit {
	var out []Credit
	for _, para := range strings.Split(entry, "\n\n") {
		var nums []int
		for _, m := range citedNumber.FindAllStringSubmatch(para, -1) {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			nums = append(nums, n)
		}
		for _, m := range linkedHandle.FindAllStringSubmatch(para, -1) {
			out = append(out, Credit{Handle: m[1], Cites: nums})
		}
	}
	return out
}

// authorOf asks GitHub who opened an issue or pull request. Issues and
// pull requests share a number space, and `gh issue view` answers for both,
// so one call covers either.
func authorOf(root string, number int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "issue", "view", fmt.Sprint(number),
		"--json", "author", "--jq", ".author.login") // nosemgrep -- a number, formatted from an int
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		// A pull request that `gh issue view` will not answer for is still
		// reachable through the pull-request endpoint on some versions.
		ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel2()
		alt := exec.CommandContext(ctx2, "gh", "pr", "view", fmt.Sprint(number),
			"--json", "author", "--jq", ".author.login") // nosemgrep -- a number, formatted from an int
		alt.Dir = root
		raw2, err2 := alt.Output()
		if err2 != nil {
			return "", fmt.Errorf("%v", firstLine(err))
		}
		raw = raw2
	}
	return strings.TrimSpace(string(raw)), nil
}

// VerifyCredits checks that every contributor the newest entry names
// actually opened one of the issues or pull requests that paragraph cites.
//
// It lives in the release controller rather than the test suite on
// purpose. The suite runs on every commit and offline; this needs GitHub.
// The release controller already cannot do its job without GitHub — the
// tag it prepares is published by a job that talks to the API — so a
// network check costs nothing here that was not already required, and
// buying correctness with a call that has to happen anyway is a trade
// worth making.
//
// A misattributed credit is worse than none: it takes the thanks owed to
// one person and hands it to another, permanently, in the release notes
// GitHub publishes. That happened in this repository. The fix at the time
// was to write a rule asking the author to check by hand — which is what
// had just failed.
//
// GitHub not answering is NOT a pass. It is reported as unverified and
// blocks, like every other check here that could not run.
func VerifyCredits(root, entry string) []string {
	credits := CreditsIn(entry)
	if len(credits) == 0 {
		return nil
	}

	authors := map[int]string{}
	var problems []string
	for _, c := range credits {
		if len(c.Cites) == 0 {
			problems = append(problems, fmt.Sprintf(
				"@%s is credited in a paragraph that links no issue or pull request — a credit a reader cannot trace is one they take on faith", c.Handle))
			continue
		}
		matched := false
		for _, n := range c.Cites {
			who, seen := authors[n]
			if !seen {
				var err error
				who, err = authorOf(root, n)
				if err != nil {
					problems = append(problems, fmt.Sprintf(
						"credits NOT verified — GitHub would not say who opened #%d (%v); a name nothing checked is how the wrong one ships", n, err))
					return problems
				}
				authors[n] = who
			}
			if strings.EqualFold(who, c.Handle) {
				matched = true
				break
			}
		}
		if !matched {
			var who []string
			for _, n := range c.Cites {
				who = append(who, fmt.Sprintf("#%d by @%s", n, authors[n]))
			}
			problems = append(problems, fmt.Sprintf(
				"@%s is credited but opened none of what that paragraph cites (%s) — verify with `gh issue view <n>` and correct the handle",
				c.Handle, strings.Join(who, ", ")))
		}
	}
	return problems
}

func firstLine(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		for _, l := range strings.Split(string(ee.Stderr), "\n") {
			if t := strings.TrimSpace(l); t != "" {
				return t
			}
		}
	}
	return err.Error()
}

// EntryFor returns the body of the `## <version>` section of the
// repository's changelog — the text the release job publishes verbatim.
func EntryFor(root, version string) string {
	raw, err := os.ReadFile(filepath.Join(root, changelogName))
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "## "+version) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			return strings.Join(lines[start+1:i], "\n")
		}
	}
	return strings.Join(lines[start+1:], "\n")
}
