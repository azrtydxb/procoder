// Package plugincache answers one question: which cached copies of this
// plugin are safe to remove, and how much would that reclaim.
//
// `claude plugin update` writes each version into its own directory under
// the cache and removes none of the previous ones. Nothing else does
// either — `claude plugin prune` covers auto-installed dependencies and
// reports "nothing to prune" against a cache full of superseded versions.
// On the maintainer's machine that reached 55 versions and 1.11 GB, one of
// them in use (#181).
//
// The risk is entirely one-sided. Reporting too little wastes disk;
// removing too much leaves somebody with no working install and no
// rollback. Everything here is shaped by that: the plan is computed
// separately from being carried out, the active version is protected
// twice, and an unreadable state is a refusal rather than a careful guess.
package plugincache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Keep is how many versions survive a sweep: the active one and one
// previous.
//
// One rollback target is the minimum that still counts as a rollback —
// repointing installed_plugins.json at the directory below is the only
// cheap recovery there is, and a window of one leaves none. More is
// affordable now that a version costs ~15MB rather than ~47MB (ADR 0004),
// but the window's job is recovery, not archival.
const Keep = 2

// Plan is what a sweep would do, computed without doing any of it.
//
// The report and the sweep both read this one value, so there is no second
// implementation to drift: `procoder prune` cannot name one set while
// `--apply` removes another.
type Plan struct {
	// Active is the version installed_plugins.json points at.
	Active string
	// Kept and Removable are directory names, newest first.
	Kept      []string
	Removable []string
	// Bytes is the summed size of Removable, measured by walking them.
	Bytes int64
	// Notes are things a person should read but which do not stop a
	// sweep — an unrecognised directory, a version that is not parseable.
	Notes []string
}

// Refusal is a state procoder will not act on. Not an error in the
// ordinary sense: it means the answer to "which version is in use" is
// unknown, and not knowing is never a licence to delete.
type Refusal struct{ Reason string }

func (r *Refusal) Error() string { return r.Reason }

// CacheDir is where the marketplace installer puts this plugin's versions.
func CacheDir(home string) string {
	return filepath.Join(home, ".claude", "plugins", "cache", "procoder", "procoder")
}

// registryPath is the installer's own record of what is installed.
// procoder reads it and never writes it — that file belongs to the
// installer, and rewriting it to make a sweep tidier would be procoder
// deciding something it was not asked to decide.
func registryPath(home string) string {
	return filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
}

// ActiveVersion reads which version is installed.
//
// Every failure here is a Refusal rather than a zero value. A missing
// file, a parse error, a plugin absent from the registry: each of them
// means the active version is unknown, and every one of them would
// otherwise leave the caller free to treat "unknown" as "none", which
// removes the version in use.
func ActiveVersion(home string) (string, error) {
	path := registryPath(home)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", &Refusal{Reason: fmt.Sprintf("%s could not be read (%v) — which version is in use is unknown, and that is not a licence to delete", path, err)}
	}
	var doc struct {
		Plugins map[string][]struct {
			Version     string `json:"version"`
			InstallPath string `json:"installPath"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", &Refusal{Reason: fmt.Sprintf("%s could not be parsed (%v) — refusing to sweep against a record procoder cannot read", path, err)}
	}
	for name, entries := range doc.Plugins {
		if !strings.HasPrefix(name, "procoder@") && name != "procoder" {
			continue
		}
		for _, e := range entries {
			if e.Version != "" {
				return e.Version, nil
			}
		}
	}
	return "", &Refusal{Reason: fmt.Sprintf("procoder is not listed in %s — refusing to sweep a cache whose active version is unknown", path)}
}

// Compute works out what could go. It reads and measures; it removes
// nothing, and cannot: that is Apply's job, and keeping them apart is what
// makes the report trustworthy.
//
// running is the directory the current binary is executing from, or "" if
// that cannot be determined. It is the SECOND protection on the version in
// use, independent of the registry: a binary can be running from a
// directory the registry no longer points at, and either check alone
// leaves a way to delete what is executing.
func Compute(home, running string) (*Plan, error) {
	active, err := ActiveVersion(home)
	if err != nil {
		return nil, err
	}
	dir := CacheDir(home)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Not an error. procoder may be installed from a release
			// binary rather than the marketplace, and a cache that was
			// never created is not a problem to report.
			return &Plan{Active: active}, nil
		}
		return nil, &Refusal{Reason: fmt.Sprintf("%s could not be listed (%v)", dir, err)}
	}

	plan := &Plan{Active: active}
	var versions []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if parseVersion(e.Name()) == nil {
			// A partial download, an editor's backup, something a later
			// installer invented. It cannot be ranked, so the window has
			// no opinion about it — and guessing what an unrecognised
			// directory is worth is exactly what this must not do.
			plan.Notes = append(plan.Notes, fmt.Sprintf("%s is not a version directory — kept, and never considered for removal", e.Name()))
			continue
		}
		versions = append(versions, e.Name())
	}
	sort.Slice(versions, func(i, j int) bool { return compareVersions(versions[i], versions[j]) > 0 })

	activePresent := false
	for _, v := range versions {
		if v == active {
			activePresent = true
		}
	}
	if !activePresent {
		// The state is not one this understands: the version the registry
		// names is not on disk. Removing the rest could leave nothing that
		// works at all.
		plan.Kept = versions
		plan.Notes = append(plan.Notes, fmt.Sprintf("the active version %s is not in %s — nothing will be removed until that is explained", active, dir))
		return plan, nil
	}

	// The window is anchored on the ACTIVE version, not on the newest.
	// Someone running an older version deliberately must not have the
	// newest kept and their own swept.
	kept := map[string]bool{active: true}
	for _, v := range versions {
		if len(kept) >= Keep {
			break
		}
		if compareVersions(v, active) < 0 {
			kept[v] = true
		}
	}
	for _, v := range versions {
		switch {
		case kept[v]:
			plan.Kept = append(plan.Kept, v)
		case running != "" && sameDir(filepath.Join(dir, v), running):
			// The second protection, and the reason it is separate: this
			// is true even when the registry says otherwise.
			plan.Kept = append(plan.Kept, v)
			plan.Notes = append(plan.Notes, fmt.Sprintf("%s is the directory this binary is running from — kept", v))
		default:
			plan.Removable = append(plan.Removable, v)
		}
	}
	for _, v := range plan.Removable {
		plan.Bytes += dirSize(filepath.Join(dir, v))
	}
	return plan, nil
}

// Apply removes what Compute planned and reports what actually went.
//
// The reclaimed figure is summed from directories that were removed, never
// from the plan: a sweep that removed nothing must not report a number
// that says otherwise. A directory that cannot be removed does not stop
// the others — it is named with its reason and left out of the total.
func Apply(home string, plan *Plan) (removed []string, reclaimed int64, failures []string) {
	dir := CacheDir(home)
	for _, v := range plan.Removable {
		target := filepath.Join(dir, v)
		size := dirSize(target)
		if err := os.RemoveAll(target); err != nil {
			failures = append(failures, fmt.Sprintf("%s NOT removed (%v)", v, err))
			continue
		}
		if _, err := os.Stat(target); err == nil {
			failures = append(failures, fmt.Sprintf("%s NOT removed (it is still there)", v))
			continue
		}
		removed = append(removed, v)
		reclaimed += size
	}
	return removed, reclaimed, failures
}

// sameDir compares two paths after resolving symlinks, so a cache reached
// through a link is still recognised as the directory in use. The prefix
// test catches the running binary sitting inside the version directory
// rather than being it.
func sameDir(a, b string) bool {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		ra = filepath.Clean(a)
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		rb = filepath.Clean(b)
	}
	return ra == rb || strings.HasPrefix(rb+string(filepath.Separator), ra+string(filepath.Separator))
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// parseVersion returns nil for anything that is not a dotted numeric
// version. Deliberately strict: a directory this cannot rank must not be
// ranked by a fallback that happens to sort.
func parseVersion(s string) []int {
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) < 2 {
		return nil
	}
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil
		}
		out = append(out, n)
	}
	return out
}

func compareVersions(a, b string) int {
	x, y := parseVersion(a), parseVersion(b)
	for i := 0; i < len(x) || i < len(y); i++ {
		var xi, yi int
		if i < len(x) {
			xi = x[i]
		}
		if i < len(y) {
			yi = y[i]
		}
		if xi != yi {
			if xi < yi {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Human renders a byte count the way a person reads one.
func Human(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
