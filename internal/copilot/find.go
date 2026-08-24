package copilot

// The read half: what Copilot's auto-review already said, fetched through the
// same `gh` CLI every other domain uses. The one rule that shapes this file is
// the second return value — false means the question was NOT answered, and a
// caller that cannot tell "Copilot found nothing" from "we never managed to
// look" will eventually report the second as the first.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"procoder/internal/textutil"
)

// findTimeout is the budget for one gh call: it goes to GitHub's API over the
// network, and a hung request must not hang the command that asked.
const findTimeout = 60 * time.Second

// findLimit caps how many issues gh returns. The search already narrows to the
// window; this is the ceiling that keeps a busy repository from paging.
const findLimit = "60"

// AutoLabel is the label Copilot's auto-review path puts on the issues it
// opens, and the label procoder puts back on the ones it creates.
const AutoLabel = "auto-copilot"

// OwnLabel marks the issues procoder itself filed. Capture puts AutoLabel on
// them too — that is the family this finder queries — so without a mark of our
// own, run two inside the same window finds run one's issues, files them
// again, and every run doubles the last one's output.
const OwnLabel = "copilot-leak"

// reviewQuote is Copilot's review annotation block, the last resort when an
// instance neither labels nor uses a recognisable bot account.
const reviewQuote = "> **Copilot**"

// copilotAuthor covers both bot accounts in the wild: copilot[bot] and
// copilot-preview[bot] (Q3 in the spec).
var copilotAuthor = regexp.MustCompile(`(?i)copilot.*\[bot\]`)

// notCodeQuality is the best-effort filter for Copilot issues that are not
// about a defect. Wish-list and documentation items teach nothing about which
// gate let a bug through, and a ledger full of them is a ledger nobody reads.
var notCodeQuality = regexp.MustCompile(`(?i)\b(feature request|enhancement request|feature suggestion|documentation (suggestion|request|improvement)|docs? (suggestion|request))\b`)

// notCodeQualityLabels is the same filter by label, which instances that
// triage properly give us for free.
var notCodeQualityLabels = map[string]bool{"enhancement": true, "documentation": true, "question": true}

// filePosition finds the first `path:line` Copilot quoted, which is the only
// part of its location worth keeping — the path itself is the user's and never
// leaves the machine unsanitised.
var filePosition = regexp.MustCompile(`[\w./\\-]+\.[A-Za-z0-9]+:(\d+)`)

// ghIssue is one issue as `gh issue list --json` reports it.
type ghIssue struct {
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

// Find returns the Copilot auto-review issues touched in the last `since`, and
// whether the question could be answered at all. False means NOT checked — gh
// missing, gh refusing, the network gone, output we cannot parse — and every
// one of those is said out loud through out before returning. A repository with
// no GitHub remote is the one quiet case: there is nothing to query, which is
// an answered question with an empty answer, not an unknown.
//
// The window is applied twice on purpose: gh is asked to search for it so a
// busy repository does not page, and the parsed timestamps are checked again
// here so the answer is exact regardless of what the search engine rounded to.
func Find(root string, since time.Duration, out func(string)) ([]Finding, bool) {
	say := func(s string) {
		if out != nil {
			out(s)
		}
	}
	bin, err := exec.LookPath("gh")
	if err != nil {
		say("Copilot auto-reviews NOT checked — gh is not installed (https://cli.github.com)")
		return nil, false
	}
	remotes, ok := gitRemotes(root)
	if !ok {
		say("Copilot auto-reviews NOT checked — git could not list this repository's remotes")
		return nil, false
	}
	if !strings.Contains(remotes, "github.com") {
		return nil, true // no GitHub, no auto-reviews: nothing to ask, and nothing unknown
	}
	if since <= 0 {
		// The caller validates the window; reaching here means it did not, and
		// silently querying a different one than the caller prints is how a
		// report comes to describe a window nobody asked for.
		say("copilot-leak: a window of " + since.String() + " asks about nothing — using 24h")
		since = 24 * time.Hour
	}
	cutoff := time.Now().Add(-since)

	stdout, stderr, runErr, timedOut := ghIssues(root, bin, cutoff)
	if timedOut {
		say(fmt.Sprintf("Copilot auto-reviews NOT checked — gh gave no answer in %s and was killed", findTimeout))
		return nil, false
	}
	if runErr != nil {
		say("Copilot auto-reviews NOT checked — " + ghReason(stderr+stdout, runErr))
		return nil, false
	}
	issues, perr := parseIssues(stdout)
	if perr != nil {
		say("Copilot auto-reviews NOT checked — " + perr.Error())
		return nil, false
	}

	var finds []Finding
	for _, is := range issues {
		if ours(is) || !within(is, cutoff) || !fromCopilot(is) || !aboutCodeQuality(is) {
			continue
		}
		finds = append(finds, Finding{
			OriginalURL: is.URL,
			Title:       strings.TrimSpace(is.Title),
			Body:        is.Body,
			Line:        positionIn(is.Body),
			Repo:        repoOf(is.URL),
			Created:     is.CreatedAt,
		})
	}

	// The second source. Both must answer, or the caller cannot tell
	// "Copilot found nothing" from "one of the two places was never
	// looked at" — which is the whole point of this file's second return
	// value, and exactly how the review half came to be missing.
	reviews, rok := reviewFindings(root, bin, cutoff, say)
	if !rok {
		return nil, false
	}
	finds = append(finds, reviews...)
	return finds, true
}

// ours reports whether procoder filed this issue: a capture of a capture
// teaches nobody and multiplies. The hand-filing template carries the same
// mark, so a leak filed by a person is not re-filed either.
func ours(is ghIssue) bool {
	for _, l := range is.Labels {
		if strings.EqualFold(strings.TrimSpace(l.Name), OwnLabel) {
			return true
		}
	}
	return false
}

// fromCopilot is the three-way match from the spec: the label, the bot author,
// or Copilot's own review annotation in the body. Any one is enough — an
// instance that renames its bot still labels, and one that does neither still
// writes the quote block.
func fromCopilot(is ghIssue) bool {
	for _, l := range is.Labels {
		if strings.EqualFold(strings.TrimSpace(l.Name), AutoLabel) {
			return true
		}
	}
	if copilotAuthor.MatchString(is.Author.Login) {
		return true
	}
	return strings.Contains(is.Body, reviewQuote)
}

// aboutCodeQuality drops the issues that are wishes rather than defects. It is
// deliberately a best-effort text filter: a false negative costs one noisy
// ledger entry, while being strict here would drop real escapes.
func aboutCodeQuality(is ghIssue) bool {
	for _, l := range is.Labels {
		if notCodeQualityLabels[strings.ToLower(strings.TrimSpace(l.Name))] {
			return false
		}
	}
	return !notCodeQuality.MatchString(is.Title) && !notCodeQuality.MatchString(is.Body)
}

// within answers whether the issue moved inside the window. An issue gh
// returned with no usable timestamps is kept: the search already applied the
// window, and dropping it here would silently lose a real finding.
func within(is ghIssue, cutoff time.Time) bool {
	switch {
	case !is.UpdatedAt.IsZero():
		return !is.UpdatedAt.Before(cutoff)
	case !is.CreatedAt.IsZero():
		return !is.CreatedAt.Before(cutoff)
	}
	return true
}

// positionIn is the line number of the first file:line Copilot cited, or 0 when
// it cited none.
func positionIn(body string) int {
	m := filePosition.FindStringSubmatch(body)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// repoOf reads owner/name out of an issue URL, which is the only place gh's
// issue list carries it.
func repoOf(url string) string {
	rest := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	parts := strings.Split(rest, "/")
	if len(parts) < 3 {
		return ""
	}
	return parts[1] + "/" + parts[2]
}

// parseIssues insists on JSON. Empty output where a list was expected is a
// failure, not an empty list — gh prints `[]` when it means none.
func parseIssues(raw string) ([]ghIssue, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("gh printed nothing where a JSON issue list was expected")
	}
	var issues []ghIssue
	if err := json.Unmarshal([]byte(trimmed), &issues); err != nil {
		return nil, fmt.Errorf("gh printed unparseable JSON: %v", err)
	}
	return issues, nil
}

// ghIssues runs the one query this file makes, under a timeout.
func ghIssues(root, bin string, cutoff time.Time) (string, string, error, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), findTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, // nosemgrep -- gh resolved from PATH with fixed subcommands
		"issue", "list", "--state", "all", "--limit", findLimit,
		"--search", "updated:>="+cutoff.UTC().Format("2006-01-02T15:04:05Z"),
		"--json", "title,body,url,author,labels,createdAt,updatedAt")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err, ctx.Err() == context.DeadlineExceeded
}

// ghReason keeps gh's own first line, and the `gh auth login` hint when gh
// offered one — the fix belongs next to the reason that needs it.
func ghReason(raw string, err error) string {
	first, hint := "", ""
	for _, l := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if first == "" {
			first = t
		}
		if strings.Contains(t, "gh auth login") {
			hint = "gh auth login"
		}
	}
	if len(first) > 160 {
		first = first[:160]
	}
	if first == "" {
		if err != nil {
			return "gh failed with no output (" + err.Error() + ")"
		}
		return "gh failed with no output"
	}
	if hint != "" && !strings.Contains(first, "gh auth login") {
		first += " (run: gh auth login)"
	}
	return first
}

// gitRemotes returns the remote list and whether git could produce one. The
// second value separates "no GitHub remote" from "no idea", which the caller
// reports very differently.
func gitRemotes(root string) (string, bool) {
	cmd := exec.Command("git", "remote", "-v") // nosemgrep -- fixed subcommand, no user input
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// ---------------------------------------------------------------------
// The second source: pull request review comments.
//
// Copilot's auto-review does not open issues on this repository or most
// others — it leaves inline comments on the pull request. The issue query
// above cannot see those, so `copilot-leak` reported "no findings" while
// four real defects sat in a review of the very branch that added it.
// That is the failure this whole domain exists to catch, in the domain
// that exists to catch it.

// reviewCommentLimit is one page of GitHub's repo-wide review-comment
// endpoint. The window already narrows it; this is the ceiling.
const reviewCommentLimit = "100"

// copilotReviewAuthor is the author test for a review comment, which is
// NOT the issue test. The bot posts issues as `copilot[bot]` and review
// comments as `Copilot` with no suffix at all, so the issue pattern —
// which requires `[bot]` — matches none of them. Measured against a real
// review: `{"login":"Copilot","type":"Bot"}`.
//
// Both halves are required. `type == "Bot"` alone would take every bot in
// the repository, and the login alone would take a person who happened to
// call themselves copilotfan.
var copilotReviewLogin = regexp.MustCompile(`(?i)^copilot`)

// ghReviewComment is one review comment as the REST API reports it.
type ghReviewComment struct {
	Body string `json:"body"`
	Path string `json:"path"`
	// Line is null for a comment whose anchor has drifted out of the
	// diff, in which case original_line still holds where it was written.
	Line         *int   `json:"line"`
	OriginalLine *int   `json:"original_line"`
	HTMLURL      string `json:"html_url"`
	User         struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// reviewFindings returns the Copilot review comments touched in the
// window, and whether the question was answered. The contract is the
// file's contract: false means NOT checked, said out loud, never zero.
func reviewFindings(root, bin string, cutoff time.Time, say func(string)) ([]Finding, bool) {
	stdout, stderr, runErr, timedOut := ghReviewComments(root, bin, cutoff)
	if timedOut {
		say(fmt.Sprintf("Copilot review comments NOT checked — gh gave no answer in %s and was killed", findTimeout))
		return nil, false
	}
	if runErr != nil {
		say("Copilot review comments NOT checked — " + ghReason(stderr+stdout, runErr))
		return nil, false
	}
	var comments []ghReviewComment
	if err := json.Unmarshal([]byte(stdout), &comments); err != nil {
		say("Copilot review comments NOT checked — gh returned output that is not the expected JSON")
		return nil, false
	}
	var finds []Finding
	for _, c := range comments {
		if !fromCopilotReview(c) || !c.UpdatedAt.After(cutoff) {
			continue
		}
		if notCodeQuality.MatchString(c.Body) {
			continue
		}
		finds = append(finds, Finding{
			OriginalURL: c.HTMLURL,
			Title:       reviewTitle(c.Body),
			Body:        c.Body,
			Line:        reviewLine(c),
			Repo:        repoOf(c.HTMLURL),
			Created:     c.CreatedAt,
		})
	}
	return finds, true
}

// fromCopilotReview is the author test described above: a Bot account
// whose login begins with copilot.
func fromCopilotReview(c ghReviewComment) bool {
	return strings.EqualFold(c.User.Type, "Bot") && copilotReviewLogin.MatchString(c.User.Login)
}

// reviewLine prefers the current anchor and falls back to where the
// comment was originally written. Zero means the API gave neither, which
// the ledger renders as no line rather than as line zero.
func reviewLine(c ghReviewComment) int {
	if c.Line != nil {
		return *c.Line
	}
	if c.OriginalLine != nil {
		return *c.OriginalLine
	}
	return 0
}

// reviewTitle is the comment's first sentence, which is how Copilot writes
// them: the claim first, the reasoning after. Capped so a ledger line stays
// a line.
func reviewTitle(body string) string {
	t := strings.TrimSpace(textutil.FirstLine(body))
	if i := strings.Index(t, ". "); i > 0 {
		t = t[:i]
	}
	if len(t) > 120 {
		t = strings.TrimSpace(t[:120]) + "…"
	}
	if t == "" {
		return "Copilot review comment"
	}
	return t
}

// ghReviewComments asks for every review comment in the repository updated
// since the cutoff. The repo-wide endpoint is used rather than one call per
// pull request: one request answers for the window, and a repository with
// no pull requests answers with an empty list rather than an error.
func ghReviewComments(root, bin string, cutoff time.Time) (string, string, error, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), findTimeout)
	defer cancel()
	path := fmt.Sprintf("repos/{owner}/{repo}/pulls/comments?since=%s&per_page=%s",
		cutoff.UTC().Format("2006-01-02T15:04:05Z"), reviewCommentLimit)
	cmd := exec.CommandContext(ctx, bin, "api", path) // nosemgrep -- gh resolved from PATH, path built from a formatted timestamp
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err, ctx.Err() == context.DeadlineExceeded
}
