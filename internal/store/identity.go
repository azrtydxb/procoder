package store

import (
	"path/filepath"
	"sort"
	"strings"

	"procoder/internal/gitx"
)

// Identity is the repository key and the rung of the ladder that produced
// it.
//
// The rung is not decoration. A key that surprises somebody has to be
// traceable to the thing that decided it, or the only way to explain an
// unexpected identity is to guess.
type Identity struct {
	// Key is the repository's stable name.
	Key string
	// Rung is which answer won: "config", "origin", "remote" or "path".
	Rung string
	// Detail names the remote, for the "remote" rung. Empty otherwise.
	Detail string
}

// Source renders the rung for a person reading `procoder config`.
func (i Identity) Source() string {
	switch i.Rung {
	case "config":
		return "[service] repo in .procoder/config.toml"
	case "origin":
		return "origin remote"
	case "remote":
		return "first remote alphabetically: " + i.Detail
	default:
		return "no remote — repository root path"
	}
}

// IdentityFor resolves the repository key down a fixed ladder: the
// configured value, then the origin remote, then the first remote in
// alphabetical order, then the resolved absolute path of the root.
//
// origin beats an alphabetically earlier remote deliberately. Pure
// alphabetical order looks simpler and is wrong: a colleague who adds a
// personal remote named "fork" would key the same repository differently
// from everybody else, which defeats the one thing an identity is for.
//
// A rung that produces an EMPTY key does not answer. A remote URL that
// normalises to nothing — a malformed one, or a bare host with no path —
// would otherwise hand every such repository the same empty identity,
// which is the collision this exists to prevent.
//
// The path rung is last and is always available, so this never fails and
// never returns an empty key.
func IdentityFor(root, cfgRepo string) Identity {
	if s := strings.TrimSpace(cfgRepo); s != "" {
		return Identity{Key: s, Rung: "config"}
	}

	remotes := gitx.Remotes(root)
	if key := normalise(remotes["origin"]); key != "" {
		return Identity{Key: key, Rung: "origin"}
	}
	names := make([]string, 0, len(remotes))
	for name := range remotes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if key := normalise(remotes[name]); key != "" {
			return Identity{Key: key, Rung: "remote", Detail: name}
		}
	}

	// Resolved, not raw: two paths reaching one checkout through a symlink
	// are one repository and must not be two keys. Slash-separated, so the
	// key for a checkout is the same string on Windows as anywhere else.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolved = root
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		abs = resolved
	}
	return Identity{Key: filepath.ToSlash(abs), Rung: "path"}
}

// normalise reduces a remote URL to host/path, so the same repository keys
// the same however each person happens to have cloned it. It returns ""
// for anything it cannot reduce to both a host and a path, and the ladder
// treats that as "this rung did not answer".
//
// The host is lower-cased and the PATH IS NOT. Two repositories differing
// only in the case of their path are rare; two hosts differing only in
// case are the same host, always. Merging two repositories onto one key is
// the worse failure of the two, so the path keeps its case.
func normalise(url string) string {
	s := strings.TrimSpace(url)
	scheme := false
	if i := strings.Index(s, "://"); i >= 0 {
		s, scheme = s[i+3:], true
	}

	// Split the authority from the path. The scp-like form git@host:o/r
	// separates them with a colon and has no scheme; every other form uses
	// the first slash.
	var authority, rest string
	slash := strings.Index(s, "/")
	colon := strings.Index(s, ":")
	switch {
	case !scheme && colon >= 0 && (slash < 0 || colon < slash):
		authority, rest = s[:colon], s[colon+1:]
	case slash >= 0:
		authority, rest = s[:slash], s[slash+1:]
	default:
		authority = s
	}

	// A port is not part of the identity, and leaving it in makes
	// https://host:8443/o/r collide with https://host/8443/o/r — two
	// different repositories on one key.
	if i := strings.LastIndex(authority, ":"); i >= 0 && allDigits(authority[i+1:]) {
		authority = authority[:i]
	}
	// The user, and only within the authority. Searching the whole URL
	// throws the host away for any path containing an "@".
	if i := strings.LastIndex(authority, "@"); i >= 0 {
		authority = authority[i+1:]
	}

	host := strings.ToLower(authority)
	rest = strings.Trim(rest, "/")
	rest = strings.TrimSuffix(rest, ".git")
	rest = strings.Trim(rest, "/")
	if host == "" || rest == "" {
		return ""
	}
	return host + "/" + rest
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
