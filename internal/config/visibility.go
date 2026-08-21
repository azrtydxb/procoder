package config

import "fmt"

// A repository may weaken any default, and must not be able to do it
// quietly. The gate already refuses to let "nothing checked this" look
// like "this is clean"; a loosened setting is the same claim wearing a
// different hat — a green verdict that means the config was relaxed, not
// that the code was good.
//
// Strengthening prints nothing. A team raising its own bar does not need
// to be told, and a line for every tightened setting would train the
// reader to skim exactly the place the relaxations appear.

// Setting is one effective configuration value and where it came from.
type Setting struct {
	Key     string
	Value   string
	Source  string // "default", or "config.toml:<line>"
	Default string
	Relaxed bool   // weaker than the default
	Why     string // what the relaxation costs, in one clause
}

// Problem is a setting the file names that procoder could not use. It
// blocks: a config that silently reverts to defaults lets a team believe
// a setting is in force when it never was, which is the failure this
// whole feature would otherwise introduce.
type Problem struct {
	Line   int
	Text   string
	Reason string
}

func (p Problem) String() string {
	return fmt.Sprintf(".procoder/config.toml:%d — %s (%s)", p.Line, p.Reason, p.Text)
}

// relaxations describes, for each setting that CAN be weakened, what
// weaker means. A setting absent here cannot be relaxed — either because
// any value is a lateral move, or because the stricter direction is the
// only one the key offers.
var relaxations = map[string]func(value, def string) (bool, string){
	"git.commit_gate": func(v, _ string) (bool, string) {
		return v != "block", "commits with blocking findings are no longer stopped"
	},
	"version.check": func(v, _ string) (bool, string) {
		return v == "off", "an out-of-date procoder no longer says so"
	},
	"sprint.retro": func(v, _ string) (bool, string) {
		return v == "off", "a sprint can close without recording what it taught"
	},
	"git.max_file_mb": func(v, def string) (bool, string) {
		return atoiOr(v, 0) > atoiOr(def, 0), "larger files reach the repository"
	},
	"bench.threshold": func(v, def string) (bool, string) {
		return atoiOr(v, 0) > atoiOr(def, 0), "a bigger performance regression passes"
	},
	"security.sast_blocks_at": func(v, def string) (bool, string) {
		return severityRank(v) > severityRank(def), "SAST findings below this severity no longer block"
	},
}

// severityRank orders the severities semgrep reports. Higher means fewer
// findings block, so a higher rank than the default is a relaxation.
func severityRank(s string) int {
	switch s {
	case "INFO":
		return 0
	case "WARNING":
		return 1
	case "ERROR":
		return 2
	}
	return -1 // unknown: never treated as a relaxation, and reported elsewhere
}

// KnownSeverity reports whether s is a severity procoder can act on.
func KnownSeverity(s string) bool { return severityRank(s) >= 0 }
