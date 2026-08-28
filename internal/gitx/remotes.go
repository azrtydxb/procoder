package gitx

import "strings"

// Remotes maps remote name to URL, as git reports them.
//
// One git call rather than `git remote` followed by a get-url each: this
// runs on the identity path, which the daemon will ask for on every repo
// it serves.
//
// A directory that is not a repository, or one with no remotes, returns an
// empty map and never an error. Absence is the answer at that rung of the
// identity ladder, not a fault — treating it as one would make a fresh
// `git init` look broken.
func Remotes(root string) map[string]string {
	out, err := git(root, "config", "--get-regexp", `^remote\..*\.url$`)
	if err != nil || out == "" {
		return map[string]string{}
	}

	remotes := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		key, url, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || url == "" {
			continue
		}
		// remote.<name>.url, and <name> may itself contain dots — so the
		// name is everything between the FIRST and the LAST dot, never the
		// second field of a split.
		name := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")
		if name == "" || name == key {
			continue
		}
		remotes[name] = url
	}
	return remotes
}
