package bench

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

func joined(lines *[]string) string { return strings.Join(*lines, "\n") }

func write(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireGo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
}

// recorded `go test -bench . -benchmem` output, several rows plus noise.
const recordedOutput = `goos: darwin
goarch: arm64
pkg: example.com/demo
BenchmarkParse-8   	 1256821	       951.3 ns/op	     112 B/op	       3 allocs/op
BenchmarkRender-8  	   50000	     24011 ns/op	    4096 B/op	      12 allocs/op
BenchmarkTiny-8    	1000000000	         0.2571 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	example.com/demo	4.211s
`

func TestParseRowsFromRecordedOutput(t *testing.T) {
	rows := parseRows(strings.Split(recordedOutput, "\n"))
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d: %v", len(rows), rows)
	}
	if rows[0].Name != "BenchmarkParse-8" || rows[0].NsOp != 951.3 || rows[0].BOp != 112 {
		t.Fatalf("first row parsed wrong: %+v", rows[0])
	}
	if rows[2].NsOp != 0.2571 || rows[2].BOp != 0 {
		t.Fatalf("sub-ns row parsed wrong: %+v", rows[2])
	}
}

func TestParseRowsWithoutBenchmemColumns(t *testing.T) {
	rows := parseRows([]string{"BenchmarkLean-4   	  500000	      2100 ns/op"})
	if len(rows) != 1 || rows[0].NsOp != 2100 {
		t.Fatalf("plain row parsed wrong: %v", rows)
	}
	if rows[0].BOp != -1 {
		t.Fatalf("missing B/op column must read -1, got %v", rows[0].BOp)
	}
}

func TestParseBaselineSkipsHeader(t *testing.T) {
	raw := "# procoder bench baseline 2026-08-19 d74dd34 linux/amd64\n" +
		"BenchmarkParse-8   	 1256821	       951.3 ns/op	     112 B/op	       3 allocs/op\n"
	rows, goos, goarch := parseBaseline(raw)
	if len(rows) != 1 || rows[0].Name != "BenchmarkParse-8" {
		t.Fatalf("baseline rows parsed wrong: %v", rows)
	}
	if goos != "linux" || goarch != "amd64" {
		t.Fatalf("header platform parsed wrong: %s/%s", goos, goarch)
	}
}

func baselineFor(t *testing.T, root, platform, rows string) {
	t.Helper()
	write(t, root, filepath.FromSlash(Dir)+"/baseline.txt",
		"# procoder bench baseline 2026-08-19 abc1234 "+platform+"\n"+rows)
}

func here() string { return runtime.GOOS + "/" + runtime.GOARCH }

func TestCompareMarksRegressionBeyondThreshold(t *testing.T) {
	root := t.TempDir()
	baselineFor(t, root, here(),
		"BenchmarkParse-8   	 1000000	      1000 ns/op	     100 B/op	       3 allocs/op\n")
	out, lines := collect()
	current := []row{{Name: "BenchmarkParse-8", NsOp: 1200, BOp: 100}}
	if !compareBaseline(root, current, 10, out) {
		t.Fatal("a +20% ns/op delta at threshold 10 must regress")
	}
	got := joined(lines)
	if !strings.Contains(got, "REGRESSION") || !strings.Contains(got, "ns/op 1000 → 1200 (+20.0%)") {
		t.Fatalf("delta line wrong:\n%s", got)
	}
}

func TestCompareWithinThresholdIsClean(t *testing.T) {
	root := t.TempDir()
	baselineFor(t, root, here(),
		"BenchmarkParse-8   	 1000000	      1000 ns/op	     100 B/op	       3 allocs/op\n")
	out, lines := collect()
	current := []row{{Name: "BenchmarkParse-8", NsOp: 1050, BOp: 100}}
	if compareBaseline(root, current, 10, out) {
		t.Fatal("a +5% delta at threshold 10 must not regress")
	}
	if strings.Contains(joined(lines), "REGRESSION") {
		t.Fatalf("no REGRESSION mark expected:\n%s", joined(lines))
	}
}

func TestCompareRenamedBenchmarkIsGonePlusNewNotRegression(t *testing.T) {
	root := t.TempDir()
	baselineFor(t, root, here(),
		"BenchmarkOldName-8   	 1000000	      1000 ns/op	     100 B/op	       3 allocs/op\n")
	out, lines := collect()
	current := []row{{Name: "BenchmarkNewName-8", NsOp: 5000, BOp: 100}}
	if compareBaseline(root, current, 10, out) {
		t.Fatal("a rename must never read as a regression")
	}
	got := joined(lines)
	if !strings.Contains(got, "BenchmarkNewName-8  new") {
		t.Fatalf("renamed benchmark must be listed new:\n%s", got)
	}
	if !strings.Contains(got, "BenchmarkOldName-8  gone") {
		t.Fatalf("old name must be listed gone:\n%s", got)
	}
}

func TestCompareCrossPlatformWarns(t *testing.T) {
	other := "linux/amd64"
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		other = "darwin/arm64"
	}
	root := t.TempDir()
	baselineFor(t, root, other,
		"BenchmarkParse-8   	 1000000	      1000 ns/op	     100 B/op	       3 allocs/op\n")
	out, lines := collect()
	compareBaseline(root, []row{{Name: "BenchmarkParse-8", NsOp: 1000, BOp: 100}}, 10, out)
	got := joined(lines)
	if !strings.Contains(got, "WARNING — baseline recorded on "+other) {
		t.Fatalf("cross-platform warning missing:\n%s", got)
	}
}

func TestCompareCorruptBaselineSkipsWithReason(t *testing.T) {
	root := t.TempDir()
	write(t, root, filepath.FromSlash(Dir)+"/baseline.txt", "not a baseline at all\n")
	out, lines := collect()
	if compareBaseline(root, []row{{Name: "BenchmarkX-8", NsOp: 1, BOp: 0}}, 10, out) {
		t.Fatal("a corrupt baseline must not regress anything")
	}
	got := joined(lines)
	if !strings.Contains(got, "comparison skipped") || !strings.Contains(got, "--save") {
		t.Fatalf("corrupt baseline must skip with reason and offer --save:\n%s", got)
	}
}

func TestThresholdZeroMeansTen(t *testing.T) {
	if effectiveThreshold(0) != 10 {
		t.Fatal("threshold 0 must default to 10")
	}
	if effectiveThreshold(25) != 25 {
		t.Fatal("an explicit threshold must stand")
	}
}

func TestNegativeThresholdIsUsageError(t *testing.T) {
	out, lines := collect()
	if code := Run(t.TempDir(), false, -1, out); code != 2 {
		t.Fatalf("negative threshold must exit 2, got %d", code)
	}
	if !strings.Contains(joined(lines), "threshold") {
		t.Fatalf("usage error must name the threshold:\n%s", joined(lines))
	}
}

func TestNonGoRepoWithOtherManifestSaysScope(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Cargo.toml", "[package]\nname = \"demo\"\n")
	out, lines := collect()
	if code := Run(root, false, 0, out); code != 0 {
		t.Fatalf("non-Go repo with a manifest is informational, got %d", code)
	}
	if !strings.Contains(joined(lines), "bench covers Go in this version") {
		t.Fatalf("scope line missing:\n%s", joined(lines))
	}
}

func TestEmptyRepoCannotRun(t *testing.T) {
	out, lines := collect()
	if code := Run(t.TempDir(), false, 0, out); code != 2 {
		t.Fatalf("nothing to run must exit 2, got %d", code)
	}
	if !strings.Contains(joined(lines), "NOT run") {
		t.Fatalf("must say NOT run:\n%s", joined(lines))
	}
}

func TestNoBenchmarksIsSaidPlainly(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	write(t, root, "go.mod", "module fixture\n\ngo 1.21\n")
	write(t, root, "lib.go", "package fixture\n\nfunc Add(a, b int) int { return a + b }\n")
	out, lines := collect()
	if code := Run(root, false, 0, out); code != 0 {
		t.Fatalf("no benchmarks is informational, got %d", code)
	}
	if !strings.Contains(joined(lines), "no benchmarks in this repository") {
		t.Fatalf("no-benchmarks line missing:\n%s", joined(lines))
	}
}

const fixtureBenchmark = `package fixture

import "testing"

var sink int

func BenchmarkAdd(b *testing.B) {
	s := 0
	for i := 0; i < b.N; i++ {
		s += i
	}
	sink = s
}
`

// The live leg: --save writes the baseline with its header, a second run
// compares at ~0% and exits 0.
func TestLiveSaveThenCompare(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	write(t, root, "go.mod", "module fixture\n\ngo 1.21\n")
	write(t, root, "add_test.go", fixtureBenchmark)

	out, lines := collect()
	if code := Run(root, true, 0, out); code != 0 {
		t.Fatalf("save run must exit 0, got %d\n%s", code, joined(lines))
	}
	if !strings.Contains(joined(lines), "baseline saved to "+Dir+"/baseline.txt") {
		t.Fatalf("save must say the path:\n%s", joined(lines))
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Dir), "baseline.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "# procoder bench baseline ") ||
		!strings.Contains(string(raw), here()) {
		t.Fatalf("baseline header wrong:\n%s", raw)
	}
	if !strings.Contains(string(raw), "BenchmarkAdd") {
		t.Fatalf("baseline must hold the raw benchmark lines:\n%s", raw)
	}

	out2, lines2 := collect()
	if code := Run(root, false, 0, out2); code != 0 {
		t.Fatalf("second run must compare clean and exit 0, got %d\n%s", code, joined(lines2))
	}
	got := joined(lines2)
	if !strings.Contains(got, "ns/op") || !strings.Contains(got, "→") {
		t.Fatalf("second run must print the delta:\n%s", got)
	}
	if strings.Contains(got, "REGRESSION") {
		t.Fatalf("identical code must not regress:\n%s", got)
	}
}
