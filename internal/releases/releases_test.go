package releases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stub stands in for GitHub. Every test that would otherwise reach the
// network points APIHost here: a suite that depends on api.github.com fails
// on a plane, in CI without egress, and whenever somebody is rate limited.
func stub(t *testing.T, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/repos/"+Repo+"/releases/latest") {
			t.Errorf("asked for the wrong path: %s", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prev := APIHost
	APIHost = srv.URL
	t.Cleanup(func() { APIHost = prev })
}

func releaseJSON(t *testing.T, tag string, assets ...string) string {
	t.Helper()
	r := Release{TagName: tag}
	for _, a := range assets {
		r.Assets = append(r.Assets, Asset{Name: a, URL: "https://example.invalid/" + a})
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLatestReadsTheTagAndItsAssets(t *testing.T) {
	stub(t, http.StatusOK, releaseJSON(t, "v1.2.3", "procoder-linux-amd64", AssetName()))
	rel, err := Latest(Timeout)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.TagName != "v1.2.3" {
		t.Errorf("tag = %q", rel.TagName)
	}
	if _, err := rel.AssetFor(AssetName()); err != nil {
		t.Errorf("this platform's asset must be found: %v", err)
	}
}

// Every way the question can fail to be answered, and the one rule that
// binds them: an error is never an up-to-date verdict.
// proved by: returned ("", nil) for a non-200 — the caller then reports the
// user is current on the strength of a rate-limit page.
func TestAnUnanswerableCheckIsNeverUpToDate(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
		wants      string
	}{
		{"rate limited", `{"message":"API rate limit exceeded"}`, http.StatusForbidden, "403"},
		{"not found", `{}`, http.StatusNotFound, "404"},
		{"not json", `<html>maintenance</html>`, http.StatusOK, "not a release"},
		{"no tag", `{"assets":[]}`, http.StatusOK, "named no tag"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stub(t, c.status, c.body)
			if _, err := Latest(Timeout); err == nil {
				t.Fatal("a check that could not run must return an error, not an empty answer")
			} else if !strings.Contains(err.Error(), c.wants) {
				t.Errorf("the reason must say what happened: %v", err)
			}
			if _, warn, err := Check("1.0.0", Timeout); err == nil || warn {
				t.Errorf("Check must surface the failure and never warn on it: warn=%v err=%v", warn, err)
			}
		})
	}
}

// N-02: the timeout is a promise about how long a session start can wait.
// proved by: dropped the client Timeout — the test then hangs on the
// deliberately slow handler instead of failing in a second.
func TestTheTimeoutIsEnforced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()
	prev := APIHost
	APIHost = srv.URL
	defer func() { APIHost = prev }()

	start := time.Now()
	_, err := Latest(50 * time.Millisecond)
	if err == nil {
		t.Fatal("a slow GitHub must time out, not answer")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("the deadline was not enforced: waited %s", elapsed)
	}
}

// A dev build asks GitHub nothing: there is no version to compare, and a
// second spent learning that is a second of every session start.
func TestADevBuildDoesNotEvenAsk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a dev build must not query GitHub at all")
	}))
	defer srv.Close()
	prev := APIHost
	APIHost = srv.URL
	defer func() { APIHost = prev }()

	latest, warn, err := Check(Dev, Timeout)
	if latest != "" || warn || err != nil {
		t.Errorf("dev = (%q, %v, %v), want the quiet answer", latest, warn, err)
	}
}

func TestCheckWarnsOnlyWhenBehind(t *testing.T) {
	stub(t, http.StatusOK, releaseJSON(t, "v2.0.0"))
	if _, warn, err := Check("1.9.9", Timeout); err != nil || !warn {
		t.Errorf("behind must warn: warn=%v err=%v", warn, err)
	}
	if _, warn, err := Check("2.0.0", Timeout); err != nil || warn {
		t.Errorf("current must stay quiet: warn=%v err=%v", warn, err)
	}
	if _, warn, err := Check("2.1.0", Timeout); err != nil || warn {
		t.Errorf("ahead of the newest release is not behind it: warn=%v err=%v", warn, err)
	}
}

// A release that shipped without this platform's binary is a real state,
// and the miss names the file it looked for.
func TestAMissingAssetNamesWhatItLookedFor(t *testing.T) {
	rel := Release{TagName: "v1.0.0", Assets: []Asset{{Name: "procoder-plan9-mips"}}}
	_, err := rel.AssetFor(AssetName())
	if err == nil {
		t.Fatal("a release with no asset for this platform cannot be installed")
	}
	if !strings.Contains(err.Error(), AssetName()) || !strings.Contains(err.Error(), "v1.0.0") {
		t.Errorf("the error must name the asset and the release: %v", err)
	}
}

func TestAssetNameFollowsThePlatform(t *testing.T) {
	want := fmt.Sprintf("procoder-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if got := AssetName(); got != want {
		t.Errorf("AssetName() = %q, want %q", got, want)
	}
}

func TestWarningLineNamesBothVersions(t *testing.T) {
	got := WarningLine("1.0.0", "v1.2.0")
	for _, want := range []string{"1.2.0", "1.0.0", "self-upgrade"} {
		if !strings.Contains(got, want) {
			t.Errorf("the warning must carry %q: %s", want, got)
		}
	}
	if strings.Contains(got, "v1.2.0") {
		t.Errorf("the tag's v prefix is not part of a version a user reads: %s", got)
	}
}

// A tag the comparator cannot read is a question that did not get answered.
// It compares as equal — which "you are current" is indistinguishable from —
// so the check has to refuse it at the door rather than pass the ambiguity
// on to a caller that will print a verdict.
// proved by: returned rel with the unparseable tag — Check then answers
// warn=false, err=nil and `version --check` prints "is the latest release"
// against a nightly build nobody can compare against.
func TestATagThatCannotBeComparedIsNotAnUpToDateAnswer(t *testing.T) {
	for _, tag := range []string{"nightly-2026-08-21", "latest", "release-2026.08"} {
		stub(t, http.StatusOK, `{"tag_name":"`+tag+`"}`)
		if _, err := Latest(Timeout); err == nil {
			t.Errorf("tag %q cannot be compared and must not be returned as an answer", tag)
		} else if !strings.Contains(err.Error(), tag) {
			t.Errorf("the reason must name the tag it could not read: %v", err)
		}
		_, warn, err := Check("1.0.0", Timeout)
		if err == nil {
			t.Errorf("Check must surface it: tag %q", tag)
		}
		if warn {
			t.Errorf("nothing is known to be newer: tag %q", tag)
		}
	}
}

// The asset name is spelled twice: here, derived from GOOS/GOARCH, and in
// the release workflow, which stages the files by hand. They agree today,
// and nothing has ever checked that they do — a rename on either side
// breaks self-upgrade for every user, with no test going red and the
// failure surfacing only after a tag is cut.
// proved by: changed the prefix in AssetName to "procoder_" — this test
// then names every workflow asset it can no longer account for.
func TestEveryReleaseAssetIsANameThisCanAskFor(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Skip("no workflow to compare against: ", err)
	}
	// The staged basenames, as the release job writes them.
	staged := regexp.MustCompile(`/tmp/assets/(procoder-[a-z0-9]+-[a-z0-9]+(?:\.exe)?)`).FindAllStringSubmatch(string(raw), -1)
	if len(staged) == 0 {
		t.Fatal("no staged assets found in the workflow — repoint this test before trusting it")
	}
	seen := map[string]bool{}
	for _, m := range staged {
		seen[m[1]] = true
	}
	for name := range seen {
		goos, goarch, ok := platformOf(name)
		if !ok {
			t.Errorf("%s is not a name AssetName could produce", name)
			continue
		}
		if want := assetNameFor(goos, goarch); want != name {
			t.Errorf("the workflow stages %s where this package asks for %s", name, want)
		}
	}
	// The platform running this test must be one the release actually
	// publishes, or self-upgrade cannot work here.
	if !seen[AssetName()] && runtime.GOOS != "windows" {
		t.Errorf("no release asset named %s — self-upgrade has nothing to fetch on this platform", AssetName())
	}
}

// platformOf reverses an asset name into the pair that produced it.
func platformOf(name string) (goos, goarch string, ok bool) {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(name, "procoder-"), ".exe")
	parts := strings.SplitN(trimmed, "-", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// The request names the caller. GitHub accepts Go's default agent — the
// live check worked without this — but every Go program on a machine sends
// the same one, so a rate limit or a block is unattributable in GitHub's
// logs and in the user's.
// proved by: dropped the header — the stub then sees Go-http-client and the
// assertion fails.
func TestTheRequestNamesProcoder(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0"}`))
	}))
	defer srv.Close()
	prevHost, prevRunning := APIHost, Running
	APIHost, Running = srv.URL, "1.2.3"
	defer func() { APIHost, Running = prevHost, prevRunning }()

	if _, err := Latest(Timeout); err != nil {
		t.Fatal(err)
	}
	if got != "procoder/1.2.3" {
		t.Errorf("User-Agent = %q, want procoder/1.2.3", got)
	}
}
