// Package release is `procoder release` — the pre-tag controller. It runs
// the whole release checklist in one pass and lists EVERY failure, never just
// the first: version-sync across the files config.toml names, the changelog
// heading, a clean working tree, the gate, and the suite when the repo blocks
// on tests. On success it prints the tag command for the agent to run —
// P-CONTROL, the binary tags nothing.
package release

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"procoder/internal/config"
)

// versionRe is the one shape a release version has: a plausible N.N.N.
// Anything fancier (pre-releases, build metadata) is out of scope by design.
var versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

const changelogName = "CHANGELOG.md"

// Run verifies the release checklist for version under root. An empty
// version means: read the newest version heading from CHANGELOG.md and
// report the checklist for it. gateClean and suite are passed in so the
// controller reuses the same verdicts `procoder check` and the todo closer
// use — the disciplines can never disagree. suite is nil when the repo does
// not block on tests. Exit 0 ship / 1 blocked / 2 usage.
func Run(root, version string, gateClean func() bool, suite func() (bool, string), out func(string)) int {
	if version == "" {
		v, err := newestChangelogVersion(root)
		if err != nil {
			out(err.Error())
			return 2
		}
		version = v
		out(fmt.Sprintf("newest version in %s: %s", changelogName, version))
	}
	if !versionRe.MatchString(version) {
		out(fmt.Sprintf("%q is not a release version — expected N.N.N (like 1.2.3)", version))
		return 2
	}

	// Collect ALL failures before saying anything about readiness: a
	// release blocked three ways should hear all three at once.
	var failures []string

	// 1. version-sync: every listed file carries the literal version.
	// Substring by design — a version string appears inside longer lines
	// (badges, install commands) and that still counts as synced.
	files := config.Load(root).ReleaseFiles
	if len(files) == 0 {
		out("version-sync verified nothing — set [release] files in .procoder/config.toml")
	}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			// Honesty: an unreadable file is a named failure, not a skip.
			failures = append(failures, fmt.Sprintf("%s cannot be read (%v)", filepath.ToSlash(f), err))
			continue
		}
		if !strings.Contains(string(raw), version) {
			failures = append(failures, fmt.Sprintf("%s does not contain %s — bump it", filepath.ToSlash(f), version))
		}
	}

	// 2. changelog: a `## <version>` heading, matched exactly.
	if !changelogHasVersion(root, version) {
		failures = append(failures, fmt.Sprintf("%s has no `## %s` heading — write the entry", changelogName, version))
	}

	// 3. clean tree, untracked included: a release comes from committed
	// state and nothing else. git failing to run is NOT clean — unknown
	// is never clean.
	if msg := dirtyTree(root); msg != "" {
		failures = append(failures, msg)
	}

	// 4. the gate.
	if !gateClean() {
		failures = append(failures, "the gate is not clean — run `procoder check` and fix what it lists")
	}

	// 5. the suite, only when the repo blocks on tests.
	if suite != nil {
		if ok, summary := suite(); !ok {
			failures = append(failures, "the test suite is not passing: "+summary)
		}
	}

	// 6. the tag this controller is about to tell somebody to create. It
	// said "ready — tag it: git tag -a v0.2.0" for a version already
	// tagged and already published, and the printed command answers
	// "fatal: tag 'v0.2.0' already exists". Whether a version has already
	// shipped is the first thing a release controller ought to know, and
	// printing an instruction that cannot succeed is worse than silence:
	// the reader has to work out that procoder was wrong rather than
	// their repository.
	if msg := tagExists(root, version); msg != "" {
		failures = append(failures, msg)
	}

	// 7. the credits in the entry about to be published. GitHub is asked
	// who actually opened each cited issue, which the suite cannot do —
	// it runs offline on every commit — and which this controller can,
	// because the tag it is preparing gets published by a job that talks
	// to the same API. A misattributed credit hands one person's thanks
	// to another, permanently, in notes GitHub publishes verbatim.
	// One read of the changelog entry for both credit checks. Raised in
	// review on #213.
	entry := EntryFor(root, version)
	for _, p := range VerifyCredits(root, entry) {
		failures = append(failures, p)
	}
	// And the other half: who is OWED a credit and does not have one.
	// VerifyCredits catches a wrong handle, which is the loud failure —
	// the person it was taken from says nothing, and neither did anything
	// else until this. See MissingCredits for the rule.
	for _, p := range MissingCredits(root, entry) {
		failures = append(failures, p)
	}

	if len(failures) > 0 {
		out(fmt.Sprintf("release %s is NOT ready:", version))
		for _, f := range failures {
			out("  " + f)
		}
		return 1
	}
	out(fmt.Sprintf("release %s is ready — tag it:", version))
	out(fmt.Sprintf("  git tag -a v%s -m %q", version, version))
	return 0
}

// newestChangelogVersion reads the first `## ` heading whose next token is a
// plausible version. Changelogs run newest-first, so the first hit is the
// release being cut.
func newestChangelogVersion(root string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, changelogName))
	if err != nil {
		return "", fmt.Errorf("%s cannot be read (%v) — pass the version explicitly or write the changelog", changelogName, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		fields := strings.Fields(line[3:])
		if len(fields) > 0 && versionRe.MatchString(fields[0]) {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("%s has no `## N.N.N` heading — pass the version explicitly or write the entry", changelogName)
}

// changelogHasVersion reports whether CHANGELOG.md carries a `## <version>`
// heading. Exact on the version token — 0.27.0 does not satisfy 0.2.0.
func changelogHasVersion(root, version string) bool {
	raw, err := os.ReadFile(filepath.Join(root, changelogName))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		fields := strings.Fields(line[3:])
		if len(fields) > 0 && fields[0] == version {
			return true
		}
	}
	return false
}

// dirtyTree returns a failure line when the working tree is not clean, or
// when git cannot say — unknown counts as dirty, because a release must not
// depend on state nobody committed.
// tagExists reports the release already having a tag. git failing to
// answer is NOT "no tag", for the same reason an unreadable tree is not a
// clean one — unknown is never clean.
//
// `--list` takes a glob, and the exact tag is passed as the pattern, so
// v0.2.0-rc1 and v10.2.0 do not answer for v0.2.0. That is safe only
// because Run has already required the version to match N.N.N, which
// carries no glob characters. The equality below is belt and braces on
// top of that filtering, and is deliberately kept: it costs nothing and
// it is what makes the pattern's job explicit to the next reader.
func tagExists(root, version string) string {
	tag := "v" + version
	cmd := exec.Command("git", "-C", root, "tag", "--list", tag)
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("existing tags NOT verified — git tag failed (%v)", err)
	}
	if strings.TrimSpace(string(raw)) == tag {
		return fmt.Sprintf("%s already exists — this version is already tagged; bump the version, or delete the tag if it was never pushed", tag)
	}
	return ""
}

func dirtyTree(root string) string {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain")
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("clean tree NOT verified — git status failed (%v)", err)
	}
	if len(strings.TrimSpace(string(raw))) > 0 {
		return "the working tree is dirty (untracked counts) — commit everything first"
	}
	return ""
}
