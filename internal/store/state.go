package store

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// The five gitignored state owners, named here rather than in each owning
// package so one place knows what procoder's session state consists of.
// These are the files the daemon will serialise; the paths themselves do
// not move.
const (
	dispatchPath = ".procoder/state/dispatch.json"
	claimsPath   = ".procoder/state/claims.json"
	envPath      = ".procoder/state/env.json"
	learnPath    = ".procoder/state/learn.jsonl"
	handoffPath  = ".procoder/state/handoff.md"
	markerDir    = ".procoder/state"
)

// LoadDispatch reads the parallel-dispatch ledger.
func LoadDispatch(root string) ([]byte, error) { return ReadFile(root, dispatchPath) }

// SaveDispatch replaces the parallel-dispatch ledger.
func SaveDispatch(root string, data []byte) error { return save(root, dispatchPath, data) }

// LoadClaims reads the work-claims ledger.
func LoadClaims(root string) ([]byte, error) { return ReadFile(root, claimsPath) }

// SaveClaims replaces the work-claims ledger.
func SaveClaims(root string, data []byte) error { return save(root, claimsPath, data) }

// LoadEnvState reads the environment baseline.
func LoadEnvState(root string) ([]byte, error) { return ReadFile(root, envPath) }

// SaveEnvState replaces the environment baseline.
func SaveEnvState(root string, data []byte) error { return save(root, envPath, data) }

// LoadLearn reads the timing records.
func LoadLearn(root string) ([]byte, error) { return ReadFile(root, learnPath) }

// AppendLearn adds one record line.
//
// Read, append, write under one lock rather than an O_APPEND handle. An
// O_APPEND write is atomic only up to the pipe buffer and only on some
// filesystems, and "usually" is not a property a measurement can be built
// on — a lost record is a measurement that quietly understates itself.
func AppendLearn(root string, line []byte) error {
	release, _, err := Lock(root, learnPath)
	if err != nil {
		return err
	}
	defer release()

	old, err := ReadFile(root, learnPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("procoder: could not read %s (%v) — the record was NOT appended", learnPath, err)
	}
	return WriteFile(root, learnPath, append(old, line...), 0o644)
}

// LoadHandoff reads the session handoff note.
func LoadHandoff(root string) ([]byte, error) { return ReadFile(root, handoffPath) }

// SaveHandoff replaces the session handoff note.
func SaveHandoff(root string, data []byte) error { return save(root, handoffPath, data) }

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

// markerPath refuses a name that is a path.
//
// A marker is named by a file name, not by a location, and joining an
// unchecked name would let ".." walk out of .procoder/state and overwrite
// whatever it found. Nothing in procoder passes a hostile name today; this
// costs one comparison and removes the question.
func markerPath(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return "", errors.New("procoder: a marker is named by a file name, not a path: " + name)
	}
	return markerDir + "/" + name, nil
}

// save is the shape every state write shares: take the file's lock, replace
// it atomically, release.
//
// Reads deliberately do NOT lock. The atomic rename means a reader always
// sees a whole file, which is the property that matters, and a reader that
// waited on a writer would put the gate's slowest write on the hot path of
// every hook.
func save(root, relPath string, data []byte) error {
	release, _, err := Lock(root, relPath)
	if err != nil {
		return err
	}
	defer release()
	return WriteFile(root, relPath, data, 0o644)
}
