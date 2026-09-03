package api

import (
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"strings"
	"testing"
)

// Exactly four commands run what a repository — or a prior agent session
// — declared, and each one only in the form that runs it.
//
// The read-only form of the same command is not one of them: `run` prints
// the declared launch commands and `init` prints the install commands, and
// treating those as executing would put procoder's most useful read-only
// surfaces behind a door most machines will never open.
func TestExecutesNamesTheFour(t *testing.T) {
	executing := [][]string{
		{"run", "--exec"},
		{"evidence", "record", "echo", "hi"},
		{"init", "--yes"},
		{"self-upgrade"},
		{"self-upgrade", "--force"},
	}
	for _, argv := range executing {
		if !Executes(argv) {
			t.Errorf("%v runs something and is not being treated as executing", argv)
		}
	}
	reading := [][]string{
		{"run"}, {"evidence"}, {"init"}, {"check"}, {"test"}, {},
	}
	for _, argv := range reading {
		if Executes(argv) {
			t.Errorf("%v reads and is being treated as executing", argv)
		}
	}
}

// The work socket refuses the executing four and says where they live;
// the exec socket serves them.
//
// proved by: dropping the Exec check in serveConn — an agent session's own
// shell can then reach `run --exec` through the same door the hooks use,
// which is the exposure #201 was filed to close.
func TestExecutingCommandsRefusedOnWorkSocket(t *testing.T) {
	ran := false
	run := func(Request, io.Writer, io.Writer) (int, *Result) {
		ran = true
		return 0, nil
	}

	work, _ := testServer(t, run)
	res, err := Client{Path: work}.Do(Request{Argv: []string{"run", "--exec"}})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if res.Exit == nil || *res.Exit != 2 {
		t.Fatalf("want exit 2, got %v", res.Exit)
	}
	if ran {
		t.Fatal("the work socket RAN an executing command")
	}
	if !strings.Contains(res.Stderr, "#201") {
		t.Errorf("the refusal does not name the boundary it is holding: %q", res.Stderr)
	}
	if !strings.Contains(res.Stderr, "exec socket") {
		t.Errorf("the refusal does not say where the command is served: %q", res.Stderr)
	}

	exec, srv := testServer(t, run)
	srv.Exec = true
	if _, err := (Client{Path: exec}).Do(Request{Argv: []string{"run", "--exec"}}); err != nil {
		t.Fatalf("the exec socket refused its own command: %v", err)
	}
	if !ran {
		t.Fatal("the exec socket did not run what it is for")
	}
}

// The transport executes nothing itself.
//
// Asserted by reading the sources rather than by running them, in the
// shape internal/hook/noexec_test.go already uses for the hook package: a
// behavioural test proves only the paths it exercised, and what is wanted
// here is absence from all of them.
//
// proved by: adding an import of procoder/internal/runcmd to any file in
// internal/api — the test names the file.
func TestClientTransportExecutesNothing(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("could not read the package: %v", err)
	}
	checked := 0
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			checked++
			for _, imp := range file.Imports {
				if strings.Trim(imp.Path.Value, `"`) == "procoder/internal/runcmd" {
					t.Errorf("%s imports internal/runcmd — the transport must not be able to run "+
						"what a repository declared, whatever the caller asked for", path)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no files were read — the guard is not looking at anything")
	}
}
