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
// The path rung is last and is always available, so this never fails and
// never returns an empty key — an empty identity is one every repository
// would share.
func IdentityFor(root, cfgRepo string) Identity {
	if s := strings.TrimSpace(cfgRepo); s != "" {
		return Identity{Key: s, Rung: "config"}
	}

	remotes := gitx.Remotes(root)
	if url, ok := remotes["origin"]; ok {
		return Identity{Key: normalise(url), Rung: "origin"}
	}
	if len(remotes) > 0 {
		names := make([]string, 0, len(remotes))
		for name := range remotes {
			names = append(names, name)
		}
		sort.Strings(names)
		return Identity{Key: normalise(remotes[names[0]]), Rung: "remote", Detail: names[0]}
	}

	// Resolved, not raw: two paths reaching one checkout through a symlink
	// are one repository and must not be two keys.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolved = root
	}
	return Identity{Key: resolved, Rung: "path"}
}

// normalise reduces a remote URL to host/path, so the same repository keys
// the same however each person happens to have cloned it.
//
// A string matching none of the known shapes comes back trimmed and
// otherwise untouched: a surprising key that can be traced to its remote is
// better than a confident wrong one.
func normalise(url string) string {
	s := strings.TrimSpace(url)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}

	// The scp-like form, git@host:owner/repo — the colon is the separator
	// the URL forms spell as a slash.
	host, rest := s, ""
	if i := strings.IndexAny(s, ":/"); i >= 0 {
		host, rest = s[:i], s[i+1:]
	}

	rest = strings.TrimSuffix(strings.TrimSuffix(strings.TrimRight(rest, "/"), ".git"), "/")
	host = strings.ToLower(host)
	if rest == "" {
		return host
	}
	return host + "/" + rest
}
