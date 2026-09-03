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
)

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
	return []string{KindFindings, KindConfig, KindTodo, KindVersion}
}
