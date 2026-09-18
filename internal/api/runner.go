package api

import (
	"bytes"
	"io"
)

// Runner is one command, run. The main package supplies it: this package
// owns the envelope and the socket, and knows nothing about the 112
// dispatch branches on the other side.
//
// The import runs one way only. cmd/procoder imports internal/api to hand
// it a Runner; internal/api never imports cmd/procoder, which it could not
// do anyway, and would not want to if it could.
type Runner func(req Request, stdout, stderr io.Writer) (int, *Result)

// Serve runs one request and builds its response. No socket: this is the
// whole of what a daemon does to a request, minus the transport, which is
// what makes parity testable without one.
func Serve(req Request, run Runner) Response {
	var stdout, stderr bytes.Buffer
	code, result := run(req, &stdout, &stderr)
	return Response{
		Protocol: Protocol,
		Exit:     &code,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Result:   result,
	}
}
