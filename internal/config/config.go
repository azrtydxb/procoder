// Package config reads .procoder/config.toml — the one place a repository
// tunes the harness (D-HOME). The parser is a deliberate subset of TOML:
// [sections], key = "string" | integer | true/false, and # comments. That is
// every shape the config uses, and a subset parser the tests fully cover beats
// a dependency the design forbids.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds every knob the harness reads. Zero values are the defaults, so
// a repository with no .procoder/ directory gets sensible behaviour.
type Config struct {
	// AskBlock: pending questions block the gate instead of being reported
	// (`[ask] policy = "block"`). Default report — a question is a request
	// for judgement, not a defect, and blocking a commit on one stops work
	// the human may not be awake to unblock.
	AskBlock bool

	// VersionCheckOff silences the session-start version check
	// (`[version] check = "off"`). It is a knob for CI and scripted runs,
	// where nobody is going to act on the warning and the second spent
	// asking GitHub buys nothing. There is deliberately no third value: a
	// setting that upgraded without asking would break the consent this
	// feature is built on.
	VersionCheckOff bool

	// BlockDefaultBranch: work on the default branch blocks the gate instead
	// of being reported. Off by default — solo repositories commit to main
	// routinely, and teams usually enforce this server-side anyway.
	BlockDefaultBranch bool
	// MaxFileMB is the oversized-file threshold for the gate.
	MaxFileMB int
	// LintBlock: lint findings block the gate instead of being reported.
	LintBlock bool
	// TestBlock: todo and story closes run the test suite and refuse
	// while it fails (or cannot be verified).
	TestBlock bool
	// PinActions: unpinned GitHub Action refs block instead of being reported.
	PinActions bool
	// Maintain thresholds for the complexity report; zero means the default.
	Gocyclo          int
	FunlenLines      int
	FunlenStatements int
	// DebtMarker is the comment marker `procoder debt` harvests.
	DebtMarker string
	// SprintRetroOff disables the retro gate: without it, opening a new
	// sprint refuses while the last closed sprint's retro is empty.
	SprintRetroOff bool
	// ReleaseFiles are the version-bearing files `procoder release`
	// verifies; unset means the version-sync leg verifies nothing (said).
	ReleaseFiles []string
	// BenchThreshold is the regression percentage `procoder bench` marks;
	// zero means the default of 10.
	BenchThreshold int
	// CommitGate governs the commit interception: "block" (default) stops
	// a commit whose gate has blocking findings, "report" prints them and
	// lets it through, "off" skips the check.
	CommitGate string
	// DocsBlock: the documentation obligation blocks the gate instead of
	// being reported. Off by default — procoder never blocks a repository
	// by surprise on upgrade.
	DocsBlock bool
}

// Defaults per the design contract.
const defaultMaxFileMB = 5

// Load reads .procoder/config.toml under root. A missing file is the normal
// case and returns defaults; an unreadable line is skipped rather than
// guessed at.
func Load(root string) Config {
	cfg := Config{MaxFileMB: defaultMaxFileMB, DebtMarker: "debt:", CommitGate: "block"}
	raw, err := os.ReadFile(filepath.Join(root, ".procoder", "config.toml"))
	if err != nil {
		return cfg
	}
	section := ""
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = section + "." + strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		value = strings.Trim(value, `"`)
		switch key {
		case "git.default_branch_policy":
			cfg.BlockDefaultBranch = value == "block"
		case "lint.policy":
			cfg.LintBlock = value == "block"
		case "test.policy":
			cfg.TestBlock = value == "block"
		case "sprint.retro":
			cfg.SprintRetroOff = value == "off"
		case "release.files":
			cfg.ReleaseFiles = parseList(value)
		case "bench.threshold":
			cfg.BenchThreshold = atoiOr(value, 0)
		case "git.commit_gate":
			if value == "block" || value == "report" || value == "off" {
				cfg.CommitGate = value
			}
		case "ask.policy":
			cfg.AskBlock = value == "block"
		case "version.check":
			// Only the two documented values mean anything. A typo — "of",
			// or the spec's own "warn" misspelt — must not silently leave
			// the check in a state the writer did not choose.
			if value == "off" || value == "warn" {
				cfg.VersionCheckOff = value == "off"
			}
		case "docs.policy":
			cfg.DocsBlock = value == "block"
		case "ci.pin_actions_policy":
			cfg.PinActions = value == "block"
		case "maintain.gocyclo":
			cfg.Gocyclo = atoiOr(value, 0)
		case "maintain.funlen_lines":
			cfg.FunlenLines = atoiOr(value, 0)
		case "maintain.funlen_statements":
			cfg.FunlenStatements = atoiOr(value, 0)
		case "debt.marker":
			if value != "" {
				cfg.DebtMarker = value
			}
		case "git.max_file_mb":
			if n, err := strconv.Atoi(value); err == nil && n > 0 {
				cfg.MaxFileMB = n
			}
		}
	}
	return cfg
}

// parseList reads the one list shape the config uses: ["a", "b"]. The
// value arrives with outer quotes already trimmed only when it was a plain
// string, so lists are detected by their brackets before that trim bites.
func parseList(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil
	}
	var out []string
	for _, part := range strings.Split(value[1:len(value)-1], ",") {
		if p := strings.Trim(strings.TrimSpace(part), `"`); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func atoiOr(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
