package releases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
