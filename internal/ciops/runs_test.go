package ciops

import (
	"strings"
	"testing"
	"time"
)

// runListJSON is a recorded `gh run list --json workflowName,status,conclusion,
// createdAt,headSha,databaseId` payload: a failed CI run, an older successful
// run of the same workflow (which must be ignored), and a lint run still going.
const runListJSON = `[
  {"workflowName":"ci","status":"completed","conclusion":"failure","createdAt":"2026-08-19T10:00:00Z","headSha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","databaseId":991},
  {"workflowName":"ci","status":"completed","conclusion":"success","createdAt":"2026-08-18T10:00:00Z","headSha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","databaseId":990},
  {"workflowName":"lint","status":"in_progress","conclusion":"","createdAt":"2026-08-19T10:30:00Z","headSha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","databaseId":992}
]`

// runViewJSON is a recorded `gh run view <id> --json jobs` payload.
const runViewJSON = `{"jobs":[
  {"name":"build","status":"completed","conclusion":"success"},
  {"name":"test (ubuntu-latest)","status":"completed","conclusion":"failure"},
  {"name":"test (windows-latest)","status":"completed","conclusion":"cancelled"}
]}`

// now is the fixed clock the report tests read ages against.
var now = time.Date(2026, 8, 19, 11, 0, 0, 0, time.UTC)

const headSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func stubJobs(failing []string, reason string) jobsFor {
	return func(int64) ([]string, string) { return failing, reason }
}

func report(t *testing.T, raw, head string, pushed bool, jobs jobsFor) (string, int) {
	t.Helper()
	runs, err := parseRuns(raw)
	if err != nil {
		t.Fatalf("parseRuns: %v", err)
	}
	var lines []string
	code := reportRuns(runs, "feature/x", head, pushed, now, jobs, func(s string) { lines = append(lines, s) })
	return strings.Join(lines, "\n"), code
}

func has(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("missing %q in:\n%s", want, got)
	}
}

func TestRunListParsesTheRecordedShape(t *testing.T) {
	runs, err := parseRuns(runListJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Fatalf("got %d runs, want 3", len(runs))
	}
	newest := newestPerWorkflow(runs)
	if len(newest) != 2 {
		t.Fatalf("got %d workflows, want 2: %+v", len(newest), newest)
	}
	if newest[0].WorkflowName != "ci" || newest[0].DatabaseID != 991 {
		t.Fatalf("newest ci run is %+v, want databaseId 991", newest[0])
	}
}

func TestFailedRunNamesTheFailingJobsAndInProgressClaimsNoConclusion(t *testing.T) {
	failing, err := parseFailingJobs(runViewJSON)
	if err != nil {
		t.Fatal(err)
	}
	got, code := report(t, runListJSON, headSHA, true, stubJobs(failing, ""))
	if code != 0 {
		t.Fatalf("a report exits 0, got %d:\n%s", code, got)
	}
	has(t, got, "ci: failure — 1h 0m ago")
	has(t, got, "failing job(s): test (ubuntu-latest), test (windows-latest)")
	has(t, got, "lint: in progress — started 30m ago, no conclusion yet")
	if strings.Contains(got, "lint: success") || strings.Contains(got, "conclusion: ") {
		t.Fatalf("an unfinished run must not be given a conclusion:\n%s", got)
	}
	if strings.Contains(got, "build") {
		t.Fatalf("a passing job must not be listed as failing:\n%s", got)
	}
	if strings.Contains(got, "predates your latest push") {
		t.Fatalf("a run of HEAD is not stale:\n%s", got)
	}
}

func TestStaleRunSaysCIHasNotJudgedThisCommit(t *testing.T) {
	got, code := report(t, runListJSON, "cccccccccccccccccccccccccccccccccccccccc", true, stubJobs(nil, ""))
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, got)
	}
	has(t, got, "the newest run predates your latest push — CI has not judged this commit yet")
}

func TestUnpushedHeadSkipsTheStalenessVerdict(t *testing.T) {
	got, _ := report(t, runListJSON, "cccccccccccccccccccccccccccccccccccccccc", false, stubJobs(nil, ""))
	has(t, got, "HEAD is not pushed — CI cannot have seen it")
	if strings.Contains(got, "predates your latest push") {
		t.Fatalf("an unpushed HEAD cannot be judged stale:\n%s", got)
	}
}

func TestEmptyRunListIsNeverReadAsGreen(t *testing.T) {
	got, code := report(t, "[]", headSHA, true, stubJobs(nil, ""))
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, got)
	}
	has(t, got, "no CI runs for this branch (feature/x)")
	has(t, got, "not a green verdict")
	if strings.Contains(got, "success") {
		t.Fatalf("an empty list must not read as success:\n%s", got)
	}
}

func TestUnparseableRunListIsAnError(t *testing.T) {
	if _, err := parseRuns("not json"); err == nil {
		t.Fatal("garbage JSON must be an error, never an empty run table")
	}
	if _, err := parseRuns("   "); err == nil {
		t.Fatal("empty gh output must be an error, never an empty run table")
	}
	if _, err := parseFailingJobs("{oops"); err == nil {
		t.Fatal("garbage job JSON must be an error")
	}
}

func TestFailedRunWithUnreachableJobsSaysNotChecked(t *testing.T) {
	got, _ := report(t, runListJSON, headSHA, true, stubJobs(nil, "gh: HTTP 500"))
	has(t, got, "failing job(s) NOT checked — gh: HTTP 500")
}

func TestGhAbsentFromPathYieldsNotChecked(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // a stub PATH with no gh on it
	var lines []string
	code := Runs(t.TempDir(), func(s string) { lines = append(lines, s) })
	got := strings.Join(lines, "\n")
	if code != 1 {
		t.Fatalf("a check that could not run exits 1, got %d:\n%s", code, got)
	}
	has(t, got, "CI runs NOT checked — gh is not installed (https://cli.github.com)")
}

// TestBareHygieneReportIsUnchanged pins the default `procoder ci` behaviour:
// the run report is a separate entry point and must not have touched it.
func TestBareHygieneReportIsUnchanged(t *testing.T) {
	root := writeWorkflow(t, sloppyWorkflow)
	got := Check(root, false)
	want := []string{
		"action pinned to a mutable ref (actions/checkout@v4) — a tag can be silently repointed; pin the commit SHA (ci)",
		"job \"build\" has no timeout-minutes — a hang holds the runner for GitHub's six-hour default (ci)",
		"no concurrency group with cancel-in-progress — stacked pushes run CI on stale commits (ci)",
		"no workflow mentions tests — continuous testing is the CT in CI/CD/CT (ci)",
	}
	if len(got) != len(want) {
		t.Fatalf("hygiene findings changed: got %d, want %d: %+v", len(got), len(want), got)
	}
	for i, f := range got {
		if f.Message != want[i] {
			t.Fatalf("hygiene finding %d changed:\n got %q\nwant %q", i, f.Message, want[i])
		}
		if f.Blocking {
			t.Fatalf("hygiene report mode must not block: %+v", f)
		}
	}
}

func TestAgesReadTheWayAReaderThinks(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "less than a minute"},
		{-time.Hour, "less than a minute"},
		{42 * time.Minute, "42m"},
		{3*time.Hour + 5*time.Minute, "3h 5m"},
		{50 * time.Hour, "2d 2h"},
	} {
		if got := humanAge(c.d); got != c.want {
			t.Fatalf("humanAge(%s) = %q, want %q", c.d, got, c.want)
		}
	}
}
