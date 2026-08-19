// Package deps is the freshness report: which dependencies are behind,
// by how much, and under what licenses — per ecosystem, native tools
// only. The security pass catches vulnerable dependencies; this one
// catches stale ones, because updates get cheaper the smaller they are.
// Report-only: findings are judgment, so exit is 0 with findings and 1
// only when an ecosystem's tool actually errored. A missing optional
// tool (cargo-outdated, go-licenses) is said honestly as NOT checked —
// never faked as clean, never an error.
package deps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"procoder/internal/tools"
)

// toolTimeout is the per-tool budget; these tools go to the network and
// a hung registry must not hang the report.
const toolTimeout = 120 * time.Second

// maxRows caps each section's table; past that the count is the point.
const maxRows = 30

// row is one dependency that is behind.
type row struct {
	name    string
	current string
	latest  string
	wanted  string // npm only; empty elsewhere
}

// Run prints the freshness report for root and returns the exit code:
// 0 normally (findings are judgment), 1 only when an ecosystem's tool
// errored — which is distinct from a tool that simply is not installed.
func Run(root string, out func(string)) int {
	goMod := exists(root, "go.mod")
	pkgJSON := exists(root, "package.json")
	cargoToml := exists(root, "Cargo.toml")
	pyProject := exists(root, "pyproject.toml") || exists(root, "requirements.txt")

	if !goMod && !pkgJSON && !cargoToml && !pyProject {
		out("no dependency manifests in this repository")
		return 0
	}

	errored := false
	var behind, majors, ecosystems int
	count := func(rows []row) {
		if len(rows) == 0 {
			return
		}
		ecosystems++
		behind += len(rows)
		for _, r := range rows {
			if majorBehind(r.current, r.latest) {
				majors++
			}
		}
	}

	if goMod {
		rows, failed := checkGo(root, out)
		errored = errored || failed
		count(rows)
	}
	if pkgJSON {
		rows, failed := checkJS(root, out)
		errored = errored || failed
		count(rows)
	}
	if cargoToml {
		rows, failed := checkRust(root, out)
		errored = errored || failed
		count(rows)
	}
	if pyProject {
		rows, failed := checkPython(root, out)
		errored = errored || failed
		count(rows)
	}

	reportLicenses(root, out, goMod, pkgJSON, cargoToml, pyProject)

	out(fmt.Sprintf("%d dependency(ies) behind across %d ecosystem(s), %d major(s)", behind, ecosystems, majors))
	if errored {
		return 1
	}
	return 0
}

// checkGo asks `go list -u -m all`: a bracketed second version is an
// available update, everything else is current.
func checkGo(root string, out func(string)) ([]row, bool) {
	bin, err := exec.LookPath("go")
	if err != nil {
		out("go: NOT checked — the go toolchain is not installed")
		return nil, false
	}
	stdout, stderr, err, timedOut := execute(root, bin, "list", "-u", "-m", "all")
	if timedOut {
		out("go: NOT checked — go list gave no answer in " + toolTimeout.String())
		return nil, true
	}
	if err != nil {
		out("go: NOT checked — " + firstLine(stderr+stdout, err))
		return nil, true
	}
	rows := parseGoList(stdout)
	if len(rows) == 0 {
		out("go: up to date")
		return nil, false
	}
	out(fmt.Sprintf("go: %d dependency(ies) behind", len(rows)))
	emit(rows, out, func(r row) string {
		return fmt.Sprintf("  %s  %s → %s", r.name, r.current, r.latest)
	})
	return rows, false
}

// checkJS asks `npm outdated --json`. npm exits 1 WITH json whenever
// outdated packages exist — that is the answer, not an error; empty
// output with exit 0 means up to date.
func checkJS(root string, out func(string)) ([]row, bool) {
	bin, err := exec.LookPath("npm")
	if err != nil {
		out("js: NOT checked — npm is not installed")
		return nil, false
	}
	stdout, stderr, err, timedOut := execute(root, bin, "outdated", "--json")
	if timedOut {
		out("js: NOT checked — npm outdated gave no answer in " + toolTimeout.String())
		return nil, true
	}
	if strings.TrimSpace(stdout) == "" {
		if err != nil {
			out("js: NOT checked — " + firstLine(stderr, err))
			return nil, true
		}
		out("js: up to date")
		return nil, false
	}
	rows, perr := parseNpmOutdated(stdout)
	if perr != nil {
		out("js: NOT checked — " + perr.Error())
		return nil, true
	}
	if len(rows) == 0 {
		out("js: up to date")
		return nil, false
	}
	out(fmt.Sprintf("js: %d dependency(ies) behind", len(rows)))
	emit(rows, out, func(r row) string {
		return fmt.Sprintf("  %s  %s → %s (wanted %s)", r.name, r.current, r.latest, r.wanted)
	})
	return rows, false
}

// checkRust runs `cargo outdated` only when the plugin answers a
// version probe — it is a cargo plugin, and procoder installs nothing
// into anyone's toolchain.
func checkRust(root string, out func(string)) ([]row, bool) {
	bin, err := exec.LookPath("cargo")
	if err != nil {
		out("rust: NOT checked — cargo is not installed")
		return nil, false
	}
	if _, _, perr, ptimedOut := execute(root, bin, "outdated", "--version"); perr != nil || ptimedOut {
		out("rust: NOT checked — cargo-outdated is not installed (cargo install cargo-outdated)")
		return nil, false
	}
	stdout, stderr, err, timedOut := execute(root, bin, "outdated", "--format", "json")
	if timedOut {
		out("rust: NOT checked — cargo outdated gave no answer in " + toolTimeout.String())
		return nil, true
	}
	if err != nil {
		out("rust: NOT checked — " + firstLine(stderr+stdout, err))
		return nil, true
	}
	var report struct {
		Dependencies []struct {
			Name    string `json:"name"`
			Project string `json:"project"`
			Latest  string `json:"latest"`
		} `json:"dependencies"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &report); uerr != nil {
		out("rust: NOT checked — cargo outdated printed unparseable JSON: " + uerr.Error())
		return nil, true
	}
	var rows []row
	for _, d := range report.Dependencies {
		if d.Latest == "" || d.Latest == "---" || d.Latest == d.Project {
			continue
		}
		rows = append(rows, row{name: d.Name, current: d.Project, latest: d.Latest})
	}
	if len(rows) == 0 {
		out("rust: up to date")
		return nil, false
	}
	out(fmt.Sprintf("rust: %d dependency(ies) behind", len(rows)))
	emit(rows, out, func(r row) string {
		return fmt.Sprintf("  %s  %s → %s", r.name, r.current, r.latest)
	})
	return rows, false
}

// checkPython asks pip only when a project virtualenv is present —
// `pip list --outdated` against a system Python answers a different
// question than the project's, and a wrong answer is worse than none.
func checkPython(root string, out func(string)) ([]row, bool) {
	pip := pipBinary(root)
	if pip == "" {
		out("python: NOT checked — no virtualenv is active and no .venv exists in the repository")
		return nil, false
	}
	stdout, stderr, err, timedOut := execute(root, pip, "list", "--outdated", "--format", "json")
	if timedOut {
		out("python: NOT checked — pip list gave no answer in " + toolTimeout.String())
		return nil, true
	}
	if err != nil {
		out("python: NOT checked — " + firstLine(stderr+stdout, err))
		return nil, true
	}
	rows, perr := parsePipOutdated(stdout)
	if perr != nil {
		out("python: NOT checked — " + perr.Error())
		return nil, true
	}
	if len(rows) == 0 {
		out("python: up to date")
		return nil, false
	}
	out(fmt.Sprintf("python: %d dependency(ies) behind", len(rows)))
	emit(rows, out, func(r row) string {
		return fmt.Sprintf("  %s  %s → %s", r.name, r.current, r.latest)
	})
	return rows, false
}

// pipBinary finds the project virtualenv's pip: the active venv first
// (VIRTUAL_ENV), then root/.venv. Empty means no venv — NOT checked.
func pipBinary(root string) string {
	dirs := []string{}
	if v := os.Getenv("VIRTUAL_ENV"); v != "" {
		dirs = append(dirs, v)
	}
	if venv := filepath.Join(root, ".venv"); exists(root, ".venv") {
		dirs = append(dirs, venv)
	}
	sub := []string{"bin"}
	if runtime.GOOS == "windows" {
		sub = []string{"Scripts", "bin"}
	}
	for _, d := range dirs {
		for _, s := range sub {
			for _, name := range []string{"pip", "pip3", "pip.exe", "pip3.exe"} {
				p := filepath.Join(d, s, name)
				if info, err := os.Stat(p); err == nil && !info.IsDir() {
					return p
				}
			}
		}
	}
	return ""
}

// reportLicenses answers with honest scope: Go via go-licenses when
// installed (doctor recommends it, procoder never requires it); every
// other ecosystem has no canonical no-install tool and says so —
// never a fake all-clear.
func reportLicenses(root string, out func(string), goMod, pkgJSON, cargoToml, pyProject bool) {
	if goMod {
		reportGoLicenses(root, out)
	}
	for _, e := range []struct {
		name string
		on   bool
	}{{"js", pkgJSON}, {"rust", cargoToml}, {"python", pyProject}} {
		if e.on {
			out("licenses (" + e.name + "): NOT checked — no canonical no-install tool")
		}
	}
}

func reportGoLicenses(root string, out func(string)) {
	bin := tools.Resolve(&tools.Tool{Name: "go-licenses"}, root)
	if bin == "" {
		out("licenses (go): NOT checked — go-licenses is not installed (go install github.com/google/go-licenses@latest)")
		return
	}
	stdout, stderr, err, timedOut := execute(root, bin, "report", "./...")
	if timedOut {
		out("licenses (go): NOT checked — go-licenses gave no answer in " + toolTimeout.String())
		return
	}
	// go-licenses report prints CSV rows: module,url,license — and often
	// exits nonzero over one unclassifiable module while the rest of the
	// report is sound; rows in hand beat a blanket NOT checked.
	var rows []row
	for _, line := range strings.Split(stdout, "\n") {
		parts := strings.Split(strings.TrimSpace(line), ",")
		if len(parts) < 3 || parts[0] == "" {
			continue
		}
		rows = append(rows, row{name: parts[0], latest: parts[len(parts)-1]})
	}
	if len(rows) == 0 {
		if err != nil {
			out("licenses (go): NOT checked — " + firstLine(stderr+stdout, err))
			return
		}
		out("licenses (go): no dependencies to report")
		return
	}
	out(fmt.Sprintf("licenses (go): %d module(s)", len(rows)))
	emit(rows, out, func(r row) string { return "  " + r.name + ": " + r.latest })
	if err != nil {
		out("  (go-licenses also reported: " + firstLine(stderr, err) + ")")
	}
}

// parseGoList reads `go list -u -m all` output: `module vX [vY]` means
// an update to vY exists; a line without brackets — the main module,
// an up-to-date dependency, a replace target — is current as reported.
func parseGoList(raw string) []row {
	var rows []row
	for _, line := range strings.Split(raw, "\n") {
		// tolerate trailing comments (// indirect and friends)
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		for i := 1; i < len(fields); i++ {
			f := fields[i]
			if len(f) > 2 && f[0] == '[' && f[len(f)-1] == ']' {
				rows = append(rows, row{
					name:    fields[0],
					current: fields[i-1],
					latest:  f[1 : len(f)-1],
				})
				break
			}
		}
	}
	return rows
}

// parseNpmOutdated reads `npm outdated --json`: an object keyed by
// package name. Empty output is a full answer (up to date); anything
// unparseable is an error to report, never a silently empty table.
func parseNpmOutdated(raw string) ([]row, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return nil, fmt.Errorf("npm outdated printed unparseable JSON: %v", err)
	}
	if msg, ok := report["error"]; ok {
		var e struct {
			Summary string `json:"summary"`
			Code    string `json:"code"`
		}
		if json.Unmarshal(msg, &e) == nil && (e.Summary != "" || e.Code != "") {
			return nil, fmt.Errorf("npm reported an error: %s %s", e.Code, strings.TrimSpace(e.Summary))
		}
	}
	var rows []row
	for name, msg := range report {
		var d struct {
			Current string `json:"current"`
			Wanted  string `json:"wanted"`
			Latest  string `json:"latest"`
		}
		if err := json.Unmarshal(msg, &d); err != nil {
			// npm emits an array per package under workspaces; take each entry
			var many []struct {
				Current string `json:"current"`
				Wanted  string `json:"wanted"`
				Latest  string `json:"latest"`
			}
			if json.Unmarshal(msg, &many) != nil {
				return nil, fmt.Errorf("npm outdated entry for %s has an unexpected shape", name)
			}
			for _, m := range many {
				rows = append(rows, row{name: name, current: orMissing(m.Current), latest: m.Latest, wanted: m.Wanted})
			}
			continue
		}
		rows = append(rows, row{name: name, current: orMissing(d.Current), latest: d.Latest, wanted: d.Wanted})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	return rows, nil
}

// parsePipOutdated reads `pip list --outdated --format json`: an array
// of {name, version, latest_version}.
func parsePipOutdated(raw string) ([]row, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var report []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Latest  string `json:"latest_version"`
	}
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return nil, fmt.Errorf("pip list printed unparseable JSON: %v", err)
	}
	var rows []row
	for _, d := range report {
		rows = append(rows, row{name: d.Name, current: d.Version, latest: d.Latest})
	}
	return rows, nil
}

// majorBehind reports whether latest crosses a semver major boundary
// from cur. Versions that carry no leading numeric major (git hashes,
// "missing") answer false — a major cannot be claimed, so it is not.
func majorBehind(cur, latest string) bool {
	a, aok := majorOf(cur)
	b, bok := majorOf(latest)
	return aok && bok && a != b
}

func majorOf(v string) (int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	i := 0
	for i < len(v) && v[i] >= '0' && v[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(v[:i])
	if err != nil {
		return 0, false
	}
	return n, true
}

// emit prints rows capped at maxRows with an honest "…N more" tail.
func emit(rows []row, out func(string), format func(row) string) {
	for i, r := range rows {
		if i == maxRows {
			out(fmt.Sprintf("  …%d more", len(rows)-maxRows))
			return
		}
		out(format(r))
	}
}

func execute(root, bin string, args ...string) (string, string, error, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- resolved from the fixed tool table, never user input
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err, ctx.Err() == context.DeadlineExceeded
}

func firstLine(raw string, err error) string {
	for _, l := range strings.Split(raw, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			if len(t) > 160 {
				t = t[:160]
			}
			return t
		}
	}
	if err != nil {
		return "no output, " + err.Error()
	}
	return "no output"
}

func orMissing(s string) string {
	if s == "" {
		return "missing"
	}
	return s
}

func exists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}
