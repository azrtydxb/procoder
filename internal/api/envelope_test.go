package api

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// A request survives the wire with every field intact, including the two
// that carry meaning by being nil rather than empty.
//
// proved by: dropping Confirm from the encoded shape — the round trip then
// returns nil for a confirmation the caller sent, and six commands take
// the non-interactive path for a caller who had an answer.
func TestEnvelopeRoundTrips(t *testing.T) {
	want := Request{
		Protocol: Protocol,
		Argv:     []string{"check", "--paths-from", "-"},
		Cwd:      "/x/src/thing",
		Env:      map[string]string{"QODER_SESSION_ID": "s"},
		Stdin:    []byte{0x00, 0xff, '\n'},
		Confirm:  ptr("yes"),
		Job:      "j7f3a2",
	}
	var buf bytes.Buffer
	if err := WriteRequest(&buf, want); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}
	got, err := ReadRequest(&buf)
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip lost something:\n got %+v\nwant %+v", got, want)
	}
}

// A response round trips too, and a nil Exit stays nil — that is how a
// caller tells a job still running from one that exited 0.
func TestResponseKeepsANilExitNil(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteResponse(&buf, Response{Protocol: Protocol, Job: &Job{ID: "j1", State: "running"}}); err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}
	got, err := ReadResponse(&buf)
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if got.Exit != nil {
		t.Fatalf("a running job reported an exit code: %d", *got.Exit)
	}
	if got.Job == nil || got.Job.ID != "j1" {
		t.Fatalf("the job did not survive: %+v", got.Job)
	}
}

// A request over the cap is refused with the limit named, rather than
// buffered until the daemon dies.
//
// proved by: removing the scanner's buffer cap — the oversized request is
// then read whole, and one caller can spend the daemon's memory.
func TestRequestOverTheCapIsRefused(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteRequest(&buf, Request{Protocol: Protocol, Stdin: make([]byte, MaxRequestBytes+1)}); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}
	_, err := ReadRequest(&buf)
	if err == nil {
		t.Fatal("an oversized request was accepted")
	}
	if !strings.Contains(err.Error(), "8388608") {
		t.Fatalf("the refusal does not name the limit: %v", err)
	}
}
