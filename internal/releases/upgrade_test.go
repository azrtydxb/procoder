package releases

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeInstall builds a directory holding a "binary", and a server serving a
// release whose asset is the new one. It returns the path of the binary and
// the lines the controller printed.
func run(t *testing.T, current string, force bool, consent func(string) bool, body []byte, tag string) (string, []string, int) {
	t.Helper()
	dir := t.TempDir()
	self := filepath.Join(dir, "procoder")
	if err := os.WriteFile(self, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases/latest") {
			_, _ = w.Write([]byte(`{"tag_name":"` + tag + `","assets":[{"name":"` + AssetName() + `","browser_download_url":"` + assetURL + `"}]}`))
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	prevHost, prevAsset := APIHost, assetURL
	APIHost = srv.URL
	assetURL = srv.URL + "/download/" + AssetName()
	// The release payload is written with the asset URL, so it is rebuilt
	// once the server's address is known.
	t.Cleanup(func() { APIHost, assetURL = prevHost, prevAsset })

	var lines []string
	code := upgradeAt(self, current, force, consent, func(s string) { lines = append(lines, s) })
	return self, lines, code
}

var assetURL string

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

// The happy path, end to end: the new bytes are on disk, executable, and
// the old binary is gone.
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
	entries, _ := os.ReadDir(filepath.Dir(self))
	if len(entries) != 1 {
		t.Errorf("a declined upgrade leaves no litter: %v", entries)
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
	_, lines, code = run(t, Dev, false, yes, nil, "v1.0.0")
	if code != 2 || !strings.Contains(strings.Join(lines, "\n"), "no version") {
		t.Errorf("a dev build has nothing to compare: exit %d %v", code, lines)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases/latest") {
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[{"name":"` + AssetName() + `","browser_download_url":"` + srvURL(r) + `/download"}]}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	prev := APIHost
	APIHost = srv.URL
	defer func() { APIHost = prev }()

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

func srvURL(r *http.Request) string { return "http://" + r.Host }

// D-3: a manager's binary is refused with that manager's own command, and
// --force is the way past it.
func TestAPackageManagerBinaryIsRefusedWithItsOwnCommand(t *testing.T) {
	cases := map[string]string{
		"/opt/homebrew/Cellar/procoder/1.0.0/bin/procoder": "brew upgrade",
		"/snap/procoder/current/bin/procoder":              "snap refresh",
		"/nix/store/abc-procoder-1.0.0/bin/procoder":       "nix profile upgrade",
		"/usr/bin/procoder":                                "package manager",
	}
	for path, want := range cases {
		if runtime.GOOS == "windows" {
			continue // the prefixes under test are POSIX paths
		}
		hint, owned := ManagerOwned(path)
		if !owned {
			t.Errorf("%s belongs to a package manager", path)
			continue
		}
		if !strings.Contains(hint, want) {
			t.Errorf("%s must point at %q, got %q", path, want, hint)
		}
	}
	if _, owned := ManagerOwned("/home/pascal/bin/procoder"); owned {
		t.Error("a hand-installed binary is the user's to replace")
	}
}
