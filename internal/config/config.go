// Package config reads .procoder/config.toml — the one place a repository
// tunes the harness (D-HOME). The parser is a deliberate subset of TOML:
// [sections], key = "string" | integer | true/false, and # comments. That is
// every shape the config uses, and a subset parser the tests fully cover beats
// a dependency the design forbids.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"procoder/internal/tools"
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
	// SastBlocksAt is the lowest semgrep severity that stops a commit.
	// ERROR by default: the level the tool itself reserves for findings it
	// is confident about.
	SastBlocksAt string
	// Settings is every effective value and where it came from — the data
	// behind `procoder config`. A reader of an unfamiliar repository has to
	// be able to ask which defaults are still in force.
	Settings []Setting
	// Tools is the repository's choice of tool per language, from [tools].
	// A name procoder does not ship is reported and dropped, so the default
	// still runs — a mistyped tool name must not leave code unchecked.
	Tools map[string]string
	// Problems are settings the file names that could not be used. They
	// block: a config that silently falls back lets a team believe a
	// setting is in force when it never was.
	Problems []Problem
}

// Defaults per the design contract.
const defaultMaxFileMB = 5

// defaultSastBlocksAt is the severity semgrep reserves for findings it is
// confident about. Lower it and more blocks; raise it and less does.
const defaultSastBlocksAt = "ERROR"

// Load reads .procoder/config.toml under root. A missing file is the normal
// case and returns defaults; an unreadable line is skipped rather than
// guessed at.
func Load(root string) Config {
	cfg := Config{MaxFileMB: defaultMaxFileMB, DebtMarker: "debt:", CommitGate: "block",
		SastBlocksAt: defaultSastBlocksAt}
	defaults := map[string]string{
		"git.commit_gate": "block",
		"git.max_file_mb": strconv.Itoa(defaultMaxFileMB),
		"version.check":   "warn",
		"sprint.retro":    "on",
		// 10, not 0: zero is the field's unset value and bench turns it
		// into 10 downstream. Calling 10 a relaxation from 0 would print a
		// warning at every repository that set the default explicitly, and
		// a false relaxation line teaches the reader to skim the real ones.
		"bench.threshold":         "10",
		"security.sast_blocks_at": defaultSastBlocksAt,
	}
	raw, err := os.ReadFile(filepath.Join(root, ".procoder", "config.toml"))
	if err != nil {
		cfg.Settings = defaultSettings(defaults)
		return cfg
	}
	seen := map[string]Setting{}
	section := ""
	for n, line := range strings.Split(string(raw), "\n") {
		lineNo := n + 1
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
			// A line that is neither a section nor an assignment was meant
			// to be something. Reporting it is the difference between a
			// typo the writer can see and a setting they think is on.
			cfg.Problems = append(cfg.Problems, Problem{Line: lineNo, Text: line,
				Reason: "not a section header or a key = value assignment"})
			continue
		}
		key = section + "." + strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		value = strings.Trim(value, `"`)

		// [tools] is keyed by language rather than by a fixed set of names,
		// so it is handled here rather than enumerated in the switch below.
		if section == "tools" {
			lang := strings.TrimPrefix(key, "tools.")
			switch {
			case !tools.IsLanguage(lang):
				cfg.Problems = append(cfg.Problems, Problem{Line: lineNo, Text: line,
					Reason: "procoder ships no alternative tools for " + lang})
			case tools.Alternatives[lang][value] == nil:
				cfg.Problems = append(cfg.Problems, Problem{Line: lineNo, Text: line,
					Reason: fmt.Sprintf("not a tool procoder ships for %s — it has %s",
						lang, strings.Join(tools.KnownFor(lang), ", "))})
			default:
				if cfg.Tools == nil {
					cfg.Tools = map[string]string{}
				}
				cfg.Tools[lang] = value
				seen[key] = Setting{Key: key, Value: value,
					Source: fmt.Sprintf(".procoder/config.toml:%d", lineNo)}
			}
			continue
		}

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
			} else {
				cfg.Problems = append(cfg.Problems, Problem{Line: lineNo, Text: line,
					Reason: "git.max_file_mb wants a positive whole number"})
			}
		case "security.sast_blocks_at":
			if KnownSeverity(value) {
				cfg.SastBlocksAt = value
			} else {
				cfg.Problems = append(cfg.Problems, Problem{Line: lineNo, Text: line,
					Reason: "not a severity semgrep reports (INFO, WARNING, ERROR)"})
			}
		default:
			// A key procoder does not know is a key that does nothing, and
			// a writer who mistypes `policy` believes their policy is set.
			cfg.Problems = append(cfg.Problems, Problem{Line: lineNo, Text: line,
				Reason: "no setting by this name — it has no effect"})
			continue
		}
		seen[key] = Setting{Key: key, Value: value,
			Source: fmt.Sprintf(".procoder/config.toml:%d", lineNo)}
	}
	cfg.Settings = mergeSettings(defaults, seen)
	return cfg
}

// defaultSettings is the effective set for a repository with no config.
func defaultSettings(defaults map[string]string) []Setting {
	return mergeSettings(defaults, nil)
}

// mergeSettings lists every setting that has a default, plus anything the
// file set, marking the ones whose value is weaker than the default.
func mergeSettings(defaults map[string]string, seen map[string]Setting) []Setting {
	keys := map[string]bool{}
	for k := range defaults {
		keys[k] = true
	}
	for k := range seen {
		keys[k] = true
	}
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)

	out := make([]Setting, 0, len(names))
	for _, k := range names {
		def := defaults[k]
		s, ok := seen[k]
		if !ok {
			s = Setting{Key: k, Value: def, Source: "default"}
		}
		s.Default = def
		if ok && def != "" {
			if weaker, why := isRelaxed(k, s.Value, def); weaker {
				s.Relaxed, s.Why = true, why
			}
		}
		out = append(out, s)
	}
	return out
}

func isRelaxed(key, value, def string) (bool, string) {
	f, ok := relaxations[key]
	if !ok {
		return false, ""
	}
	return f(value, def)
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
