package releases

import "testing"

// The ordering itself, in the shapes procoder tags. proved by: compared
// patch before major — 1.0.0 → 0.9.9 then reads as an upgrade.
func TestCompareOrdersMajorThenMinorThenPatch(t *testing.T) {
	cases := []struct {
		current, latest string
		want            int
	}{
		{"0.28.0", "0.28.1", 1},
		{"0.28.1", "0.28.1", 0},
		{"0.28.1", "0.29.0", 1},
		{"0.29.0", "0.28.1", -1},
		{"0.9.9", "1.0.0", 1},
		{"1.0.0", "0.9.9", -1},
		{"1.0.0", "v1.0.1", 1},
		{"v1.0.1", "1.0.1", 0},
		{"1.2.3", "1.2.3-rc1", 0},
		{"0.28.1", "0.28.10", 1}, // ten is not one, and not a string compare
	}
	for _, c := range cases {
		if got := Compare(c.current, c.latest); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.current, c.latest, got, c.want)
		}
	}
}

// An unknown version is a third answer. A build with no stamp, an empty
// string, or GitHub answering with something that is not a version must not
// read as 0.0.0 — that would make every release look like an upgrade and
// would tell a maintainer on an unreleased branch to install backwards.
// proved by: returned Version{} with ok true for "dev" — the dev build then
// reports that 0.0.1 is newer than what it is running.
func TestUnknownVersionsCompareAsEqualAndNeverWarn(t *testing.T) {
	for _, c := range [][2]string{
		{Dev, "1.2.3"},
		{"1.2.3", Dev},
		{"", "1.2.3"},
		{"1.2.3", ""},
		{"banana", "1.2.3"},
		{"1.2", "1.2.3"},
		{"1.2.3.4", "1.2.5"},
	} {
		if got := Compare(c[0], c[1]); got != 0 {
			t.Errorf("Compare(%q, %q) = %d, want 0 — unknown is not older or newer", c[0], c[1], got)
		}
		if ShouldWarn(c[0], c[1]) {
			t.Errorf("ShouldWarn(%q, %q) must be false — nothing is known to be newer", c[0], c[1])
		}
	}
}

// D-1: every newer release earns the warning, majors included.
func TestShouldWarnOnEveryNewerRelease(t *testing.T) {
	for _, c := range [][2]string{{"1.2.3", "1.2.4"}, {"1.2.3", "1.3.0"}, {"1.2.3", "2.0.0"}} {
		if !ShouldWarn(c[0], c[1]) {
			t.Errorf("ShouldWarn(%q, %q) must warn — a newer release is a newer release", c[0], c[1])
		}
	}
	for _, c := range [][2]string{{"1.2.3", "1.2.3"}, {"1.2.4", "1.2.3"}, {"2.0.0", "1.9.9"}} {
		if ShouldWarn(c[0], c[1]) {
			t.Errorf("ShouldWarn(%q, %q) must stay quiet", c[0], c[1])
		}
	}
}

// Parse says whether it understood, and the caller depends on that half.
func TestParseReportsWhetherItUnderstood(t *testing.T) {
	if v, ok := Parse("v0.28.1"); !ok || v != (Version{0, 28, 1}) {
		t.Errorf("Parse(v0.28.1) = %+v, %v", v, ok)
	}
	if _, ok := Parse(Dev); ok {
		t.Error("a dev build carries no version to compare")
	}
}
