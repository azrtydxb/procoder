package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var when = time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

// A wave launched properly: every task started, then sealed, then returns.
//
// proved by: the `w.Sealed == ""` test in Apply's return branch inverted —
// a correct wave records an early return and reads as serial.
func TestAWaveLaunchedTogetherReadsAsParallel(t *testing.T) {
	var ws []Wave
	ws, _, _ = Apply(ws, "open", "w", "", when)
	ws, _, _ = Apply(ws, "start", "w", "a", when)
	ws, _, _ = Apply(ws, "start", "w", "b", when)
	ws, _, _ = Apply(ws, "seal", "w", "", when)
	ws, _, _ = Apply(ws, "return", "w", "a", when)
	if v := Verdict(ws[0]); !strings.Contains(v, "parallel") || strings.Contains(v, "NOT") {
		t.Fatalf("a correctly launched wave did not read as parallel: %s", v)
	}
}

// The check that matters: a return before the seal means the first task
// finished before the last was launched, which is what serial work is.
//
// proved by: the EarlyReturn append removed — serial work reads as
// parallel and the whole barrier does nothing.
func TestAReturnBeforeTheSealIsNotParallel(t *testing.T) {
	var ws []Wave
	ws, _, _ = Apply(ws, "open", "w", "", when)
	ws, _, _ = Apply(ws, "start", "w", "a", when)
	ws, _, _ = Apply(ws, "return", "w", "a", when) // before the seal
	ws, _, _ = Apply(ws, "start", "w", "b", when)
	ws, _, _ = Apply(ws, "seal", "w", "", when)
	v := Verdict(ws[0])
	if !strings.Contains(v, "NOT parallel") {
		t.Fatalf("serial work read as parallel: %s", v)
	}
	// The evidence has to survive the later seal, or asking afterwards
	// finds a wave that looks clean.
	if len(ws[0].EarlyReturn) != 1 {
		t.Errorf("the early return was not remembered: %+v", ws[0])
	}
}

// Not verified is not the same as verified serial. A wave nobody sealed
// has not been shown to be parallel — and saying that is different from
// saying it was not.
//
// proved by: the `w.Sealed == ""` case removed from Verdict — an unsealed
// wave reports as parallel on no evidence.
func TestAnUnsealedWaveIsNotVerifiedRatherThanSerial(t *testing.T) {
	var ws []Wave
	ws, _, _ = Apply(ws, "open", "w", "", when)
	ws, _, _ = Apply(ws, "start", "w", "a", when)
	v := Verdict(ws[0])
	if !strings.Contains(v, "NOT verified") {
		t.Fatalf("an unsealed wave did not report as unverified: %s", v)
	}
	if strings.Contains(v, "NOT parallel") {
		t.Errorf("an unsealed wave was reported as serial, which is a claim nothing supports: %s", v)
	}
}

// A task starting after the seal was not part of the wave. Accepting it
// would let an agent seal early, keep launching, and collect a clean
// verdict.
//
// proved by: the sealed check removed from the start branch — a late task
// joins and the barrier means nothing.
func TestATaskCannotJoinAfterTheSeal(t *testing.T) {
	var ws []Wave
	ws, _, _ = Apply(ws, "open", "w", "", when)
	ws, _, _ = Apply(ws, "seal", "w", "", when)
	ws, msg, code := Apply(ws, "start", "w", "late", when)
	if code == 0 {
		t.Fatalf("a task joined a sealed wave: %s", msg)
	}
	if len(ws[0].Started) != 0 {
		t.Errorf("the late task was recorded anyway: %+v", ws[0])
	}
}

// Reopening a wave would overwrite the record of what happened.
//
// proved by: the existing-wave check removed from open — the second open
// silently replaces the first wave's history.
func TestAWaveCannotBeReopened(t *testing.T) {
	var ws []Wave
	ws, _, _ = Apply(ws, "open", "w", "", when)
	ws, _, code := Apply(ws, "open", "w", "", when)
	if code == 0 {
		t.Fatal("a wave was reopened, overwriting its record")
	}
	if len(ws) != 1 {
		t.Errorf("a duplicate wave was appended: %d", len(ws))
	}
}

// An unreadable ledger is not "no waves ran" — the shape this repository
// has found six times, guarded here rather than fixed later.
//
// proved by: the JSON error branch in Load made to return nil, nil — a
// corrupt ledger reports no waves and Status reports success.
func TestAnUnreadableLedgerIsNotNoWaves(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(File))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("a corrupt ledger loaded as no waves")
	}
	var lines []string
	if code := Status(root, "", func(s string) { lines = append(lines, s) }); code == 0 {
		t.Errorf("status reported success over a ledger it could not read: %v", lines)
	}
}
