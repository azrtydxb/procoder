// Package runcmd answers "how do I start this project?" with evidence
// instead of a guess. It reads what the repository itself declared —
// package.json scripts, Makefile targets, main packages, Cargo.toml,
// Python entry points, docker-compose.yml, a Procfile — and PRINTS every
// launch command it found together with the file and line that declared
// it, most specific first. Nothing is invented: a candidate without a
// file and a line to point at is not a candidate.
//
// The default path writes nothing and executes nothing (P-CONTROL).
// --exec is the operator's explicit opt-in, and it runs only when there is
// exactly one candidate AND that candidate is not a long-running server:
// a server's lifetime belongs to the agent's own shell, where log capture
// belongs, never to procoder.
package runcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// execTimeout bounds --exec. On expiry the command is killed and the
// answer is NOT run — never a pass.
const execTimeout = 120 * time.Second

// Candidate is one launch command the repository declared, with the
// evidence that declared it. Source is always repo-relative with forward
// slashes, on every platform.
type Candidate struct {
	Command     string
	Source      string
	Line        int
	LongRunning bool
	// Note carries the rest of the evidence when the source file alone does
	// not name it — "main package cmd/procoder" for a go.mod candidate.
	Note string
	// rank orders the listing: lower is more specific. Unexported because
	// the ranking rules are this package's business, not its callers'.
	rank int
	// root is the directory --exec runs the command in — the repository
	// that declared it, never the process's own working directory.
	root string
}

// Ranks: lower wins. An explicit `run` target beats a package.json script,
// dev beats start beats serve, and the language-level fallbacks (go run,
// cargo run) come after anything the repository named on purpose.
const (
	rankMakeRun     = 10
	rankPkgDev      = 20
	rankMakeDev     = 21
	rankPkgStart    = 30
	rankMakeStart   = 31
	rankPkgServe    = 40
	rankProcWeb     = 50
	rankProcOther   = 51
	rankManagePy    = 60
	rankPyScript    = 61
	rankPyMain      = 62
	rankCargo       = 70
	rankGoMain      = 80
	rankDockerCompo = 90
)

// serverVerbs are the words that mark a command as long-running. A command
// naming one of these is printed and refused by --exec: procoder does no
// process management of any kind.
var serverVerbs = map[string]bool{
	"serve": true, "server": true, "runserver": true, "dev": true,
	"start": true, "up": true, "watch": true,
}

// Run detects, prints, and — under exec — runs. Exit codes: 0 when
// candidates were printed or none exist, 0 when --exec succeeded, 1 when
// --exec ran and the command failed or timed out, 2 when --exec was
// refused (several candidates, or a long-running server).
func Run(root string, exec bool, out func(string)) int {
	cands, notes := detect(root)
	// Unreadable sources are stated out loud before the listing: a source
	// procoder could not parse is never silently skipped.
	for _, n := range notes {
		out(n)
	}
	return Report(cands, exec, out)
}

func detect(root string) ([]Candidate, []string) {
	var cands []Candidate
	var notes []string
	for _, src := range []func(string) ([]Candidate, []string){
		detectPackageJSON, detectMakefile, detectProcfile,
		detectPython, detectCargo, detectGo, detectCompose,
	} {
		c, n := src(root)
		cands = append(cands, c...)
		notes = append(notes, n...)
	}
	// Stable: within one rank, the order the file declared them wins.
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].rank < cands[j].rank })
	return cands, notes
}

// Report prints the candidates and returns the command's exit code. It is
// the whole of the default path: print, do not run.
func Report(cands []Candidate, execute bool, out func(string)) int {
	if len(cands) == 0 {
		out("no launch command is declared in this repository — nothing to run (a library has nothing to launch, and that is not a failure)")
		return 0
	}
	longRunning := false
	for _, c := range cands {
		out(render(c))
		if c.LongRunning {
			longRunning = true
		}
	}
	if longRunning {
		out("long-running commands are marked: start those in your OWN background shell, where log capture and the process's lifetime belong")
	}
	if !execute {
		return 0
	}
	if len(cands) > 1 {
		out(fmt.Sprintf("--exec refused — %d candidates were detected and choosing one would be the guess this command exists to avoid; run the one you want yourself", len(cands)))
		return 2
	}
	c := cands[0]
	if c.LongRunning {
		out("--exec refused — " + c.Command + " is long-running; procoder does no process management, so start it in your own background shell")
		return 2
	}
	return execOne(c, out)
}

// render is one listing line: the command, then its evidence as
// source:line with forward slashes.
func render(c Candidate) string {
	ev := fmt.Sprintf("%s:%d", filepath.ToSlash(c.Source), c.Line)
	if c.Note != "" {
		ev += ", " + c.Note
	}
	line := fmt.Sprintf("%-40s (%s)", c.Command, ev)
	if c.LongRunning {
		line += "  [long-running]"
	}
	return line
}

// execOne runs a single one-shot candidate. stdin is /dev/null on purpose:
// a command that wants input fails fast instead of hanging to the timeout.
func execOne(c Candidate, out func(string)) int {
	argv := splitArgs(c.Command)
	if len(argv) == 0 {
		out("--exec refused — the declared command is empty")
		return 2
	}
	bin, err := exec.LookPath(argv[0])
	// Say which binary this actually resolved to, before running it.
	//
	// The command comes from the repository; the binary comes from PATH.
	// The same declared `npm start` is a different program depending on
	// which npm is first on the path, and somebody approving --exec could
	// not see which one they were approving. Naming it is the difference
	// between consenting to a command and consenting to a string (#200).
	if err == nil {
		out("--exec resolving " + argv[0] + " -> " + bin)
	}
	if err != nil {
		out("--exec FAILED — " + argv[0] + " is not installed; the command and its evidence are printed above, so run it once you have it")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, argv[1:]...) // nosemgrep -- the argv comes from the repository's own declaration, run only on explicit --exec
	cmd.Dir = c.root
	cmd.Stdin = nil // /dev/null: an interactive command fails fast
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	runErr := cmd.Run()
	for _, l := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if l != "" {
			out(l)
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		out("NOT run — " + c.Command + " gave no answer in " + execTimeout.String() + " and was killed; a hung command is never a pass")
		return 1
	}
	if runErr != nil {
		out("--exec FAILED — " + c.Command + " exited " + fmt.Sprint(exitCode(runErr)))
		return 1
	}
	out("--exec ok — " + c.Command)
	return 0
}

func exitCode(err error) int {
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode()
	}
	return -1
}

// splitArgs splits a declared command into argv. There is no shell here:
// quoted runs stay one element, and nothing is expanded or interpreted.
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	quote := rune(0)
	started := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, started = r, true
		case r == ' ' || r == '\t':
			if started {
				out = append(out, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	if started {
		out = append(out, cur.String())
	}
	return out
}

// longRunning reports whether any word of the text is a known server verb.
// It is applied to the command AND, for Makefiles, to the recipe: a target
// named `run` whose recipe is `docker compose up` is long-running by the
// recipe, whatever the target is called.
func longRunning(text string) bool {
	isWord := func(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') }
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !isWord(r) }) {
		if serverVerbs[w] {
			return true
		}
	}
	return false
}

// ---------- package.json ----------

func detectPackageJSON(root string) ([]Candidate, []string) {
	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil, nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil, []string{"package.json could NOT be parsed (" + firstLine(err.Error()) + ") — it contributed no candidates; other sources still report"}
	}
	mgr := manager(root)
	var out []Candidate
	for _, s := range []struct {
		name string
		rank int
	}{{"dev", rankPkgDev}, {"start", rankPkgStart}, {"serve", rankPkgServe}} {
		body, ok := pkg.Scripts[s.name]
		if !ok {
			continue
		}
		line := jsonKeyLine(string(raw), s.name)
		if line == 0 {
			continue // evidence is mandatory: no line, no candidate
		}
		out = append(out, Candidate{
			Command:     mgr + " run " + s.name,
			Source:      "package.json",
			Line:        line,
			LongRunning: longRunning(s.name + " " + body),
			rank:        s.rank,
			root:        root,
		})
	}
	return out, nil
}

// manager picks the package manager the lockfiles declare, npm by default.
func manager(root string) string {
	switch {
	case exists(root, "bun.lockb") || exists(root, "bun.lock"):
		return "bun"
	case exists(root, "pnpm-lock.yaml"):
		return "pnpm"
	case exists(root, "yarn.lock"):
		return "yarn"
	}
	return "npm"
}

// jsonKeyLine finds the 1-based line declaring "key": in a JSON document,
// which is the evidence a candidate needs. 0 means not found.
func jsonKeyLine(raw, key string) int {
	needle := `"` + key + `"`
	for i, l := range strings.Split(raw, "\n") {
		idx := strings.Index(l, needle)
		if idx < 0 {
			continue
		}
		if rest := strings.TrimLeft(l[idx+len(needle):], " \t"); strings.HasPrefix(rest, ":") {
			return i + 1
		}
	}
	return 0
}

// ---------- Makefile ----------

func detectMakefile(root string) ([]Candidate, []string) {
	name, raw, err := readFirst(root, "Makefile", "makefile", "GNUmakefile")
	if name == "" {
		return nil, nil
	}
	if err != nil {
		return nil, []string{name + " could NOT be read (" + firstLine(err.Error()) + ") — it contributed no candidates"}
	}
	var notes []string
	targets, partial := parseMakefile(raw)
	if partial != "" {
		notes = append(notes, name+" was only partially parsed — "+partial+"; the targets that could be read are reported")
	}
	wanted := map[string]int{"run": rankMakeRun, "dev": rankMakeDev, "start": rankMakeStart}
	var out []Candidate
	for _, t := range targets {
		r, ok := wanted[t.name]
		if !ok {
			continue
		}
		out = append(out, Candidate{
			Command:     "make " + t.name,
			Source:      name,
			Line:        t.line,
			LongRunning: longRunning(t.name + " " + t.recipe),
			rank:        r,
			root:        root,
		})
	}
	return out, notes
}

type makeTarget struct {
	name   string
	line   int
	recipe string
}

// parseMakefile reads the target lines it can and says what it could not:
// includes and generated targets are named rather than pretended away.
func parseMakefile(raw string) ([]makeTarget, string) {
	var out []makeTarget
	var partial []string
	lines := strings.Split(raw, "\n")
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "\t"), trimmed == "", strings.HasPrefix(trimmed, "#"):
			continue
		case strings.HasPrefix(trimmed, "include ") || strings.HasPrefix(trimmed, "-include "):
			partial = append(partial, "it has include directives, whose targets were not read")
			continue
		}
		name, ok := targetName(l)
		if !ok {
			continue
		}
		if strings.Contains(name, "$") || strings.Contains(name, "%") {
			partial = append(partial, "it has generated or pattern targets, which cannot be named")
			continue
		}
		var recipe []string
		for _, r := range lines[i+1:] {
			if !strings.HasPrefix(r, "\t") {
				break
			}
			recipe = append(recipe, strings.TrimSpace(r))
		}
		out = append(out, makeTarget{name: name, line: i + 1, recipe: strings.Join(recipe, " ")})
	}
	return out, strings.Join(dedupe(partial), "; ")
}

// targetName pulls the target out of a `name: deps` line, rejecting
// variable assignments (`name := x`) and .PHONY-style directives.
func targetName(l string) (string, bool) {
	colon := strings.Index(l, ":")
	if colon <= 0 {
		return "", false
	}
	if eq := strings.Index(l, "="); eq >= 0 && eq < colon+2 {
		return "", false // `x = y` or `x := y`
	}
	name := strings.TrimSpace(l[:colon])
	if name == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, " \t") {
		return "", false
	}
	return name, true
}

// ---------- Procfile ----------

func detectProcfile(root string) ([]Candidate, []string) {
	name, raw, err := readFirst(root, "Procfile")
	if name == "" {
		return nil, nil
	}
	if err != nil {
		return nil, []string{"Procfile could NOT be read (" + firstLine(err.Error()) + ") — it contributed no candidates"}
	}
	var out []Candidate
	for i, l := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		colon := strings.Index(t, ":")
		if colon <= 0 {
			continue
		}
		proc := strings.TrimSpace(t[:colon])
		cmd := strings.TrimSpace(t[colon+1:])
		if cmd == "" {
			continue
		}
		r := rankProcOther
		if proc == "web" {
			r = rankProcWeb // the web process is what "run this" usually means
		}
		out = append(out, Candidate{
			Command:     cmd,
			Source:      name,
			Line:        i + 1,
			LongRunning: longRunning(cmd) || proc == "web",
			Note:        "Procfile process " + proc,
			rank:        r,
			root:        root,
		})
	}
	return out, nil
}

// ---------- Python ----------

func detectPython(root string) ([]Candidate, []string) {
	var out []Candidate
	var notes []string
	if exists(root, "manage.py") {
		out = append(out, Candidate{
			Command: "python manage.py runserver", Source: "manage.py", Line: 1,
			LongRunning: true, Note: "Django management script", rank: rankManagePy, root: root,
		})
	}
	if raw, err := os.ReadFile(filepath.Join(root, "pyproject.toml")); err == nil {
		scripts, malformed := pyScripts(string(raw))
		if malformed != "" {
			notes = append(notes, "pyproject.toml [project.scripts] could NOT be fully read — "+malformed)
		}
		for _, s := range scripts {
			out = append(out, Candidate{
				Command: s.name, Source: "pyproject.toml", Line: s.line,
				LongRunning: longRunning(s.name), Note: "[project.scripts] entry point",
				rank: rankPyScript, root: root,
			})
		}
	} else if !os.IsNotExist(err) {
		notes = append(notes, "pyproject.toml could NOT be read ("+firstLine(err.Error())+") — it contributed no candidates")
	}
	out = append(out, pyMains(root)...)
	return out, notes
}

type pyScript struct {
	name string
	line int
}

// pyScripts reads the [project.scripts] table. Every entry is a candidate:
// which one is "the" entry point is not procoder's to guess.
func pyScripts(raw string) ([]pyScript, string) {
	var out []pyScript
	inSection, malformed := false, ""
	for i, l := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			inSection = t == "[project.scripts]"
			continue
		}
		if !inSection || t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		eq := strings.Index(t, "=")
		if eq <= 0 {
			malformed = fmt.Sprintf("line %d is not a `name = \"module:function\"` entry", i+1)
			continue
		}
		name := strings.Trim(strings.TrimSpace(t[:eq]), `"'`)
		if name != "" {
			out = append(out, pyScript{name: name, line: i + 1})
		}
	}
	return out, malformed
}

// pyMains finds runnable __main__.py modules: one in a package directory is
// `python -m <pkg>`, one at the root is `python __main__.py`.
func pyMains(root string) []Candidate {
	var out []Candidate
	if exists(root, "__main__.py") {
		out = append(out, Candidate{
			Command: "python __main__.py", Source: "__main__.py", Line: 1,
			rank: rankPyMain, root: root,
		})
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || skipDir(e.Name()) {
			continue
		}
		rel := e.Name() + "/__main__.py"
		if !exists(root, filepath.FromSlash(rel)) {
			continue
		}
		out = append(out, Candidate{
			Command: "python -m " + e.Name(), Source: rel, Line: 1,
			Note: "runnable module", rank: rankPyMain, root: root,
		})
	}
	return out
}

// ---------- Rust ----------

func detectCargo(root string) ([]Candidate, []string) {
	raw, err := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if err != nil {
		return nil, nil
	}
	var out []Candidate
	inBin := false
	for i, l := range strings.Split(string(raw), "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			inBin = t == "[[bin]]"
			continue
		}
		if !inBin || !strings.HasPrefix(t, "name") {
			continue
		}
		if eq := strings.Index(t, "="); eq > 0 {
			name := strings.Trim(strings.TrimSpace(t[eq+1:]), `"'`)
			if name != "" {
				out = append(out, Candidate{
					Command: "cargo run --bin " + name, Source: "Cargo.toml", Line: i + 1,
					Note: "[[bin]] target", rank: rankCargo, root: root,
				})
			}
		}
	}
	if len(out) == 0 && exists(root, filepath.FromSlash("src/main.rs")) {
		out = append(out, Candidate{
			Command: "cargo run", Source: "src/main.rs", Line: 1,
			Note: "binary crate root", rank: rankCargo, root: root,
		})
	}
	return out, nil
}

// ---------- Go ----------

// detectGo finds every main package under a Go module. Several mains are
// several candidates — all printed, none guessed to be "the" one — and a
// library module contributes nothing at all.
func detectGo(root string) ([]Candidate, []string) {
	if !exists(root, "go.mod") {
		return nil, nil
	}
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil || !isGoMain(body) {
			return nil
		}
		rel, rerr := filepath.Rel(root, filepath.Dir(path))
		if rerr != nil {
			return nil
		}
		dirs = append(dirs, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(dirs)
	var out []Candidate
	seen := map[string]bool{}
	for _, d := range dirs {
		if seen[d] {
			continue
		}
		seen[d] = true
		pkg, cmd := d, "go run ./"+d
		if d == "." {
			pkg, cmd = "the module root", "go run ."
		}
		out = append(out, Candidate{
			Command: cmd, Source: "go.mod", Line: 1,
			Note: "main package " + pkg, rank: rankGoMain, root: root,
		})
	}
	return out, nil
}

// isGoMain reports whether a file declares a runnable main. Both clauses
// must sit at column 0, where Go puts them: a substring match would call
// any file that merely MENTIONS "package main" — this one, for instance —
// a launch candidate.
func isGoMain(body []byte) bool {
	pkg, fn := false, false
	for _, l := range strings.Split(string(body), "\n") {
		l = strings.TrimRight(l, " \t\r")
		switch {
		case l == "package main" || strings.HasPrefix(l, "package main //"):
			pkg = true
		case strings.HasPrefix(l, "func main()"):
			fn = true
		}
	}
	return pkg && fn
}

// ---------- docker compose ----------

func detectCompose(root string) ([]Candidate, []string) {
	name, _, err := readFirst(root, "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml")
	if name == "" {
		return nil, nil
	}
	if err != nil {
		return nil, []string{name + " could NOT be read (" + firstLine(err.Error()) + ") — it contributed no candidates"}
	}
	// A declared compose file is a candidate whether or not docker is
	// installed: the repository declared it, and --exec refuses it as
	// long-running long before docker's absence could matter.
	return []Candidate{{
		Command: "docker compose up", Source: name, Line: 1,
		LongRunning: true, rank: rankDockerCompo, root: root,
	}}, nil
}

// ---------- shared helpers ----------

// readFirst returns the first of names that exists, with its contents. An
// existing but unreadable file returns its name and the error, so the
// caller can say so instead of skipping it silently.
func readFirst(root string, names ...string) (string, string, error) {
	for _, n := range names {
		p := filepath.Join(root, n)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		raw, err := os.ReadFile(p)
		return n, string(raw), err
	}
	return "", "", nil
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "target", "dist", "build", ".venv", "venv", "testdata":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		return s[:160]
	}
	return s
}

func exists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}
