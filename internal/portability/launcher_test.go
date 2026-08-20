package portability

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// launcherRoot builds a throwaway plugin root: the real hooks/launcher.sh over
// a dist/ tree whose binaries print the path they were exec'd as. The launcher
// resolves relative to its own location, so the only way to observe which
// binary it picked is to let it actually exec one.
func launcherRoot(t *testing.T, platforms ...string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "hooks/launcher.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hooks/launcher.sh"), src, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range platforms {
		bin := filepath.Join(root, "dist", p)
		if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s' \"$0\"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// unameStub puts a uname on PATH that answers exactly what the host under test
// would answer, so the launcher's own case arms are what decide the outcome.
func unameStub(t *testing.T, sysname, machine string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\n-s) printf '%s\\n' '" + sysname + "' ;;\n" +
		"-m) printf '%s\\n' '" + machine + "' ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "uname"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runLauncher(t *testing.T, root, sysname, machine string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(filepath.Join(root, "hooks/launcher.sh"))
	cmd.Env = append(os.Environ(), "PATH="+unameStub(t, sysname, machine)+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return string(out), stderr.String(), err
}

// TestLauncherResolvesWindowsShells drives the launcher with the uname strings
// Git Bash, MSYS2 and Cygwin really answer on Windows 10/11. Before the
// MINGW/MSYS/CYGWIN arm existed, every one of these exited 1 — which is every
// hook and every slash command failing on a fresh Windows install (issue #78).
//
// proved by: deleting the `MINGW* | MSYS* | CYGWIN*)` arm from
// hooks/launcher.sh, or dropping the `$ext` suffix from the bin= line.
func TestLauncherResolvesWindowsShells(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub binaries are POSIX shell scripts")
	}
	root := launcherRoot(t,
		"windows-amd64/procoder.exe",
		"windows-arm64/procoder.exe",
		"darwin-arm64/procoder",
		"linux-amd64/procoder",
	)
	cases := []struct {
		sysname string
		machine string
		want    string
	}{
		{"MINGW64_NT-10.0-26200", "x86_64", "windows-amd64/procoder.exe"},
		{"MINGW32_NT-6.2", "x86_64", "windows-amd64/procoder.exe"},
		{"MSYS_NT-10.0", "x86_64", "windows-amd64/procoder.exe"},
		{"CYGWIN_NT-10.0", "x86_64", "windows-amd64/procoder.exe"},
		{"MINGW64_NT-10.0-26200", "aarch64", "windows-arm64/procoder.exe"},
		// The platforms that already worked must keep working — and must keep
		// resolving the suffix-less name.
		{"Darwin", "arm64", "darwin-arm64/procoder"},
		{"Linux", "x86_64", "linux-amd64/procoder"},
	}
	for _, c := range cases {
		out, stderr, err := runLauncher(t, root, c.sysname, c.machine)
		if err != nil {
			t.Errorf("uname -s %s -m %s: launcher failed: %v (stderr: %s)", c.sysname, c.machine, err, stderr)
			continue
		}
		// ToSlash both sides: the launcher prints the path a POSIX shell
		// built, and filepath.Join answers in the host's separator. They
		// agree today only because the Windows case skips, and that skip is
		// the only thing holding the comparison together.
		if want := filepath.ToSlash(filepath.Join(root, "dist", c.want)); filepath.ToSlash(out) != want {
			t.Errorf("uname -s %s -m %s: ran %s, want %s", c.sysname, c.machine, out, want)
		}
	}
}

// TestLauncherRejectsUnknownOS keeps the unsupported path loud. An unknown
// uname silently falling through to some default would ship a launcher that
// execs the wrong binary — or nothing — without ever saying which OS it did
// not recognise.
//
// proved by: replacing the `*)` arm's echo+exit with a default `os=linux`.
func TestLauncherRejectsUnknownOS(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub binaries are POSIX shell scripts")
	}
	root := launcherRoot(t, "linux-amd64/procoder")
	out, stderr, err := runLauncher(t, root, "Plan9", "x86_64")
	if err == nil {
		t.Fatalf("unknown OS succeeded, ran %s", out)
	}
	if !strings.Contains(stderr, "unsupported OS Plan9") {
		t.Errorf("stderr does not name the uname it rejected: %q", stderr)
	}
}

// TestLauncherWindowsBinaryIsShipped closes the loop between the name the
// launcher resolves and the tree the plugin actually ships: a windows arm the
// repository has no binary for would still leave Windows users with nothing.
//
// proved by: deleting dist/windows-amd64/procoder.exe.
func TestLauncherWindowsBinaryIsShipped(t *testing.T) {
	path := filepath.Join(repoRoot(t), "dist/windows-amd64/procoder.exe")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("launcher resolves windows-amd64/procoder.exe but it is not shipped: %v", err)
	}
}
