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
