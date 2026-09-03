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
