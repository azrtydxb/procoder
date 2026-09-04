// Package api is procoder's second door: the envelope every command can be
// asked in, and the local socket that carries it.
//
// The first door is the binary itself, and it does not move. A command run
// from a terminal or from CI behaves exactly as it did, with no daemon, no
// socket and no setup — that property is the reason this package is a
// transport and never a rewrite. What it adds is that a caller who would
// rather call a function than spawn a process may do so, and get the same
// bytes and the same exit code either way.
package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Protocol is the envelope's version. It travels in both directions so a
// client and a daemon built from different releases can tell.
const Protocol = 1

// MaxRequestBytes bounds one request. A daemon serves whoever can open its
// socket, and an unbounded read is a way for one caller to spend all of
// its memory.
const MaxRequestBytes = 8 << 20

// Request is one command, asked.
type Request struct {
	Protocol int      `json:"protocol"`
	Argv     []string `json:"argv"`
	// Cwd is where the caller was, not where the daemon is. Every root
	// resolution starts here.
	Cwd string `json:"cwd"`
	// Env is what the caller's environment said, not the daemon's. One
	// daemon serves several hosts and its own environment belongs to
	// whichever session started it.
	Env   map[string]string `json:"env"`
	Stdin []byte            `json:"stdin"`
	// Confirm is what a person would have typed, or nil when there was no
	// person. Nil is not the same as "no": it is the non-interactive path,
	// which is exactly what a command takes today when nothing is
	// attached to its stdin.
	Confirm *string `json:"confirm"`
	// Job names a job to poll instead of a command to run. Set means Argv
	// is empty and the daemon is being asked how an earlier request got on.
	Job string `json:"job"`
	// Since is how many bytes of this job's output the caller already
	// has, so a poll returns only what is new.
	//
	// Without it a poll returns everything accumulated, every time: a
	// two-minute suite polled every 100ms re-sends a growing buffer 1200
	// times, which is gigabytes of socket traffic for megabytes of output
	// and gets worse the longer the job runs. The offset makes it linear.
	//
	// Counted separately per stream, because they grow independently.
	Since    int `json:"since"`
	SinceErr int `json:"since_err"`
}

// Job is a command that outlived the connection that asked for it.
type Job struct {
	ID string `json:"id"`
	// State is running, done, or lost. Lost is a daemon that restarted,
	// and it is deliberately not failed: nobody knows how the command got
	// on, and saying it failed would be inventing an answer.
	State string `json:"state"`
}

// Response is what came back.
type Response struct {
	Protocol int `json:"protocol"`
	// Exit is nil while a job runs. A caller that read a nil as a zero
	// would call a running suite green.
	Exit   *int    `json:"exit"`
	Stdout string  `json:"stdout"`
	Stderr string  `json:"stderr"`
	Job    *Job    `json:"job"`
	Result *Result `json:"result"`
}

// Every envelope is one JSON object and a newline. Newline-delimited
// rather than length-prefixed because both ends are Go, both ends already
// have encoding/json, and a framing nobody can read by eye is a debugging
// cost paid on every future bug.

func WriteRequest(w io.Writer, r Request) error   { return write(w, r) }
func WriteResponse(w io.Writer, r Response) error { return write(w, r) }

func write(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("procoder: could not encode the envelope (%v)", err)
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// ReadRequest reads one request, refusing anything over the cap.
func ReadRequest(r io.Reader) (Request, error) {
	var req Request
	err := read(r, &req)
	return req, err
}

// ReadResponse reads one response.
func ReadResponse(r io.Reader) (Response, error) {
	var res Response
	err := read(r, &res)
	return res, err
}

func read(r io.Reader, v any) error {
	sc := bufio.NewScanner(r)
	// The cap is on the scanner rather than checked after the fact,
	// because checking after the fact means having already read it.
	sc.Buffer(make([]byte, 0, 64<<10), MaxRequestBytes)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return fmt.Errorf("procoder: request over the %d-byte limit (%v)", MaxRequestBytes, err)
		}
		return fmt.Errorf("procoder: the connection carried no envelope")
	}
	if err := json.Unmarshal(sc.Bytes(), v); err != nil {
		return fmt.Errorf("procoder: could not decode the envelope (%v)", err)
	}
	return nil
}
