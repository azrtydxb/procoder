package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireGit skips the fixture tests where git is absent — the clean-tree
// leg shells to git, so these prove the integration only where it can run.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	return root
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	base := []string{"-C", root, "-c", "user.email=test@example.com", "-c", "user.name=test"}
	cmd := exec.Command("git", append(base, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func capture() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

func joined(lines *[]string) string { return strings.Join(*lines, "\n") }

func gateGreen() bool { return true }

// Every failure in one output: a stale listed file, the missing changelog
// heading, and the dirty tree are all named at once, exit 1.
func TestAllFailuresReportedTogether(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeFile(t, root, ".procoder/config.toml", "[release]\nfiles = [\"fresh.txt\", \"stale.txt\"]\n")
	writeFile(t, root, "fresh.txt", "version 1.2.3\n")
	writeFile(t, root, "stale.txt", "version 1.2.2\n")
	// no CHANGELOG.md at all, and nothing committed — the tree is dirty

	out, lines := capture()
	if code := Run(root, "1.2.3", gateGreen, nil, out); code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, joined(lines))
	}
	got := joined(lines)
	for _, want := range []string{
		"NOT ready",
		"stale.txt does not contain 1.2.3",
		"CHANGELOG.md has no `## 1.2.3` heading",
		"working tree is dirty",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "fresh.txt does not contain") {
		t.Errorf("fresh.txt wrongly reported stale:\n%s", got)
	}
}

// A listed file that does not exist is its own named failure, never a skip.
func TestMissingListedFileIsAFailure(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeFile(t, root, ".procoder/config.toml", "[release]\nfiles = [\"gone.txt\"]\n")

	out, lines := capture()
	if code := Run(root, "1.2.3", gateGreen, nil, out); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(joined(lines), "gone.txt cannot be read") {
		t.Errorf("missing file not named:\n%s", joined(lines))
	}
}

// With everything fixed the controller prints the tag command, exits 0, and
// has tagged NOTHING — P-CONTROL, the agent runs the printed step.
func TestReadyPrintsTagCommandAndTagsNothing(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeFile(t, root, ".procoder/config.toml", "[release]\nfiles = [\"fresh.txt\", \"stale.txt\"]\n")
	writeFile(t, root, "fresh.txt", "version 1.2.3\n")
	writeFile(t, root, "stale.txt", "now also 1.2.3\n")
	writeFile(t, root, "CHANGELOG.md", "# changelog\n\n## 1.2.3\n\n- everything\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "release fixture")

	out, lines := capture()
	if code := Run(root, "1.2.3", gateGreen, nil, out); code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, joined(lines))
	}
	got := joined(lines)
	if !strings.Contains(got, "release 1.2.3 is ready") {
		t.Errorf("no ready line:\n%s", got)
	}
	if !strings.Contains(got, `git tag -a v1.2.3 -m "1.2.3"`) {
		t.Errorf("tag command not printed:\n%s", got)
	}
	if tags := strings.TrimSpace(git(t, root, "tag", "-l")); tags != "" {
		t.Errorf("the binary created tags: %q", tags)
	}
}

// An unset [release] files list says out loud that version-sync verified
// nothing — never a silent pass.
func TestEmptyReleaseFilesSaysVerifiedNothing(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeFile(t, root, "CHANGELOG.md", "## 1.2.3\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "fixture")

	out, lines := capture()
	if code := Run(root, "1.2.3", gateGreen, nil, out); code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, joined(lines))
	}
	if !strings.Contains(joined(lines), "version-sync verified nothing — set [release] files") {
		t.Errorf("verified-nothing line missing:\n%s", joined(lines))
	}
}

// No-argument mode reads the newest `## N.N.N` heading from CHANGELOG.md —
// changelogs run newest-first, so the first heading wins.
func TestNoArgumentReadsNewestChangelogVersion(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeFile(t, root, "CHANGELOG.md", "# changelog\n\n## 0.2.0 — today\n\n- new\n\n## 0.1.0\n\n- old\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "fixture")

	out, lines := capture()
	if code := Run(root, "", gateGreen, nil, out); code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, joined(lines))
	}
	got := joined(lines)
	if !strings.Contains(got, "newest version in CHANGELOG.md: 0.2.0") {
		t.Errorf("newest version not reported:\n%s", got)
	}
	if !strings.Contains(got, "release 0.2.0 is ready") {
		t.Errorf("checklist not run for 0.2.0:\n%s", got)
	}
}

// No argument and no changelog is a usage problem: the controller cannot
// guess a version, exit 2.
func TestNoArgumentWithoutChangelogExits2(t *testing.T) {
	root := t.TempDir()
	out, lines := capture()
	if code := Run(root, "", gateGreen, nil, out); code != 2 {
		t.Fatalf("exit %d, want 2\n%s", code, joined(lines))
	}
	if !strings.Contains(joined(lines), "CHANGELOG.md cannot be read") {
		t.Errorf("missing-changelog message absent:\n%s", joined(lines))
	}
}

// A malformed version is refused up front with the expected shape, exit 2.
func TestMalformedVersionExits2(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"v1.2", "banana", "1.2", "1.2.3.4"} {
		out, lines := capture()
		if code := Run(root, bad, gateGreen, nil, out); code != 2 {
			t.Errorf("%q: exit %d, want 2", bad, code)
		}
		if !strings.Contains(joined(lines), "expected N.N.N") {
			t.Errorf("%q: expected shape not named:\n%s", bad, joined(lines))
		}
	}
}

// A red suite blocks the release with the suite's own summary in the line.
func TestRedSuiteBlocksWithItsSummary(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeFile(t, root, "CHANGELOG.md", "## 1.2.3\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "fixture")

	redSuite := func() (bool, string) { return false, "go test: 2 packages failing" }
	out, lines := capture()
	if code := Run(root, "1.2.3", gateGreen, redSuite, out); code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, joined(lines))
	}
	if !strings.Contains(joined(lines), "test suite is not passing: go test: 2 packages failing") {
		t.Errorf("suite summary missing:\n%s", joined(lines))
	}
}

// A dirty gate is a failure line of its own.
func TestDirtyGateBlocks(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeFile(t, root, "CHANGELOG.md", "## 1.2.3\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "fixture")

	out, lines := capture()
	if code := Run(root, "1.2.3", func() bool { return false }, nil, out); code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, joined(lines))
	}
	if !strings.Contains(joined(lines), "the gate is not clean") {
		t.Errorf("gate failure missing:\n%s", joined(lines))
	}
}

// The controller said "release 0.2.0 is ready — tag it: git tag -a v0.2.0"
// for a version already tagged and already published on GitHub, and the
// command it printed answers "fatal: tag 'v0.2.0' already exists". Whether
// a version has shipped is the first thing a release controller ought to
// know, and an instruction that cannot succeed is worse than silence — the
// reader has to work out that procoder was wrong rather than their repo.
//
// Found by cutting a real release on the campaign's throwaway repository
// and then asking for the same version again.
//
// proved by: deleting the tagExists call from Run — this test then sees
// the controller pronounce an already-tagged version ready.
func TestAnAlreadyTaggedVersionIsNotReady(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeFile(t, root, "CHANGELOG.md", "# Changelog\n\n## 0.2.0\n\n- a thing\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "release: 0.2.0")
	git(t, root, "tag", "-a", "v0.2.0", "-m", "0.2.0")

	out, lines := capture()
	code := Run(root, "0.2.0", gateGreen, nil, out)

	if code == 0 {
		t.Fatalf("an already-tagged version was pronounced ready:\n%s", joined(lines))
	}
	if !strings.Contains(joined(lines), "v0.2.0 already exists") {
		t.Errorf("the refusal must name the tag:\n%s", joined(lines))
	}
	if strings.Contains(joined(lines), "git tag -a") {
		t.Errorf("it must not print a tag command that cannot succeed:\n%s", joined(lines))
	}
}

// The tag for a DIFFERENT version is not this version's tag. A check that
// matched loosely would refuse 0.2.0 because v0.2.0-rc1 or v10.2.0 exists.
//
// Two mechanisms keep that out and either alone suffices: `--list` is
// given the exact tag as its glob, and the result is compared for
// equality. So no SINGLE mutation fails this test — passing `tag+"*"` is
// caught by the equality, and swapping the equality for a Contains is
// caught by the pattern. That redundancy is deliberate and the honest way
// to state the proof is as the pair.
//
// proved by: making the pattern `tag+"*"` AND the comparison a
// strings.Contains, together — this test then reports 0.2.0 blocked by
// v0.2.0-rc1. Either mutation alone leaves it passing, which is what
// redundancy is for and why claiming a single-mutation proof here would
// have been false.
func TestAnUnrelatedTagDoesNotBlockTheRelease(t *testing.T) {
	requireGit(t)
	root := gitRepo(t)
	writeFile(t, root, "CHANGELOG.md", "# Changelog\n\n## 0.2.0\n\n- a thing\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "release: 0.2.0")
	git(t, root, "tag", "-a", "v0.2.0-rc1", "-m", "rc")
	git(t, root, "tag", "-a", "v10.2.0", "-m", "later")

	out, lines := capture()
	if code := Run(root, "0.2.0", gateGreen, nil, out); code != 0 {
		t.Fatalf("0.2.0 was blocked by a tag that is not v0.2.0:\n%s", joined(lines))
	}
}

// git failing to answer is not "no tag", for the same reason an unreadable
// tree is not a clean one.
//
// proved by: returning "" instead of the NOT-verified message when git
// errors — this test then sees a release pronounced ready by a check that
// could not run.
func TestTagCheckThatCannotRunIsNotAPass(t *testing.T) {
	if msg := tagExists(filepath.Join(t.TempDir(), "nonexistent"), "0.2.0"); msg == "" {
		t.Error("git failing to list tags must be reported, never read as no-tag")
	} else if !strings.Contains(msg, "NOT verified") {
		t.Errorf("the message must say the check did not run, got %q", msg)
	}
}
