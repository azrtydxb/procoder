package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// The five gitignored state owners, named here rather than in each owning
// package so one place knows what procoder's session state consists of.
// These are the files the daemon will serialise; the paths themselves do
// not move.
const (
	// StateDir and the paths under it are exported so the packages that
	// own this data can NAME the same string rather than declare their own
	// copy of it. Two declarations of one path drift, and the drift is
	// silent until one writes where the other no longer reads.
	StateDir     = ".procoder/state"
	DispatchPath = StateDir + "/dispatch.json"
	ClaimsPath   = StateDir + "/claims.json"
	EnvPath      = StateDir + "/env.json"
	LearnPath    = StateDir + "/learn.jsonl"
	HandoffPath  = StateDir + "/handoff.md"
)

// LoadDispatch reads the parallel-dispatch ledger.
func LoadDispatch(root string) ([]byte, error) { return ReadFile(root, DispatchPath) }

// SaveDispatch replaces the parallel-dispatch ledger.
func SaveDispatch(root string, data []byte) error { return save(root, DispatchPath, data) }

// LoadClaims reads the work-claims ledger.
func LoadClaims(root string) ([]byte, error) { return ReadFile(root, ClaimsPath) }

// SaveClaims replaces the work-claims ledger.
func SaveClaims(root string, data []byte) error { return save(root, ClaimsPath, data) }

// LoadEnvState reads the environment baseline.
func LoadEnvState(root string) ([]byte, error) { return ReadFile(root, EnvPath) }

// SaveEnvState replaces the environment baseline.
func SaveEnvState(root string, data []byte) error { return save(root, EnvPath, data) }

// LoadLearn reads the timing records.
func LoadLearn(root string) ([]byte, error) { return ReadFile(root, LearnPath) }

// AppendLearn adds one record line.
//
// O_APPEND under the lock, not a read-modify-write. The lock is what makes
// the append safe now, so rewriting the whole file to add one line would
// buy nothing and cost everything: learn.Append runs on EVERY procoder
// invocation, the record file has no bound today (see the open todo), and a full rewrite would
// make each command pay for the length of its own history.
func AppendLearn(root string, line []byte) error {
	release, err := Lock(root, LearnPath)
	if err != nil {
		return err
	}
	defer release()

	p := filepath.Join(root, filepath.FromSlash(LearnPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("procoder: could not create %s (%v) — the record was NOT appended", StateDir, err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("procoder: could not open %s (%v) — the record was NOT appended", LearnPath, err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("procoder: could not append to %s (%v) — the record was NOT appended", LearnPath, err)
	}
	// A write that fails on close is a failed write.
	if err := f.Close(); err != nil {
		return fmt.Errorf("procoder: could not close %s (%v) — the record was NOT appended", LearnPath, err)
	}
	return nil
}

// LoadHandoff reads the session handoff note.
func LoadHandoff(root string) ([]byte, error) { return ReadFile(root, HandoffPath) }

// SaveHandoff replaces the session handoff note.
func SaveHandoff(root string, data []byte) error { return save(root, HandoffPath, data) }

// LoadMarker reads one of the small single-line markers beside the handoff
// note — last-decisions-digest, last-unasked-decision.
func LoadMarker(root, name string) ([]byte, error) {
	p, err := markerPath(name)
	if err != nil {
		return nil, err
	}
	return ReadFile(root, p)
}

// SaveMarker replaces one of those markers.
func SaveMarker(root, name string, data []byte) error {
	p, err := markerPath(name)
	if err != nil {
		return err
	}
	return save(root, p, data)
}

// markerPath refuses a name that is a path, which is inDir's job — a
// marker is a file in .procoder/state and nothing more.
func markerPath(name string) (string, error) { return inDir(StateDir, name) }

// save is the shape EVERY write in this package shares — state and content
// alike: take the file's lock, replace it atomically, release. Per file,
// not per directory, so two people editing two different stories have no
// reason to wait for each other.
//
// Reads deliberately do NOT lock. The atomic rename means a reader always
// sees a whole file, which is the property that matters, and a reader that
// waited on a writer would put the gate's slowest write on the hot path of
// every hook.
func save(root, relPath string, data []byte) error {
	release, err := Lock(root, relPath)
	if err != nil {
		return err
	}
	defer release()
	return WriteFile(root, relPath, data, 0o644)
}
