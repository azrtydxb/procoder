package releases

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// Repo is the repository the check asks about. It is a constant rather than
// config: a procoder binary upgrades to procoder, and a configurable source
// would be a way to point an upgrade at somebody else's binary.
const Repo = "azrtydxb/procoder"

// APIHost is GitHub's API, overridable ONLY so tests can point at a local
// server. Nothing reads it from the environment or from config.
var APIHost = "https://api.github.com"

// Timeout caps the whole check. It runs on every session start, so the
// budget is what a user will never notice, not what a slow network needs
// (N-02).
const Timeout = time.Second

// Asset is one downloadable file on a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release is the shape this package reads from GitHub's answer. Everything
// else in the payload is ignored: the fewer fields, the fewer ways a change
// on their side breaks a session start on ours.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Latest asks GitHub for the newest published release. The error is the
// point of the signature: a check that could not run is NOT an up-to-date
// answer, and every caller has to decide what to do with that rather than
// receiving a silent empty string.
func Latest(timeout time.Duration) (Release, error) {
	client := &http.Client{Timeout: timeout}
	url := fmt.Sprintf("%s/repos/%s/releases/latest", APIHost, Repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	// The API version header keeps the shape stable; no token is sent, and
	// nothing about who is asking leaves the machine.
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Rate limiting is the common one and it is worth naming: a user
		// who sees "403" and a reset time knows to wait, where "could not
		// check" invites them to look for a bug.
		return Release{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	// A capped read: a body that never ends must not become memory that
	// never stops growing.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Release{}, err
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return Release{}, fmt.Errorf("GitHub's answer was not a release: %v", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return Release{}, fmt.Errorf("GitHub named no tag for the latest release")
	}
	return rel, nil
}

// AssetName is the file this platform needs from a release, matching the
// names the release workflow publishes.
func AssetName() string {
	name := fmt.Sprintf("procoder-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// AssetFor finds this platform's asset on a release. The miss is an error
// with the name that was looked for: a release published without one
// platform's binary is a real state, and the user needs to know which name
// was missing to see why.
func (r Release) AssetFor(name string) (Asset, error) {
	for _, a := range r.Assets {
		if strings.EqualFold(a.Name, name) {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("release %s publishes no %s — this platform has no binary in it", r.TagName, name)
}

// Check is the whole question in one call: what is running, what is newest,
// and is there anything to say. A caller that gets ok=false has an answer it
// could not obtain, never an up-to-date verdict.
func Check(current string, timeout time.Duration) (latest string, warn bool, err error) {
	if _, parsed := Parse(current); !parsed {
		// A dev build has nothing to compare against; asking GitHub would
		// spend a second to learn nothing.
		return "", false, nil
	}
	rel, err := Latest(timeout)
	if err != nil {
		return "", false, err
	}
	return rel.TagName, ShouldWarn(current, rel.TagName), nil
}

// WarningLine is the sentence a user sees when they are behind. It names
// both versions, because "a newer version exists" without saying which one
// leaves the reader unable to judge whether they care.
func WarningLine(current, latest string) string {
	return fmt.Sprintf("== procoder: newer version %s is available (current: %s) — `procoder self-upgrade` installs it",
		strings.TrimPrefix(latest, "v"), current)
}

// Stderr is where a version warning goes when the command's stdout belongs
// to a hook payload: a hook's stdout is parsed as JSON, and a friendly line
// in the middle of it is a corrupted payload (R-07, R-08).
var Stderr io.Writer = os.Stderr
