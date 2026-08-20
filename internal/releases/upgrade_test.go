package releases

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// sumsFor is the SHA256SUMS body a release publishes for these asset bytes.
func sumsFor(body []byte) string {
	return fmt.Sprintf("%x  %s\n", sha256.Sum256(body), AssetName())
}

// runSums stands the whole thing up: a directory holding an "old binary",
// and a server answering the release query, the asset download and the
// checksums file. sums is the SHA256SUMS body the release publishes; a nil
// sums is a release that publishes no checksums asset at all.
func runSums(t *testing.T, current string, force bool, consent func(string) bool, body []byte, tag string, sums *string) (string, []string, int) {
	t.Helper()
	dir := t.TempDir()
	self := filepath.Join(dir, "procoder")
	if err := os.WriteFile(self, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := stubRelease(t, tag, body, sums)
	prevHost := APIHost
	APIHost = srv.URL
	t.Cleanup(func() { APIHost = prevHost })
	// The stub serves plain http from 127.0.0.1, which the real URL guard
	// refuses by design. The guard is exercised by its own tests below,
	// including one that leaves it in place and watches Upgrade refuse.
	prevCheck := checkAssetURL
	checkAssetURL = func(string) error { return nil }
	t.Cleanup(func() { checkAssetURL = prevCheck })

	var lines []string
	code := upgradeAt(self, current, force, consent, func(s string) { lines = append(lines, s) })
	return self, lines, code
}

// run is runSums with the checksums file the release honestly should have
// published for these bytes.
func run(t *testing.T, current string, force bool, consent func(string) bool, body []byte, tag string) (string, []string, int) {
	t.Helper()
	sums := sumsFor(body)
	return runSums(t, current, force, consent, body, tag, &sums)
}

// upgradeAt is Upgrade with the binary path injected, so a test never
// depends on where the test binary itself lives.
func upgradeAt(self, current string, force bool, consent func(string) bool, out func(string)) int {
	prev := selfPath
	selfPath = func() (string, error) { return self, nil }
	defer func() { selfPath = prev }()
	return Upgrade(current, force, consent, out)
}

func yes(string) bool { return true }
func no(string) bool  { return false }

// left names what is in the directory holding the binary, so a test can say
// "and nothing else was left behind".
func left(t *testing.T, self string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(self))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// The happy path, end to end: the new bytes are on disk, executable, the
// old binary is gone, and the verification is stated rather than implied.
func TestUpgradeReplacesTheBinaryOnlyAfterAYes(t *testing.T) {
	self, lines, code := run(t, "1.0.0", false, yes, []byte("new binary"), "v1.1.0")
	if code != 0 {
		t.Fatalf("exit %d: %v", code, lines)
	}
	got, err := os.ReadFile(self)
	if err != nil || string(got) != "new binary" {
		t.Fatalf("the binary was not replaced: %q %v", got, err)
	}
	info, err := os.Stat(self)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Errorf("the installed binary must be executable, got %v", info.Mode())
	}
	if !strings.Contains(strings.Join(lines, "\n"), "verified "+AssetName()) {
		t.Errorf("a verified install must say so: %v", lines)
	}
	// replace() moves the running binary aside; the aside copy is litter
	// once the rename succeeded.
	// proved by: dropped the final os.Remove(old) in replace — every
	// upgrade then leaves a procoder.old beside the binary forever.
	if names := left(t, self); len(names) != 1 {
		t.Errorf("a finished upgrade leaves one file, got %v", names)
	}
}

// The mode of the binary being replaced is the mode the replacement gets.
// proved by: went back to a fixed 0o755 — a 0700 install then becomes
// world-executable because an upgrade guessed.
func TestUpgradeKeepsTheModeTheBinaryAlreadyHad(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	dir := t.TempDir()
	self := filepath.Join(dir, "procoder")
	if err := os.WriteFile(self, []byte("old binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte("new binary")
	sums := sumsFor(body)
	srv := stubRelease(t, "v1.1.0", body, &sums)
	prevHost, prevCheck := APIHost, checkAssetURL
	APIHost = srv.URL
	checkAssetURL = func(string) error { return nil }
	t.Cleanup(func() { APIHost, checkAssetURL = prevHost, prevCheck })

	if code := upgradeAt(self, "1.0.0", false, yes, func(string) {}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	info, err := os.Stat(self)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("the replacement must keep mode 0700, got %v", info.Mode().Perm())
	}
}

// stubRelease serves a release, its asset and its checksums file. A nil
// sums is a release that publishes no checksums asset at all.
func stubRelease(t *testing.T, tag string, body []byte, sums *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases/latest"):
			// The asset URLs carry the server's own address, which is only
			// known once it is listening — so the payload is built here.
			assets := `{"name":"` + AssetName() + `","browser_download_url":"http://` + r.Host + `/download/` + AssetName() + `"}`
			if sums != nil {
				assets += `,{"name":"` + ChecksumsName + `","browser_download_url":"http://` + r.Host + `/download/` + ChecksumsName + `"}`
			}
			_, _ = w.Write([]byte(`{"tag_name":"` + tag + `","assets":[` + assets + `]}`))
		case strings.HasSuffix(r.URL.Path, "/"+ChecksumsName):
			if sums != nil {
				_, _ = io.WriteString(w, *sums)
			}
		default:
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// N-08: no consent, no download and no replacement.
// proved by: called install() before consent — the old binary is then
// overwritten by someone who said no.
func TestADeclinedUpgradeChangesNothing(t *testing.T) {
	self, lines, code := run(t, "1.0.0", false, no, []byte("new binary"), "v1.1.0")
	if code != 2 {
		t.Fatalf("a declined upgrade exits 2, got %d: %v", code, lines)
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the binary must be untouched, got %q", got)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "nothing was downloaded") {
		t.Errorf("the refusal must say nothing happened: %v", lines)
	}
	// Nothing is left beside it either.
	if names := left(t, self); len(names) != 1 {
		t.Errorf("a declined upgrade leaves no litter: %v", names)
	}
}

// N-06: refusing to move backwards, and the quiet answer when current.
func TestUpgradeRefusesToGoBackwardsAndSaysWhenCurrent(t *testing.T) {
	_, lines, code := run(t, "2.0.0", false, yes, nil, "v1.0.0")
	if code != 2 || !strings.Contains(strings.Join(lines, "\n"), "refusing to move backwards") {
		t.Errorf("ahead of the release must refuse: exit %d %v", code, lines)
	}
	_, lines, code = run(t, "1.0.0", false, yes, nil, "v1.0.0")
	if code != 0 || !strings.Contains(strings.Join(lines, "\n"), "nothing to do") {
		t.Errorf("current must exit 0 quietly: exit %d %v", code, lines)
	}
}

// A dev build is refused BEFORE GitHub is asked: there is nothing to
// compare it against, so the request could only spend a second to learn
// that.
// proved by: moved the Parse check back below Latest — point APIHost at a
// dead address and this test hangs for the timeout instead of answering.
func TestADevBuildIsRefusedWithoutAskingGitHub(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "procoder")
	if err := os.WriteFile(self, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	asked := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = true
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[]}`))
	}))
	t.Cleanup(srv.Close)
	prev := APIHost
	APIHost = srv.URL
	t.Cleanup(func() { APIHost = prev })

	var lines []string
	code := upgradeAt(self, Dev, false, yes, func(s string) { lines = append(lines, s) })
	if code != 2 || !strings.Contains(strings.Join(lines, "\n"), "no version") {
		t.Errorf("a dev build has nothing to compare: exit %d %v", code, lines)
	}
	if asked {
		t.Error("a dev build must not spend a request to learn it cannot compare")
	}
}

// The whole point: a tampered asset — same length, different bytes — is
// refused, and the binary that works is still there.
// proved by: kept the old code that printed the first 12 digits of the
// digest without comparing it to anything; the tampered bytes then install
// exactly as cleanly as the real ones.
func TestATamperedDownloadIsRefusedAndTheBinarySurvives(t *testing.T) {
	honest, tampered := []byte("new binary"), []byte("NEW BINARY")
	if len(honest) != len(tampered) {
		t.Fatal("the tampered body must be the same length, or Content-Length alone would catch it")
	}
	sums := sumsFor(honest)
	self, lines, code := runSums(t, "1.0.0", false, yes, tampered, "v1.1.0", &sums)
	if code != 2 {
		t.Fatalf("a mismatched download exits 2, got %d: %v", code, lines)
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the working binary must survive a mismatch, got %q", got)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "does NOT match") || !strings.Contains(joined, "still the binary") {
		t.Errorf("the refusal must name the mismatch and what is still installed: %v", lines)
	}
	if names := left(t, self); len(names) != 1 {
		t.Errorf("the rejected download must not be left beside the binary: %v", names)
	}
}

// A release that publishes no checksums file can prove nothing about what
// it served, and unknown is never the same as verified. The refusal lands
// before the download, and before the question.
// proved by: warned loudly and installed anyway — deleting one small file
// from a release is then the entire attack.
func TestAReleaseWithoutChecksumsIsRefused(t *testing.T) {
	consulted := false
	consent := func(string) bool { consulted = true; return true }
	self, lines, code := runSums(t, "1.0.0", false, consent, []byte("new binary"), "v1.1.0", nil)
	if code != 2 {
		t.Fatalf("an unverifiable release exits 2, got %d: %v", code, lines)
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the working binary must survive, got %q", got)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "publishes no "+ChecksumsName) || !strings.Contains(joined, "unverified is not the same as verified") {
		t.Errorf("the refusal must name the missing file and the reason: %v", lines)
	}
	if consulted {
		t.Error("there is nothing to consent to when the answer is already no")
	}
	if names := left(t, self); len(names) != 1 {
		t.Errorf("nothing was downloaded, so nothing is beside it: %v", names)
	}
}

// A checksums file that names every asset except this one verifies nothing
// about this one.
// proved by: returned an empty digest for a missing entry instead of an
// error; install then compares "" against "" for an empty download, or
// reports a mismatch that reads like corruption when the real fault is a
// check that never ran.
func TestChecksumsThatSkipThisAssetAreRefused(t *testing.T) {
	sums := strings.Repeat("a", 64) + "  procoder-some-other-platform\n"
	self, lines, code := runSums(t, "1.0.0", false, yes, []byte("new binary"), "v1.1.0", &sums)
	if code != 2 {
		t.Fatalf("an unlisted asset exits 2, got %d: %v", code, lines)
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the working binary must survive, got %q", got)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "names no entry for "+AssetName()) {
		t.Errorf("the refusal must name what it could not find: %v", lines)
	}
}

// A download that fails leaves the working binary in place — the rename is
// the last step, so there is no half-installed state to recover from.
// proved by: renamed before the copy finished; the binary is then truncated
// and the tool is gone.
func TestAFailedDownloadLeavesTheWorkingBinary(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "procoder")
	if err := os.WriteFile(self, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	sums := sumsFor([]byte("new binary"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases/latest"):
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[` +
				`{"name":"` + AssetName() + `","browser_download_url":"http://` + r.Host + `/download/` + AssetName() + `"},` +
				`{"name":"` + ChecksumsName + `","browser_download_url":"http://` + r.Host + `/download/` + ChecksumsName + `"}]}`))
		case strings.HasSuffix(r.URL.Path, "/"+ChecksumsName):
			_, _ = io.WriteString(w, sums)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	prevHost, prevCheck := APIHost, checkAssetURL
	APIHost = srv.URL
	checkAssetURL = func(string) error { return nil }
	defer func() { APIHost, checkAssetURL = prevHost, prevCheck }()

	var lines []string
	code := upgradeAt(self, "1.0.0", false, yes, func(s string) { lines = append(lines, s) })
	if code != 2 {
		t.Fatalf("a failed download exits 2, got %d: %v", code, lines)
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the working binary must survive, got %q", got)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "did NOT happen") || !strings.Contains(joined, "still the binary") {
		t.Errorf("the failure must say what is still installed: %v", lines)
	}
}

// A connection that dies mid-copy is the only path that reaches
// os.CreateTemp and then fails, so it is the only one that proves the
// partial file is cleaned up. The 500 above never gets that far.
// proved by: dropped `defer os.Remove(tmpName)`; a half-downloaded binary
// then sits beside the real one forever, one tab-complete away from being
// run.
func TestADownloadThatDiesMidCopyLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "procoder")
	if err := os.WriteFile(self, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	sums := sumsFor([]byte("new binary"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases/latest"):
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[` +
				`{"name":"` + AssetName() + `","browser_download_url":"http://` + r.Host + `/download/` + AssetName() + `"},` +
				`{"name":"` + ChecksumsName + `","browser_download_url":"http://` + r.Host + `/download/` + ChecksumsName + `"}]}`))
		case strings.HasSuffix(r.URL.Path, "/"+ChecksumsName):
			_, _ = io.WriteString(w, sums)
		default:
			// Promise a megabyte, send a few bytes, hang up. The client
			// sees an unexpected EOF partway through the copy.
			w.Header().Set("Content-Length", "1048576")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("partial"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			panic(http.ErrAbortHandler)
		}
	}))
	defer srv.Close()
	prevHost, prevCheck := APIHost, checkAssetURL
	APIHost = srv.URL
	checkAssetURL = func(string) error { return nil }
	defer func() { APIHost, checkAssetURL = prevHost, prevCheck }()

	var lines []string
	code := upgradeAt(self, "1.0.0", false, yes, func(s string) { lines = append(lines, s) })
	if code != 2 {
		t.Fatalf("a truncated download exits 2, got %d: %v", code, lines)
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the working binary must survive, got %q", got)
	}
	if names := left(t, self); len(names) != 1 {
		t.Errorf("the partial download must be cleaned up, found %v", names)
	}
}

// A server that keeps sending is not allowed to fill the filesystem the
// binary lives on. The ceiling is shrunk here rather than transferred.
// proved by: dropped the `n > maxAssetBytes` refusal — the oversized body
// then installs. (The LimitReader beside it is what stops the writing; only
// this check is what stops the install, so only this check is provable
// without moving a quarter of a gigabyte.)
func TestAnOversizedDownloadIsRefused(t *testing.T) {
	prevMax := maxAssetBytes
	maxAssetBytes = 16
	t.Cleanup(func() { maxAssetBytes = prevMax })

	body := []byte(strings.Repeat("x", 64))
	sums := sumsFor(body)
	self, lines, code := runSums(t, "1.0.0", false, yes, body, "v1.1.0", &sums)
	if code != 2 {
		t.Fatalf("an oversized download exits 2, got %d: %v", code, lines)
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the working binary must survive, got %q", got)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "larger than") {
		t.Errorf("the refusal must name the ceiling: %v", lines)
	}
	if names := left(t, self); len(names) != 1 {
		t.Errorf("the oversized download must be cleaned up, found %v", names)
	}
}

// The URL comes verbatim out of a release payload, and the bytes it returns
// are chmod'd and renamed over the user's only procoder. This is the one
// test that leaves the real guard in place.
// proved by: passed asset.URL straight to client.Get; a payload naming
// http://somewhere-else/ then downloads and installs.
func TestAnAssetURLOffGitHubIsNeverFetched(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "procoder")
	if err := os.WriteFile(self, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	fetched := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases/latest") {
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[` +
				`{"name":"` + AssetName() + `","browser_download_url":"http://` + r.Host + `/evil/` + AssetName() + `"},` +
				`{"name":"` + ChecksumsName + `","browser_download_url":"http://` + r.Host + `/evil/` + ChecksumsName + `"}]}`))
			return
		}
		fetched = true
	}))
	defer srv.Close()
	prev := APIHost
	APIHost = srv.URL
	defer func() { APIHost = prev }()

	var lines []string
	code := upgradeAt(self, "1.0.0", false, yes, func(s string) { lines = append(lines, s) })
	if code != 2 {
		t.Fatalf("an off-GitHub asset URL exits 2, got %d: %v", code, lines)
	}
	if fetched {
		t.Error("the download must be refused before the request, not after it")
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the working binary must survive, got %q", got)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "refusing to download over http") {
		t.Errorf("the refusal must name the reason: %v", lines)
	}
}

// D-3: a manager's binary is refused with that manager's own command, and
// --force is the way past it.
func TestAPackageManagerBinaryIsRefusedWithItsOwnCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the prefixes under test are POSIX paths")
	}
	cases := map[string]string{
		"/opt/homebrew/Cellar/procoder/1.0.0/bin/procoder": "brew upgrade",
		"/snap/procoder/current/bin/procoder":              "snap refresh",
		"/nix/store/abc-procoder-1.0.0/bin/procoder":       "nix profile upgrade",
		"/usr/bin/procoder":                                "package manager",
	}
	for path, want := range cases {
		hint, owned := ManagerOwned(path)
		if !owned {
			t.Errorf("%s belongs to a package manager", path)
			continue
		}
		if !strings.Contains(hint, want) {
			t.Errorf("%s must point at %q, got %q", path, want, hint)
		}
	}
	for _, mine := range []string{
		"/home/pascal/bin/procoder",
		// A directory that merely CONTAINS a manager's name somewhere in
		// the middle is the user's own file.
		// proved by: went back to strings.Contains on the POSIX list —
		// these are then all refused as somebody else's to upgrade.
		"/home/pascal/snap/procoder",
		"/home/pascal/projects/nix/store/procoder",
		"/tmp/usr/bin/procoder",
	} {
		if _, owned := ManagerOwned(mine); owned {
			t.Errorf("%s is the user's to replace", mine)
		}
	}
}

// NTFS is case-insensitive and a manager's directory sits under a drive
// letter, so the Windows entries are matched anywhere and case-blind.
// proved by: matched the Windows list case-sensitively —
// C:\Program Files\procoder\procoder.exe then walks straight past the guard
// and gets overwritten under the package database's nose.
func TestWindowsManagerPathsAreMatchedCaseBlind(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the prefixes under test are Windows paths")
	}
	for _, path := range []string{
		`C:\Program Files\procoder\procoder.exe`,
		`C:\Users\pascal\scoop\shims\procoder.exe`,
		`C:\ProgramData\Chocolatey\bin\procoder.exe`,
	} {
		if _, owned := ManagerOwned(path); !owned {
			t.Errorf("%s belongs to a package manager", path)
		}
	}
	if _, owned := ManagerOwned(`C:\Users\pascal\bin\procoder.exe`); owned {
		t.Error("a hand-installed binary is the user's to replace")
	}
}

// The checksums parser reads sha256sum's own output, and every way of not
// finding a usable digest is an error rather than an empty string.
// proved by: returned "" and nil for a missing name — install then compares
// the download against nothing at all.
func TestChecksumForReadsSha256sumFormat(t *testing.T) {
	digest := strings.Repeat("ab", 32)
	body := "# a comment line\n" +
		digest + "  procoder-linux-amd64\n" +
		strings.ToUpper(digest) + " *procoder-windows-amd64.exe\n" +
		"deadbeef  procoder-darwin-arm64\n"

	for _, tc := range []struct {
		name, want string
		wantErr    string
	}{
		{name: "procoder-linux-amd64", want: digest},
		// Binary mode writes "digest *name"; the star belongs to the
		// format, not to the file name.
		{name: "procoder-windows-amd64.exe", want: digest},
		{name: "procoder-darwin-arm64", wantErr: "not a sha256 digest"},
		{name: "procoder-linux-arm64", wantErr: "names no entry"},
	} {
		got, err := checksumFor([]byte(body), tc.name)
		switch {
		case tc.wantErr != "":
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%s: want an error naming %q, got %q %v", tc.name, tc.wantErr, got, err)
			}
		case err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case got != tc.want:
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
	if _, err := checksumFor(nil, "procoder-linux-amd64"); err == nil {
		t.Error("an empty checksums file verifies nothing and must say so")
	}
}

// The host is matched whole, over TLS only.
// proved by: matched the host with strings.Contains or HasSuffix —
// github.com.example.invalid and evil-github.com then pass as GitHub.
func TestOnlyGitHubOverHTTPSIsFetched(t *testing.T) {
	for _, ok := range []string{
		"https://github.com/azrtydxb/procoder/releases/download/v1/procoder-linux-amd64",
		"https://objects.githubusercontent.com/x",
		"https://GitHub.com/x",
	} {
		if err := gitHubAssetURL(ok); err != nil {
			t.Errorf("%s is a real release asset URL: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"http://github.com/x",
		"https://github.com.example.invalid/x",
		"https://evil-github.com/x",
		"https://raw.githubusercontent.com/x",
		"file:///etc/passwd",
		"://",
	} {
		if err := gitHubAssetURL(bad); err == nil {
			t.Errorf("%s must not be a place procoder is downloaded from", bad)
		}
	}
}

// dist/SHA256SUMS is what the release publishes and what self-upgrade
// verifies every download against. A digest that drifts from the binary it
// names does not fail quietly: it refuses every upgrade for every user, and
// it would only surface after a tag is cut. So it is checked against the
// committed binaries here, where a rebuild that forgot the script goes red
// in the test job instead.
// proved by: rebuilt one dist/ binary without rerunning
// scripts/build-dist.sh — this test then names the file whose digest no
// longer matches.
func TestTheCommittedChecksumsMatchTheCommittedBinaries(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "dist", "SHA256SUMS"))
	if err != nil {
		t.Skip("no committed checksums to compare against: ", err)
	}
	lines := 0
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		lines++
		name := strings.TrimPrefix(fields[1], "*")
		goos, goarch, ok := platformOf(name)
		if !ok {
			t.Errorf("%s is not a name AssetName could produce", name)
			continue
		}
		// The release stages dist/<goos>-<goarch>/procoder[.exe] under the
		// asset name; the digest has to be of the file that gets uploaded.
		binary := filepath.Join(root, "dist", goos+"-"+goarch, "procoder")
		if goos == "windows" {
			binary += ".exe"
		}
		bytes, err := os.ReadFile(binary)
		if err != nil {
			t.Errorf("%s names %s, which is not in dist/: %v", ChecksumsName, name, err)
			continue
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(bytes)); got != fields[0] {
			t.Errorf("%s records %s for %s, the committed binary is %s — rerun scripts/build-dist.sh", ChecksumsName, fields[0], name, got)
		}
	}
	if lines != 5 {
		t.Errorf("the release publishes five binaries, %s records %d", ChecksumsName, lines)
	}
}

// The asset name is spelled a third time, in the build script that writes
// the checksums file self-upgrade reads. A rename on either side leaves
// every download unverifiable — the refusal is loud, but it is still an
// outage, and it would only surface after a tag is cut.
// proved by: changed the printf in scripts/build-dist.sh to
// "procoder_%s%s" — this test then names every line it cannot account for.
func TestTheBuildScriptChecksumsTheNamesThisAsksFor(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "scripts", "build-dist.sh"))
	if err != nil {
		t.Skip("no build script to compare against: ", err)
	}
	if !strings.Contains(string(raw), `printf '%s  procoder-%s%s\n'`) {
		t.Fatal("the build script no longer writes 'procoder-<target><ext>' lines — repoint this test before trusting it")
	}
	targets := regexp.MustCompile(`for target in ([^;\n]+); do`).FindStringSubmatch(string(raw))
	if targets == nil {
		t.Fatal("no target list found in the build script — repoint this test before trusting it")
	}
	seen := map[string]bool{}
	for _, target := range strings.Fields(targets[1]) {
		goos, goarch, ok := strings.Cut(target, "-")
		if !ok {
			t.Errorf("%q is not a GOOS-GOARCH pair", target)
			continue
		}
		seen[assetNameFor(goos, goarch)] = true
	}
	// Every platform the release publishes must be one the script hashes,
	// or self-upgrade refuses there for want of a line.
	if !seen[AssetName()] && runtime.GOOS != "windows" {
		t.Errorf("the build script writes no checksum for %s — self-upgrade could never verify a download here", AssetName())
	}
	if len(seen) != 5 {
		t.Errorf("the release publishes five binaries, the script hashes %d: %v", len(seen), seen)
	}
}

// The one outcome the user must hear about in full: the new binary could
// not be renamed into place AND the old one could not be put back, so the
// tool they would use to recover is the one that is gone. Reporting only
// the first error hides that.
// proved by: swallowed the rollback error (`_ = os.Rename(old, self)`) —
// the message then names one failure and the user never learns their
// binary is sitting beside itself under another name.
func TestAFailedRollbackIsReportedInFull(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "procoder")
	if err := os.WriteFile(self, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Move it aside for real, then fail everything after: the first rename
	// is what the rollback has to undo, so it must actually happen.
	calls := 0
	prev := renameFile
	renameFile = func(from, to string) error {
		calls++
		if calls == 1 {
			return os.Rename(from, to)
		}
		return errors.New("device is full")
	}
	defer func() { renameFile = prev }()

	err := replace(filepath.Join(dir, "incoming"), self)
	if err == nil {
		t.Fatal("both renames failed; that is not a success")
	}
	for _, want := range []string{"rollback failed", "device is full", filepath.ToSlash(self + ".old"), filepath.ToSlash(self)} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message must carry %q so the binary can be recovered by hand: %v", want, err)
		}
	}
}
