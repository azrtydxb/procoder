package api

import (
	"io"
	"strings"
	"testing"
	"time"
)

// Which commands outlive a request is a fact about procoder, known here.
// Discovering it by waiting would make every caller pay the wait once to
// find out.
func TestIsLongRunning(t *testing.T) {
	long := [][]string{
		{"test"}, {"audit"}, {"release"}, {"bench"}, {"deps"},
		{"security", "--deep"}, {"docs", "--external"}, {"ci", "--runs"},
		{"index", "build"},
	}
	for _, argv := range long {
		if !IsLongRunning(argv) {
			t.Errorf("%v should answer with a job", argv)
		}
	}
	short := [][]string{
		{"config"}, {"status"}, {"check"}, {"todo", "list"},
		// The same commands without the flag that makes them long.
		{"security"}, {"docs"}, {"ci"}, {"index", "find", "x"}, {},
	}
	for _, argv := range short {
		if IsLongRunning(argv) {
			t.Errorf("%v should answer inline", argv)
		}
	}
}

// A job outlives the connection that started it — which is the whole
// point, since the connection is what a caller wants back.
//
// proved by: answering long-running commands inline — the submit then
// blocks for the command's duration and closing the connection loses it.
func TestJobSurvivesItsConnection(t *testing.T) {
	path, _ := testServer(t, func(req Request, stdout, stderr io.Writer) (int, *Result) {
		time.Sleep(200 * time.Millisecond)
		io.WriteString(stdout, "done")
		return 5, &Result{Kind: KindFindings, Findings: []Finding{}}
	})

	start := time.Now()
	res, err := Client{Path: path}.Do(Request{Argv: []string{"test"}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("the submit blocked for %v — that is not a job, that is a call", elapsed)
	}
	if res.Job == nil || res.Job.State != JobRunning {
		t.Fatalf("want a running job, got %+v", res.Job)
	}
	if res.Exit != nil {
		t.Fatalf("a running job reported an exit code: %d", *res.Exit)
	}

	// The connection that started it is gone by now: Do closes it.
	deadline := time.Now().Add(5 * time.Second)
	for {
		polled, err := Client{Path: path}.Do(Request{Job: res.Job.ID})
		if err != nil {
			t.Fatalf("poll: %v", err)
		}
		if polled.Job != nil && polled.Job.State == JobDone {
			if polled.Exit == nil || *polled.Exit != 5 {
				t.Fatalf("the exit code did not survive: %v", polled.Exit)
			}
			if !strings.Contains(polled.Stdout, "done") {
				t.Fatalf("the output did not survive: %q", polled.Stdout)
			}
			if polled.Result == nil {
				t.Fatal("the typed result did not survive the job")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the job never finished")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A job the daemon never issued — or issued before it restarted — is
// lost, which is not the same as failed. Nobody knows how that command got
// on, and saying it failed would be inventing an answer.
func TestLostJobSaysLostNotFailed(t *testing.T) {
	path, _ := testServer(t, func(Request, io.Writer, io.Writer) (int, *Result) { return 0, nil })
	res, err := Client{Path: path}.Do(Request{Job: "jdeadbeef"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Job == nil || res.Job.State != JobLost {
		t.Fatalf("want a lost job, got %+v", res.Job)
	}
	if res.Exit != nil {
		t.Fatalf("a lost job reported an exit code: %d — a caller would read that as a verdict", *res.Exit)
	}
}

// A poll returns only what the caller has not seen.
//
// Without the offset a poll returns everything accumulated, every time: a
// two-minute suite polled every 100ms re-sends a growing buffer 1200
// times, so the traffic is quadratic in the output and worst on exactly
// the runs the job model exists to support.
//
// proved by: ignoring Since in jobs.poll — the second poll then repeats
// the first chunk, and this test sees it twice.
func TestPollReturnsOnlyWhatIsNew(t *testing.T) {
	release := make(chan struct{})
	var j jobs
	job := j.start(Request{Argv: []string{"test"}}, func(_ Request, stdout, stderr io.Writer) (int, *Result) {
		io.WriteString(stdout, "first")
		io.WriteString(stderr, "e1")
		<-release
		io.WriteString(stdout, "second")
		return 0, nil
	})

	first := waitForOutput(t, &j, job.ID, 0, 0)
	if first.Stdout != "first" || first.Stderr != "e1" {
		t.Fatalf("first poll: stdout %q stderr %q", first.Stdout, first.Stderr)
	}

	// Nothing new yet: the same offsets must return nothing at all, not
	// the bytes already delivered.
	repeat := j.poll(job.ID, len("first"), len("e1"))
	if repeat.Stdout != "" || repeat.Stderr != "" {
		t.Fatalf("a poll with nothing new repeated itself: stdout %q stderr %q", repeat.Stdout, repeat.Stderr)
	}

	close(release)
	second := waitForOutput(t, &j, job.ID, len("first"), len("e1"))
	if second.Stdout != "second" {
		t.Fatalf("second poll returned %q, want only the new bytes", second.Stdout)
	}
}

// An offset past what the daemon holds reads empty rather than crashing.
// A client with more than the daemon has is a daemon that restarted.
func TestPollBeyondTheEndIsEmptyNotAPanic(t *testing.T) {
	var j jobs
	job := j.start(Request{Argv: []string{"test"}}, func(_ Request, stdout, _ io.Writer) (int, *Result) {
		io.WriteString(stdout, "short")
		return 0, nil
	})
	waitForOutput(t, &j, job.ID, 0, 0)
	res := j.poll(job.ID, 9999, 9999)
	if res.Stdout != "" || res.Stderr != "" {
		t.Fatalf("an offset past the end returned %q / %q", res.Stdout, res.Stderr)
	}
}

func waitForOutput(t *testing.T, j *jobs, id string, since, sinceErr int) Response {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		res := j.poll(id, since, sinceErr)
		if res.Stdout != "" || res.Stderr != "" {
			return res
		}
		if time.Now().After(deadline) {
			t.Fatal("the job produced no output")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
