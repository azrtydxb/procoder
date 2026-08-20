package releases

import (
	"fmt"
	"net/url"
	"strings"
)

// ChecksumsName is the file a release publishes beside its binaries: one
// sha256sum line per asset, written by scripts/build-dist.sh. The name is a
// constant rather than something read from the payload, because a checksums
// file an attacker gets to name is a checksums file an attacker gets to
// choose.
//
// debt: this file travels in the same release as the binary and carries no
// signature, so it proves a download matches what the release publishes —
// not who published it. The ceiling is corruption and a substituted asset;
// a compromised release account is above it. Revisit when GitHub build
// attestations are wired into the release job, the follow-up #79 names.
const ChecksumsName = "SHA256SUMS"

// checksums finds the release's checksums file. The miss is a real state a
// caller must decide about, not a zero Asset that would be fetched from an
// empty URL.
func (r Release) checksums() (Asset, bool) {
	for _, a := range r.Assets {
		if strings.EqualFold(a.Name, ChecksumsName) {
			return a, true
		}
	}
	return Asset{}, false
}

// checksumFor reads sha256sum's own format and returns the digest recorded
// for name. Every way of not finding one is an error: returning an empty
// string would be compared against the download and would match nothing,
// which reads like a mismatch but is really a check that never ran.
func checksumFor(sums []byte, name string) (string, error) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// sha256sum writes "digest  name" in text mode and "digest *name"
		// in binary mode; the star is part of the format, not of the name.
		if !strings.EqualFold(strings.TrimPrefix(fields[1], "*"), name) {
			continue
		}
		digest := strings.ToLower(fields[0])
		if !isSHA256(digest) {
			return "", fmt.Errorf("%s records %q for %s, which is not a sha256 digest", ChecksumsName, fields[0], name)
		}
		return digest, nil
	}
	return "", fmt.Errorf("%s names no entry for %s — there is nothing to verify the download against", ChecksumsName, name)
}

// isSHA256 reports whether s is exactly a 64-character hex digest. The
// length matters as much as the alphabet: a truncated digest would compare
// unequal to everything and turn a verification failure into a mystery.
func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// assetHosts are the only hosts a release binary is fetched from. GitHub
// serves release assets from github.com and redirects them to its object
// store; nothing else is a place procoder comes from.
var assetHosts = []string{"github.com", "objects.githubusercontent.com"}

// checkAssetURL vets a download URL before anything is fetched from it. It
// is a variable so the tests can point the download at their local stub —
// the same reason APIHost is one. Nothing reads it from config.
var checkAssetURL = gitHubAssetURL

// gitHubAssetURL refuses a URL that is not GitHub over TLS. The URL arrives
// verbatim in a release payload and the bytes it returns are made
// executable and renamed over the user's only procoder: an http:// URL, or
// a redirect that leaves TLS, hands that decision to whoever answers the
// request. Hosts are matched whole, never as a suffix, because
// github.com.example.invalid is somebody else's domain.
func gitHubAssetURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("the release names a download URL that cannot be read: %v", err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("refusing to download over %s — a release asset is fetched from GitHub over https or not at all (%s)", u.Scheme, raw)
	}
	for _, h := range assetHosts {
		if strings.EqualFold(u.Hostname(), h) {
			return nil
		}
	}
	return fmt.Errorf("refusing to download procoder from %s — a release asset comes from %s, and nowhere else", u.Hostname(), strings.Join(assetHosts, " or "))
}
