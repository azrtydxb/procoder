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

// maxAssetBytes caps a download. The asset is a Go binary of tens of
// megabytes; a server that keeps sending past this ceiling is filling the
// filesystem beside the very binary it claims to replace, and two minutes
// of that is a full disk. It is a variable only so a test can shrink it —
// the alternative is a suite that has to transfer a quarter of a gigabyte
// to prove the ceiling exists.
var maxAssetBytes int64 = 256 << 20

// maxRedirects caps a redirect chain. GitHub uses one hop to its object
// store; a chain longer than this is a loop or a tour, and every hop is a
// fresh chance to leave the hosts the download is allowed to come from.
const maxRedirects = 5

// managerOwner is one package manager's territory and the command that
// owns it. Prefix and hint live in one entry rather than in two parallel
// tables: two tables have to agree, and the path had to be scanned twice
// to consult both.
type managerOwner struct{ prefix, hint string }

// managerOwners are the paths a package manager owns on each platform
// (D-3). Overwriting a file that brew, apt or scoop believes it installed
// leaves the user with a version their manager will silently revert on its
// next upgrade, and a checksum mismatch in somebody's audit.
//
// debt: matching a path is a heuristic, not a query to the package
// database — revisit when a user reports either a false refusal on a
// hand-installed binary under /usr/local/bin or a manager we failed to
// spot. It errs toward refusing, and --force is the documented way past it.
func managerOwners() []managerOwner {
	if runtime.GOOS == "windows" {
		// Lower-case on purpose: these are matched case-insensitively,
		// because NTFS is.
		return []managerOwner{
			{`\scoop\`, "scoop update procoder"},
			{`\chocolatey\`, "choco upgrade procoder"},
			{`\program files\`, "the package manager that installed it"},
		}
	}
	// One POSIX list rather than one per platform: a path that does not
	// exist on this system cannot match anything, and two lists drift.
	return []managerOwner{
		{"/opt/homebrew/", "brew upgrade procoder"},
		{"/usr/local/Cellar/", "brew upgrade procoder"},
		{"/usr/local/opt/", "brew upgrade procoder"},
		{"/home/linuxbrew/", "brew upgrade procoder"},
		{"/snap/", "snap refresh procoder"},
		{"/nix/store/", "nix profile upgrade procoder"},
		{"/var/lib/flatpak/", "flatpak update procoder"},
		{"/usr/bin/", "your distribution's package manager (apt, dnf, pacman)"},
	}
}

// ownsPath matches a manager's territory against a real path. A POSIX
// territory is anchored at the root — HasPrefix, not Contains, or
// /home/pascal/snap-archive/bin/procoder is refused as snap's. A Windows
// one sits under a drive or user directory that cannot be anchored, so it
// is matched anywhere in the path and case-insensitively: a case-sensitive
// match let `C:\program files\procoder\procoder.exe` past the guard
// entirely.
func ownsPath(path, prefix string) bool {
	if runtime.GOOS == "windows" {
		return strings.Contains(strings.ToLower(path), prefix)
	}
	return strings.HasPrefix(path, prefix)
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
	for _, owner := range managerOwners() {
		if ownsPath(real, owner.prefix) {
			return owner.hint, true
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
// skips only the package-manager refusal — never the consent, and never
// the checksum.
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
	// Asked before GitHub is: a build with no version has nothing to
	// compare, so the request would spend a second to learn nothing.
	if _, known := Parse(current); !known {
		out("this build carries no version, so there is nothing to compare — install a release deliberately rather than upgrading")
		return 2
	}
	rel, err := Latest(Timeout)
	if err != nil {
		out("the latest version is NOT known — " + err.Error())
		return 2
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	switch Compare(current, rel.TagName) {
	case -1:
		// N-06: a maintainer on an unreleased branch is ahead, not behind.
		out(fmt.Sprintf("procoder %s is newer than the latest release (%s) — refusing to move backwards", current, latest))
		return 2
	case 0:
		out("procoder " + current + " is the latest release — nothing to do")
		return 0
	}
	asset, err := rel.AssetFor(AssetName())
	if err != nil {
		out(err.Error())
		return 2
	}
	// Refused here, before the question and before the network: a release
	// with no checksums file can never prove what it served, and this tool
	// does not treat unknown as verified. The alternative — warn and
	// install anyway — makes deleting one small file the whole attack.
	sums, published := rel.checksums()
	if !published {
		out("release " + latest + " publishes no " + ChecksumsName + ", so a download from it could NOT be checked against anything")
		out("unverified is not the same as verified — nothing was downloaded and nothing was replaced")
		return 2
	}
	// The question comes before the download AND before anything runs: a
	// binary already on disk is a decision half made.
	if consent == nil || !consent(latest) {
		out("nothing was downloaded and nothing was replaced")
		return 2
	}
	if err := install(self, asset, sums, out); err != nil {
		out("the upgrade did NOT happen — " + err.Error())
		out("procoder " + current + " is still the binary at " + filepath.ToSlash(self))
		return 2
	}
	out("procoder upgraded to " + latest + " at " + filepath.ToSlash(self))
	return 0
}

// assetClient fetches release assets and nothing else. Every URL is vetted
// before it is requested, and CheckRedirect vets each hop too: a download
// that starts on GitHub and is redirected off it is a download from
// somewhere else, and the bytes end up chmod'd and renamed over the user's
// only procoder.
func assetClient() *http.Client {
	return &http.Client{
		Timeout: downloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("the download redirected more than %d times", maxRedirects)
			}
			return checkAssetURL(req.URL.String())
		},
	}
}

// fetch performs one vetted GET. what names the file in the error, so a
// failure says which of the two downloads went wrong.
func fetch(client *http.Client, url, what string) (*http.Response, error) {
	if err := checkAssetURL(url); err != nil {
		return nil, err
	}
	resp, err := client.Get(url) // nosemgrep -- checkAssetURL vets scheme and host above, and every redirect hop
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("downloading %s answered %s", what, resp.Status)
	}
	return resp, nil
}

// expectedSum reads the digest the release publishes for one asset.
func expectedSum(client *http.Client, sums Asset, name string) (string, error) {
	resp, err := fetch(client, sums.URL, ChecksumsName)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	// A capped read, like every other body this package takes from the
	// network: a checksums file that never ends must not become memory
	// that never stops growing.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return checksumFor(body, name)
}

// install downloads the asset beside the target, proves it is the file the
// release published, and only then puts it in place. Beside, not in the
// system temp directory: a rename across filesystems fails, and the
// fallback copy would be exactly the non-atomic write this avoids.
func install(self string, asset, sums Asset, out func(string)) error {
	client := assetClient()
	// The checksum is fetched FIRST. A release whose checksums file cannot
	// be read can never verify the binary, and discovering that after
	// megabytes have crossed the wire is discovering it too late.
	want, err := expectedSum(client, sums, asset.Name)
	if err != nil {
		return err
	}

	resp, err := fetch(client, asset.URL, asset.Name)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(self), ".procoder-upgrade-*")
	if err != nil {
		return fmt.Errorf("cannot write beside the binary (%s): %v", filepath.ToSlash(filepath.Dir(self)), err)
	}
	tmpName := tmp.Name()
	// Any failure from here on removes the partial file: a half-downloaded
	// binary left beside the real one is litter at best and a foot-gun at
	// worst. It also removes the rejected file after a checksum mismatch,
	// so a refusal leaves nothing for anyone to run by accident.
	defer func() { _ = os.Remove(tmpName) }()

	sum := sha256.New()
	// The LimitReader is what stops the writing; the check below is what
	// stops the install. One byte past the ceiling is read on purpose:
	// reading exactly the ceiling cannot tell a file of that size from one
	// that was truncated at it.
	n, err := io.Copy(io.MultiWriter(tmp, sum), io.LimitReader(resp.Body, maxAssetBytes+1))
	if err != nil {
		_ = tmp.Close()
		return err
	}
	if n > maxAssetBytes {
		_ = tmp.Close()
		return fmt.Errorf("%s is larger than %d MiB — refusing to fill the disk beside the binary it would replace", asset.Name, maxAssetBytes>>20)
	}
	// rename(2) is atomic against other processes, not against a crash:
	// without this the directory entry can point at blocks that were never
	// written, with the old binary already gone.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	// The checked close: a write that fails on close is a failed write, and
	// renaming it over the working binary would install the failure.
	if err := tmp.Close(); err != nil {
		return err
	}

	got := hex.EncodeToString(sum.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%s does NOT match the digest %s publishes for it (expected %s, downloaded %s) — refusing to install bytes the release did not vouch for",
			asset.Name, ChecksumsName, want, got)
	}
	if err := os.Chmod(tmpName, installMode(self)); err != nil {
		return err
	}
	out("verified " + asset.Name + " against " + ChecksumsName + " (sha256 " + got[:12] + ")")
	return replace(tmpName, self)
}

// installMode is the mode the replacement gets: the one the binary being
// replaced already had. A fixed 0755 would take a tool somebody installed
// 0700 and hand it to every account on the machine. The owner-execute bit
// is forced on regardless, because an upgrade that lands unrunnable is an
// uninstall.
func installMode(self string) os.FileMode {
	if info, err := os.Stat(self); err == nil {
		return info.Mode().Perm() | 0o100
	}
	return 0o755
}

// replace puts the downloaded file at self. Windows holds an image-section
// lock on a running .exe: the file can be renamed, but it cannot be
// replaced, so os.Rename onto it always fails with "Access is denied" — on
// a platform where AssetName publishes procoder-windows-amd64.exe and the
// docs promise the command works. Moving the running binary aside first is
// the way through, and it runs on every platform rather than under a GOOS
// branch, so the path Windows depends on is the path the tests exercise.
func replace(tmpName, self string) error {
	old := self + ".old"
	// A leftover from a previous upgrade must not be what blocks this one.
	_ = os.Remove(old)
	if err := os.Rename(self, old); err != nil {
		return err
	}
	if err := os.Rename(tmpName, self); err != nil {
		// Put the working binary back. A failure here that left self
		// missing would have uninstalled procoder instead of upgrading it.
		_ = os.Rename(old, self)
		return err
	}
	// Best effort: Windows cannot delete the image of a running process, so
	// the file survives until it exits. A leftover .old is litter beside a
	// working install, never a broken one.
	_ = os.Remove(old)
	return nil
}
