package security

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// A planted high-entropy token must come back as a blocking finding that
// names the rule and location and NEVER echoes the secret itself.
func TestSecretIsBlockingAndNeverEchoed(t *testing.T) {
	if tools.Resolve(Gitleaks, "") == "" {
		t.Skip("gitleaks not installed; the not-checked path is tested below")
	}
	root := t.TempDir()
	// assembled at runtime so no scanner ever matches this SOURCE file —
	// the written fixture file is what must be caught
	token := "ghp_" + "x9J2mQ8v" + "LpZ4tR7n" + "W3kY6bC1" + "dF5gH0aSeD"
	f := filepath.Join(root, "cfg.py")
	os.WriteFile(f, []byte("token = \""+token+"\"\n"), 0o644)

	got := Secrets(root, []string{f})
	if len(got) != 1 || !got[0].Blocking || got[0].Line != 1 {
		t.Fatalf("want one blocking line-1 finding, got %+v", got)
	}
	if strings.Contains(got[0].Message, token) || strings.Contains(got[0].File, token) {
		t.Fatal("the secret value must never be echoed")
	}
	if !strings.Contains(got[0].Message, "rotate") {
		t.Fatalf("the finding must tell the agent to rotate: %+v", got[0])
	}
	if got[0].File != f {
		t.Fatalf("single-file scan must keep the path as given: %q", got[0].File)
	}
}

func TestCleanFileIsSilent(t *testing.T) {
	if tools.Resolve(Gitleaks, "") == "" {
		t.Skip("gitleaks not installed")
	}
	root := t.TempDir()
	f := filepath.Join(root, "ok.py")
	os.WriteFile(f, []byte("x = 1\n"), 0o644)
	if got := Secrets(root, []string{f}); len(got) != 0 {
		t.Fatalf("clean file must be silent: %+v", got)
	}
}

// The audit's tree scan covers ONLY the given file set — a secret in a file
// outside the set (gitignored in real life) must not surface — reports
// repo paths, and honours a committed repo-relative .gitleaksignore.
func TestSecretsTreeScopeAndRelativeFingerprints(t *testing.T) {
	if tools.Resolve(Gitleaks, "") == "" {
		t.Skip("gitleaks not installed")
	}
	root := t.TempDir()
	token := "ghp_" + "x9J2mQ8v" + "LpZ4tR7n" + "W3kY6bC1" + "dF5gH0aSeD"
	inScope := filepath.Join(root, "src", "cfg.py")
	os.MkdirAll(filepath.Dir(inScope), 0o755)
	os.WriteFile(inScope, []byte("token = \""+token+"\"\n"), 0o644)
	ignored := filepath.Join(root, "node_modules", "cache.py")
	os.MkdirAll(filepath.Dir(ignored), 0o755)
	os.WriteFile(ignored, []byte("token = \""+token+"\"\n"), 0o644)

	got := SecretsTree(root, []string{inScope})
	if len(got) != 1 || !got[0].Blocking || got[0].File != inScope {
		t.Fatalf("want one blocking finding at %s, got %+v", inScope, got)
	}

	// a repo-relative fingerprint in .gitleaksignore must suppress it —
	// the whole point of scanning relative; the rule name comes from the
	// finding itself so the test survives gitleaks ruleset renames
	ruleID := got[0].Message[strings.Index(got[0].Message, "(")+1 : strings.Index(got[0].Message, ")")]
	ignoreFile := filepath.Join(root, ".gitleaksignore")
	os.WriteFile(ignoreFile, []byte("src/cfg.py:"+ruleID+":1\n"), 0o644)
	if got := SecretsTree(root, []string{inScope, ignoreFile}); len(got) != 0 {
		t.Fatalf("repo-relative fingerprint must suppress the finding: %+v", got)
	}
}

// Missing scanners are blocking NOT-checked, never silence — a security
// check that quietly didn't run is worse than a red one.
func TestMissingToolsReadAsBlockingNotChecked(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if got := Secrets("/tmp", []string{"/tmp"}); len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("missing gitleaks: %+v", got)
	}
	if got := Sast("/tmp"); len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("missing semgrep: %+v", got)
	}
	if got := Deps("/tmp"); len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("missing osv-scanner: %+v", got)
	}
}

// A dependency-free package.json (the agent-layer manifest) must not
// break or block the deep scan; one declaring deps without a lockfile is
// an honest NOT-checked.
func TestNpmManifestWithoutLockfile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasNpmDepsWithoutLockfile(root) {
		t.Error("no dependencies -> nothing to check, no finding")
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"left-pad":"1.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasNpmDepsWithoutLockfile(root) {
		t.Error("deps without lockfile -> must surface as unscannable")
	}
	if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasNpmDepsWithoutLockfile(root) {
		t.Error("a lockfile exists -> osv scans it, no gap")
	}
}

// ---------------------------------------------------------------------------
// Stub-driven legs. CI's test runner has no gitleaks, semgrep or osv-scanner,
// and the logic worth testing is the PARSING of what they print — so these
// legs plant a shell stub on an isolated PATH and exercise the real parsers
// against recorded tool output. They skip on Windows, where the stub is not
// executable.
// ---------------------------------------------------------------------------

// theFakeSecret is planted as the VALUE in every stub fixture. It is obvious
// nonsense, and no finding may ever contain it.
const theFakeSecret = "FAKE-not-a-real-credential-000000"

// stub writes an executable shell stub named name into dir and points PATH
// (and HOME, which tools.Resolve also searches) at nothing else.
func stub(t *testing.T, name, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the tool stub is a shell script")
	}
	dir := t.TempDir()
	// the stub itself needs the ordinary unix utilities; PATH below is
	// emptied so the real scanner can never answer for it
	preamble := "#!/bin/sh\nPATH=/usr/bin:/bin\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(preamble+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("HOME", t.TempDir())
	return dir
}

// reportWriter is the stub preamble that finds --report-path in argv; the
// script that follows writes the recorded report to "$report".
const reportWriter = `report=""
prev=""
for a in "$@"; do
  if [ "$prev" = "--report-path" ]; then report="$a"; fi
  prev="$a"
done
`

// proved by: dropped the `loc = filepath.Join(src, loc)` normalisation in
// scanOne — the finding then carries the bare relative path gitleaks printed.
func TestSecretsDirScanNormalisesPathsAndHidesTheValue(t *testing.T) {
	gl := reportWriter + `cat > "$report" <<'JSON'
[{"File":"sub/cfg.py","StartLine":7,"RuleID":"generic-api-key","Secret":"` + theFakeSecret + `","Match":"token = ` + theFakeSecret + `"}]
JSON
exit 1
`
	stub(t, "gitleaks", gl)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "cfg.py"), []byte("token = \""+theFakeSecret+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Secrets(root, []string{root})
	if len(got) != 1 {
		t.Fatalf("want one finding, got %+v", got)
	}
	f := got[0]
	if !f.Blocking {
		t.Error("a secret is blocking, always")
	}
	if f.Line != 7 {
		t.Errorf("StartLine must reach the finding, got %d", f.Line)
	}
	if want := filepath.Join(root, "sub", "cfg.py"); f.File != want {
		t.Errorf("dir-scan path must be joined onto the scanned root: got %q want %q", f.File, want)
	}
	if !strings.Contains(f.Message, "generic-api-key") {
		t.Errorf("the finding must name the rule: %q", f.Message)
	}
	if strings.Contains(f.Message, theFakeSecret) || strings.Contains(f.File, theFakeSecret) {
		t.Fatalf("SECRET VALUE LEAKED into the finding: %q", f.Message)
	}
}

// proved by: deleted the legacy `detect --no-git --source` retry in scanOne —
// the pre-8.19 gitleaks then reads as "produced no report".
func TestSecretsFallsBackToLegacyDetect(t *testing.T) {
	gl := reportWriter + `if [ "$1" = "dir" ]; then
  echo 'Error: unknown command "dir" for "gitleaks"' >&2
  exit 1
fi
cat > "$report" <<'JSON'
[{"File":"cfg.py","StartLine":2,"RuleID":"legacy-rule"}]
JSON
exit 1
`
	stub(t, "gitleaks", gl)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cfg.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Secrets(root, []string{root})
	if len(got) != 1 || !got[0].Blocking || got[0].Line != 2 {
		t.Fatalf("legacy gitleaks must still report its leak, got %+v", got)
	}
	if !strings.Contains(got[0].Message, "legacy-rule") {
		t.Errorf("wrong finding: %q", got[0].Message)
	}
}

// proved by: made the no-report path return nil instead of a finding — a
// broken scanner then reads as a clean file, which is the one thing the
// honesty rule forbids.
func TestSecretsScannerFailureIsNotChecked(t *testing.T) {
	stub(t, "gitleaks", "echo 'failed to load config: bad toml' >&2\necho 'second line of noise' >&2\nexit 2\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cfg.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Secrets(root, []string{root})
	if len(got) != 1 || !got[0].Blocking {
		t.Fatalf("a scanner that produced nothing must block, got %+v", got)
	}
	if !strings.Contains(got[0].Message, "NOT checked") {
		t.Errorf("must say NOT checked: %q", got[0].Message)
	}
	if !strings.Contains(got[0].Message, "failed to load config: bad toml") {
		t.Errorf("must quote the scanner's diagnosis: %q", got[0].Message)
	}
	if strings.Contains(got[0].Message, "second line of noise") {
		t.Errorf("only the first line belongs in the finding: %q", got[0].Message)
	}
}

// proved by: made the unmarshal failure return nil — garbage in the report
// then silently reads as "no secrets".
func TestSecretsUnreadableReportIsNotChecked(t *testing.T) {
	stub(t, "gitleaks", reportWriter+"printf 'not json at all' > \"$report\"\nexit 0\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cfg.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Secrets(root, []string{root})
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "unreadable") {
		t.Fatalf("an unreadable report must block as NOT checked, got %+v", got)
	}
}

// proved by: dropped the `strings.HasPrefix(rel, "..")` guard in SecretsTree
// — a path outside the repo is then mirrored in and reported.
// The stub reports EVERY regular file it can see in its working directory,
// so what comes back is exactly what was staged for the scan.
func TestSecretsTreeStagesOnlyTheGivenFilesUnderRoot(t *testing.T) {
	gl := reportWriter + `find . -type f | sed 's|^\./||' | sort | awk 'BEGIN{printf "["} {printf "%s{\"File\":\"%s\",\"StartLine\":1,\"RuleID\":\"stub-rule\"}", (NR>1?",":""), $0} END{printf "]"}' > "$report"
exit 1
`
	stub(t, "gitleaks", gl)
	root := t.TempDir()
	inside := filepath.Join(root, "src", "cfg.py")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("token = \""+theFakeSecret+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	untouched := filepath.Join(root, "other.py")
	if err := os.WriteFile(untouched, []byte("token = \""+theFakeSecret+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "elsewhere.py")
	if err := os.WriteFile(outside, []byte("token = \""+theFakeSecret+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.py")
	if err := os.Symlink(inside, link); err != nil {
		t.Fatal(err)
	}

	got := SecretsTree(root, []string{inside, outside, link})
	if len(got) != 1 {
		t.Fatalf("only the in-scope regular file may be scanned, got %+v", got)
	}
	if got[0].File != inside {
		t.Errorf("mirror paths must map back into the repository: got %q want %q", got[0].File, inside)
	}
	if !got[0].Blocking {
		t.Error("a secret is blocking, always")
	}
	if strings.Contains(got[0].Message, theFakeSecret) {
		t.Fatalf("SECRET VALUE LEAKED into the finding: %q", got[0].Message)
	}
}

// proved by: made SecretsTree scan the empty mirror anyway (dropped the
// `staged == 0` return) — the stub then reports nothing but the pass is a
// lie about having scanned something.
func TestSecretsTreeWithNothingInScopeStaysSilent(t *testing.T) {
	stub(t, "gitleaks", reportWriter+"printf '[{\"File\":\"ghost.py\",\"StartLine\":1,\"RuleID\":\"stub-rule\"}]' > \"$report\"\nexit 1\n")
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "elsewhere.py")
	if err := os.WriteFile(outside, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SecretsTree(root, []string{outside}); len(got) != 0 {
		t.Fatalf("nothing in scope means nothing scanned and nothing reported: %+v", got)
	}
}

// proved by: dropped the stat/IsDir filter in SecretsChangedFiles — a deleted
// path or a directory then reaches gitleaks, and with none installed the gate
// blocks on a NOT-checked for files that are not there.
func TestSecretsChangedFilesIgnoresDeletedPathsAndDirectories(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	sub := filepath.Join(root, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got := SecretsChangedFiles(root, []string{filepath.Join(root, "deleted.go"), sub})
	if len(got) != 0 {
		t.Fatalf("a deleted file and a directory are not files to scan: %+v", got)
	}
}

// proved by: flipped the severity test to `!= "ERROR"` — WARNING then blocks
// and ERROR does not, which inverts what the gate stops for.
func TestSastSeverityDecidesWhatBlocks(t *testing.T) {
	long := strings.Repeat("A", 250)
	report := `{"results":[
{"path":"api/auth.py","start":{"line":42},"check_id":"python.lang.security.audit.dangerous-exec","extra":{"severity":"ERROR","message":"exec with user input"}},
{"path":"web/app.js","start":{"line":9},"check_id":"javascript.express.security.audit.cookie","extra":{"severity":"WARNING","message":"` + long + `"}},
{"path":"README.md","start":{"line":1},"check_id":"bare-check","extra":{"severity":"INFO","message":"style nit"}}
],"errors":[]}`
	stub(t, "semgrep", "cat <<'JSON'\n"+report+"\nJSON\nexit 1\n")

	got := Sast(t.TempDir())
	if len(got) != 3 {
		t.Fatalf("want three findings, got %+v", got)
	}
	if !got[0].Blocking {
		t.Error("ERROR severity must block")
	}
	if got[0].File != "api/auth.py" || got[0].Line != 42 {
		t.Errorf("location wrong: %+v", got[0])
	}
	if !strings.Contains(got[0].Message, "dangerous-exec") || strings.Contains(got[0].Message, "python.lang.security") {
		t.Errorf("the check id must be shortened to its last segment: %q", got[0].Message)
	}
	if got[1].Blocking {
		t.Error("WARNING reports, it does not block")
	}
	if strings.Contains(got[1].Message, strings.Repeat("A", 201)) {
		t.Errorf("a long message must be cut at 200 characters: %d chars", len(got[1].Message))
	}
	if got[2].Blocking {
		t.Error("INFO reports, it does not block")
	}
	if !strings.Contains(got[2].Message, "[INFO bare-check]") {
		t.Errorf("an id with no dot stays whole: %q", got[2].Message)
	}
}

// proved by: made the unmarshal failure return nil — a crashed semgrep then
// reads as a clean SAST pass.
func TestSastUnreadableOutputIsNotRun(t *testing.T) {
	stub(t, "semgrep", "echo 'Traceback (most recent call last):' >&2\necho 'boom' >&2\nexit 2\n")
	got := Sast(t.TempDir())
	if len(got) != 1 || !got[0].Blocking {
		t.Fatalf("a crashed semgrep must block, got %+v", got)
	}
	// "boom", not "Traceback (most recent call last):" — a traceback's
	// header is not its reason, and semgrep, like every scanner here,
	// puts what went wrong on the last line.
	if !strings.Contains(got[0].Message, "NOT run") || !strings.Contains(got[0].Message, "boom") {
		t.Errorf("must say NOT run and quote the tool's diagnosis: %q", got[0].Message)
	}
}

// proved by: lowered vulnBlockScore to 6.0 — the 6.9 package then blocks,
// which is not the agreed high/critical line.
func TestDepsBlocksAtTheHighCriticalLine(t *testing.T) {
	report := `{"results":[{"packages":[
{"package":{"name":"github.com/mild/thing","version":"1.0.0"},"vulnerabilities":[{"id":"GHSA-mild"}],"groups":[{"max_severity":"6.9"}]},
{"package":{"name":"github.com/edge/case","version":"2.0.0"},"vulnerabilities":[{"id":"GHSA-a"},{"id":"GHSA-b"}],"groups":[{"max_severity":"4.3"},{"max_severity":"7.0"}]},
{"package":{"name":"github.com/clean/pkg","version":"3.0.0"},"vulnerabilities":[],"groups":[]},
{"package":{"name":"github.com/bad/pkg","version":"0.1.0"},"vulnerabilities":[{"id":"GHSA-c"}],"groups":[{"max_severity":""},{"max_severity":"9.8"}]}
]}]}`
	stub(t, "osv-scanner", "cat <<'JSON'\n"+report+"\nJSON\nexit 1\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Deps(root)
	if len(got) != 3 {
		t.Fatalf("a package with no vulnerabilities is not a finding; got %+v", got)
	}
	if got[0].Blocking {
		t.Errorf("6.9 is below the blocking line: %+v", got[0])
	}
	if !strings.Contains(got[0].Message, "max severity 6.9") {
		t.Errorf("wrong message: %q", got[0].Message)
	}
	if !got[1].Blocking {
		t.Errorf("7.0 is at the blocking line and must block: %+v", got[1])
	}
	want := "github.com/edge/case 2.0.0 has 2 known vulnerability(s), max severity 7.0 — upgrade it (security)"
	if got[1].Message != want {
		t.Errorf("message wrong:\n got %q\nwant %q", got[1].Message, want)
	}
	if !got[2].Blocking || !strings.Contains(got[2].Message, "max severity 9.8") {
		t.Errorf("an unparseable score must not hide the 9.8 next to it: %+v", got[2])
	}
}

// proved by: made Deps pass every name in DepManifests to -L regardless of
// whether the file exists — osv then fails the whole invocation over a
// manifest the repository does not have.
func TestDepsPassesOnlyTheManifestsThatExist(t *testing.T) {
	stub(t, "osv-scanner", "echo \"$@\" > args.txt\nprintf '{\"results\":[]}'\nexit 0\n")
	root := t.TempDir()
	for _, f := range []string{"go.mod", "Cargo.lock"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got := Deps(root); len(got) != 0 {
		t.Fatalf("a clean scan is silent: %+v", got)
	}
	raw, err := os.ReadFile(filepath.Join(root, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	args := string(raw)
	for _, want := range []string{"--format json", "-L go.mod", "-L Cargo.lock"} {
		if !strings.Contains(args, want) {
			t.Errorf("missing %q in osv argv: %s", want, args)
		}
	}
	for _, unwanted := range []string{"requirements.txt", "pom.xml", "Gemfile.lock"} {
		if strings.Contains(args, unwanted) {
			t.Errorf("%s does not exist here and must not be passed: %s", unwanted, args)
		}
	}
}

// proved by: made the no-manifest finding Blocking — a repository with no
// dependencies at all would then fail the security gate.
func TestDepsWithNoManifestsReportsWithoutBlocking(t *testing.T) {
	stub(t, "osv-scanner", "printf '{\"results\":[]}'\n")
	got := Deps(t.TempDir())
	if len(got) != 1 || got[0].Blocking {
		t.Fatalf("nothing to scan is a report, not a block: %+v", got)
	}
	if !strings.Contains(got[0].Message, "no dependency manifests found") {
		t.Errorf("wrong message: %q", got[0].Message)
	}
}

// proved by: made hasNpmDepsWithoutLockfile return false always — the
// unscannable npm tree then vanishes from the report entirely.
func TestDepsSurfacesNpmDepsWithNoLockfile(t *testing.T) {
	stub(t, "osv-scanner", "printf '{\"results\":[]}'\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"left-pad":"1.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Deps(root)
	if len(got) != 1 || !got[0].Blocking {
		t.Fatalf("declared deps with no lockfile is a blocking NOT-checked: %+v", got)
	}
	if !strings.Contains(got[0].Message, "npm dependencies NOT checked") {
		t.Errorf("wrong message: %q", got[0].Message)
	}
}

// proved by: made the osv unmarshal failure return nil — a crashed scanner
// then reads as no known vulnerabilities.
func TestDepsUnreadableOutputIsNotChecked(t *testing.T) {
	stub(t, "osv-scanner", "echo 'panic: bad lockfile' >&2\nexit 127\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Deps(root)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("a crashed osv-scanner must block as NOT checked: %+v", got)
	}
	if !strings.Contains(got[0].Message, "panic: bad lockfile") {
		t.Errorf("must quote the scanner's first line: %q", got[0].Message)
	}
}

// proved by: made firstLine return "" for empty input — the NOT-checked
// message then trails off with nothing where the reason belongs.
func TestFirstLineAndShortCheck(t *testing.T) {
	if got := firstLine(""); got != "no output" {
		t.Errorf("firstLine(\"\") = %q, want \"no output\"", got)
	}
	if got := firstLine("head\ntail\n"); got != "head" {
		t.Errorf("firstLine multi-line = %q, want \"head\"", got)
	}
	if got := firstLine(strings.Repeat("z", 300)); len(got) != 160 {
		t.Errorf("firstLine must cut at 160, got %d", len(got))
	}
	if got := shortCheck("a.b.c"); got != "c" {
		t.Errorf("shortCheck(\"a.b.c\") = %q, want \"c\"", got)
	}
	if got := shortCheck("plain"); got != "plain" {
		t.Errorf("shortCheck(\"plain\") = %q, want \"plain\"", got)
	}
}

// The defect: pyproject.toml sat in DepManifests, osv-scanner has no
// extractor for it — it declares ranges, not pinned versions — and osv
// exits 127 emitting NO json at all. One unscannable manifest therefore
// took every other manifest in the same invocation down with it, so a
// repository with a pyproject.toml was told "output unreadable" while a
// CVE-carrying go.mod sat beside it unexamined.
//
// proved by: putting "pyproject.toml" back in DepManifests — this test
// then finds it in the argv.
func TestPyprojectTomlIsNeverPassedToOsvAsALockfile(t *testing.T) {
	stub(t, "osv-scanner", "echo \"$@\" > args.txt\nprintf '{\"results\":[]}'\nexit 0\n")
	root := t.TempDir()
	for _, f := range []string{"go.mod", "pyproject.toml"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("dependencies = []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	Deps(root)

	raw, err := os.ReadFile(filepath.Join(root, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	args := string(raw)
	if strings.Contains(args, "pyproject.toml") {
		t.Errorf("osv has no extractor for pyproject.toml and aborts the whole run on it: %s", args)
	}
	if !strings.Contains(args, "-L go.mod") {
		t.Errorf("the manifests osv CAN read must still be scanned: %s", args)
	}
}

// Dropping pyproject.toml from the scan without saying so would trade a
// loud failure for a silent one: a Python repository reported clean by a
// scanner that never looked. It gets the same honest gap a bare
// package.json gets.
//
// proved by: made pythonGaps return nil — this test then finds no gap
// reported for a pyproject.toml with dependencies and no lock file.
func TestAPyprojectWithDependenciesAndNoLockFileIsAnHonestGap(t *testing.T) {
	stub(t, "osv-scanner", "printf '{\"results\":[]}'\nexit 0\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"),
		[]byte("[project]\nname = \"x\"\ndependencies = [\"requests==2.0.0\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Deps(root)

	found := false
	for _, f := range got {
		if strings.Contains(f.Message, "python dependencies NOT checked") {
			found = true
			if !f.Blocking {
				t.Error("a dependency set nothing scanned must block, like the npm one")
			}
			if f.File != "pyproject.toml" {
				t.Errorf("the gap must name the file, got %q", f.File)
			}
		}
	}
	if !found {
		t.Errorf("no gap reported for a pyproject.toml nothing can scan: %+v", got)
	}
}

// An empty declaration is not a dependency set. The first version of this
// check matched the word "dependencies" anywhere in the file, so a project
// with `dependencies = []` was told its dependencies had not been checked
// — a gap reported about nothing at all. The fixture caught it.
//
// proved by: matching the bare word instead of the regexp — every empty
// case below then reports a gap.
func TestAnEmptyDependencyDeclarationIsNotADependencySet(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"pep621 empty", "[project]\ndependencies = []\n", false},
		{"pep621 one", "[project]\ndependencies = [\"requests==2.0.0\"]\n", true},
		{"pep621 multiline", "[project]\ndependencies = [\n  \"requests\",\n]\n", true},
		{"poetry empty table", "[tool.poetry.dependencies]\n", false},
		{"poetry one", "[tool.poetry.dependencies]\nrequests = \"^2.0\"\n", true},
		{"groups empty", "[dependency-groups]\n", false},
		{"groups one", "[dependency-groups]\ndev = [\"pytest\"]\n", true},
		{"only prose", "# this project lists no dependencies anywhere\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := hasPythonDepsWithoutLockfile(root); got != c.want {
				t.Errorf("%s: got %v want %v for:\n%s", c.name, got, c.want, c.body)
			}
		})
	}
}

// proved by: made hasPythonDepsWithoutLockfile ignore the lock files —
// this test then reports a gap for a repository that pinned its versions.
func TestAPyprojectWithALockFileBesideItIsNotAGap(t *testing.T) {
	stub(t, "osv-scanner", "printf '{\"results\":[]}'\nexit 0\n")
	root := t.TempDir()
	for _, f := range []string{"pyproject.toml", "poetry.lock"} {
		if err := os.WriteFile(filepath.Join(root, f),
			[]byte("[project]\ndependencies = [\"requests==2.0.0\"]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, f := range Deps(root) {
		if strings.Contains(f.Message, "python dependencies NOT checked") {
			t.Errorf("poetry.lock is beside it and osv reads that: %+v", f)
		}
	}
}

// A scanner prints its progress first and the reason it gave up last, so
// reporting the FIRST line of stderr told the reader "dependencies were
// NOT checked: Starting filesystem walk for root: /" — which is alarming,
// is not the reason, and gives a person nothing to act on.
//
// proved by: changing lastLine back to firstLine's behaviour — this test
// then gets the progress line instead of the diagnostic.
func TestAFailedToolIsReportedByItsDiagnosticNotItsProgress(t *testing.T) {
	stderr := "Starting filesystem walk for root: /\n" +
		"Scanned /repo/go.mod file and found 1 package\n" +
		"could not determine extractor suitable to this file: \"/repo/pyproject.toml\"\n"

	got := why(stderr, errors.New("exit status 127"))

	if !strings.Contains(got, "could not determine extractor") {
		t.Errorf("the reason is the last line, got %q", got)
	}
	if strings.Contains(got, "filesystem walk") {
		t.Errorf("progress is not a diagnosis, got %q", got)
	}
	if strings.Contains(got, "exit status") {
		t.Errorf("the exit status is the fallback, not the answer, got %q", got)
	}
	// A tool that failed without a word still has to explain itself, so
	// the exit status is what is left to say.
	if got := why("", errors.New("exit status 127")); got != "exit status 127" {
		t.Errorf("silent tool must fall back to the status, got %q", got)
	}
	if got := why("   \n  \n", nil); got != "no output" {
		t.Errorf("whitespace is not a diagnosis, got %q", got)
	}
}
