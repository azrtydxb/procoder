package gate

import (
	"os"
	"path/filepath"
	"strings"
)

// Scope is how much of procoder a repository is subject to.
type Scope int

const (
	// Adopted: everything runs. This repository asked for procoder's
	// opinions, so it gets them.
	Adopted Scope = iota
	// Universal: only the checks that are true regardless of house style.
	// A repository that never adopted procoder is somebody else's, and
	// procoder's conventions are not theirs to answer for.
	Universal
)

// ScopeEnv lets a repository that cannot carry configuration force either
// mode — a fork somebody is about to submit upstream, where adding
// .procoder/ to the tree would itself be a change they do not want to make.
const ScopeEnv = "PROCODER_GATE_SCOPE"

// ScopeFor decides which of the two a repository gets, and says why.
//
// The evidence is the repository, never the environment. A contributor's
// machine looks identical whether they are in their own repository or
// somebody else's — same binary, same plugin, same shell — so only the
// repository can answer, and it answers with something it did
// deliberately: a .procoder/ directory, or an AGENTS.md that names
// procoder.
//
// Absence of evidence is not adoption. The failure direction here is
// saying less about somebody else's code, not more.
//
// Written once and used by every caller. Two callers deciding this
// separately would give `procoder check` and the pre-commit hook different
// answers in the same repository, and the one a person sees would not be
// the one that blocks them.
func ScopeFor(root string, cfgScope string) (Scope, string) {
	// The forced answers first, most local last: a repository's own
	// config beats the environment, because the file is the repository's
	// deliberate choice and the variable is whoever's shell this is.
	if s, ok := parseScope(cfgScope); ok {
		return s, "[gate] scope in .procoder/config.toml"
	}
	if s, ok := parseScope(os.Getenv(ScopeEnv)); ok {
		return s, ScopeEnv
	}
	if _, err := os.Stat(filepath.Join(root, ".procoder")); err == nil {
		return Adopted, ".procoder/ is here"
	}
	if agentsNamesProcoder(root) {
		return Adopted, "AGENTS.md names procoder"
	}
	return Universal, "no .procoder/ and no AGENTS.md naming procoder"
}

func parseScope(s string) (Scope, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "adopted":
		return Adopted, true
	case "universal":
		return Universal, true
	}
	// Universal, not Adopted, as the value that rides along with false.
	// Callers check the boolean, but a future one that forgets would treat
	// an invalid override as "run everything" — and this spec's stated
	// failure direction is saying LESS about somebody else's code, not
	// more. Raised in review on #187.
	return Universal, false
}

// agentsNamesProcoder reads the repository's AGENTS.md and reports whether
// procoder is named in it.
//
// Named, not merely present. An AGENTS.md is a file many projects keep for
// their own agents and their own reasons; its existence says nothing about
// procoder. An unreadable one is not evidence either — it is an absence,
// and absence is not adoption.
func agentsNamesProcoder(root string) bool {
	raw, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(raw)), "procoder")
}

// String is what the gate prints, so a reader can tell a quiet gate from a
// clean one.
func (s Scope) String() string {
	if s == Universal {
		return "universal"
	}
	return "adopted"
}
