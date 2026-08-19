package deps

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// recorded `go list -u -m all` shape: main module, current deps,
// updates, an indirect comment, and a replace directive as the tool
// reports them.
const goListRecorded = `procoder
github.com/fatih/color v1.16.0
golang.org/x/text v0.14.0 [v0.17.0]
golang.org/x/sys v0.15.0 [v0.24.0] // indirect
gopkg.in/yaml.v3 v3.0.1
example.com/old v1.2.3 => example.com/fork v1.2.4 [v1.3.0]
`

func TestParseGoList(t *testing.T) {
	rows := parseGoList(goListRecorded)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].name != "golang.org/x/text" || rows[0].current != "v0.14.0" || rows[0].latest != "v0.17.0" {
		t.Errorf("row 0 wrong: %+v", rows[0])
	}
	if rows[1].name != "golang.org/x/sys" || rows[1].latest != "v0.24.0" {
		t.Errorf("indirect row wrong: %+v", rows[1])
	}
	if rows[2].name != "example.com/old" || rows[2].current != "v1.2.4" || rows[2].latest != "v1.3.0" {
		t.Errorf("replace row wrong: %+v", rows[2])
	}
}

func TestParseGoListEverythingCurrent(t *testing.T) {
	raw := "procoder\ngithub.com/fatih/color v1.16.0\ngopkg.in/yaml.v3 v3.0.1\n"
	if rows := parseGoList(raw); len(rows) != 0 {
		t.Fatalf("up-to-date output must yield no rows, got %+v", rows)
	}
}

// recorded `npm outdated --json` shape.
const npmRecorded = `{
  "left-pad": {"current": "1.0.0", "wanted": "1.3.0", "latest": "1.3.0", "location": "node_modules/left-pad"},
  "react": {"current": "17.0.2", "wanted": "17.0.2", "latest": "18.3.1", "dependent": "app"},
  "ghost": {"wanted": "2.0.0", "latest": "2.0.0"}
}`

func TestParseNpmOutdated(t *testing.T) {
	rows, err := parseNpmOutdated(npmRecorded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d: %+v", len(rows), rows)
	}
	// rows are sorted by name: ghost, left-pad, react
	if rows[0].name != "ghost" || rows[0].current != "missing" {
		t.Errorf("uninstalled package must read missing: %+v", rows[0])
	}
	if rows[1].name != "left-pad" || rows[1].current != "1.0.0" || rows[1].latest != "1.3.0" || rows[1].wanted != "1.3.0" {
		t.Errorf("left-pad row wrong: %+v", rows[1])
	}
	if rows[2].name != "react" || rows[2].latest != "18.3.1" {
		t.Errorf("react row wrong: %+v", rows[2])
	}
}

func TestParseNpmOutdatedEmptyMeansUpToDate(t *testing.T) {
	for _, raw := range []string{"", "  \n", "{}"} {
		rows, err := parseNpmOutdated(raw)
		if err != nil || len(rows) != 0 {
			t.Errorf("empty output %q must be up to date, got rows=%v err=%v", raw, rows, err)
		}
	}
}

func TestParseNpmOutdatedUnparseable(t *testing.T) {
	if _, err := parseNpmOutdated("npm ERR! network tunneling socket"); err == nil {
		t.Fatal("garbage must be an error, never a silently empty table")
	}
}

func TestParseNpmOutdatedErrorObject(t *testing.T) {
	raw := `{"error": {"code": "ENETWORK", "summary": "registry unreachable"}}`
	_, err := parseNpmOutdated(raw)
	if err == nil || !strings.Contains(err.Error(), "registry unreachable") {
		t.Fatalf("npm error object must surface its summary, got %v", err)
	}
}

func TestParsePipOutdated(t *testing.T) {
	raw := `[{"name": "requests", "version": "2.28.0", "latest_version": "2.32.3", "latest_filetype": "wheel"},
	         {"name": "urllib3", "version": "1.26.5", "latest_version": "2.2.2"}]`
	rows, err := parsePipOutdated(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 || rows[0].name != "requests" || rows[0].current != "2.28.0" || rows[0].latest != "2.32.3" {
		t.Fatalf("pip rows wrong: %+v", rows)
	}
	if rows[1].name != "urllib3" || rows[1].latest != "2.2.2" {
		t.Errorf("urllib3 row wrong: %+v", rows[1])
	}
}

func TestParsePipOutdatedEmptyAndUnparseable(t *testing.T) {
	if rows, err := parsePipOutdated("[]"); err != nil || len(rows) != 0 {
		t.Errorf("empty array must be up to date, got rows=%v err=%v", rows, err)
	}
	if rows, err := parsePipOutdated(""); err != nil || len(rows) != 0 {
		t.Errorf("empty output must be up to date, got rows=%v err=%v", rows, err)
	}
	if _, err := parsePipOutdated("WARNING: pip is being invoked"); err == nil {
		t.Error("garbage must be an error, never a silently empty table")
	}
}

func TestMajorBehind(t *testing.T) {
	cases := []struct {
		cur, latest string
		want        bool
	}{
		{"v1.2.3", "v2.0.0", true},
		{"1.2.3", "2.0.0", true},
		{"v1.2.3", "v1.9.9", false},
		{"17.0.2", "18.3.1", true},
		{"0.14.0", "0.17.0", false},
		{"v2.0.0+incompatible", "v3.0.0", true},
		{"missing", "2.0.0", false},
		{"v0.0.0-20230101000000-abcdef123456", "v0.1.0", false},
		{"", "1.0.0", false},
	}
	for _, c := range cases {
		if got := majorBehind(c.cur, c.latest); got != c.want {
			t.Errorf("majorBehind(%q, %q) = %v, want %v", c.cur, c.latest, got, c.want)
		}
	}
}

func TestRunNoManifests(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	if code != 0 {
		t.Fatalf("no manifests must exit 0, got %d", code)
	}
	if len(lines) != 1 || lines[0] != "no dependency manifests in this repository" {
		t.Fatalf("wrong output: %v", lines)
	}
}

// TestRunCargoOutdatedMissing proves the honesty line: cargo present,
// the cargo-outdated plugin absent — NOT checked naming the install,
// informational (exit 0), never an error and never a clean section.
func TestRunCargoOutdatedMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub PATH script is a shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"fixture\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := t.TempDir()
	// a cargo that has no outdated subcommand: the probe fails, so the
	// section must say the plugin is not installed
	script := "#!/bin/sh\nexit 101\n"
	if err := os.WriteFile(filepath.Join(stub, "cargo"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stub)
	t.Setenv("VIRTUAL_ENV", "")

	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	if code != 0 {
		t.Fatalf("a missing optional tool is informational, want exit 0, got %d\n%s", code, strings.Join(lines, "\n"))
	}
	joined := strings.Join(lines, "\n")
	want := "rust: NOT checked — cargo-outdated is not installed (cargo install cargo-outdated)"
	if !strings.Contains(joined, want) {
		t.Errorf("missing honest line %q in:\n%s", want, joined)
	}
	if !strings.Contains(joined, "licenses (rust): NOT checked — no canonical no-install tool") {
		t.Errorf("missing rust licenses honesty line in:\n%s", joined)
	}
	if !strings.Contains(joined, "0 dependency(ies) behind across 0 ecosystem(s), 0 major(s)") {
		t.Errorf("missing zero summary in:\n%s", joined)
	}
}

func TestEmitCapsAtThirty(t *testing.T) {
	var rows []row
	for i := 0; i < 42; i++ {
		rows = append(rows, row{name: fmt.Sprintf("pkg%02d", i), current: "1.0.0", latest: "1.1.0"})
	}
	var lines []string
	emit(rows, func(s string) { lines = append(lines, s) }, func(r row) string { return "  " + r.name })
	if len(lines) != 31 {
		t.Fatalf("want 30 rows plus the more line, got %d", len(lines))
	}
	if lines[30] != "  …12 more" {
		t.Errorf("wrong tail line: %q", lines[30])
	}
}
