package runcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

func write(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
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

// find answers the candidate whose command matches, or fails the test:
// every assertion below is about a specific declared command.
func find(t *testing.T, cands []Candidate, command string) Candidate {
	t.Helper()
	for _, c := range cands {
		if c.Command == command {
			return c
		}
	}
	t.Fatalf("no candidate %q in %+v", command, cands)
	return Candidate{}
}

func TestDetectsPackageJSONScriptsWithEvidence(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", "{\n  \"name\": \"x\",\n  \"scripts\": {\n    \"build\": \"tsc\",\n    \"dev\": \"vite\",\n    \"start\": \"node server.js\"\n  }\n}\n")
	cands := Detect(root)
	dev := find(t, cands, "npm run dev")
	if dev.Source != "package.json" || dev.Line != 5 {
		t.Fatalf("dev must point at the line that declared it: %+v", dev)
	}
	if !dev.LongRunning {
		t.Fatalf("dev is a server verb: %+v", dev)
	}
	if start := find(t, cands, "npm run start"); start.Line != 6 {
		t.Fatalf("start must carry its own line: %+v", start)
	}
	// the lockfile decides the manager, never a guess
	write(t, root, "pnpm-lock.yaml", "lockfileVersion: 6\n")
	find(t, Detect(root), "pnpm run dev")
}

func TestDetectsMakefileGoCargoPythonComposeProcfile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Makefile", "build:\n\tgo build ./...\n\nrun:\n\t./bin/app --once\n")
	write(t, root, "go.mod", "module demo\n\ngo 1.22\n")
	write(t, root, "cmd/app/main.go", "package main\n\nfunc main() {}\n")
	write(t, root, "Cargo.toml", "[package]\nname = \"demo\"\n\n[[bin]]\nname = \"tool\"\n")
	write(t, root, "manage.py", "#!/usr/bin/env python\n")
	write(t, root, "pyproject.toml", "[project]\nname = \"demo\"\n\n[project.scripts]\ndemo = \"demo.cli:main\"\nother = \"demo.other:main\"\n")
	write(t, root, "docker-compose.yml", "services:\n  web:\n    image: x\n")
	write(t, root, "Procfile", "web: gunicorn app:app\nworker: python worker.py\n")
	cands := Detect(root)

	for cmd, want := range map[string]struct {
		src  string
		line int
	}{
		"make run":                   {"Makefile", 4},
		"go run ./cmd/app":           {"go.mod", 1},
		"cargo run --bin tool":       {"Cargo.toml", 5},
		"python manage.py runserver": {"manage.py", 1},
		"demo":                       {"pyproject.toml", 5},
		"other":                      {"pyproject.toml", 6},
		"docker compose up":          {"docker-compose.yml", 1},
		"gunicorn app:app":           {"Procfile", 1},
		"python worker.py":           {"Procfile", 2},
	} {
		c := find(t, cands, cmd)
		if c.Source != want.src || c.Line != want.line {
			t.Fatalf("%q evidence must be %s:%d, got %s:%d", cmd, want.src, want.line, c.Source, c.Line)
		}
	}
	if !find(t, cands, "docker compose up").LongRunning {
		t.Fatal("docker compose up is long-running")
	}
	// the Procfile web process comes before the other process types
	web, worker := indexOf(cands, "gunicorn app:app"), indexOf(cands, "python worker.py")
	if web > worker {
		t.Fatalf("Procfile web must rank first: %+v", cands)
	}
}

func indexOf(cands []Candidate, command string) int {
	for i, c := range cands {
		if c.Command == command {
			return i
		}
	}
	return -1
}

func TestRankingIsMostSpecificFirst(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", "{\n  \"scripts\": {\n    \"start\": \"node .\",\n    \"dev\": \"vite\",\n    \"serve\": \"http-server\"\n  }\n}\n")
	write(t, root, "Makefile", "run:\n\t./app\n")
	got := []string{}
	for _, c := range Detect(root) {
		got = append(got, c.Command)
	}
	want := "make run npm run dev npm run start npm run serve"
	if strings.Join(got, " ") != want {
		t.Fatalf("an explicit run target outranks dev outranks start outranks serve:\n got %v\nwant %s", got, want)
	}
}

func TestMultipleGoMainsAreEachCandidates(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module demo\n\ngo 1.22\n")
	write(t, root, "cmd/a/main.go", "package main\n\nfunc main() {}\n")
	write(t, root, "cmd/b/main.go", "package main\n\nfunc main() {}\n")
	write(t, root, "lib/lib.go", "package lib\n")
	cands := Detect(root)
	if len(cands) != 2 {
		t.Fatalf("each main package is its own candidate, never a guess between them: %+v", cands)
	}
	find(t, cands, "go run ./cmd/a")
	find(t, cands, "go run ./cmd/b")

	// a library module declares nothing to run — not even a file that talks
	// about "package main" and "func main()" in a comment or a string
	lib := t.TempDir()
	write(t, lib, "go.mod", "module lib\n\ngo 1.22\n")
	write(t, lib, "lib.go", "package lib\n\n// detect looks for \"package main\" plus \"func main()\".\nvar Clause = \"package main\"\n")
	if cands := Detect(lib); len(cands) != 0 {
		t.Fatalf("a library has nothing to launch: %+v", cands)
	}
}

func TestMakefileRecipeDecidesLongRunning(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Makefile", "run:\n\tdocker compose up\n")
	c := find(t, Detect(root), "make run")
	if !c.LongRunning {
		t.Fatalf("the RECIPE makes this long-running, whatever the target is called: %+v", c)
	}
}

func TestNoCandidatesSaysSoAndExitsZero(t *testing.T) {
	out, lines := collect()
	if code := Run(t.TempDir(), false, out); code != 0 {
		t.Fatalf("nothing to run is not a failure, got %d: %v", code, *lines)
	}
	if len(*lines) != 1 || !strings.Contains((*lines)[0], "no launch command") {
		t.Fatalf("the no-candidates line must be plain: %v", *lines)
	}
}

func TestMalformedPackageJSONIsStatedOutLoud(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", "{\"scripts\": {\"dev\": ")
	write(t, root, "Makefile", "run:\n\t./app\n")
	out, lines := collect()
	if code := Run(root, false, out); code != 0 {
		t.Fatalf("an unreadable source is reported, not fatal: %d", code)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "package.json could NOT be parsed") {
		t.Fatalf("a source that could not be read is never silently skipped: %v", *lines)
	}
	if !strings.Contains(joined, "make run") {
		t.Fatalf("the other sources still report: %v", *lines)
	}
}

func TestMakefileWithIncludesIsNotedPartial(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Makefile", "include common.mk\n\nrun:\n\t./app\n")
	out, lines := collect()
	Run(root, false, out)
	if !strings.Contains(strings.Join(*lines, "\n"), "partially parsed") {
		t.Fatalf("include directives must be admitted: %v", *lines)
	}
}

func TestExecRefusesSeveralCandidates(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", "{\n  \"scripts\": {\n    \"dev\": \"vite\"\n  }\n}\n")
	write(t, root, "Makefile", "run:\n\t./app\n")
	out, lines := collect()
	if code := Run(root, true, out); code != 2 {
		t.Fatalf("--exec with two candidates must refuse with 2, got %d: %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "--exec refused") {
		t.Fatalf("the refusal must be printed: %v", *lines)
	}
}

func TestExecRefusesALongRunningCandidate(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", "{\n  \"scripts\": {\n    \"dev\": \"vite\"\n  }\n}\n")
	out, lines := collect()
	if code := Run(root, true, out); code != 2 {
		t.Fatalf("a server must be refused with 2, got %d: %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "npm run dev") || !strings.Contains(joined, "background shell") {
		t.Fatalf("the command and the run-it-yourself note must both be printed: %v", *lines)
	}
}

func TestExecRunsTheOneOneShotCandidate(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	write(t, root, "Procfile", "worker: go version\n")
	out, lines := collect()
	if code := Run(root, true, out); code != 0 {
		t.Fatalf("a single one-shot candidate must run and exit 0, got %d: %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "go version") {
		t.Fatalf("the command's own output belongs in the report: %v", *lines)
	}
}

func TestExecFailureExitsOne(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	write(t, root, "Procfile", "worker: go thiscommanddoesnotexist\n")
	out, lines := collect()
	if code := Run(root, true, out); code != 1 {
		t.Fatalf("a failing command exits 1, got %d: %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "--exec FAILED") {
		t.Fatalf("the failure must be named: %v", *lines)
	}
}

func TestExecMissingBinaryExitsOneAndStillPrintsEvidence(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Procfile", "worker: procoder-no-such-binary-xyz --once\n")
	out, lines := collect()
	if code := Run(root, true, out); code != 1 {
		t.Fatalf("a missing binary exits 1, got %d: %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "procoder-no-such-binary-xyz is not installed") || !strings.Contains(joined, "Procfile:1") {
		t.Fatalf("name the missing binary and keep the evidence: %v", *lines)
	}
}

func TestExecClosesStdinSoInteractiveCommandsFailFast(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	// The program blocks on stdin; with stdin at /dev/null the read returns
	// EOF at once, so this finishes in a second instead of at the 120s timeout.
	write(t, root, "go.mod", "module reader\n\ngo 1.22\n")
	write(t, root, "main.go", "package main\n\nimport (\n\t\"io\"\n\t\"os\"\n)\n\nfunc main() {\n\tif _, err := io.ReadAll(os.Stdin); err != nil {\n\t\tos.Exit(3)\n\t}\n}\n")
	out, lines := collect()
	if code := Run(root, true, out); code != 0 {
		t.Fatalf("a stdin reader must hit EOF and finish, got %d: %v", code, *lines)
	}
}

func TestRenderedOutputHasNoBackslashes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module demo\n\ngo 1.22\n")
	write(t, root, "cmd/app/main.go", "package main\n\nfunc main() {}\n")
	write(t, root, "pkg/__main__.py", "print('hi')\n")
	write(t, root, "src/main.rs", "fn main() {}\n")
	write(t, root, "Cargo.toml", "[package]\nname = \"demo\"\n")
	out, lines := collect()
	Run(root, false, out)
	joined := strings.Join(*lines, "\n")
	if strings.Contains(joined, `\`) {
		t.Fatalf("every path is forward-slashed on every platform: %q", joined)
	}
	for _, want := range []string{"go run ./cmd/app", "python -m pkg", "cargo run", "src/main.rs:1", "pkg/__main__.py:1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
}

func TestSplitArgsKeepsQuotedRunsWhole(t *testing.T) {
	got := splitArgs(`python -c "print('a b')" --flag=x  y`)
	want := []string{"python", "-c", "print('a b')", "--flag=x", "y"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("no shell runs here — quoted runs stay one argv element: %q", got)
	}
	if len(splitArgs("   ")) != 0 {
		t.Fatal("an empty command splits to nothing")
	}
}

func TestPyprojectScriptsAreEachCandidates(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pyproject.toml", "[project.scripts]\nalpha = \"a:main\"\nbeta = \"b:main\"\n")
	cands := Detect(root)
	if len(cands) != 2 {
		t.Fatalf("every entry point is a candidate, none is \"the\" one: %+v", cands)
	}
}
