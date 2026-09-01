package release

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"procoder/internal/config"
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
	// A bot login is a login with a [bot] suffix, so the capture takes the
	// whole thing: a credit written as the documentation tells it to be —
	// `[@github-actions[bot]](https://github.com/github-actions[bot])` —
	// has to count as a credit. The first version stopped at the bracket
	// and reported every bot contributor as uncredited no matter what was
	// written.
	linkedHandle = regexp.MustCompile(`\[@([A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\[bot\])?)\]\(https://github\.com/`)
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

// origin is one cited number, resolved: who opened it, and whether it is a
// pull request or an issue.
type origin struct {
	number int
	login  string
	isPR   bool
}

// resolveOrigin asks GitHub who opened a number and which kind it is.
// One API call answers both: the issues endpoint serves pull requests too
// and carries a `pull_request` key when it is one.
func resolveOrigin(root string, number int) (origin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "api",
		fmt.Sprintf("repos/azrtydxb/procoder/issues/%d", number), // nosemgrep -- a number, formatted from an int
		"--jq", `[.user.login, (.pull_request != null)] | @tsv`)
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		return origin{}, fmt.Errorf("%v", firstLine(err))
	}
	parts := strings.Fields(strings.TrimSpace(string(raw)))
	if len(parts) < 2 {
		return origin{}, fmt.Errorf("unreadable answer for #%d", number)
	}
	return origin{number: number, login: parts[0], isPR: parts[1] == "true"}, nil
}

// excluded is the handles the credit rule does not ask about: the people
// whose release notes these are. Thanking yourself in your own notes is
// noise, and a rule that demanded it would be ignored inside one release.
//
// Configuration first, and configuration is the answer that works
// everywhere. `gh api user` answers only where a person is logged in — in
// CI the token is an app installation token with no user behind it, and
// asking returns 403. That made the check unrunnable in the one place it
// most needed to run, which is how it was found. And "whoever triggered
// the workflow" is worse than useless: on a contributor's pull request
// that is the contributor, who would then be excluded from the credit
// they are owed.
//
// The local fallback stays, so a repository that has configured nothing
// still gets the rule when a person runs it by hand.
func excluded(root string) ([]string, error) {
	if names := config.Load(root).Maintainers; len(names) > 0 {
		return names, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "api", "user", "--jq", ".login")
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%v — set `[release] maintainers` in .procoder/config.toml, which is the answer that works in CI too", firstLine(err))
	}
	login := strings.TrimSpace(string(raw))
	if login == "" {
		return nil, fmt.Errorf("gh answered with no login — set `[release] maintainers` in .procoder/config.toml")
	}
	return []string{login}, nil
}

// MissingCredits reports contributors a paragraph OWES a credit and does
// not give, and names the handle to add.
//
// VerifyCredits already catches a handle that opened none of what its
// paragraph cites. That is only half the question, and the cheaper half: a
// wrong credit is loud, and the person it was taken from is silent. This
// is the other half — who should be here and is not — and it is what makes
// the rule mechanical rather than a reminder to be careful.
//
// The rule, which is the maintainer's:
//
//   - a cited ISSUE owes its author a credit;
//   - a cited PULL REQUEST owes its author a credit;
//   - when the same person did both, that is one credit, not two;
//   - when they are different people, BOTH are owed one — the report and
//     the fix are separate contributions and crediting only the second
//     quietly erases the first.
//
// Whoever is cutting the release is excluded. GitHub not answering is not
// a pass: it is reported and blocks, like everything else here that could
// not run.
func MissingCredits(root, entry string) []string {
	return missingCreditsWith(entry,
		func() ([]string, error) { return excluded(root) },
		func(n int) (origin, error) { return resolveOrigin(root, n) })
}

// missingCreditsWith is the rule with its two GitHub questions injected,
// so the logic can be tested without a network — the suite runs offline on
// every commit, and a rule this fiddly is exactly the kind that needs
// tests rather than one live run that happened to look right.
func missingCreditsWith(entry string, who func() ([]string, error), resolve func(int) (origin, error)) []string {
	// Nothing cited anywhere means nothing is owed, and asking GitHub who
	// is releasing would be a network call to answer a question nobody
	// asked. VerifyCredits returns early for the same reason — and without
	// this, every offline release test started failing on a guard that was
	// working exactly as intended. Found by CI.
	if !citedNumber.MatchString(entry) {
		return nil
	}
	// The releaser guard lives HERE rather than in the networked caller, so
	// a test can reach it. An empty handle excludes nobody, so proceeding
	// would quietly turn the exclusion off and start demanding the
	// maintainer credit themselves — the rule silently not applying, which
	// is the failure this file exists to end.
	mine, meErr := who()
	if meErr != nil || len(mine) == 0 {
		reason := "no maintainer handle available"
		if meErr != nil {
			reason = meErr.Error()
		}
		return []string{fmt.Sprintf("who these release notes belong to could not be determined (%s) — the exclusion cannot be applied, and a credit rule that silently stops applying is worse than none", reason)}
	}
	isMine := map[string]bool{}
	for _, m := range mine {
		isMine[strings.ToLower(m)] = true
	}
	var gaps []string
	for _, para := range strings.Split(entry, "\n\n") {
		var nums []int
		for _, m := range citedNumber.FindAllStringSubmatch(para, -1) {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			nums = append(nums, n)
		}
		if len(nums) == 0 {
			continue
		}
		credited := map[string]bool{}
		for _, m := range linkedHandle.FindAllStringSubmatch(para, -1) {
			credited[strings.ToLower(m[1])] = true
		}
		// Owed, in the order the numbers appear, so the report reads the
		// way the paragraph does.
		var owed []origin
		seen := map[string]bool{}
		for _, n := range nums {
			o, err := resolve(n)
			if err != nil {
				gaps = append(gaps, fmt.Sprintf("#%d could not be resolved (%v) — who to credit is unknown, and unknown is not a pass", n, err))
				continue
			}
			if o.login == "" || isMine[strings.ToLower(o.login)] || seen[strings.ToLower(o.login)] {
				// Same person for issue and PR collapses here: one credit,
				// not two.
				seen[strings.ToLower(o.login)] = true
				continue
			}
			seen[strings.ToLower(o.login)] = true
			owed = append(owed, o)
		}
		for _, o := range owed {
			if credited[strings.ToLower(o.login)] {
				continue
			}
			kind := "reported"
			if o.isPR {
				kind = "contributed"
			}
			gaps = append(gaps, fmt.Sprintf("@%s %s #%d and is not credited in the paragraph citing it — add `%s by [@%s](https://github.com/%s)`",
				o.login, kind, o.number, kind[:1]+kind[1:], o.login, o.login))
		}
	}
	return gaps
}

// Credits runs the two credit checks alone and nothing else.
//
// It exists so CI can ENFORCE them. Until now the rule ran only when
// somebody typed `procoder release` on their own machine, which means it
// ran when they remembered — and "remember to check" is precisely what
// kept failing, which is why the rule was written in the first place.
//
// The rest of the controller asks about the working tree, the tag and the
// version files. None of that means anything on a pull request, and all of
// it would fail for reasons unrelated to whether anybody is credited.
func Credits(root, version string, out func(string)) int {
	if version == "" {
		v, err := newestChangelogVersion(root)
		if err != nil {
			out("no version to check — " + err.Error())
			return 2
		}
		version = v
	}
	entry := EntryFor(root, version)
	if strings.TrimSpace(entry) == "" {
		out(fmt.Sprintf("no changelog entry for %s — nothing to check credits against", version))
		return 2
	}
	var problems []string
	problems = append(problems, VerifyCredits(root, entry)...)
	problems = append(problems, MissingCredits(root, entry)...)
	if len(problems) == 0 {
		out(fmt.Sprintf("credits for %s: every contributor the entry cites is credited, and every credit traces to something they opened", version))
		return 0
	}
	out(fmt.Sprintf("credits for %s are NOT right:", version))
	for _, p := range problems {
		out("  " + p)
	}
	return 1
}
