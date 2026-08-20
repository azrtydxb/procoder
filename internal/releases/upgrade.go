package releases

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// downloadTimeout is generous where the check is strict: nobody is waiting
// on a session start here, a person has just said yes, and a binary is
// megabytes rather than a JSON line.
const downloadTimeout = 2 * time.Minute

// managerPrefixes are the paths a package manager owns on each platform
// (D-3). Overwriting a file that brew, apt or scoop believes it installed
// leaves the user with a version their manager will silently revert on its
// next upgrade, and a checksum mismatch in somebody's audit.
//
// debt: prefix matching is a heuristic, not a query to the package
// database — revisit when a user reports either a false refusal on a
// hand-installed binary under /usr/local/bin or a manager we failed to
// spot. It errs toward refusing, and --force is the documented way past it.
func managerPrefixes() []string {
	if runtime.GOOS == "windows" {
		return []string{`\scoop\`, `\chocolatey\`, `\Program Files\`}
	}
	// One POSIX list rather than one per platform: a path that does not
	// exist on this system cannot match anything, and two lists drift.
	return []string{
		"/usr/bin/", "/opt/homebrew/", "/usr/local/Cellar/", "/opt/homebrew/Cellar/",
		"/usr/local/opt/", "/home/linuxbrew/", "/snap/", "/nix/store/", "/var/lib/flatpak/",
	}
}

// managerHint names the command that owns a path, so a refusal ends in
// something the user can run rather than in a dead end.
func managerHint(path string) string {
	switch {
	case strings.Contains(path, "Cellar") || strings.Contains(path, "homebrew") || strings.Contains(path, "linuxbrew"):
		return "brew upgrade procoder"
	case strings.Contains(path, "/snap/"):
		return "snap refresh procoder"
	case strings.Contains(path, "/nix/store/"):
		return "nix profile upgrade procoder"
	case strings.Contains(path, "scoop"):
		return "scoop update procoder"
	case strings.Contains(path, "chocolatey"):
		return "choco upgrade procoder"
	case strings.HasPrefix(path, "/usr/bin/"):
		return "your distribution's package manager (apt, dnf, pacman)"
	}
	return "the package manager that installed it"
}

// ManagerOwned reports whether this path looks like a package manager's to
// keep, and names the command that would upgrade it. Symlinks are resolved
// first: a Homebrew install is usually a link from /usr/local/bin into the
// cellar, and judging the link rather than its target would miss every one.
func ManagerOwned(path string) (string, bool) {
	real := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		real = resolved
	}
	for _, prefix := range managerPrefixes() {
		if strings.Contains(real, prefix) {
			return managerHint(real), true
		}
	}
	return "", false
}

// selfPath is the binary this process is running from, with symlinks
// resolved — the file an upgrade would replace (N-07). It is a variable so
// a test can point the controller at a scratch file: the alternative is a
// suite that overwrites the binary running it.
var selfPath = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// Upgrade installs a release over the running binary. It is the whole
// controller: every refusal is a printed reason and a non-zero exit, and
// the rename is the last thing that happens, so a failure anywhere leaves
// the working binary exactly where it was (N-04).
//
// consent is asked BEFORE anything is downloaded or executed (N-08). force
// skips only the package-manager refusal — never the consent.
func Upgrade(current string, force bool, consent func(latest string) bool, out func(string)) int {
	self, err := selfPath()
	if err != nil {
		out("cannot tell which binary is running, so there is nothing safe to replace: " + err.Error())
		return 2
	}
	if hint, owned := ManagerOwned(self); owned && !force {
		out("procoder at " + filepath.ToSlash(self) + " belongs to a package manager — upgrading it here would be reverted by the next")
		out("  " + hint)
		out("run that instead, or `procoder self-upgrade --force` if this install is really yours to replace")
		return 2
	}
	rel, err := Latest(Timeout)
	if err != nil {
		out("the latest version is NOT known — " + err.Error())
		return 2
	}
	switch Compare(current, rel.TagName) {
	case -1:
		// N-06: a maintainer on an unreleased branch is ahead, not behind.
		out(fmt.Sprintf("procoder %s is newer than the latest release (%s) — refusing to move backwards",
			current, strings.TrimPrefix(rel.TagName, "v")))
		return 2
	case 0:
		if _, known := Parse(current); !known {
			out("this build carries no version, so there is nothing to compare — install a release deliberately rather than upgrading")
			return 2
		}
		out("procoder " + current + " is the latest release — nothing to do")
		return 0
	}
	asset, err := rel.AssetFor(AssetName())
	if err != nil {
		out(err.Error())
		return 2
	}
	// The question comes before the download AND before anything runs: a
	// binary already on disk is a decision half made.
	if consent == nil || !consent(strings.TrimPrefix(rel.TagName, "v")) {
		out("nothing was downloaded and nothing was replaced")
		return 2
	}
	if err := install(self, asset, out); err != nil {
		out("the upgrade did NOT happen — " + err.Error())
		out("procoder " + current + " is still the binary at " + filepath.ToSlash(self))
		return 2
	}
	out("procoder upgraded to " + strings.TrimPrefix(rel.TagName, "v") + " at " + filepath.ToSlash(self))
	return 0
}

// install downloads the asset beside the target and renames it into place.
// Beside, not into the system temp directory: a rename across filesystems
// fails, and the fallback copy would be exactly the non-atomic write this
// avoids.
func install(self string, asset Asset, out func(string)) error {
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(asset.URL) // nosemgrep -- URL comes from the pinned repo's release payload
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s answered %s", asset.Name, resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(self), ".procoder-upgrade-*")
	if err != nil {
		return fmt.Errorf("cannot write beside the binary (%s): %v", filepath.ToSlash(filepath.Dir(self)), err)
	}
	tmpName := tmp.Name()
	// Any failure from here on removes the partial file: a half-downloaded
	// binary left beside the real one is litter at best and a foot-gun at
	// worst.
	defer func() { _ = os.Remove(tmpName) }()

	sum := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, sum), resp.Body); err != nil {
		_ = tmp.Close()
		return err
	}
	// The checked close: a write that fails on close is a failed write, and
	// renaming it over the working binary would install the failure.
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	out("downloaded " + asset.Name + " (sha256 " + hex.EncodeToString(sum.Sum(nil))[:12] + ")")
	return os.Rename(tmpName, self)
}
