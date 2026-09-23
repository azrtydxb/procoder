package testrun

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The Go runner reads `go test -json`, not the text stream, because the
// text stream cannot say which test a line belongs to.
//
// It used to scrape `^--- FAIL: (\S+)` out of the combined output, and
// that attributes wrongly in two ways that were demonstrated rather than
// imagined (#283):
//
//   - A test that prints `--- FAIL: TestSomethingElse` to stdout is
//     indistinguishable from the framework reporting that test's failure,
//     so an innocent name is reported and the count is wrong. In JSON the
//     framework's verdict is an event with an Action; what a test prints
//     is only ever Output.
//   - A package that failed outside any test — a build or setup failure, a
//     panic in a background goroutine, a timeout — has no `--- FAIL:` line
//     at all. Beside one real test failure it vanished from the report,
//     and alone it was summarised as the first line of the output, which
//     is as likely to be an unrelated package's "ok".

// goEvent is one line of `go test -json` (cmd/test2json), the fields read.
type goEvent struct {
	Action      string
	Package     string
	Test        string
	Output      string
	ImportPath  string // build-output / build-fail events (Go 1.24+)
	FailedBuild string // on a package's fail event: which build broke it
}

// goKey names one test in one package. Test is empty for the package's
// own lines — the ones outside any test function.
type goKey struct{ pkg, test string }

// goRun is what one `go test -json` invocation said, sorted by owner.
type goRun struct {
	// summary is the package-level output alone — "ok", "FAIL", "?",
	// coverage lines — which is what the non-verbose text stream used to
	// be, and what the pass-side parsing reads.
	summary strings.Builder
	// stray is every line that was not JSON. Before Go 1.24 build errors
	// went to stderr as text; nothing else should land here.
	stray []string

	lines       map[goKey][]string
	keyOrder    []goKey             // first appearance, so excerpts are stable
	build       map[string][]string // build-output, by ImportPath
	buildOrder  []string
	failedTests []goKey // in the order the framework failed them
	failedPkgs  []string
	failBuild   map[string]string // package -> FailedBuild
	coverage    map[string]float64
	coverOrder  []string
}

var goCoverLineRe = regexp.MustCompile(`coverage:\s+([0-9.]+)% of statements`)

func parseGoJSON(raw string) *goRun {
	r := &goRun{
		lines:     map[goKey][]string{},
		build:     map[string][]string{},
		failBuild: map[string]string{},
		coverage:  map[string]float64{},
	}
	for _, line := range strings.Split(raw, "\n") {
		var ev goEvent
		if !strings.HasPrefix(line, "{") || json.Unmarshal([]byte(line), &ev) != nil || ev.Action == "" {
			if strings.TrimSpace(line) != "" {
				r.stray = append(r.stray, line)
			}
			continue
		}
		switch ev.Action {
		case "build-output":
			if _, seen := r.build[ev.ImportPath]; !seen {
				r.buildOrder = append(r.buildOrder, ev.ImportPath)
			}
			r.build[ev.ImportPath] = append(r.build[ev.ImportPath], strings.TrimRight(ev.Output, "\n"))
		case "output":
			r.addOutput(ev)
		case "fail":
			if ev.Test != "" {
				r.failedTests = append(r.failedTests, goKey{ev.Package, ev.Test})
				continue
			}
			r.failedPkgs = append(r.failedPkgs, ev.Package)
			if ev.FailedBuild != "" {
				r.failBuild[ev.Package] = ev.FailedBuild
			}
		}
	}
	return r
}

// addOutput files one output line under the test that printed it, and a
// package-level line into the summary as well.
func (r *goRun) addOutput(ev goEvent) {
	text := strings.TrimRight(ev.Output, "\n")
	k := goKey{ev.Package, ev.Test}
	if _, seen := r.lines[k]; !seen {
		r.keyOrder = append(r.keyOrder, k)
	}
	r.lines[k] = append(r.lines[k], text)
	if ev.Test != "" {
		return
	}
	r.summary.WriteString(ev.Output)
	// One number per package. The binary prints its coverage and the "ok"
	// line repeats it; counting both would double the package count the
	// mean is reported over.
	if m := goCoverLineRe.FindStringSubmatch(text); m != nil {
		if _, seen := r.coverage[ev.Package]; !seen {
			r.coverOrder = append(r.coverOrder, ev.Package)
		}
		r.coverage[ev.Package], _ = strconv.ParseFloat(m[1], 64)
	}
}

// leafFailures are the failing tests with no failing subtest of their own.
// A failed subtest fails its parent too; naming the parent says less, and
// counting both says the same failure twice.
func (r *goRun) leafFailures() []goKey {
	var out []goKey
	for _, k := range r.failedTests {
		leaf := true
		for _, o := range r.failedTests {
			if o.pkg == k.pkg && strings.HasPrefix(o.test, k.test+"/") {
				leaf = false
				break
			}
		}
		if leaf {
			out = append(out, k)
		}
	}
	return out
}

// packageOnlyFailures are packages that failed with no failing test in
// them: a build or setup failure, a crash outside a test's own goroutine,
// a timeout. These have no test to name, and are named by package.
func (r *goRun) packageOnlyFailures() []string {
	hasTest := map[string]bool{}
	for _, k := range r.failedTests {
		hasTest[k.pkg] = true
	}
	var out []string
	for _, p := range r.failedPkgs {
		if !hasTest[p] {
			out = append(out, p)
		}
	}
	return out
}

// pkgFailLabel is a package name with the reason go test gave in its FAIL
// line, when there is one: "procoder/x [build failed]".
func (r *goRun) pkgFailLabel(pkg string) string {
	for _, l := range r.lines[goKey{pkg, ""}] {
		if rest, ok := strings.CutPrefix(l, "FAIL\t"+pkg+" "); ok && strings.HasPrefix(rest, "[") {
			return pkg + " " + rest
		}
	}
	return pkg
}

// failDetail is the one-line verdict for a failed run.
func (r *goRun) failDetail() (string, int) {
	leaves := r.leafFailures()
	pkgs := r.packageOnlyFailures()
	names := make([]string, 0, len(leaves))
	for _, k := range leaves {
		names = append(names, k.test)
	}
	labels := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		labels = append(labels, r.pkgFailLabel(p))
	}
	switch {
	case len(names) > 0 && len(labels) > 0:
		return fmt.Sprintf("%d test(s) failing: %s; %d package(s) failed outside any test: %s",
			len(names), strings.Join(cap3(names), ", "), len(labels), strings.Join(cap3(labels), ", ")), len(names)
	case len(names) > 0:
		return fmt.Sprintf("%d test(s) failing: %s", len(names), strings.Join(cap3(names), ", ")), len(names)
	case len(labels) > 0:
		return fmt.Sprintf("%d package(s) failed outside any test: %s", len(labels), strings.Join(cap3(labels), ", ")), 0
	}
	return "", 0
}

// The excerpt is bounded on every axis, because it lands in a gate that
// already prints a great deal: a few failures, a few lines of each, each
// line capped, and the whole capped again.
const (
	excerptEntries  = 3
	excerptHead     = 5  // the top of a failure: where a panic says what it was
	excerptLines    = 20 // per failure, head included
	excerptLineMax  = 240
	excerptTotalMax = 4096
)

// excerpt is the raw output behind the failures the detail names — the
// assertion, the panic, the build error — so a failure that does not
// reproduce still says what it was. Empty when there is nothing to show.
func (r *goRun) excerpt() string {
	type entry struct {
		title string
		lines []string
	}
	var entries []entry
	for _, k := range r.leafFailures() {
		entries = append(entries, entry{"--- FAIL: " + k.test + " (" + k.pkg + ")", testLines(r.lines[k], k.test)})
	}
	pkgFails := r.packageOnlyFailures()
	if len(pkgFails) > 0 && len(r.build) == 0 && len(r.stray) > 0 {
		// Before Go 1.24 a build error is stderr text, not an event, and
		// cannot be tied to a package. Shown once and labelled as such:
		// copied under each failed package it would read as every
		// package's own error.
		entries = append(entries, entry{"--- go output not attributed to a package", r.stray})
	}
	for _, p := range pkgFails {
		entries = append(entries, entry{"--- FAIL: " + r.pkgFailLabel(p), r.packageLines(p)})
	}
	if len(entries) == 0 && len(r.stray) > 0 {
		// Nothing JSON said failed, yet the run did: whatever go printed
		// outside the stream is the only account there is.
		entries = append(entries, entry{"--- go test output", r.stray})
	}
	var b strings.Builder
	for i, e := range entries {
		if i == excerptEntries {
			fmt.Fprintf(&b, "… %d more failure(s) not shown\n", len(entries)-i)
			break
		}
		// Capped like every other line: a long subtest name or import
		// path must not spend the budget before the diagnosis prints.
		b.WriteString(capLine(e.title) + "\n")
		for _, l := range bound(e.lines) {
			b.WriteString("    " + capLine(l) + "\n")
		}
	}
	out := b.String()
	if len(out) > excerptTotalMax {
		// At a line boundary: a line cut mid-way reads as the line.
		cut := strings.LastIndex(out[:excerptTotalMax], "\n")
		out = out[:cut+1] + "… output truncated\n"
	}
	return strings.TrimRight(out, "\n")
}

// testLines is a test's own output without the framework's run markers,
// which say nothing a reader of a failure needs.
func testLines(lines []string, test string) []string {
	var out []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if isFrame(t) || strings.HasPrefix(t, "--- FAIL: "+test+" ") {
			continue
		}
		out = append(out, l)
	}
	return out
}

func isFrame(t string) bool {
	for _, p := range []string{"=== RUN", "=== PAUSE", "=== CONT", "=== NAME", "--- PASS:", "--- SKIP:"} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// packageLines is everything a package printed that bears on a failure
// no test owns: its build errors, and every line of its run — a crash
// outside a test is attributed to whichever test happened to be running,
// so the package's own lines alone would miss it.
func (r *goRun) packageLines(pkg string) []string {
	var out []string
	for _, ip := range r.buildOrder {
		if ip == pkg || strings.HasPrefix(ip, pkg+" [") || ip == r.failBuild[pkg] {
			out = append(out, r.build[ip]...)
		}
	}
	for _, k := range r.keyOrder {
		if k.pkg != pkg || k.test == "" {
			continue
		}
		for _, l := range r.lines[k] {
			if !isFrame(strings.TrimSpace(l)) {
				out = append(out, l)
			}
		}
	}
	return append(out, r.lines[goKey{pkg, ""}]...)
}

// bound keeps the head and the tail of a long failure: the head is where a
// panic names itself, the tail is where the assertion that ended the test
// sits.
func bound(lines []string) []string {
	if len(lines) <= excerptLines {
		return lines
	}
	tail := excerptLines - excerptHead
	out := append([]string{}, lines[:excerptHead]...)
	out = append(out, fmt.Sprintf("… %d line(s) elided", len(lines)-excerptLines))
	return append(out, lines[len(lines)-tail:]...)
}

func capLine(l string) string {
	if len(l) > excerptLineMax {
		cut := excerptLineMax
		for cut > 0 && !utf8.RuneStart(l[cut]) {
			cut--
		}
		return l[:cut] + "…"
	}
	return l
}
