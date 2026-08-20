package copilot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// issue is the shape gh prints, built here so the fixtures read like the API
// response they stand in for.
type issue struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// mk builds one issue: author, labels, title, body — with timestamps inside
// the window unless a test moves them.
func mk(author, title, body string, labels ...string) issue {
	var is issue
	is.Author.Login = author
	is.Title = title
	is.Body = body
	is.URL = "https://github.com/acme/widget/issues/7"
	is.CreatedAt = time.Now().Add(-time.Hour)
	is.UpdatedAt = time.Now().Add(-time.Hour)
	for _, l := range labels {
		is.Labels = append(is.Labels, struct {
			Name string `json:"name"`
		}{Name: l})
	}
	return is
}

// stubPath puts a fake `gh` and a fake `git` on PATH and makes that PATH the
// whole PATH. Find must reach gh the way any host does — through PATH — so the
// stub is a real executable; and a real gh installed on this machine would
// otherwise answer these tests, including the one about gh being missing.
// ghOut is what the fake gh prints; an empty ghOut means "install no gh at all".
// remote is what `git remote -v` prints.
func stubPath(t *testing.T, ghOut, remote string) (dir string, argsFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stubs are POSIX shell scripts")
	}
	dir = t.TempDir()
	argsFile = filepath.Join(dir, "gh.args")
	if ghOut != "" {
		// printf, not cat: PATH is the stub directory and nothing else, so the
		// script may use shell builtins only
		script := "#!/bin/sh\nprintf '%s' \"$*\" > " + argsFile + "\nprintf '%s' " + quoted(ghOut) + "\n"
		if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	git := "#!/bin/sh\nprintf '%s' " + quoted(remote) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return dir, argsFile
}

// quoted wraps text as a single shell word, apostrophes and all.
func quoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

const githubRemote = "origin\tgit@github.com:acme/widget.git (fetch)\n"

// find runs Find against a gh that answers with the given issues, and returns
// everything Find said and decided.
func find(t *testing.T, since time.Duration, issues ...issue) ([]Finding, bool, string, string) {
	t.Helper()
	raw, err := json.Marshal(issues)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		raw = []byte("[]")
	}
	_, argsFile := stubPath(t, string(raw), githubRemote)
	var said strings.Builder
	got, ok := Find(t.TempDir(), since, func(s string) { said.WriteString(s + "\n") })
	args, _ := os.ReadFile(argsFile)
	return got, ok, said.String(), string(args)
}

// The three ways an auto-review issue announces itself, and the two shapes that
// must not be mistaken for one. Any single signal is enough: instances rename
// their bot, drop the label, or change the template, and losing a real escape
// because two of three signals were missing is the failure that matters.
// proved by: turned any one of the three matches in fromCopilot into a
// conjunction — the two cases relying on the other signals then vanish.
func TestWhatCountsAsACopilotAutoReview(t *testing.T) {
	quote := "---\n" + reviewQuote + " reviewed this: nil map write in store.go:41\n"
	cases := []struct {
		name  string
		in    issue
		match bool
	}{
		{"the auto-copilot label", mk("someone", "unchecked error", "the error is dropped", AutoLabel), true},
		{"copilot[bot] as author", mk("copilot[bot]", "unchecked error", "the error is dropped"), true},
		{"copilot-preview[bot] as author", mk("copilot-preview[bot]", "off-by-one", "the loop runs one short"), true},
		{"the review quote block alone", mk("someone", "nil map", quote), true},
		{"a human issue with no signal", mk("pascal", "please add a flag", "it would be nice"), false},
		{"a bot that is not Copilot", mk("dependabot[bot]", "bump x", "bumps x from 1 to 2"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok, said, _ := find(t, time.Hour*24, c.in)
			if !ok {
				t.Fatalf("the question was answerable, got NOT checked: %s", said)
			}
			if (len(got) == 1) != c.match {
				t.Fatalf("match=%v wanted %v, got %+v", len(got) == 1, c.match, got)
			}
		})
	}
}

// A matched issue carries what the rest of the pipeline needs: where it came
// from, which repository, and the line Copilot cited.
// proved by: dropped the Line/Repo assignment in Find — the finding still
// exists but points nowhere.
func TestAMatchedIssueCarriesItsOriginAndPosition(t *testing.T) {
	body := "The map is written without a nil check in internal/store/store.go:41 — guard it."
	got, ok, said, _ := find(t, 24*time.Hour, mk("copilot[bot]", "nil map write", body))
	if !ok || len(got) != 1 {
		t.Fatalf("wanted one finding, got %+v (ok=%v) %s", got, ok, said)
	}
	f := got[0]
	if f.OriginalURL == "" || f.Repo != "acme/widget" || f.Line != 41 || f.Title != "nil map write" {
		t.Fatalf("finding lost its metadata: %+v", f)
	}
	if f.Body != body {
		t.Fatal("Find must hand the body over untouched — sanitising is Sanitise's job, not a side effect here")
	}
}

// Wishes are not escapes. A feature request in the ledger teaches nothing about
// which gate let a bug through, and noise is how a ledger stops being read.
// proved by: removed the aboutCodeQuality call — both cases below match.
func TestIssuesThatAreNotAboutCodeQualityAreSkipped(t *testing.T) {
	cases := []issue{
		mk("copilot[bot]", "Feature request: add a --json flag", "it would be handy"),
		mk("copilot[bot]", "Improve the README", "documentation suggestion: expand the install section", "documentation"),
	}
	for _, c := range cases {
		got, ok, said, _ := find(t, 24*time.Hour, c)
		if !ok {
			t.Fatalf("NOT checked where it should have answered: %s", said)
		}
		if len(got) != 0 {
			t.Fatalf("%q is not a defect and must be skipped, got %+v", c.Title, got)
		}
	}
}

// The window is the caller's, and it is applied to the timestamps as well as
// to the search string — gh's search rounds, and a finding from last week
// reported as fresh is a lie the caller cannot detect.
// proved by: deleted the `within` call in Find — the stale issue is returned;
// dropped --search from the argv — the args assertion fails.
func TestTheWindowIsBothAskedForAndEnforced(t *testing.T) {
	stale := mk("copilot[bot]", "old finding", "long since fixed")
	stale.CreatedAt = time.Now().Add(-72 * time.Hour)
	stale.UpdatedAt = time.Now().Add(-72 * time.Hour)
	fresh := mk("copilot[bot]", "new finding", "still open")

	got, ok, said, args := find(t, 6*time.Hour, stale, fresh)
	if !ok {
		t.Fatalf("NOT checked where it should have answered: %s", said)
	}
	if len(got) != 1 || got[0].Title != "new finding" {
		t.Fatalf("only the issue inside the window may be returned, got %+v", got)
	}
	if !strings.Contains(args, "--search updated:>=") {
		t.Fatalf("gh must be asked for the window, not filtered blind: %q", args)
	}
}

// No gh is not "no findings". This is the whole reason Find returns a second
// value: a caller that cannot tell the two apart eventually prints a clean
// verdict for a check that never ran.
// proved by: returned (nil, true) when LookPath fails — this test is the only
// thing standing between that and a silently green report.
func TestGhNotInstalledIsReportedAsNotChecked(t *testing.T) {
	stubPath(t, "", githubRemote) // an empty gh output means: install no gh
	var said strings.Builder
	got, ok := Find(t.TempDir(), 24*time.Hour, func(s string) { said.WriteString(s + "\n") })
	if ok {
		t.Fatal("a missing gh must not be reported as an answered question")
	}
	if len(got) != 0 {
		t.Fatalf("no gh, no findings: %+v", got)
	}
	if !strings.Contains(said.String(), "NOT checked") || !strings.Contains(said.String(), "gh is not installed") {
		t.Fatalf("the reason must be said out loud, got %q", said.String())
	}
}

// gh answering with something we cannot parse — a changed API shape, an auth
// prompt on stdout, a truncated response — is the same unknown as no gh at all.
// proved by: made parseIssues return an empty list instead of an error on bad
// JSON — Find then reports clean.
func TestMalformedGhOutputIsNotChecked(t *testing.T) {
	for _, raw := range []string{"not json at all", "", "{\"issues\": []}"} {
		stubPath(t, raw, githubRemote)
		var said strings.Builder
		got, ok := Find(t.TempDir(), 24*time.Hour, func(s string) { said.WriteString(s + "\n") })
		// an empty raw installs no gh at all, which is the other unknown; both
		// must land on the same side of the answer
		if ok || len(got) != 0 {
			t.Fatalf("output %q must be NOT checked, got %+v (ok=%v)", raw, got, ok)
		}
		if !strings.Contains(said.String(), "NOT checked") {
			t.Fatalf("output %q must say why, got %q", raw, said.String())
		}
	}
}

// The honest empty answer: gh ran, GitHub replied, nothing matched. This is the
// only shape allowed to return an empty slice with true.
// proved by: made Find return false on an empty result set — every clean repo
// would then nag that the check could not run, and the warning stops meaning
// anything.
func TestNoMatchingIssuesIsAnAnswerNotAnUnknown(t *testing.T) {
	got, ok, said, _ := find(t, 24*time.Hour)
	if !ok {
		t.Fatalf("an empty answer is still an answer: %s", said)
	}
	if len(got) != 0 {
		t.Fatalf("wanted no findings, got %+v", got)
	}
	if said != "" {
		t.Fatalf("nothing to report means nothing printed, got %q", said)
	}
}

// A repository with no GitHub remote has nothing to query — that is an absence,
// not an unknown, and it must pass in silence rather than warn on every run of
// a repo that will never have Copilot issues.
// proved by: returned false for a missing GitHub remote — every non-GitHub repo
// then reports a check it could never run.
func TestNoGitHubRemoteIsQuietAndAnswered(t *testing.T) {
	stubPath(t, "[]", "origin\tgit@gitlab.com:acme/widget.git (fetch)\n")
	var said strings.Builder
	got, ok := Find(t.TempDir(), 24*time.Hour, func(s string) { said.WriteString(s + "\n") })
	if !ok || len(got) != 0 || said.String() != "" {
		t.Fatalf("no GitHub remote must be a silent empty answer, got %+v (ok=%v) %q", got, ok, said.String())
	}
}

// Git failing outright is different again: we do not know whether there is a
// GitHub remote, so we do not know whether there was anything to find.
// proved by: folded the git failure into the "no GitHub remote" branch — a
// broken git then reads as a clean repo.
func TestAGitThatCannotAnswerIsNotAGitHublessRepo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stubs are POSIX shell scripts")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\nprintf '[]'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\nexit 128\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	var said strings.Builder
	got, ok := Find(t.TempDir(), 24*time.Hour, func(s string) { said.WriteString(s + "\n") })
	if ok || len(got) != 0 {
		t.Fatalf("a git that failed leaves the question open, got %+v (ok=%v)", got, ok)
	}
	if !strings.Contains(said.String(), "NOT checked") {
		t.Fatalf("the reason must be said out loud, got %q", said.String())
	}
}

// gh refusing — most often an expired token — must carry gh's own words and the
// fix it named, not a generic failure.
// proved by: dropped stderr from the ghReason call — the user is told the check
// failed but not that `gh auth login` is what fixes it.
func TestGhRefusingCarriesItsOwnReasonAndHint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stubs are POSIX shell scripts")
	}
	dir := t.TempDir()
	gh := "#!/bin/sh\necho 'gh: To get started with GitHub CLI, please run: gh auth login' 1>&2\nexit 4\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	git := "#!/bin/sh\nprintf '%s' '" + githubRemote + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	var said strings.Builder
	_, ok := Find(t.TempDir(), 24*time.Hour, func(s string) { said.WriteString(s + "\n") })
	if ok {
		t.Fatal("a gh that refused answered nothing")
	}
	if !strings.Contains(said.String(), "gh auth login") {
		t.Fatalf("the fix gh named must survive into the report, got %q", said.String())
	}
}

// Find is called with a nil out by callers that only want the findings; a
// panic there would take the gate down over a reporting detail.
// proved by: called out directly instead of through the nil-guarded say.
func TestANilReporterIsNotACrash(t *testing.T) {
	stubPath(t, "", githubRemote)
	if _, ok := Find(t.TempDir(), 24*time.Hour, nil); ok {
		t.Fatal("still NOT checked, just unreported")
	}
}
