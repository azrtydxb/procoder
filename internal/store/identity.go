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
	if s == "" {
		return ""
	}
	// A remote can be a path on this machine rather than a URL. Returning
	// "" for those made two clones of one /srv/git/repo.git key by their
	// own checkout paths — two identities for one repository, which is the
	// divergence identity exists to prevent.
	if p, ok := localRemote(s); ok {
		return p
	}
	// A RELATIVE path remote means something different from each clone's
	// own directory, so it cannot key two clones together. It does not
	// answer, rather than answering with a key built out of "..".
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return ""
	}

	scheme := false
	if i := strings.Index(s, "://"); i >= 0 {
		s, scheme = s[i+3:], true
	}
	// A query or a fragment is not part of the repository's name.
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}

	// Split the authority from the path. The scp-like form git@host:o/r
	// separates them with a colon and has no scheme; every other form uses
	// the first slash. A bracketed IPv6 literal is skipped before the scan,
	// or the colon inside it is read as the separator.
	scan := s
	offset := 0
	if strings.HasPrefix(s, "[") {
		if end := strings.Index(s, "]"); end >= 0 {
			scan, offset = s[end+1:], end+1
		}
	}
	var authority, rest string
	slash := indexAt(scan, "/", offset)
	colon := indexAt(scan, ":", offset)
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
	rest = trimPath(rest)
	if host == "" || rest == "" {
		return ""
	}
	return host + "/" + rest
}

// indexAt finds sep in s, returning an index into the ORIGINAL string that
// s was sliced from at offset.
func indexAt(s, sep string, offset int) int {
	if i := strings.Index(s, sep); i >= 0 {
		return i + offset
	}
	return -1
}

// trimPath reduces a repository path to its bare form: no leading or
// trailing slashes, no doubled ones, and no .git suffix in any case — the
// hosts that serve these are not case-sensitive about that suffix, and two
// keys for one repository is the failure being avoided.
func trimPath(p string) string {
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	p = strings.Trim(p, "/")
	if len(p) >= 4 && strings.EqualFold(p[len(p)-4:], ".git") {
		p = p[:len(p)-4]
	}
	return strings.Trim(p, "/")
}

// localRemote recognises a remote that names a path on this machine rather
// than a host: file:///srv/git/repo.git, /srv/git/repo.git, C:\\git\\repo.
//
// These are ordinary — a bare shared remote, a submodule, a mirror — and
// two clones of one of them are one repository. The key is the path of the
// REMOTE, which both clones agree on, not of either checkout.
//
// A RELATIVE path remote is deliberately not matched: it means something
// different from each clone's own directory, so it cannot key them
// together and pretending otherwise would be worse than falling through.
func localRemote(s string) (string, bool) {
	p := s
	switch {
	case strings.HasPrefix(s, "file://"):
		p = strings.TrimPrefix(s, "file://")
	case strings.HasPrefix(s, "/"):
	case windowsDrive(s):
	default:
		return "", false
	}
	p = trimPath(filepath.ToSlash(p))
	if p == "" {
		return "", false
	}
	return "/" + p, true
}

// windowsDrive reports whether s begins with a drive letter, as C:\\git\\repo
// does. A single-letter host is not a shape any real remote uses, so the
// ambiguity with the scp form costs nothing.
func windowsDrive(s string) bool {
	if len(s) < 3 || s[1] != ':' {
		return false
	}
	c := s[0]
	if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
		return false
	}
	return s[2] == '/' || s[2] == '\\'
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
