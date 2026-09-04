package api

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// DialTimeout bounds how long a client waits to reach the daemon.
//
// Short on purpose. The daemon is an optimisation, and the in-process path
// is always right there: a client that waited a second to save fifty
// milliseconds has made every caller slower to make some of them faster.
const DialTimeout = 250 * time.Millisecond

var (
	// ErrNoDaemon means there is nothing listening. It is not a failure of
	// the command — the caller runs it in-process and is no worse off than
	// on a machine that never had a daemon.
	ErrNoDaemon = errors.New("procoder: no daemon is listening")
	// ErrVersionSkew means the daemon is a different build. Serving the
	// request anyway would answer with another release's behaviour, which
	// is the kind of bug that costs an afternoon to even see.
	ErrVersionSkew = errors.New("procoder: the daemon is a different build")
)

// Client talks to a daemon, or says it could not.
type Client struct {
	Path    string
	Version string
	// Timeout bounds the whole exchange, not just the dial. A hook that
	// blocked would take the session with it, and the host's own timeout
	// is the real ceiling — losing to it is worse than losing the warm
	// index.
	Timeout time.Duration
}

// Do sends one request and returns the response.
//
// Every error means the same thing to a caller: this command did not run.
// There is no second path — a machine set to the daemon uses the daemon,
// and a failure here is reported rather than worked around.
func (c Client) Do(req Request) (Response, error) {
	conn, err := net.DialTimeout("unix", c.Path, DialTimeout)
	if err != nil {
		return Response{}, fmt.Errorf("%w (%s)", ErrNoDaemon, c.Path)
	}
	defer conn.Close()

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return Response{}, fmt.Errorf("%w (%v)", ErrNoDaemon, err)
	}

	req.Protocol = Protocol
	if err := WriteRequest(conn, req); err != nil {
		return Response{}, fmt.Errorf("%w (%v)", ErrNoDaemon, err)
	}
	res, err := ReadResponse(conn)
	if err != nil {
		return Response{}, fmt.Errorf("%w (%v)", ErrNoDaemon, err)
	}
	if res.Protocol != Protocol {
		return Response{}, fmt.Errorf("%w: it speaks protocol %d, this build speaks %d",
			ErrVersionSkew, res.Protocol, Protocol)
	}
	return res, nil
}
