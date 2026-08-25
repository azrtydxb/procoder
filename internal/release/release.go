// Package release is `procoder release` — the pre-tag controller. It runs
// the whole release checklist in one pass and lists EVERY failure, never just
// the first: version-sync across the files config.toml names, the changelog
// heading, a clean working tree, the gate, and the suite when the repo blocks
// on tests. On success it prints the tag command for the agent to run —
// P-CONTROL, the binary tags nothing.
package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

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
// distTimeout is the budget for asking one shipped binary its version. A
// local exec answers in milliseconds, so ten seconds is not a deadline
// anything healthy approaches — it is the ceiling that stops a corrupt or
// wedged binary hanging the release.
const distTimeout = 10 * time.Second

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

	// 7. the binaries this release actually ships. The manifests are text
	// and version-sync reads them; dist/ is what the plugin executes on
	// every session start and what `self-upgrade` downloads, and nothing
	// looked at it. 3.0.0 was tagged with dist/ still holding 2.0.1
	// binaries — every manifest said 3.0.0, the gate was green, the suite
	// was green, and the plugin would have installed a version that
	// reported one number and behaved like another.
	failures = append(failures, staleDist(root, version)...)

	// 8. the credits in the entry about to be published. GitHub is asked
	// who actually opened each cited issue, which the suite cannot do —
	// it runs offline on every commit — and which this controller can,
	// because the tag it is preparing gets published by a job that talks
	// to the same API. A misattributed credit hands one person's thanks
	// to another, permanently, in notes GitHub publishes verbatim.
	for _, p := range VerifyCredits(root, EntryFor(root, version)) {
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
// DistDir holds the per-platform binaries the plugin runs and the release
// publishes.
const DistDir = "dist"

// staleDist reports any shipped binary that does not answer with the
// version being released, and any that cannot be asked. A binary that
// will not run is not a binary that passed — the whole point of asking it
// is that the manifests cannot answer for it.
//
// Only the host's own platform can be executed here; the others are
// checked by CI, which rebuilds dist/ and compares hashes. That split is
// stated rather than hidden: a check that silently covers one of five
// files is one somebody will later believe covered all five.
func staleDist(root, version string) []string {
	dir := filepath.Join(root, DistDir)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil // a repository that ships no binaries has none to check
		}
		// Present but unreadable is not absent. Treating every Stat error
		// as "no dist" would let a permissions problem pass a release that
		// nothing checked — unknown is never clean.
		return []string{fmt.Sprintf("%s could not be read (%v) — the shipped binaries were NOT checked", DistDir, err)}
	}
	bin := filepath.Join(dir, runtime.GOOS+"-"+runtime.GOARCH, "procoder")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if _, err := os.Stat(bin); err != nil {
		if !os.IsNotExist(err) {
			// Unreadable is not missing, here as much as for the directory
			// above: "rebuild it" is the wrong instruction for a
			// permissions problem, and reporting a check as done when it
			// could not be performed is the failure this whole release is
			// about.
			return []string{fmt.Sprintf("the shipped binary for %s could not be read (%v) — it was NOT checked",
				runtime.GOOS+"-"+runtime.GOARCH, err)}
		}
		return []string{fmt.Sprintf("%s carries no binary for this platform (%s) — rebuild it with scripts/build-dist.sh",
			DistDir, runtime.GOOS+"-"+runtime.GOARCH)}
	}
	ctx, cancel := context.WithTimeout(context.Background(), distTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "version").Output() // nosemgrep -- a path under the repository's own dist/, not user input
	if err != nil {
		// `%v` on an ExitError is "exit status 1" and nothing else, while
		// the binary's own complaint sits in ExitError.Stderr. Reporting
		// only the status repeats the mistake this release fixed in the
		// scanner messages: a refusal whose reason is not the reason.
		return []string{fmt.Sprintf("the shipped binary %s would not answer `version` (%s) — rebuild it with scripts/build-dist.sh",
			filepath.ToSlash(filepath.Join(DistDir, runtime.GOOS+"-"+runtime.GOARCH, "procoder")), execReason(err))}
	}
	got := strings.TrimSpace(string(out))
	if got != version {
		return []string{fmt.Sprintf("the shipped binary reports %s, not %s — rebuild %s with scripts/build-dist.sh",
			got, version, DistDir)}
	}
	return nil
}

// execReason is what a failed exec actually said. The error alone gives
// the exit status; the process's stderr, which an ExitError carries, gives
// the sentence a person can act on.
func execReason(err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if msg := strings.TrimSpace(string(ee.Stderr)); msg != "" {
			return lastLine(msg) + " — " + err.Error()
		}
	}
	return err.Error()
}

// lastLine is the last non-empty line: a tool prints its progress first
// and the reason it gave up last.
func lastLine(s string) string {
	out := ""
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = t
		}
	}
	if len(out) > 160 {
		out = out[:160]
	}
	return out
}

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
