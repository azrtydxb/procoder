package releases

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// confirming is the consent `self-upgrade --confirm <sha256>` gives: an
// install pinned to the digest a person read.
func confirming(sum string) func(Pending) Consent {
	return func(Pending) Consent { return Consent{Install: true, Pinned: sum} }
}

// The handshake: with nobody to ask, the consent function reads the digest
// to show it, and declines. The digest is the release's, the binary never
// crosses the wire, and the refusal does not claim nothing was read.
// proved by: had PublishedSum read nothing and return "" — the printed hash
// then confirms nothing.
func TestTheHandshakeShowsThePublishedDigestAndInstallsNothing(t *testing.T) {
	body := []byte("new binary")
	var shown Pending
	var digest string
	assetFetches = 0
	self, lines, code := run(t, "1.0.0", false, func(p Pending) Consent {
		shown = p
		var err error
		if digest, err = p.PublishedSum(); err != nil {
			t.Fatal(err)
		}
		return Consent{}
	}, body, "v1.1.0")
	if code != 2 {
		t.Fatalf("a handshake installs nothing and exits 2, got %d: %v", code, lines)
	}
	if want := fmt.Sprintf("%x", sha256.Sum256(body)); digest != want {
		t.Errorf("the handshake must show the published digest %s, got %s", want, digest)
	}
	if shown.Current != "1.0.0" || shown.Latest != "1.1.0" || shown.Asset != AssetName() || !strings.HasSuffix(shown.URL, "/"+AssetName()) {
		t.Errorf("the pending upgrade must name versions, asset and URL: %+v", shown)
	}
	if assetFetches != 0 {
		t.Errorf("the binary must not be downloaded during a handshake, it was fetched %d time(s)", assetFetches)
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the binary must be untouched, got %q", got)
	}
	if joined := strings.Join(lines, "\n"); !strings.Contains(joined, "only "+ChecksumsName+" was read") {
		t.Errorf("the refusal must say what was read: %v", lines)
	}
}

// --confirm with the published digest installs — in either case, because a
// person may copy it from a tool that prints upper-case hex.
// proved by: compared the pin case-sensitively — the upper-case
// confirmation then refuses the very file it names.
func TestAConfirmedDigestInstalls(t *testing.T) {
	body := []byte("new binary")
	self, lines, code := run(t, "1.0.0", false, confirming(fmt.Sprintf("%X", sha256.Sum256(body))), body, "v1.1.0")
	if code != 0 {
		t.Fatalf("a confirmed digest installs, exit %d: %v", code, lines)
	}
	if got, _ := os.ReadFile(self); string(got) != "new binary" {
		t.Errorf("the binary was not replaced: %q", got)
	}
}

// A confirmed digest that is not this release's — a typo, or the handshake
// of an earlier release — refuses before the binary is fetched.
// proved by: dropped the pin check before install — the stale hash then
// reaches install, the binary is downloaded, and only the second check
// stops it.
func TestAWrongOrStaleConfirmedDigestRefuses(t *testing.T) {
	for name, pinned := range map[string]string{
		"wrong": strings.Repeat("ab", 32),
		"stale": fmt.Sprintf("%x", sha256.Sum256([]byte("the 1.0.5 binary"))),
	} {
		t.Run(name, func(t *testing.T) {
			assetFetches = 0
			self, lines, code := run(t, "1.0.0", false, confirming(pinned), []byte("new binary"), "v1.1.0")
			if code != 2 {
				t.Fatalf("a mismatched confirmation exits 2, got %d: %v", code, lines)
			}
			if got, _ := os.ReadFile(self); string(got) != "old binary" {
				t.Errorf("the binary must be untouched, got %q", got)
			}
			if assetFetches != 0 {
				t.Errorf("the binary must not be downloaded on a mismatched confirmation, fetched %d time(s)", assetFetches)
			}
			joined := strings.Join(lines, "\n")
			if !strings.Contains(joined, "the confirmed sha256 "+pinned+" is NOT the one") || !strings.Contains(joined, "1.1.0") {
				t.Errorf("the refusal must say the confirmed hash is not this release's: %v", lines)
			}
		})
	}
}

// The second check, inside install: bytes that match SHA256SUMS but not the
// confirmed digest are refused. Upgrade's pre-check makes this unreachable
// through Upgrade today, so it is exercised directly — it is what still
// holds if that pre-check is ever reordered or lost.
// proved by: removed the pinned comparison in install — the file then lands.
func TestInstallRefusesBytesThatAreNotTheConfirmedDigest(t *testing.T) {
	body := []byte("new binary")
	srv := stubRelease(t, "v1.1.0", body, nil)
	prev := checkAssetURL
	checkAssetURL = func(string) error { return nil }
	t.Cleanup(func() { checkAssetURL = prev })
	self := filepath.Join(t.TempDir(), "procoder")
	if err := os.WriteFile(self, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	published := fmt.Sprintf("%x", sha256.Sum256(body))
	asset := Asset{Name: AssetName(), URL: srv.URL + "/download/" + AssetName()}
	err := install(self, assetClient(), asset, published, strings.Repeat("cd", 32), func(string) {})
	if err == nil || !strings.Contains(err.Error(), "the sha256 that was confirmed") {
		t.Fatalf("install must refuse bytes that are not the confirmed digest, got %v", err)
	}
	if got, _ := os.ReadFile(self); string(got) != "old binary" {
		t.Errorf("the binary must be untouched, got %q", got)
	}
	if names := left(t, self); len(names) != 1 {
		t.Errorf("the refused download must not be left beside the binary: %v", names)
	}
}

// --force passes the package-manager refusal and nothing else: a confirmed
// digest that is wrong is still refused under it.
func TestForceDoesNotPassAWrongConfirmation(t *testing.T) {
	_, lines, code := run(t, "1.0.0", true, confirming(strings.Repeat("ab", 32)), []byte("new binary"), "v1.1.0")
	if code != 2 || !strings.Contains(strings.Join(lines, "\n"), "is NOT the one") {
		t.Errorf("--force must not skip the confirmation check: exit %d %v", code, lines)
	}
}

// A confirmation is still refused by a release moving backwards: the pin
// says which file, not that the version check may be skipped.
func TestAConfirmationDoesNotMoveBackwards(t *testing.T) {
	_, lines, code := run(t, "2.0.0", false, confirming(strings.Repeat("ab", 32)), nil, "v1.0.0")
	if code != 2 || !strings.Contains(strings.Join(lines, "\n"), "refusing to move backwards") {
		t.Errorf("a confirmation must not move backwards: exit %d %v", code, lines)
	}
}
