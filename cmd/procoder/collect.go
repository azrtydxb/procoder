package main

import (
	"procoder/internal/api"
	"procoder/internal/codeindex"
	"procoder/internal/gitx"
)

// collector gathers what a command reported, for a caller that wants the
// verdict as data rather than as the lines a person reads.
//
// nil for the CLI, and every write to it goes through the session, so a
// command cannot tell whether anybody is collecting. That is the property
// that keeps the two doors honest: a command that behaved differently when
// observed would make the parity test meaningless.
type collector struct {
	findings []api.Finding
	// reported says a findings-reporting command ran, which an empty
	// findings slice does not. It is the difference between "reports
	// findings, found none" and "does not report findings at all", and a
	// caller that flattened the two would read a clean gate and a version
	// number the same way.
	reported bool
	// A command answers in exactly one kind. Findings is the default and
	// the common case; these three are set by the commands that have a
	// value of their own, and setting one clears the findings default.
	kind     string
	settings []api.Setting
	tasks    []api.Task
	version  *api.Version
	status   *api.Status
	specs    []api.SpecVerdict
	symbols  []api.Symbol
}

// setSettings, setTasks and setVersion are how a command with a value of
// its own answers. Each one is the whole answer: a command sets one and
// never two.
//
// Every one is nil-safe, because the CLI's collector IS nil and a command
// must not have to know which caller it has. The first version of these
// guarded inside set() and assigned outside it, which compiled, passed
// every test that went through apiRunner, and segfaulted on `procoder
// version` at a terminal.
func (c *collector) setSettings(v []api.Setting) {
	if c.set(api.KindConfig) {
		c.settings = v
	}
}

func (c *collector) setTasks(v []api.Task) {
	if c.set(api.KindTodo) {
		c.tasks = v
	}
}

func (c *collector) setVersion(v *api.Version) {
	if c.set(api.KindVersion) {
		c.version = v
	}
}

func (c *collector) setStatus(v *api.Status) {
	if c.set(api.KindStatus) {
		c.status = v
	}
}

func (c *collector) setSpecs(v []api.SpecVerdict) {
	if c.set(api.KindSpec) {
		c.specs = v
	}
}

func (c *collector) setSymbols(v []api.Symbol) {
	if c.set(api.KindIndex) {
		c.symbols = v
	}
}

// set records the kind and reports whether there is a collector to record
// into at all.
func (c *collector) set(kind string) bool {
	if c == nil {
		return false
	}
	c.reported = true
	c.kind = kind
	return true
}

// add records one domain's findings. A domain that found nothing still
// counts as having reported.
func (c *collector) add(domain string, findings []gitx.Finding) {
	if c == nil {
		return
	}
	c.reported = true
	for _, f := range findings {
		c.findings = append(c.findings, api.Finding{
			File:     f.File,
			Line:     f.Line,
			Message:  f.Message,
			Blocking: f.Blocking,
			Domain:   domain,
		})
	}
}

// result is what the collector has, in the envelope's shape. A command
// that reported no findings at all answers nil.
func (c *collector) result() *api.Result {
	if c == nil || !c.reported {
		return nil
	}
	switch c.kind {
	case api.KindConfig:
		return &api.Result{Kind: api.KindConfig, Settings: c.settings}
	case api.KindTodo:
		return &api.Result{Kind: api.KindTodo, Tasks: c.tasks}
	case api.KindVersion:
		return &api.Result{Kind: api.KindVersion, Version: c.version}
	case api.KindStatus:
		return &api.Result{Kind: api.KindStatus, Status: c.status}
	case api.KindSpec:
		return &api.Result{Kind: api.KindSpec, Specs: c.specs}
	case api.KindIndex:
		return &api.Result{Kind: api.KindIndex, Symbols: c.symbols}
	}
	// Never nil where reported: the JSON must carry [] rather than
	// omitting the key, so a client can tell an empty list from an absent
	// one without knowing which commands report findings.
	findings := c.findings
	if findings == nil {
		findings = []api.Finding{}
	}
	return &api.Result{Kind: api.KindFindings, Findings: findings}
}

// collectSymbols records an index lookup's hits, and records nothing when
// the lookup itself failed — an empty symbol list would say the index
// answered and found none.
func (s session) collectSymbols(tags []codeindex.Tag, err error) {
	if s.col == nil || err != nil {
		return
	}
	got := make([]api.Symbol, 0, len(tags))
	for _, t := range tags {
		got = append(got, api.Symbol{
			Name: t.Name, File: t.Path, Line: t.Line,
			Kind: t.Kind, Signature: t.Signature,
		})
	}
	s.col.setSymbols(got)
}
