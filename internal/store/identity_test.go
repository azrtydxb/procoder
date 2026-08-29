package store

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRepo(t *testing.T, remotes map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	args := [][]string{{"init", "-q", "-b", "main"}}
	for name, url := range remotes {
		args = append(args, []string{"remote", "add", name, url})
	}
	for _, a := range args {
		if out, err := exec.Command("git", append([]string{"-C", dir}, a...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", a, err, out)
		}
	}
	return dir
}

// proved by: skipping the host lower-casing, or keeping the .git suffix,
// splits one repository into two keys and this fails.
func TestIdentityNormalisation(t *testing.T) {
	for _, url := range []string{
		"git@host:o/r.git",
		"https://host/o/r.git",
		"ssh://git@host/o/r",
		"https://HOST/o/r/",
		"https://host/o/r",
	} {
		if got := normalise(url); got != "host/o/r" {
			t.Errorf("normalise(%q) = %q, want host/o/r", url, got)
		}
	}
}

// proved by: letting the alphabetically first remote win outright makes the
// origin-plus-fork case return the fork, which is the divergence this ladder
// exists to prevent — two people with different extra remotes keying the same
// repository differently.
func TestIdentityLadder(t *testing.T) {
	t.Run("config wins", func(t *testing.T) {
		root := gitRepo(t, map[string]string{"origin": "https://host/o/r.git"})
		got := IdentityFor(root, "acme/widgets")
		if got.Key != "acme/widgets" || got.Rung != "config" {
			t.Fatalf("got %+v, want key acme/widgets rung config", got)
		}
	})
	t.Run("origin beats an earlier name", func(t *testing.T) {
		root := gitRepo(t, map[string]string{
			"origin": "https://host/o/r.git",
			"fork":   "https://host/me/r.git",
		})
		got := IdentityFor(root, "")
		if got.Key != "host/o/r" || got.Rung != "origin" {
			t.Fatalf("got %+v, want key host/o/r rung origin", got)
		}
	})
	t.Run("first alphabetically when there is no origin", func(t *testing.T) {
		root := gitRepo(t, map[string]string{
			"upstream": "https://host/o/r.git",
			"fork":     "https://host/me/r.git",
		})
		got := IdentityFor(root, "")
		if got.Key != "host/me/r" || got.Rung != "remote" || got.Detail != "fork" {
			t.Fatalf("got %+v, want key host/me/r rung remote detail fork", got)
		}
	})
	t.Run("path when there is no remote", func(t *testing.T) {
		root := gitRepo(t, nil)
		want, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		got := IdentityFor(root, "")
		if got.Key != want || got.Rung != "path" {
			t.Fatalf("got %+v, want key %s rung path", got, want)
		}
	})
}

// proved by: testing for the empty string rather than trimming leaves the key
// empty, and an empty identity is one every repository shares.
func TestIdentityBlankConfigKeyIgnored(t *testing.T) {
	root := gitRepo(t, map[string]string{"origin": "https://host/o/r.git"})
	got := IdentityFor(root, "   ")
	if got.Rung != "origin" || got.Key != "host/o/r" {
		t.Fatalf("got %+v, want the origin rung", got)
	}
}

// proved by: falling back to the raw argument rather than the resolved path
// gives two keys for one checkout reached through a symlink.
func TestIdentityWithoutGit(t *testing.T) {
	root := t.TempDir()
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	got := IdentityFor(root, "")
	if got.Key != want || got.Rung != "path" {
		t.Fatalf("got %+v, want key %s rung path", got, want)
	}
}

// proved by: a Source that names the rung without the remote's name leaves a
// surprising identity untraceable to the remote that produced it.
func TestIdentitySource(t *testing.T) {
	for _, tc := range []struct {
		id   Identity
		want string
	}{
		{Identity{Rung: "config"}, "[service] repo in .procoder/config.toml"},
		{Identity{Rung: "origin"}, "origin remote"},
		{Identity{Rung: "remote", Detail: "fork"}, "first remote alphabetically: fork"},
		{Identity{Rung: "path"}, "no remote — repository root path"},
	} {
		if got := tc.id.Source(); got != tc.want {
			t.Errorf("Source() for %+v = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// proved by: leaving a port in the authority makes https://host:8443/o/r
// and https://host/8443/o/r one key — two repositories with one identity,
// which is the collision an identity exists to prevent.
func TestIdentityNormalisationIsAdversarial(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"https://host:8443/o/r", "host/o/r"},
		{"https://host/8443/o/r", "host/8443/o/r"},
		{"ssh://git@host:2222/o/r.git", "host/o/r"},
		{"https://host/2222/o/r", "host/2222/o/r"},
		// an "@" in the PATH must not throw the host away
		{"https://host/o/r@2", "host/o/r@2"},
		// nothing reducible to a host and a path is not an identity
		{"", ""},
		{"https://", ""},
		{"git@host", ""},
		{"   ", ""},
	} {
		if got := normalise(tc.url); got != tc.want {
			t.Errorf("normalise(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

// proved by: taking a rung's answer without checking it is non-empty gives
// every repository with a malformed remote the same empty identity.
func TestIdentitySkipsARungThatCannotAnswer(t *testing.T) {
	root := gitRepo(t, map[string]string{"origin": "git@host"})
	got := IdentityFor(root, "")
	if got.Rung != "path" || got.Key == "" {
		t.Fatalf("got %+v, want the path rung with a non-empty key", got)
	}
}

// proved by: returning "" for a path-shaped remote makes two clones of one
// /srv/git/repo.git key by their own checkout paths — two identities for
// one repository, which is exactly the divergence identity prevents. That
// was a regression this branch introduced and review caught.
func TestIdentityHandlesLocalRemotes(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"/srv/git/repo.git", "/srv/git/repo"},
		{"file:///srv/git/repo.git", "/srv/git/repo"},
		{"file:///srv/git/repo", "/srv/git/repo"},
		{"/srv/git/repo/", "/srv/git/repo"},
		// relative remotes mean something different from each clone, so
		// they deliberately do not answer
		{"../sibling.git", ""},
	} {
		if got := normalise(tc.url); got != tc.want {
			t.Errorf("normalise(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

// proved by: two clones of one bare remote must agree, which is the whole
// point — and the rung must say the remote answered, not that there was none.
func TestTwoClonesOfALocalRemoteAgree(t *testing.T) {
	a := gitRepo(t, map[string]string{"origin": "/srv/git/repo.git"})
	b := gitRepo(t, map[string]string{"origin": "/srv/git/repo.git"})
	ida, idb := IdentityFor(a, ""), IdentityFor(b, "")
	if ida.Key != idb.Key {
		t.Fatalf("two clones of one remote disagree: %q vs %q", ida.Key, idb.Key)
	}
	if ida.Rung != "origin" {
		t.Fatalf("rung = %q, want origin — there plainly is a remote", ida.Rung)
	}
}

// proved by: scanning for the scp separator without skipping the brackets
// lands inside an IPv6 literal and returns garbage.
func TestIdentityNormalisationEdges(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"[::1]:o/r", "[::1]/o/r"},
		{"https://[::1]:8443/o/r", "[::1]/o/r"},
		{"https://host//o//r", "host/o/r"},
		{"https://host/o/r.GIT", "host/o/r"},
		{"https://host/o/r?x=1", "host/o/r"},
		{"https://host/o/r#frag", "host/o/r"},
	} {
		if got := normalise(tc.url); got != tc.want {
			t.Errorf("normalise(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}
