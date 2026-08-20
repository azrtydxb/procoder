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

// npm under workspaces reports an ARRAY per package name, one entry per
// workspace that depends on it.
// proved by: deleted the array fallback in parseNpmOutdated — the workspace
// shape then reads as "unexpected shape" and the whole report is lost.
func TestParseNpmOutdatedWorkspaceArray(t *testing.T) {
	raw := `{
	  "typescript": [
	    {"current": "5.1.6", "wanted": "5.5.4", "latest": "5.5.4", "dependent": "web"},
	    {"wanted": "5.5.4", "latest": "5.5.4", "dependent": "api"}
	  ],
	  "vite": {"current": "4.0.0", "wanted": "4.5.3", "latest": "5.4.0"}
	}`
	rows, err := parseNpmOutdated(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want one row per workspace entry plus vite, got %d: %+v", len(rows), rows)
	}
	if rows[0].name != "typescript" || rows[0].current != "5.1.6" || rows[0].latest != "5.5.4" || rows[0].wanted != "5.5.4" {
		t.Errorf("first workspace row wrong: %+v", rows[0])
	}
	if rows[1].name != "typescript" || rows[1].current != "missing" {
		t.Errorf("a workspace entry with no current version reads missing: %+v", rows[1])
	}
	if rows[2].name != "vite" || rows[2].latest != "5.4.0" {
		t.Errorf("vite row wrong: %+v", rows[2])
	}
}

// proved by: made the array fallback's failure `continue` instead of
// returning — an entry npm printed in a shape we cannot read then vanishes
// from the table with no word said.
func TestParseNpmOutdatedUnexpectedEntryShapeIsAnError(t *testing.T) {
	_, err := parseNpmOutdated(`{"left-pad": "1.3.0"}`)
	if err == nil || !strings.Contains(err.Error(), "left-pad") {
		t.Fatalf("an unreadable entry must be an error naming the package, got %v", err)
	}
}

// proved by: dropped the `//` comment strip in parseGoList (a bracketed
// version inside a comment then reads as a real update), and separately by
// widening the bracket test to len(f) > 0, which turns `[]` into a row with
// no version at all.
func TestParseGoListIgnoresBracketsInCommentsAndEmptyBrackets(t *testing.T) {
	raw := "example.com/commented v1.0.0 // was [v2.0.0]\n" +
		"example.com/empty v1.0.0 []\n" +
		"example.com/real v1.0.0 [v1.1.0]\n"
	rows := parseGoList(raw)
	if len(rows) != 1 {
		t.Fatalf("only the real update is a row, got %+v", rows)
	}
	if rows[0].name != "example.com/real" || rows[0].latest != "v1.1.0" {
		t.Errorf("wrong row: %+v", rows[0])
	}
}

// stubPATH puts executable shell stubs (name -> script body) on an isolated
// PATH — CI's test leg has no go, npm or pip, and these legs need none: what
// is under test is how deps reads what such a tool prints.
func stubPATH(t *testing.T, stubs map[string]string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the tool stubs are shell scripts")
	}
	dir := t.TempDir()
	for name, body := range stubs {
		// the stub needs the ordinary unix utilities; PATH below holds
		// nothing, so no real toolchain can answer for it
		script := "#!/bin/sh\nPATH=/usr/bin:/bin\n" + body
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("VIRTUAL_ENV", "")
	return dir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// proved by: made checkGo return `true` (failed) when the toolchain is
// absent — a machine without Go then exits 1 on a report it never ran.
func TestRunGoToolchainAbsentIsInformational(t *testing.T) {
	stubPATH(t, nil)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")

	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if code != 0 {
		t.Fatalf("an absent tool is informational, want exit 0, got %d\n%s", code, joined)
	}
	if !strings.Contains(joined, "go: NOT checked — the go toolchain is not installed") {
		t.Errorf("missing the honest line in:\n%s", joined)
	}
	if !strings.Contains(joined, "licenses (go): NOT checked — go-licenses is not installed") {
		t.Errorf("missing the licenses honesty line in:\n%s", joined)
	}
	if !strings.Contains(joined, "0 dependency(ies) behind across 0 ecosystem(s), 0 major(s)") {
		t.Errorf("missing the zero summary in:\n%s", joined)
	}
}

// A tool that RAN AND FAILED is the other case entirely: exit 1.
// proved by: made checkGo return `false` on the error path — a broken
// toolchain then exits 0 and the failure passes for a clean report.
func TestRunGoListFailureExitsOne(t *testing.T) {
	stubPATH(t, map[string]string{"go": "echo 'go: updates.example.com: dial tcp: i/o timeout' >&2\necho 'and more noise' >&2\nexit 1\n"})
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")

	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if code != 1 {
		t.Fatalf("a tool that ran and failed must exit 1, got %d\n%s", code, joined)
	}
	if !strings.Contains(joined, "go: NOT checked — go: updates.example.com: dial tcp: i/o timeout") {
		t.Errorf("must quote the tool's first line:\n%s", joined)
	}
	if strings.Contains(joined, "and more noise") {
		t.Errorf("only the first line belongs in the report:\n%s", joined)
	}
}

// proved by: made count() skip the majorBehind tally (majors always 0) —
// the summary then under-reports the expensive upgrades.
func TestRunGoReportsRowsAndSummaryArithmetic(t *testing.T) {
	listing := "module-under-test\n" +
		"github.com/fatih/color v1.16.0\n" +
		"golang.org/x/text v0.14.0 [v0.17.0]\n" +
		"github.com/spf13/cobra v1.8.0 [v2.0.0]\n" +
		"gopkg.in/yaml.v3 v3.0.1 [v3.1.0] // indirect\n"
	stubPATH(t, map[string]string{"go": "cat <<'OUT'\n" + listing + "OUT\n"})
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")

	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if code != 0 {
		t.Fatalf("findings are judgment, not failure: exit %d\n%s", code, joined)
	}
	for _, want := range []string{
		"go: 3 dependency(ies) behind",
		"  golang.org/x/text  v0.14.0 → v0.17.0",
		"  github.com/spf13/cobra  v1.8.0 → v2.0.0",
		"  gopkg.in/yaml.v3  v3.0.1 → v3.1.0",
		"3 dependency(ies) behind across 1 ecosystem(s), 1 major(s)",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}

// npm exits 1 WITH json whenever anything is outdated — that is the answer.
// proved by: made checkJS treat a nonzero npm exit as a failure — the normal
// outdated case then exits 1 and prints NOT checked instead of the table.
func TestRunNpmExitOneWithJSONIsTheAnswer(t *testing.T) {
	report := `{"left-pad":{"current":"1.0.0","wanted":"1.3.0","latest":"1.3.0"},` +
		`"react":{"current":"17.0.2","wanted":"17.0.2","latest":"18.3.1"}}`
	stubPATH(t, map[string]string{"npm": "cat <<'OUT'\n" + report + "\nOUT\nexit 1\n"})
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"x"}`)

	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if code != 0 {
		t.Fatalf("outdated packages are not a tool failure: exit %d\n%s", code, joined)
	}
	for _, want := range []string{
		"js: 2 dependency(ies) behind",
		"  left-pad  1.0.0 → 1.3.0 (wanted 1.3.0)",
		"  react  17.0.2 → 18.3.1 (wanted 17.0.2)",
		"licenses (js): NOT checked — no canonical no-install tool",
		"2 dependency(ies) behind across 1 ecosystem(s), 1 major(s)",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}

// proved by: made the empty-stdout branch report "js: up to date" regardless
// of the exit status — a crashed npm then reads as a clean ecosystem.
func TestRunNpmSilentFailureExitsOne(t *testing.T) {
	stubPATH(t, map[string]string{"npm": "echo 'npm ERR! code ENOTFOUND' >&2\nexit 1\n"})
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"x"}`)

	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if code != 1 {
		t.Fatalf("npm ran and failed: want exit 1, got %d\n%s", code, joined)
	}
	if !strings.Contains(joined, "js: NOT checked — npm ERR! code ENOTFOUND") {
		t.Errorf("missing the honest failure line in:\n%s", joined)
	}
	if strings.Contains(joined, "up to date") {
		t.Errorf("a failure must never read as up to date:\n%s", joined)
	}
}

// proved by: gave pipBinary an exec.LookPath("pip") fallback — the report
// then answers with the system interpreter's packages, which is a different
// question than the project's.
func TestRunPythonWithoutVirtualenvIsNotChecked(t *testing.T) {
	stubPATH(t, map[string]string{"pip": "echo '[]'\n"})
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "requests\n")

	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if code != 0 {
		t.Fatalf("no venv is informational, got exit %d\n%s", code, joined)
	}
	if !strings.Contains(joined, "python: NOT checked — no virtualenv is active and no .venv exists in the repository") {
		t.Errorf("a system pip must never answer for the project:\n%s", joined)
	}
}

// proved by: misread the pip field as `latest` instead of `latest_version`
// — every row then points at an empty target version.
func TestRunPythonUsesTheProjectVirtualenvPip(t *testing.T) {
	stubPATH(t, map[string]string{"pip": "echo 'SYSTEM PIP MUST NOT BE USED' >&2\nexit 3\n"})
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname = \"x\"\n")
	report := `[{"name":"requests","version":"2.28.0","latest_version":"3.0.0"},` +
		`{"name":"urllib3","version":"1.26.5","latest_version":"1.26.19"}]`
	writeFile(t, filepath.Join(dir, ".venv", "bin", "pip"),
		"#!/bin/sh\nPATH=/usr/bin:/bin\ncat <<'OUT'\n"+report+"\nOUT\n")

	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if code != 0 {
		t.Fatalf("want exit 0, got %d\n%s", code, joined)
	}
	for _, want := range []string{
		"python: 2 dependency(ies) behind",
		"  requests  2.28.0 → 3.0.0",
		"  urllib3  1.26.5 → 1.26.19",
		"2 dependency(ies) behind across 1 ecosystem(s), 1 major(s)",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "SYSTEM PIP") {
		t.Errorf("the venv's pip is the only one allowed to answer:\n%s", joined)
	}
}

// go-licenses often exits nonzero over one unclassifiable module while the
// rest of the report is sound — rows in hand beat a blanket NOT checked.
// proved by: made reportGoLicenses bail out to NOT checked whenever err !=
// nil — the whole license table then disappears over one bad module.
func TestReportGoLicensesKeepsRowsDespiteNonzeroExit(t *testing.T) {
	csv := "github.com/fatih/color,https://github.com/fatih/color/blob/master/LICENSE.md,MIT\n" +
		"gopkg.in/yaml.v3,https://github.com/go-yaml/yaml/blob/v3/LICENSE,Apache-2.0\n"
	stubPATH(t, map[string]string{
		"go":          "echo 'module-under-test'\n",
		"go-licenses": "cat <<'OUT'\n" + csv + "OUT\necho 'error: cannot determine license for example.com/x' >&2\nexit 1\n",
	})
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")

	var lines []string
	code := Run(dir, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if code != 0 {
		t.Fatalf("licenses are report-only, want exit 0, got %d\n%s", code, joined)
	}
	for _, want := range []string{
		"licenses (go): 2 module(s)",
		"  github.com/fatih/color: MIT",
		"  gopkg.in/yaml.v3: Apache-2.0",
		"  (go-licenses also reported: error: cannot determine license for example.com/x)",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}

// proved by: changed emit's cap test to `i == maxRows-1` — a table of
// exactly 30 then loses its last row to a "…0 more" line.
func TestEmitPrintsExactlyThirtyWithoutATail(t *testing.T) {
	var rows []row
	for i := 0; i < maxRows; i++ {
		rows = append(rows, row{name: fmt.Sprintf("pkg%02d", i)})
	}
	var lines []string
	emit(rows, func(s string) { lines = append(lines, s) }, func(r row) string { return "  " + r.name })
	if len(lines) != maxRows {
		t.Fatalf("exactly %d rows must print exactly %d lines, got %d", maxRows, maxRows, len(lines))
	}
	for _, l := range lines {
		if strings.Contains(l, "more") {
			t.Fatalf("nothing was elided, so there is no tail line: %q", l)
		}
	}
}
