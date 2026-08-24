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
	return stubPathWithReviews(t, ghOut, "[]", remote)
}

// stubPathWithReviews is stubPath plus the payload the review-comment call
// gets, for the tests that exercise the second source.
func stubPathWithReviews(t *testing.T, ghOut, reviewOut, remote string) (dir string, argsFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stubs are POSIX shell scripts")
	}
	dir = t.TempDir()
	argsFile = filepath.Join(dir, "gh.args")
	if ghOut != "" {
		// printf, not cat: PATH is the stub directory and nothing else, so the
		// script may use shell builtins only.
		//
		// Find makes TWO calls now — `issue list` and `api .../pulls/comments`
		// — so the stub branches on the subcommand and APPENDS its args.
		// Answering both with the same payload and overwriting the args file
		// made the second call erase the first's record, which is how the
		// window assertion came to read the wrong argv.
		script := "#!/bin/sh\n" +
			"printf '%s\\n' \"$*\" >> " + argsFile + "\n" +
			"if [ \"$1\" = api ]; then printf '%s' " + quoted(reviewOut) + "; else printf '%s' " + quoted(ghOut) + "; fi\n"
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

// TestOurOwnIssuesAreNeverCapturedAgain pins the loop the feature shipped
// with: Capture files each issue under AutoLabel, which is exactly what this
// finder queries, so a second run inside the same window found run one's
// issues and filed them again — three runs, four times the issues, and a
// ledger to match. OwnLabel is the mark that breaks the cycle, and the
// hand-filing template carries it too.
// proved by: removed the ours() guard in Find — the procoder-filed issue
// comes back as a fresh finding.
func TestOurOwnIssuesAreNeverCapturedAgain(t *testing.T) {
	mine := mk("pascal", "nil map write", "the map is written without a nil check", AutoLabel, OwnLabel)
	got, ok, said, _ := find(t, 24*time.Hour, mine)
	if !ok {
		t.Fatalf("the question was answerable, got NOT checked: %s", said)
	}
	if len(got) != 0 {
		t.Fatalf("an issue procoder filed is not a Copilot finding: %+v", got)
	}

	// The same issue without our mark is still a real finding — the guard is
	// the label, not the shape.
	theirs := mk("copilot[bot]", "nil map write", "the map is written without a nil check", AutoLabel)
	got, ok, said, _ = find(t, 24*time.Hour, theirs)
	if !ok || len(got) != 1 {
		t.Fatalf("a genuine auto-review must still match: ok=%v got=%+v said=%s", ok, got, said)
	}
}

// Copilot's auto-review does not open issues on most repositories — it
// leaves inline comments on the pull request. `copilot-leak` queried only
// `gh issue list`, so it reported "no findings" while four real defects
// sat in a review of the very branch that was adding this. The command
// built to catch what escapes the gates was blind to what had just
// escaped them.
//
// The author test is NOT the issue test, and that is the part worth
// pinning: measured against a real review, the bot posts review comments
// as `{"login":"Copilot","type":"Bot"}` — no `[bot]` suffix — so the issue
// pattern, which requires one, matches none of them.
//
// proved by: replacing fromCopilotReview's body with
// copilotAuthor.MatchString(c.User.Login) — the real shape is then
// rejected and this test finds nothing.
func TestAReviewCommentFromCopilotIsRecognised(t *testing.T) {
	bot := func(login, typ string) ghReviewComment {
		c := ghReviewComment{}
		c.User.Login, c.User.Type = login, typ
		return c
	}
	cases := []struct {
		login, typ string
		want       bool
	}{
		{"Copilot", "Bot", true},      // what the API actually returns
		{"copilot[bot]", "Bot", true}, // the issue-path spelling
		{"copilot-preview[bot]", "Bot", true},
		{"dependabot[bot]", "Bot", false}, // a bot, not this one
		{"copilotfan", "User", false},     // a person who chose the name
		{"Copilot", "User", false},        // a person who chose the name exactly
	}
	for _, c := range cases {
		if got := fromCopilotReview(bot(c.login, c.typ)); got != c.want {
			t.Errorf("fromCopilotReview(%q, %q) = %v, want %v", c.login, c.typ, got, c.want)
		}
	}
}

// A comment whose anchor drifted out of the diff reports line: null, and
// the line it was written against survives in original_line. Rendering
// that as line zero would put a wrong number in the ledger.
//
// proved by: returning 0 from reviewLine when Line is nil — the fallback
// case then reports no line for a comment that has one.
func TestAReviewCommentWithNoCurrentLineFallsBackToWhereItWasWritten(t *testing.T) {
	n := func(i int) *int { return &i }
	for _, c := range []struct {
		line, orig *int
		want       int
	}{
		{n(122), n(116), 122}, // both: the current anchor wins
		{nil, n(116), 116},    // drifted: where it was written
		{nil, nil, 0},         // neither: no line, not line zero
	} {
		got := reviewLine(ghReviewComment{Line: c.line, OriginalLine: c.orig})
		if got != c.want {
			t.Errorf("reviewLine(%v, %v) = %d, want %d", c.line, c.orig, got, c.want)
		}
	}
}

// proved by: returning the whole body from reviewTitle — a ledger line
// then carries a paragraph.
func TestAReviewTitleIsTheClaimNotTheParagraph(t *testing.T) {
	body := "pyDeps is currently too broad: the arm will match any list. " +
		"The match should be limited to actual dependency declarations."
	got := reviewTitle(body)
	if strings.Contains(got, "The match should be limited") {
		t.Errorf("the title should stop at the first sentence, got %q", got)
	}
	if !strings.Contains(got, "pyDeps is currently too broad") {
		t.Errorf("the title lost the claim, got %q", got)
	}
	if len(got) > 130 {
		t.Errorf("title is %d chars, which is not a line", len(got))
	}
	if reviewTitle("   ") == "" {
		t.Error("an empty body still needs a title the ledger can print")
	}
}

// Both sources must answer or the whole command reports NOT checked. If
// the issue query succeeds and the review query fails, "0 findings" means
// "half the places were never looked at" — which is the failure this
// file's second return value exists to prevent, and precisely how the
// review half came to be missing in the first place.
//
// proved by: made reviewFindings' failure path return (nil, true) — this
// test then sees a clean answer from a command that could not look.
func TestAFailedReviewQueryIsNotZeroFindings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stubs are POSIX shell scripts")
	}
	dir := t.TempDir()
	// `issue list` answers with an empty list; `api` exits non-zero.
	script := "#!/bin/sh\nif [ \"$1\" = api ]; then echo 'gh: not found' >&2; exit 1; fi\nprintf '[]'\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	git := "#!/bin/sh\nprintf '%s' " + quoted(githubRemote) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	var said strings.Builder
	got, ok := Find(t.TempDir(), 6*time.Hour, func(s string) { said.WriteString(s + "\n") })

	if ok {
		t.Fatalf("a failed review query answered as if it had looked: %+v", got)
	}
	if got != nil {
		t.Errorf("nothing may be returned from an unanswered question: %+v", got)
	}
	if !strings.Contains(said.String(), "NOT checked") {
		t.Errorf("the refusal must say NOT checked: %q", said.String())
	}
}

// The end-to-end shape of the defect this closes: a repository whose
// Copilot review left inline comments and opened no issue at all.
//
// proved by: removing the reviewFindings call from Find — this test then
// finds nothing where a real review left four comments.
func TestReviewCommentsAreFoundWhenNoIssueExists(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stubs are POSIX shell scripts")
	}
	// the shape the API actually returns, measured from a real review
	reviews := `[{"body":"pyDeps is currently too broad: the arm matches any list.",
	  "path":"internal/security/security.go","line":null,"original_line":570,
	  "html_url":"https://github.com/acme/widget/pull/171#discussion_r1",
	  "user":{"login":"Copilot","type":"Bot"},
	  "created_at":"` + time.Now().Format(time.RFC3339) + `",
	  "updated_at":"` + time.Now().Format(time.RFC3339) + `"}]`
	stubPathWithReviews(t, "[]", reviews, githubRemote)

	var said strings.Builder
	got, ok := Find(t.TempDir(), 6*time.Hour, func(s string) { said.WriteString(s + "\n") })

	if !ok {
		t.Fatalf("NOT checked where it should have answered: %s", said.String())
	}
	if len(got) != 1 {
		t.Fatalf("the review comment was not found, got %+v", got)
	}
	if got[0].Line != 570 {
		t.Errorf("line should fall back to original_line, got %d", got[0].Line)
	}
	if !strings.Contains(got[0].Title, "pyDeps") {
		t.Errorf("the title lost the claim: %q", got[0].Title)
	}
	if got[0].Repo != "acme/widget" {
		t.Errorf("repo should come from the comment URL, got %q", got[0].Repo)
	}
}
