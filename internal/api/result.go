package api

// Result is the typed answer beside the human bytes — not instead of them.
//
// A response carries both deliberately. The bytes are what a person reads
// and what the parity test compares, so they cannot become optional; the
// result is what a client acts on, so it cannot be absent either. A
// response with only the result would make every caller a renderer, and
// one with only the bytes would make every caller a parser — which is the
// situation this package exists to end.
type Result struct {
	// Kind names the shape. A client that does not know a kind treats the
	// result as absent and reads the bytes, which is why the bytes are
	// not optional.
	Kind string `json:"kind"`
	// Findings is the answer most commands have. Empty is not the same as
	// absent: an empty list is "this command reports findings and found
	// none", and a nil Result is "this command does not report findings".
	Findings []Finding `json:"findings,omitempty"`
	Settings []Setting `json:"settings,omitempty"`
	Tasks    []Task    `json:"tasks,omitempty"`
	Version  *Version  `json:"version,omitempty"`
}

// KindFindings is the shape the reporting commands answer in.
const KindFindings = "findings"

// Finding is gitx.Finding as it crosses the wire, plus the domain that
// raised it — which printFindings already knows and currently spends on a
// label.
//
// Not gitx.Finding itself: this package would then import the domain layer
// to declare its own envelope, and the wire shape would change whenever
// that struct did. The two are pinned to each other by a test instead.
type Finding struct {
	File string `json:"file"`
	// Line is 0 when the finding is about the file as a whole, exactly as
	// gitx.Finding already means it.
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
	Domain   string `json:"domain"`
}
