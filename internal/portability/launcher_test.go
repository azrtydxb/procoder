package portability

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
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
// ARM64 Windows is the case the portability page makes a promise about:
// only windows-amd64 ships, so a shell reporting aarch64 must be told there
// is no binary rather than handed one for another architecture. The fixture
// deliberately mirrors the shipped dist/ tree — stubbing a windows-arm64
// binary would test a repository that does not exist and would let a
// regression in this promise through.
// proved by: mapped the ARM64 arm to os=linux ext="" — the launcher then
// execs dist/linux-amd64/procoder and this test reports the wrong binary
// instead of the refusal.
func TestLauncherRefusesArm64Windows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub binaries are POSIX shell scripts")
	}
	// The launcher fetches now, so the refusal comes from the release
	// having no asset for this platform rather than from an empty dist/.
	// The promise is unchanged and is the reason this test exists: whoever
	// is on windows/arm64 must be told that THEIR PLATFORM is the problem,
	// not left reading a 404 and guessing at their network.
	root := fetchRoot(t, "9.9.9")
	// a release that exists and simply has nothing for this platform,
	// which is exactly what GitHub returns for windows-arm64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS") {
			w.Write([]byte(strings.Repeat("a", 64) + "  procoder-windows-amd64.exe\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cmd := exec.Command(filepath.Join(root, "hooks/launcher.sh"), "check")
	cmd.Env = append(os.Environ(),
		"PATH="+unameStub(t, "MINGW64_NT-10.0-26200", "aarch64")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PROCODER_RELEASE_BASE="+srv.URL)
	var out, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()

	if err == nil {
		t.Fatalf("no windows-arm64 binary is published; the launcher ran %s instead of refusing", out.String())
	}
	// "no binary for windows-arm64", not merely the string somewhere in
	// the message — the asset URL contains the platform too, so a looser
	// assertion passes even when the sentence naming it is deleted. That
	// mutation was run and did exactly that.
	if !strings.Contains(stderr.String(), "no binary for windows-arm64") {
		t.Errorf("the refusal must say plainly which platform has nothing, got: %s", stderr.String())
	}
}

func TestLauncherResolvesWindowsShells(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub binaries are POSIX shell scripts")
	}
	root := launcherRoot(t,
		"windows-amd64/procoder.exe",
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

// The same loop this used to close against the committed tree, closed
// against what CI publishes instead: every asset name the launcher can
// ask for must be one the release job actually uploads. A launcher arm
// with no matching asset leaves those users with nothing, which was true
// when the binaries were committed and is true now that they are fetched.
//
// It reads ci.yml rather than a list written here, because a list written
// here is a third place to forget.
//
// The old version of this test stat'd dist/windows-amd64/procoder.exe. It
// passed locally after the binaries were untracked — an ignored working
// copy was still on disk — and failed on CI's clean checkout. Reading the
// two sources against each other cannot pass for that reason.
//
// proved by: renaming one asset in ci.yml's staging step — this test then
// names the platform the launcher would ask for and CI would not publish.
func TestEveryPlatformTheLauncherResolvesIsPublished(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github/workflows/ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	// Line endings normalised before anything is matched. Windows checks
	// out with CRLF, and an anchored `$` sits before the \n but after the
	// \r, so a pattern ending in `\\?$` matched nothing there and the
	// uploaded set came back empty — on Windows only, while Linux and
	// macOS were green.
	ci := strings.ReplaceAll(string(raw), "\r\n", "\n")
	// Staged and uploaded are read SEPARATELY and required to agree. A
	// single scan of the file passes when only one of the two is renamed,
	// which is the mismatch worth catching: staged as X and uploaded as Y
	// fails the release with a missing file, at the one moment nobody is
	// watching a green pipeline.
	staged := map[string]bool{}
	for _, m := range regexp.MustCompile(`cp\s+\S+\s+/tmp/assets/(procoder-[a-z0-9-]+(?:\.exe)?)`).FindAllStringSubmatch(ci, -1) {
		staged[m[1]] = true
	}
	uploaded := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s+/tmp/assets/(procoder-[a-z0-9-]+(?:\.exe)?)\s*\\?$`).FindAllStringSubmatch(ci, -1) {
		uploaded[m[1]] = true
	}
	if len(staged) == 0 || len(uploaded) == 0 {
		t.Fatalf("read %d staged and %d uploaded asset names from ci.yml — this check proved nothing", len(staged), len(uploaded))
	}
	for name := range staged {
		if !uploaded[name] {
			t.Errorf("%s is staged and never uploaded", name)
		}
	}
	for name := range uploaded {
		if !staged[name] {
			t.Errorf("%s is uploaded and never staged — the release would fail on a missing file", name)
		}
	}
	published := uploaded
	// The five arms hooks/launcher.sh can resolve, spelled as it spells
	// them: procoder-<os>-<arch>, with .exe on windows.
	for _, want := range []string{
		"procoder-darwin-arm64", "procoder-darwin-amd64",
		"procoder-linux-amd64", "procoder-linux-arm64",
		"procoder-windows-amd64.exe",
	} {
		if !published[want] {
			t.Errorf("the launcher can ask for %s and the release job does not publish it", want)
		}
	}
}

// ---------------------------------------------------------------------
// The fetching launcher. The binary is no longer committed: CI builds it
// and the launcher fetches the one this platform needs on first use.
//
// Every test below runs against a stub server through
// PROCODER_RELEASE_BASE. A test that reached GitHub would answer about the
// network rather than about the launcher, and would be skipped the first
// time CI ran offline.

// fetchRoot is a plugin root with a manifest and NO binary: the state a
// fresh marketplace clone is in.
func fetchRoot(t *testing.T, version string) string {
	t.Helper()
	root := launcherRoot(t) // no platforms: dist/ is empty
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"procoder","version":"` + version + `"}`
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// releaseServer serves one release: the asset for this platform and a
// SHA256SUMS over it. body is what the "binary" prints when run, so a test
// can tell an executed download from a mere download. It counts requests,
// because "did not ask" is a thing several tests need to assert.
func releaseServer(t *testing.T, version, body string, corrupt bool) (base string, requests *int32) {
	t.Helper()
	asset := "procoder-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	payload := []byte("#!/bin/sh\nprintf '%s' '" + body + "'\n")
	sum := sha256.Sum256(payload)
	line := hex.EncodeToString(sum[:]) + "  " + asset + "\n"
	if corrupt {
		// a manifest that disagrees with the file it describes
		line = strings.Repeat("0", 64) + "  " + asset + "\n"
	}
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		switch {
		case strings.HasSuffix(r.URL.Path, "/SHA256SUMS") && strings.Contains(r.URL.Path, "/v"+version+"/"):
			w.Write([]byte(line))
		case strings.HasSuffix(r.URL.Path, "/"+asset) && strings.Contains(r.URL.Path, "/v"+version+"/"):
			w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &n
}

func runFetching(t *testing.T, root string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(filepath.Join(root, "hooks/launcher.sh"), args...)
	cmd.Env = append(os.Environ(), env...)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running the launcher: %v", err)
	}
	return out.String(), errb.String(), code
}

func skipUnlessPOSIX(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the launcher is a POSIX shell script; Windows runs it under Git Bash, which CI does not provide")
	}
}

// The whole point: nothing cached, a reachable release, and the launcher
// comes back with a working binary it verified first.
//
// proved by: removing the `mv -f` install step — the binary is then
// fetched, verified, and never put where the next run can find it.
func TestTheLauncherFetchesVerifiesAndCachesItsBinary(t *testing.T) {
	skipUnlessPOSIX(t)
	root := fetchRoot(t, "9.9.9")
	base, _ := releaseServer(t, "9.9.9", "FETCHED", false)

	out, errb, code := runFetching(t, root, []string{"PROCODER_RELEASE_BASE=" + base}, "anything")

	if code != 0 || out != "FETCHED" {
		t.Fatalf("the fetched binary did not run: out=%q err=%q code=%d", out, errb, code)
	}
	bin := filepath.Join(root, "dist", runtime.GOOS+"-"+runtime.GOARCH, "procoder")
	if _, err := os.Stat(bin); err != nil {
		t.Errorf("the binary was not cached at %s: %v", bin, err)
	}
}

// The launcher carries no version of its own: it asks the manifest. That
// is what makes one script work for every release, and what makes the
// staleness failure impossible rather than merely guarded.
//
// proved by: hard-coding a version in the URL — the request then goes to
// the wrong release and this test sees a 404 instead of a run.
func TestTheVersionFetchedIsTheOneTheManifestDeclares(t *testing.T) {
	skipUnlessPOSIX(t)
	root := fetchRoot(t, "4.5.6")
	base, _ := releaseServer(t, "4.5.6", "FOUR-FIVE-SIX", false)

	out, errb, code := runFetching(t, root, []string{"PROCODER_RELEASE_BASE=" + base}, "anything")

	if code != 0 || out != "FOUR-FIVE-SIX" {
		t.Fatalf("the launcher did not fetch the manifest's version: out=%q err=%q", out, errb)
	}
}

// The hot path fires on every session start, every Bash call and every
// write. With the binary present it must do nothing else at all — asserted
// by pointing the launcher at a server and requiring that the server is
// never asked.
//
// proved by: moving the cached-binary check below the fetch — the request
// count is then one instead of zero.
func TestACachedBinaryIsRunWithoutAskingTheNetwork(t *testing.T) {
	skipUnlessPOSIX(t)
	root := fetchRoot(t, "9.9.9")
	base, requests := releaseServer(t, "9.9.9", "SHOULD-NOT-BE-FETCHED", false)
	bin := filepath.Join(root, "dist", runtime.GOOS+"-"+runtime.GOARCH, "procoder")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s' 'CACHED'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, _, code := runFetching(t, root, []string{"PROCODER_RELEASE_BASE=" + base}, "anything")

	if code != 0 || out != "CACHED" {
		t.Fatalf("the cached binary was not used: out=%q code=%d", out, code)
	}
	if n := atomic.LoadInt32(requests); n != 0 {
		t.Errorf("the cached path made %d network request(s); it must make none", n)
	}
}

// A hook that cannot get its binary must not take the session with it.
// Every shape wired in hooks/claude-hooks.json is covered, including
// `principles --hook` — SessionStart is spelled that way, and a split
// matching only `hook <sub>` would refuse loudly at session start on any
// machine that could not fetch. The mechanism written to keep sessions
// alive would have broken them at the one moment it exists for.
//
// proved by: dropping the --hook arm from the is_hook test — the
// principles case then exits 1 with output on stderr and the session dies.
func TestEveryWiredHookShapeDegradesInsteadOfBreakingTheSession(t *testing.T) {
	skipUnlessPOSIX(t)
	for _, args := range [][]string{
		{"hook", "post-tool-use"},
		{"hook", "pre-tool-use"},
		{"hook", "stop"},
		{"principles", "--hook"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := fetchRoot(t, "9.9.9")
			out, errb, code := runFetching(t, root,
				[]string{"PROCODER_RELEASE_BASE=http://127.0.0.1:1/nothing"}, args...)

			if code != 0 {
				t.Errorf("a hook that cannot fetch exited %d — the host stops the session on that", code)
			}
			if out != "" {
				t.Errorf("a hook that cannot fetch wrote to stdout, which the host parses: %q", out)
			}
			if !strings.Contains(errb, "NOT running") {
				t.Errorf("the user must be told the gate is not running: %q", errb)
			}
		})
	}
}

// The other half, and the one that is easy to get wrong by being kind. A
// launcher that exits 0 having run nothing is a silent green underneath
// every check in the tool — the same defect as `check --staged` exiting 0
// having assessed a mistyped filename.
//
// proved by: making give_up exit 0 unconditionally — every command then
// passes without running.
func TestACommandThatCannotFetchRefuses(t *testing.T) {
	skipUnlessPOSIX(t)
	for _, cmd := range []string{"check", "security", "release", "version"} {
		t.Run(cmd, func(t *testing.T) {
			root := fetchRoot(t, "9.9.9")
			_, errb, code := runFetching(t, root,
				[]string{"PROCODER_RELEASE_BASE=http://127.0.0.1:1/nothing"}, cmd)

			if code == 0 {
				t.Errorf("`procoder %s` exited 0 without running: a silent green in the launcher", cmd)
			}
			if !strings.Contains(errb, "could not fetch") {
				t.Errorf("the refusal must name the reason: %q", errb)
			}
		})
	}
}

// A file whose hash does not match is never executed. The exit code then
// follows the ordinary split: not executing is the protection, and failing
// a hook adds no safety while costing the session.
//
// proved by: removing the `[ "$got" = "$want" ]` comparison — the corrupt
// download is then installed and run, and both subtests see its output.
func TestAChecksumMismatchIsNeverExecuted(t *testing.T) {
	skipUnlessPOSIX(t)
	bin := filepath.Join("dist", runtime.GOOS+"-"+runtime.GOARCH, "procoder")
	for _, c := range []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"under a hook", []string{"hook", "post-tool-use"}, 0},
		{"under a command", []string{"check"}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := fetchRoot(t, "9.9.9")
			base, _ := releaseServer(t, "9.9.9", "TAMPERED", true)

			out, errb, code := runFetching(t, root, []string{"PROCODER_RELEASE_BASE=" + base}, c.args...)

			if strings.Contains(out, "TAMPERED") {
				t.Fatal("a binary whose checksum did not match was EXECUTED")
			}
			if code != c.wantCode {
				t.Errorf("exit %d, want %d: %s", code, c.wantCode, errb)
			}
			if !strings.Contains(errb, "checksum mismatch") {
				t.Errorf("the reason must name the mismatch: %q", errb)
			}
			if _, err := os.Stat(filepath.Join(root, bin)); err == nil {
				t.Error("the rejected binary was left at the cache path for the next run to exec")
			}
		})
	}
}

// A manifest that downloads but carries no line for this platform is a
// failed verification, never a pass: the absence of a checksum is not the
// absence of a problem.
//
// proved by: treating an empty `want` as success — an unlisted asset is
// then run unverified.
func TestAnAssetWithNoChecksumLineIsNotRun(t *testing.T) {
	skipUnlessPOSIX(t)
	root := fetchRoot(t, "9.9.9")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS") {
			// a manifest for somebody else's platform entirely
			w.Write([]byte(strings.Repeat("a", 64) + "  procoder-solaris-sparc\n"))
			return
		}
		w.Write([]byte("#!/bin/sh\nprintf '%s' 'UNVERIFIED'\n"))
	}))
	defer srv.Close()

	out, errb, code := runFetching(t, root, []string{"PROCODER_RELEASE_BASE=" + srv.URL}, "check")

	if strings.Contains(out, "UNVERIFIED") {
		t.Fatal("an asset with no checksum line was executed")
	}
	if code == 0 || !strings.Contains(errb, "no line for") {
		t.Errorf("must refuse and say the manifest had no line for this platform: %q", errb)
	}
}

// No version, no guess. The launcher never falls back to the newest
// release: installing a version the plugin does not declare is worse than
// installing nothing, and it is the silent kind — everything would appear
// to work while the binary and the manifest disagreed.
//
// proved by: adding a fallback to "latest" when the manifest yields
// nothing — the launcher then fetches and runs something, and these tests
// see output where they expect a refusal.
func TestNoVersionMeansNoFetch(t *testing.T) {
	skipUnlessPOSIX(t)
	for _, c := range []struct{ name, manifest, want string }{
		{"absent", "", "cannot tell which version"},
		{"unparseable", "{ this is not json", "no version in"},
		{"no version key", `{"name":"procoder"}`, "no version in"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := fetchRoot(t, "9.9.9")
			path := filepath.Join(root, ".claude-plugin", "plugin.json")
			if c.manifest == "" {
				os.Remove(path)
			} else if err := os.WriteFile(path, []byte(c.manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			base, requests := releaseServer(t, "9.9.9", "SHOULD-NOT-RUN", false)

			out, errb, code := runFetching(t, root, []string{"PROCODER_RELEASE_BASE=" + base}, "check")

			if code == 0 || strings.Contains(out, "SHOULD-NOT-RUN") {
				t.Fatalf("fetched something without a declared version: out=%q code=%d", out, code)
			}
			if n := atomic.LoadInt32(requests); n != 0 {
				t.Errorf("made %d request(s) with no version to ask for", n)
			}
			if !strings.Contains(errb, c.want) {
				t.Errorf("message %q does not contain %q", errb, c.want)
			}
		})
	}
}

// Hooks fire dozens of times a minute. Retrying a failing download on each
// one would put a dead network call on the hot path, so a failure is
// remembered briefly — and the memory is not silence: the reason is still
// printed every time.
//
// proved by: removing the failure-marker check — the second invocation
// then makes its own request and the count is two.
func TestAFailedFetchIsRememberedRatherThanRetriedEveryTime(t *testing.T) {
	skipUnlessPOSIX(t)
	root := fetchRoot(t, "9.9.9")
	// a server that 404s everything: reachable, but no such release
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	var n int32
	counting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		http.NotFound(w, r)
	}))
	defer counting.Close()

	_, first, _ := runFetching(t, root, []string{"PROCODER_RELEASE_BASE=" + counting.URL}, "check")
	after := atomic.LoadInt32(&n)
	if after == 0 {
		t.Fatal("the first invocation never tried to fetch")
	}
	_, second, code := runFetching(t, root, []string{"PROCODER_RELEASE_BASE=" + counting.URL}, "check")

	if got := atomic.LoadInt32(&n); got != after {
		t.Errorf("the second invocation made %d more request(s); a failure must be remembered", got-after)
	}
	if code == 0 {
		t.Error("remembering a failure must not turn it into a pass")
	}
	if !strings.Contains(second, "not retrying") {
		t.Errorf("the remembered failure must still say what is wrong, every time: %q", second)
	}
	if first == "" {
		t.Error("the first failure said nothing at all")
	}
}

// Two launchers racing on a fresh install must both end with a working
// binary. The install renames a completed temporary file into place, so
// there is no window in which the cache path holds a partial one.
//
// What this test proves, honestly: that a two-way race leaves a correct
// binary and neither caller fails. What it does NOT prove is the
// atomicity itself — swapping the `mv` for a `cp` leaves this test
// passing, because copying a small file wins a two-way race almost every
// time and the unsafe window is microseconds wide. Hitting it
// deterministically would need a filesystem this suite does not control.
//
// The rename is kept regardless: it costs nothing, it is the standard
// answer, and "the test could not catch it" is a statement about the test.
// Claiming a mutation proof here would have been false, which is why this
// says so instead.
//
// proved by: nothing single-handedly. The race behaviour below is proved
// by removing the fetch entirely (both callers then fail); the atomicity
// is argued from the structure rather than measured.
func TestTwoLaunchersRacingBothEndWithAWorkingBinary(t *testing.T) {
	skipUnlessPOSIX(t)
	root := fetchRoot(t, "9.9.9")
	base, _ := releaseServer(t, "9.9.9", "RACED", false)

	type res struct {
		out  string
		code int
	}
	results := make(chan res, 2)
	for i := 0; i < 2; i++ {
		go func() {
			out, _, code := runFetching(t, root, []string{"PROCODER_RELEASE_BASE=" + base}, "anything")
			results <- res{out, code}
		}()
	}
	for i := 0; i < 2; i++ {
		r := <-results
		if r.code != 0 || r.out != "RACED" {
			t.Errorf("a racing launcher did not get a working binary: out=%q code=%d", r.out, r.code)
		}
	}
	bin := filepath.Join(root, "dist", runtime.GOOS+"-"+runtime.GOARCH, "procoder")
	body, err := os.ReadFile(bin)
	if err != nil {
		t.Fatalf("no binary at the cache path after the race: %v", err)
	}
	if !strings.Contains(string(body), "RACED") {
		t.Errorf("the cached binary is not the whole downloaded file: %q", body)
	}
}
