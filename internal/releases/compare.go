// Package releases answers one question — is there a newer procoder than the
// one running — and, when the user says yes, installs it. Everything here is
// stdlib: the check runs on every session start, and a dependency that can
// fail to resolve is a dependency that can stop a session from starting.
package releases

import (
	"regexp"
	"strconv"
)

// Dev is the version a build carries when no release stamped it. Nothing is
// newer than an unknown version and nothing is older: comparing against it
// would tell a maintainer working on an unreleased branch to downgrade.
const Dev = "dev"

// semverRe accepts the shape procoder tags: N.N.N, with an optional leading
// v and an optional pre-release or build suffix that plays no part in the
// comparison. Anything else is not a version this can reason about.
var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$`)

// Version is a parsed release number. Parse reports whether the string was
// one at all — "unknown" is a third answer, never a zero version.
type Version struct{ Major, Minor, Patch int }

// Parse reads a version string. ok is false for "dev", an empty string, or
// anything not shaped like N.N.N — the caller must not treat those as 0.0.0,
// which would make every real release look newer.
func Parse(s string) (Version, bool) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return Version{}, false
	}
	// The regexp guarantees each group is digits, so the errors cannot fire.
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return Version{major, minor, patch}, true
}

// Compare answers how latest relates to current: 1 when latest is newer, 0
// when they are equal or either is unparseable, -1 when current is ahead.
// Unparseable compares as equal on purpose — an unknown version is not a
// reason to offer an upgrade, and never a reason to claim a downgrade.
func Compare(current, latest string) int {
	c, okC := Parse(current)
	l, okL := Parse(latest)
	if !okC || !okL {
		return 0
	}
	for _, pair := range [][2]int{{l.Major, c.Major}, {l.Minor, c.Minor}, {l.Patch, c.Patch}} {
		switch {
		case pair[0] > pair[1]:
			return 1
		case pair[0] < pair[1]:
			return -1
		}
	}
	return 0
}

// ShouldWarn reports whether the user is running behind. Every newer
// release earns the warning — patch, minor and major alike (D-1): a major
// is exactly the upgrade whose behaviour changes, and hiding it to keep the
// output quiet would hide the one that matters most.
func ShouldWarn(current, latest string) bool {
	return Compare(current, latest) == 1
}
