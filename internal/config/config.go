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
	// BlockDefaultBranch: work on the default branch blocks the gate instead
	// of being reported. Off by default — solo repositories commit to main
	// routinely, and teams usually enforce this server-side anyway.
	BlockDefaultBranch bool
	// MaxFileMB is the oversized-file threshold for the gate.
	MaxFileMB int
	// LintBlock: lint findings block the gate instead of being reported.
	LintBlock bool
	// PinActions: unpinned GitHub Action refs block instead of being reported.
	PinActions bool
	// Maintain thresholds for the complexity report; zero means the default.
	Gocyclo          int
	FunlenLines      int
	FunlenStatements int
	// DebtMarker is the comment marker `procoder debt` harvests.
	DebtMarker string
}

// Defaults per the design contract.
const defaultMaxFileMB = 5

// Load reads .procoder/config.toml under root. A missing file is the normal
// case and returns defaults; an unreadable line is skipped rather than
// guessed at.
func Load(root string) Config {
	cfg := Config{MaxFileMB: defaultMaxFileMB, DebtMarker: "debt:"}
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

func atoiOr(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
