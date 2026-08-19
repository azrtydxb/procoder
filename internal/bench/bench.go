// Package bench gives the perf skill its numbers: it runs the
// repository's Go benchmarks, prints the results, and compares them
// against the committed baseline when one exists. Results are single-run
// and machine-local — the output says so — and the only file this
// package ever writes is the baseline, only under --save. Go only in
// this version, and every output where that matters says it plainly.
package bench

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Dir is the procoder-owned state directory; the baseline lives at
// Dir/baseline.txt and is meant to be committed.
const Dir = ".procoder/bench"

const (
	runTimeout       = 10 * time.Minute
	defaultThreshold = 10 // percent
)

// row is one parsed benchmark result line. BOp is -1 when the line had
// no -benchmem columns.
type row struct {
	Name string
	NsOp float64
	BOp  float64
}

// benchRowRe matches `BenchmarkName-N  iterations  X ns/op  Y B/op  Z allocs/op`;
// the B/op and allocs/op columns are optional (output without -benchmem).
var benchRowRe = regexp.MustCompile(`^(Benchmark\S+)\s+(\d+)\s+([0-9.]+) ns/op(?:\s+([0-9.]+) B/op)?(?:\s+(\d+) allocs/op)?`)

// baselineHeaderRe matches the header line --save writes:
// `# procoder bench baseline <date> <commit> <GOOS>/<GOARCH>`.
var baselineHeaderRe = regexp.MustCompile(`^# procoder bench baseline (\S+) (\S+) (\S+)/(\S+)$`)

// Run executes the repository's Go benchmarks under root, prints the
// results via out, compares against Dir/baseline.txt when it exists, and
// writes that baseline when save is set. threshold is the regression
// percentage to mark (0 means the default of 10; negative is a usage
// error). Exit code: 0 clean or informational, 1 when any REGRESSION or
// the run failed, 2 usage or nothing to run.
func Run(root string, save bool, threshold int, out func(string)) int {
	if threshold < 0 {
		out(fmt.Sprintf("threshold must be zero or positive (got %d) — [bench] threshold is a percentage", threshold))
		return 2
	}
	threshold = effectiveThreshold(threshold)
	if !exists(root, "go.mod") {
		if m := otherManifest(root); m != "" {
			out("NOT run — bench covers Go in this version (this repository has " + m + ", not go.mod)")
			return 0
		}
		out("NOT run — bench covers Go in this version and this repository has no go.mod")
		return 2
	}
	bin, err := exec.LookPath("go")
	if err != nil {
		out("NOT run — the go toolchain is not installed (a Go-less machine cannot bench Go)")
		return 2
	}
	if !hasBenchmarks(root) {
		out("no benchmarks in this repository — the perf skill starts by writing one")
		return 0
	}
	raw, runErr, timedOut := execute(root, bin, []string{"test", "-bench", ".", "-benchmem", "-run", "^$", "./..."})
	if timedOut {
		out("NOT run — go test -bench gave no answer in " + runTimeout.String())
		return 1
	}
	if runErr != nil {
		out("NOT run — the bench run failed: " + firstFailingLine(raw, runErr.Error()))
		return 1
	}
	lines := benchLines(raw)
	current := parseRows(lines)
	if len(current) == 0 {
		out("NOT run — go test -bench produced no benchmark rows: " + firstLine(raw))
		return 1
	}
	for _, l := range lines {
		out(l)
	}
	out(fmt.Sprintf("single run on %s/%s — machine-local numbers, not statistics", runtime.GOOS, runtime.GOARCH))

	regressed := compareBaseline(root, current, threshold, out)

	if save {
		if err := saveBaseline(root, lines); err != nil {
			out("baseline NOT saved — " + err.Error())
			return 1
		}
		out("baseline saved to " + Dir + "/baseline.txt")
	}
	if regressed {
		return 1
	}
	return 0
}

// compareBaseline compares current rows against Dir/baseline.txt and
// prints the deltas; it returns true when any ns/op delta exceeds the
// threshold. A missing baseline is silent (nothing to compare yet); a
// corrupt or unreadable one skips comparison with the reason and offers
// --save as the fix.
func compareBaseline(root string, current []row, threshold int, out func(string)) bool {
	path := filepath.Join(root, filepath.FromSlash(Dir), "baseline.txt")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		out("no baseline at " + Dir + "/baseline.txt — run with --save to record one")
		return false
	}
	if err != nil {
		out("comparison skipped — baseline unreadable: " + err.Error() + "; --save writes a fresh one")
		return false
	}
	base, goos, goarch := parseBaseline(string(raw))
	if len(base) == 0 {
		out("comparison skipped — baseline has no benchmark rows (corrupt or hand-edited); --save writes a fresh one")
		return false
	}
	if goos != "" && (goos != runtime.GOOS || goarch != runtime.GOARCH) {
		out(fmt.Sprintf("WARNING — baseline recorded on %s/%s, this run is %s/%s: deltas cross machines and prove little",
			goos, goarch, runtime.GOOS, runtime.GOARCH))
	}
	byName := map[string]row{}
	for _, b := range base {
		byName[b.Name] = b
	}
	regressed := false
	for _, c := range current {
		b, ok := byName[c.Name]
		if !ok {
			out(c.Name + "  new (no baseline entry)")
			continue
		}
		delete(byName, c.Name)
		nsPct := pctDelta(b.NsOp, c.NsOp)
		line := fmt.Sprintf("%s  ns/op %s → %s (%+.1f%%)", c.Name, fnum(b.NsOp), fnum(c.NsOp), nsPct)
		if b.BOp >= 0 && c.BOp >= 0 {
			line += fmt.Sprintf("  B/op %s → %s (%+.1f%%)", fnum(b.BOp), fnum(c.BOp), pctDelta(b.BOp, c.BOp))
		}
		if nsPct > float64(threshold) {
			line += "  REGRESSION"
			regressed = true
		}
		out(line)
	}
	for _, b := range base {
		if _, gone := byName[b.Name]; gone {
			out(b.Name + "  gone (in baseline, not in this run)")
		}
	}
	return regressed
}

// saveBaseline writes Dir/baseline.txt: the header line, then the raw
// benchmark lines. This is the only write the package ever performs.
func saveBaseline(root string, lines []string) error {
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	header := fmt.Sprintf("# procoder bench baseline %s %s %s/%s",
		time.Now().Format("2006-01-02"), commit(root), runtime.GOOS, runtime.GOARCH)
	body := header + "\n" + strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filepath.Join(dir, "baseline.txt"), []byte(body), 0o644)
}

// parseBaseline parses a stored baseline: rows plus the GOOS/GOARCH the
// header recorded (empty when the header is missing or malformed —
// still comparable, just without the cross-platform check).
func parseBaseline(raw string) (rows []row, goos, goarch string) {
	var lines []string
	for _, l := range strings.Split(raw, "\n") {
		if m := baselineHeaderRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			goos, goarch = m[3], m[4]
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		lines = append(lines, l)
	}
	return parseRows(lines), goos, goarch
}

// parseRows extracts benchmark rows from output lines; lines that are
// not benchmark results are ignored.
func parseRows(lines []string) []row {
	var out []row
	for _, l := range lines {
		m := benchRowRe.FindStringSubmatch(strings.TrimSpace(l))
		if m == nil {
			continue
		}
		r := row{Name: m[1], BOp: -1}
		r.NsOp, _ = strconv.ParseFloat(m[3], 64)
		if m[4] != "" {
			r.BOp, _ = strconv.ParseFloat(m[4], 64)
		}
		out = append(out, r)
	}
	return out
}

// benchLines picks the raw benchmark result lines out of go test output.
func benchLines(raw string) []string {
	var out []string
	for _, l := range strings.Split(raw, "\n") {
		if benchRowRe.MatchString(strings.TrimSpace(l)) {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

// hasBenchmarks finds Benchmark functions via git grep, falling back to
// a filesystem walk so a repository without git still answers.
func hasBenchmarks(root string) bool {
	if git, err := exec.LookPath("git"); err == nil {
		cmd := exec.Command(git, "-C", root, "grep", "-l", "func Benchmark", "--", "*_test.go")
		out, err := cmd.Output()
		if err == nil {
			return len(bytes.TrimSpace(out)) > 0
		}
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
			return false // git ran fine; there are simply no matches
		}
		// git absent from the repo or otherwise unusable — walk instead
	}
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if found {
			return filepath.SkipAll
		}
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		if raw, err := os.ReadFile(path); err == nil && bytes.Contains(raw, []byte("func Benchmark")) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// commit answers the short HEAD hash for the baseline header, "unknown"
// when git or a repository is absent — the header stays honest either way.
func commit(root string) string {
	git, err := exec.LookPath("git")
	if err != nil {
		return "unknown"
	}
	out, err := exec.Command(git, "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		return s
	}
	return "unknown"
}

// otherManifest names the first non-Go manifest present, empty when the
// repository has none we recognize.
func otherManifest(root string) string {
	for _, m := range []string{"Cargo.toml", "package.json", "pyproject.toml", "pytest.ini", "build.gradle", "build.gradle.kts", "pom.xml"} {
		if exists(root, m) {
			return m
		}
	}
	return ""
}

// effectiveThreshold maps the config value onto the percentage compare
// uses: zero means the default of 10 (negatives never reach here — Run
// refuses them as usage errors).
func effectiveThreshold(t int) int {
	if t == 0 {
		return defaultThreshold
	}
	return t
}

// pctDelta is the percentage change from old to cur; a zero old value
// answers 0 rather than dividing by it.
func pctDelta(old, cur float64) float64 {
	if old == 0 {
		return 0
	}
	return (cur - old) / old * 100
}

// fnum prints a benchmark number the way go test does: no trailing
// zeros, no scientific notation.
func fnum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func execute(root, bin string, args []string) (string, error, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- fixed go test invocation, never user input
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err, ctx.Err() == context.DeadlineExceeded
}

// firstFailingLine prefers a line that names a failure over the first
// line of noise; a run that printed nothing still answers with the exit
// status.
func firstFailingLine(raw, errText string) string {
	for _, l := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(l)
		low := strings.ToLower(t)
		if t != "" && (strings.Contains(low, "fail") || strings.Contains(low, "error") || strings.Contains(low, "panic")) {
			return trim160(t)
		}
	}
	if l := firstLine(raw); l != "no output" {
		return l
	}
	return "no output, exit status: " + errText
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			return trim160(t)
		}
	}
	return "no output"
}

func trim160(s string) string {
	if len(s) > 160 {
		return s[:160]
	}
	return s
}

func exists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}
