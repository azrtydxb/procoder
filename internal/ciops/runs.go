package ciops

// The run-health half of domain 7: not what the workflows say, but what CI
// actually did with this branch. One snapshot per invocation via gh — no
// watching, no polling — and every reason it could not answer is said out
// loud, because a run table nobody could fetch is not a green verdict.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"procoder/internal/gitx"
)

// ghTimeout is the budget for a gh call; it goes to GitHub's API and a hung
// network must not hang the report.
const ghTimeout = 60 * time.Second

// runLimit is how far back gh is asked to look — the newest run per workflow
// is the answer, and twenty rows reach it on any real branch.
const runLimit = "20"

// run is one workflow run as gh reports it.
type run struct {
	WorkflowName string    `json:"workflowName"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	CreatedAt    time.Time `json:"createdAt"`
	HeadSha      string    `json:"headSha"`
	DatabaseID   int64     `json:"databaseId"`
}

// jobsFor names the failing jobs of a run; the second value is a reason the
// lookup failed (empty when it succeeded). It is a parameter so the report
// logic is testable without gh on PATH.
type jobsFor func(id int64) (failing []string, reason string)

// Runs reports the CI verdict for the current branch: per workflow, the newest
// run's status, conclusion and age, the failing job names when it failed, and
// whether that verdict is even about the current commit. Exit code: 0 for a
// report (findings included), 1 when the check could not run.
func Runs(root string, out func(string)) int {
	bin, err := exec.LookPath("gh")
	if err != nil {
		out("CI runs NOT checked — gh is not installed (https://cli.github.com)")
		return 1
	}
	branch := gitx.CurrentBranch(root)
	if branch == "" {
		out("CI runs NOT checked — detached HEAD, so there is no current branch to ask about")
		return 1
	}
	if !hasGitHubRemote(root) {
		out("CI runs NOT checked — this repository has no GitHub remote")
		return 1
	}

	stdout, stderr, err, timedOut := ghExec(root, bin,
		"run", "list", "--branch", branch, "--limit", runLimit,
		"--json", "workflowName,status,conclusion,createdAt,headSha,databaseId")
	if timedOut {
		out("gh gave no answer in 60s — the process was killed; CI runs were NOT checked")
		return 1
	}
	if err != nil {
		out("CI runs NOT checked — " + ghError(stderr+stdout, err))
		return 1
	}
	runs, perr := parseRuns(stdout)
	if perr != nil {
		out("CI runs NOT checked — " + perr.Error())
		return 1
	}

	head := gitOut(root, "rev-parse", "HEAD")
	jobs := func(id int64) ([]string, string) {
		raw, jstderr, jerr, jtimedOut := ghExec(root, bin, "run", "view", fmt.Sprint(id), "--json", "jobs")
		if jtimedOut {
			return nil, "gh gave no answer in 60s — the process was killed"
		}
		if jerr != nil {
			return nil, ghError(jstderr+raw, jerr)
		}
		names, perr := parseFailingJobs(raw)
		if perr != nil {
			return nil, perr.Error()
		}
		return names, ""
	}
	return reportRuns(runs, branch, head, headIsPushed(root), time.Now(), jobs, out)
}

// reportRuns prints the verdict for the newest run of each workflow and the
// staleness line. It takes the clock and the job lookup so the whole report is
// exercisable from a test with no gh and no network.
func reportRuns(runs []run, branch, head string, pushed bool, now time.Time, jobs jobsFor, out func(string)) int {
	if len(runs) == 0 {
		out("no CI runs for this branch (" + branch + ") — that is an absence of evidence, not a green verdict")
		return 0
	}
	newest := newestPerWorkflow(runs)
	for _, r := range newest {
		age := humanAge(now.Sub(r.CreatedAt))
		if r.Status != "completed" {
			out(fmt.Sprintf("%s: %s — started %s ago, no conclusion yet", r.WorkflowName, statusWord(r.Status), age))
			continue
		}
		out(fmt.Sprintf("%s: %s — %s ago", r.WorkflowName, conclusionWord(r.Conclusion), age))
		if r.Conclusion != "failure" {
			continue
		}
		failing, reason := jobs(r.DatabaseID)
		switch {
		case reason != "":
			out("    failing job(s) NOT checked — " + reason)
		case len(failing) == 0:
			out("    failing job(s): none reported — the run failed outside its jobs (setup, cancellation, or a workflow error)")
		default:
			out("    failing job(s): " + strings.Join(failing, ", "))
		}
	}
	reportStaleness(newest, head, pushed, out)
	return 0
}

// reportStaleness answers the question the whole report exists for: is that
// verdict about the commit in your working tree?
func reportStaleness(newest []run, head string, pushed bool, out func(string)) {
	if !pushed {
		out("HEAD is not pushed — CI cannot have seen it")
		return
	}
	if head == "" {
		out("staleness NOT checked — HEAD's commit could not be read")
		return
	}
	for _, r := range newest {
		if r.HeadSha == head {
			return // at least one workflow judged this very commit
		}
	}
	out("the newest run predates your latest push — CI has not judged this commit yet")
}

// newestPerWorkflow keeps one run per workflow — the most recent — sorted by
// workflow name so the report reads the same way twice.
func newestPerWorkflow(runs []run) []run {
	best := map[string]run{}
	for _, r := range runs {
		name := r.WorkflowName
		if name == "" {
			name = "(unnamed workflow)"
			r.WorkflowName = name
		}
		cur, seen := best[name]
		if !seen || r.CreatedAt.After(cur.CreatedAt) || (r.CreatedAt.Equal(cur.CreatedAt) && r.DatabaseID > cur.DatabaseID) {
			best[name] = r
		}
	}
	out := make([]run, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WorkflowName < out[j].WorkflowName })
	return out
}

// parseRuns reads `gh run list --json ...`: a JSON array of runs. An empty
// array is a full answer (no runs); anything unparseable is an error to
// report, never an empty table read as green.
func parseRuns(raw string) ([]run, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("gh printed nothing where a JSON run list was expected")
	}
	var runs []run
	if err := json.Unmarshal([]byte(trimmed), &runs); err != nil {
		return nil, fmt.Errorf("gh printed unparseable JSON: %v", err)
	}
	return runs, nil
}

// parseFailingJobs reads `gh run view --json jobs`: the names of the jobs that
// did not pass. A cancelled or timed-out job counts — it is why the run failed.
func parseFailingJobs(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("gh printed nothing where a JSON job list was expected")
	}
	var view struct {
		Jobs []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(trimmed), &view); err != nil {
		return nil, fmt.Errorf("gh printed unparseable JSON: %v", err)
	}
	var names []string
	for _, j := range view.Jobs {
		switch j.Conclusion {
		case "failure", "cancelled", "timed_out", "startup_failure", "action_required":
			names = append(names, j.Name)
		}
	}
	return names, nil
}

// statusWord makes gh's snake_case status readable without inventing a verdict.
func statusWord(status string) string {
	switch status {
	case "in_progress":
		return "in progress"
	case "queued", "waiting", "requested", "pending":
		return status + " (not started)"
	case "":
		return "status unknown"
	default:
		return strings.ReplaceAll(status, "_", " ")
	}
}

// conclusionWord reports what gh concluded; a completed run with no conclusion
// is said as unknown rather than assumed green.
func conclusionWord(conclusion string) string {
	if conclusion == "" {
		return "completed with no conclusion reported"
	}
	return strings.ReplaceAll(conclusion, "_", " ")
}

// humanAge is a coarse age — the report answers "recent or stale", not
// stopwatch questions. A run stamped in the future reads as "just now".
func humanAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "less than a minute"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// hasGitHubRemote answers whether any remote points at GitHub — the only
// provider this version reads runs from.
func hasGitHubRemote(root string) bool {
	for _, line := range strings.Split(gitOut(root, "remote", "-v"), "\n") {
		if strings.Contains(line, "github.com") {
			return true
		}
	}
	return false
}

// headIsPushed answers whether any remote branch contains HEAD. False means
// CI cannot have seen this commit, and the staleness verdict says so instead
// of blaming the runs.
func headIsPushed(root string) bool {
	return strings.TrimSpace(gitOut(root, "branch", "-r", "--contains", "HEAD")) != ""
}

func gitOut(root string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...) // nosemgrep -- fixed git subcommands, never user input
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ghExec(root, bin string, args ...string) (string, string, error, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), ghTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- gh resolved from PATH with fixed subcommands
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err, ctx.Err() == context.DeadlineExceeded
}

// ghError picks gh's own first error line, keeping the `gh auth login` hint
// when gh offered one — the fix belongs next to the reason.
func ghError(raw string, err error) string {
	first := ""
	hint := ""
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
