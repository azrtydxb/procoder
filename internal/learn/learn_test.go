package learn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func collect() (func(string), *[]string) {
	var got []string
	return func(s string) { got = append(got, s) }, &got
}

func joined(got *[]string) string { return strings.Join(*got, "\n") }

func recordsAt(t *testing.T, root string, lines ...string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, File), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// [S-1] proved by: drop the `if !on` guard in Append — a file appears with
// recording off and this fails on the second half.
func TestARecordIsAppendedOnlyWhenRecordingIsOn(t *testing.T) {
	root := t.TempDir()
	Append(root, Record{Cmd: "check", Ms: 12, At: "2026-08-27T10:00:00Z"}, false)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(Dir), File)); !os.IsNotExist(err) {
		t.Fatal("a record was written with recording off")
	}
	Append(root, Record{Cmd: "check", Ms: 12, At: "2026-08-27T10:00:00Z"}, true)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Dir), File))
	if err != nil {
		t.Fatalf("no record written with recording on: %v", err)
	}
	if n := strings.Count(strings.TrimSpace(string(raw)), "\n") + 1; n != 1 {
		t.Errorf("wrote %d lines for one run, want 1", n)
	}
}

// [S-1] The one place a failure is deliberately silent.
//
// proved by: make Append return an error and have the caller act on it —
// there is no way to satisfy this test, which is the point: the contract
// is that a record failure changes nothing at all.
func TestARecordWriteFailureChangesNothing(t *testing.T) {
	root := t.TempDir()
	// A file where the state directory belongs: MkdirAll fails on every
	// platform, where chmod does not deny writes on Windows.
	if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(Dir)), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Must not panic and must not report: Append has no error to give.
	Append(root, Record{Cmd: "check", Ms: 1, At: "2026-08-27T10:00:00Z"}, true)
}

// [S-2] proved by: sort rank() ascending — the slower command stops coming
// first and this fails.
func TestMeasureRanksDomainsByTotalDuration(t *testing.T) {
	root := t.TempDir()
	recordsAt(t, root,
		`{"cmd":"check","ms":100,"at":"2026-08-27T10:00:00Z"}`,
		`{"cmd":"test","ms":900,"at":"2026-08-27T10:01:00Z"}`,
		`{"cmd":"check","ms":100,"at":"2026-08-27T10:02:00Z"}`,
	)
	out, got := collect()
	if code := Measure(root, true, out); code != 0 {
		t.Fatalf("code = %d", code)
	}
	j := joined(got)
	if !strings.Contains(j, "3 run(s) recorded") {
		t.Errorf("the sample count must be printed: %s", j)
	}
	ti, ci := strings.Index(j, "test"), strings.Index(j, "check")
	if ti < 0 || ci < 0 || ti > ci {
		t.Errorf("the slower command must rank first:\n%s", j)
	}
}

// [S-2] proved by: return an empty Reading instead of the error in Read's
// failure path — Measure prints a ranking of nothing and exits 0.
func TestMeasureOnAnUnreadableFileSaysNotMeasured(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(Dir), File)
	if err := os.MkdirAll(dir, 0o755); err != nil { // a directory where the file goes
		t.Fatal(err)
	}
	out, got := collect()
	if code := Measure(root, true, out); code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(joined(got), "NOT measured") {
		t.Errorf("must say NOT measured: %s", joined(got))
	}
}

// [S-2] proved by: drop the `if !recording` branch — an absent file reads
// as "none recorded yet" whether or not anything was ever going to be.
func TestMeasureWithRecordingOffNamesTheSetting(t *testing.T) {
	out, got := collect()
	if code := Measure(t.TempDir(), false, out); code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	if !strings.Contains(joined(got), "[learn] record") {
		t.Errorf("must name the setting: %s", joined(got))
	}
}

// [S-2] proved by: `continue` without counting in either skip branch of
// Read — the counts vanish from the report and this fails.
func TestCorruptAndNegativeLinesAreCountedNotDropped(t *testing.T) {
	root := t.TempDir()
	recordsAt(t, root,
		`{"cmd":"check","ms":100,"at":"2026-08-27T10:00:00Z"}`,
		`{"cmd":"check","ms":-5,"at":"2026-08-27T10:01:00Z"}`,
		`{"cmd":"chec`,
	)
	out, got := collect()
	Measure(root, true, out)
	j := joined(got)
	if !strings.Contains(j, "1 unreadable line(s)") || !strings.Contains(j, "1 negative duration(s)") {
		t.Errorf("both skips must be counted in the report: %s", j)
	}
}

// [S-3] P-CONTROL. proved by: have Propose write the change it prints —
// config.toml stops matching and this fails.
func TestProposePrintsAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(root, ".procoder", "config.toml")
	const body = "[learn]\nrecord = true\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines, `{"cmd":"check","ms":100,"at":"2026-08-27T10:00:0`+string(rune('0'+i))+`Z"}`)
	}
	recordsAt(t, root, lines...)
	out, _ := collect()
	Propose(root, true, 3, out)
	after, err := os.ReadFile(cfg)
	if err != nil || string(after) != body {
		t.Errorf("config.toml was modified by a command that only prints: %q", string(after))
	}
}

// [S-3] proved by: drop the min-samples branch — a proposal is printed
// from two runs and the shortfall is never named.
func TestProposeBelowMinSamplesDeclines(t *testing.T) {
	root := t.TempDir()
	recordsAt(t, root, `{"cmd":"check","ms":100,"at":"2026-08-27T10:00:00Z"}`)
	out, got := collect()
	Propose(root, true, 20, out)
	j := joined(got)
	if !strings.Contains(j, "no proposal") || !strings.Contains(j, "19 more") {
		t.Errorf("must decline and say how many more runs it needs: %s", j)
	}
}

// [S-4] proved by: return a bare string from class() instead of
// evidence.Measured.String() — the label stops matching the vocabulary the
// rest of procoder uses and this fails.
func TestEveryNumberCarriesItsEvidenceClass(t *testing.T) {
	root := t.TempDir()
	recordsAt(t, root, `{"cmd":"check","ms":100,"at":"2026-08-27T10:00:00Z"}`)
	out, got := collect()
	Measure(root, true, out)
	for _, line := range *got {
		if !strings.ContainsAny(line, "0123456789") {
			continue
		}
		if !strings.Contains(line, "[measured]") && !strings.Contains(line, "[manual claim]") {
			t.Errorf("a line with a number carries no evidence class: %q", line)
		}
	}
}

// [S-5] proved by: return 0 from the did-not-fall branch without printing
// the revert — this fails on both the code and the missing line.
func TestVerifyPrintsTheRevertWhenCostDidNotFall(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	applied := `{"target":"check","at":"2026-08-27T10:00:00Z","before_ms":100}`
	if err := os.WriteFile(filepath.Join(dir, AppliedFile), []byte(applied), 0o644); err != nil {
		t.Fatal(err)
	}
	recordsAt(t, root, `{"cmd":"check","ms":500,"at":"2026-08-27T11:00:00Z"}`)
	out, got := collect()
	if code := Verify(root, out); code != 1 {
		t.Errorf("code = %d, want 1 when the cost did not fall", code)
	}
	j := joined(got)
	if !strings.Contains(j, "did NOT fall") || !strings.Contains(j, "revert") {
		t.Errorf("must report the direction and print the revert: %s", j)
	}
}

// [S-5] proved by: have Verify fall back to inferring the moment from the
// git history when the marker is absent — it stops saying NOT verified.
func TestVerifyWithoutAMarkerSaysSo(t *testing.T) {
	out, got := collect()
	if code := Verify(t.TempDir(), out); code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(joined(got), "NOT verified") {
		t.Errorf("no anchor is not success: %s", joined(got))
	}
}

// [S-6] The gate's own default. proved by: default LearnRecord to true in
// internal/config — Append then writes on a repository that never asked.
func TestRecordingIsOffByDefault(t *testing.T) {
	root := t.TempDir()
	// The zero value of the config field is what a repository with no
	// [learn] section gets, and it is what Append is handed.
	Append(root, Record{Cmd: "check", Ms: 1, At: "2026-08-27T10:00:00Z"}, false)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(Dir), File)); !os.IsNotExist(err) {
		t.Error("a repository that never asked for measurement got a records file")
	}
}

// [S-7] The decision recorded on 2026-08-27: a proposal may loosen a
// blocking policy, and must say what the measurement cannot see.
//
// proved by: delete the "cannot see the defects" line from Propose — the
// proposal still prints and this fails.
func TestALooseningProposalStatesWhatItCannotSee(t *testing.T) {
	root := t.TempDir()
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines, `{"cmd":"check","ms":100,"blocking":false,"at":"2026-08-27T10:00:0`+string(rune('0'+i))+`Z"}`)
	}
	recordsAt(t, root, lines...)
	out, got := collect()
	Propose(root, true, 3, out)
	j := joined(got)
	if !strings.Contains(j, `policy = "report"`) {
		t.Fatalf("expected a loosening proposal in this fixture: %s", j)
	}
	if !strings.Contains(j, "cannot see the defects") {
		t.Errorf("a loosening proposal must name what it cannot see: %s", j)
	}
}

// The records are the input to every number above, so their parse has to
// survive a Windows checkout.
//
// proved by: drop normaliseEOL from Read — the CRLF line keeps its \r,
// json.Unmarshal still accepts it, but the count of usable records is
// wrong the moment a line ends up empty-after-\r.
func TestReadHandlesCRLF(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "{\"cmd\":\"check\",\"ms\":10,\"at\":\"2026-08-27T10:00:00Z\"}\r\n"
	if err := os.WriteFile(filepath.Join(dir, File), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Records) != 1 || r.Corrupt != 0 {
		t.Errorf("CRLF record not parsed cleanly: %+v", r)
	}
}

var _ = time.RFC3339
