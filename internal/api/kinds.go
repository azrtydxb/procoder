package api

// The kinds a result can be, and the shapes that go with them.
//
// One declaration per kind, in one file, so a client reads a schema rather
// than a paragraph. Kinds() is what makes that checkable: a constant added
// without a shape, or a shape added without a constant, fails the guard in
// kinds_test.go rather than reaching a client as an object nobody can
// name.

const (
	// KindConfig is `procoder config`: every effective setting and where
	// its value came from.
	KindConfig = "config"
	// KindTodo is `procoder todo list`.
	KindTodo = "todo"
	// KindVersion is `procoder version`.
	KindVersion = "version"
	// KindStatus is `procoder status`.
	KindStatus = "status"
	// KindSpec is `procoder spec check`.
	KindSpec = "spec"
	// KindIndex is the index's lookups: find, search.
	KindIndex = "index"
)

// Status is where the work stands.
type Status struct {
	Branch string `json:"branch,omitempty"`
	// Default is the branch Branch is measured against, empty when there
	// is none to measure against.
	Default string `json:"default,omitempty"`
	// Dirty is -1 when git did not answer, which is not the same as a
	// clean tree — and a caller that read it as zero would call an
	// unknown tree clean.
	Dirty int `json:"dirty"`
}

// SpecVerdict is one spec's answer from the quality controller.
type SpecVerdict struct {
	Name    string   `json:"name"`
	Verdict string   `json:"verdict"`
	Gaps    []string `json:"gaps,omitempty"`
}

// Symbol is one hit from the code index.
type Symbol struct {
	Name      string `json:"name"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Kind      string `json:"kind,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// Setting is one effective setting and its provenance.
type Setting struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
	// Relaxed says the repository chose a value weaker than procoder's
	// default. It does not block — the repository chose it — but it is
	// never silent, because a green gate must not be able to mean "the
	// config was loosened" without saying so.
	Relaxed bool   `json:"relaxed"`
	Default string `json:"default,omitempty"`
}

// Task is one entry in the quality-gated task list.
type Task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	State string `json:"state"`
}

// Version is what is running, and what is newest where that was asked.
type Version struct {
	Running string `json:"running"`
	// Latest is empty when nobody asked GitHub — `procoder version`
	// without --check does not, and reporting the running version as the
	// latest would be inventing an answer.
	Latest string `json:"latest,omitempty"`
}

// Kinds is every kind this package declares. The guard reads the Kind
// constants out of the source and compares them to this, so the two
// cannot drift.
func Kinds() []string {
	return []string{KindFindings, KindConfig, KindTodo, KindVersion, KindStatus, KindSpec, KindIndex}
}
