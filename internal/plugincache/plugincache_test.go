package plugincache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cache builds a fake ~/.claude with the given versions cached and one of
// them active. Each version directory gets a file of a known size, so a
// reclaimed figure can be checked against arithmetic rather than against
// whatever the code computed.
func cache(t *testing.T, active string, sizes map[string]int) string {
	t.Helper()
	home := t.TempDir()
	dir := CacheDir(home)
	for v, n := range sizes {
		if err := os.MkdirAll(filepath.Join(dir, v), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, v, "payload"), make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeRegistry(t, home, active)
	return home
}

func writeRegistry(t *testing.T, home, active string) {
	t.Helper()
	doc := map[string]any{
		"version": 2,
		"plugins": map[string]any{
			"procoder@procoder": []any{map[string]any{
				"version":     active,
				"installPath": filepath.Join(CacheDir(home), active),
			}},
		},
	}
	raw, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(t *testing.T, home, v string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(CacheDir(home), v))
	return err == nil
}

// S-2: computing a plan removes nothing. The report and the sweep are
// separate functions precisely so that typing the command to see what it
// does cannot cost you a gigabyte.
//
// proved by: an os.RemoveAll added to Compute's Removable loop (the
// directories vanish from a run that was only supposed to look).
func TestComputingAPlanRemovesNothing(t *testing.T) {
	home := cache(t, "3.1.0", map[string]int{"3.1.0": 10, "3.0.0": 10, "2.0.1": 10, "1.4.0": 10})
	plan, err := Compute(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Removable) == 0 {
		t.Fatal("the fixture proves nothing: no version was removable")
	}
	for _, v := range []string{"3.1.0", "3.0.0", "2.0.1", "1.4.0"} {
		if !exists(t, home, v) {
			t.Errorf("%s was removed by a function that only computes", v)
		}
	}
}

// S-2 and S-4: Apply removes exactly what the plan named, and the window
// leaves the active version and one previous.
//
// proved by: `Keep = 2` → `Keep = 1` (2.0.1 is swept, want it kept).
func TestApplyRemovesExactlyThePlanAndKeepsTheWindow(t *testing.T) {
	home := cache(t, "3.1.0", map[string]int{
		"3.1.0": 10, "3.0.0": 10, "2.0.1": 10, "1.4.0": 10, "1.3.0": 10,
	})
	plan, err := Compute(home, "")
	if err != nil {
		t.Fatal(err)
	}
	removed, _, failures := Apply(home, plan)
	if len(failures) != 0 {
		t.Fatalf("unexpected failures: %v", failures)
	}
	if got, want := strings.Join(removed, ","), "2.0.1,1.4.0,1.3.0"; got != want {
		t.Errorf("removed %q, want %q", got, want)
	}
	for _, v := range []string{"3.1.0", "3.0.0"} {
		if !exists(t, home, v) {
			t.Errorf("%s was inside the retention window and is gone", v)
		}
	}
	for _, v := range []string{"2.0.1", "1.4.0", "1.3.0"} {
		if exists(t, home, v) {
			t.Errorf("%s was named removable and survived", v)
		}
	}
}

// S-3, first protection: the version the registry names is never removed,
// even when the window would otherwise drop it. Someone deliberately
// running an older version must not have it swept from under them.
//
// proved by: `kept := map[string]bool{active: true}` → `kept :=
// map[string]bool{}` (the active version is swept).
func TestTheActiveVersionSurvivesEvenWhenOld(t *testing.T) {
	home := cache(t, "1.3.0", map[string]int{
		"3.1.0": 10, "3.0.0": 10, "2.0.1": 10, "1.3.0": 10,
	})
	plan, err := Compute(home, "")
	if err != nil {
		t.Fatal(err)
	}
	Apply(home, plan)
	if !exists(t, home, "1.3.0") {
		t.Fatal("the version in use was deleted")
	}
}

// S-3, second protection, asserted SEPARATELY so that one check passing
// cannot hide the other being absent. A binary can be running from a
// directory the registry no longer points at.
//
// proved by: the `running != "" && sameDir(...)` case deleted from
// Compute's switch (the executing directory is swept, and the registry
// check does not save it because the registry names a different version).
func TestTheRunningDirectorySurvivesTheRegistrySayingOtherwise(t *testing.T) {
	home := cache(t, "3.1.0", map[string]int{
		"3.1.0": 10, "3.0.0": 10, "2.0.1": 10, "1.3.0": 10,
	})
	running := filepath.Join(CacheDir(home), "1.3.0")
	plan, err := Compute(home, running)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range plan.Removable {
		if v == "1.3.0" {
			t.Fatal("the directory this binary runs from was marked removable")
		}
	}
	Apply(home, plan)
	if !exists(t, home, "1.3.0") {
		t.Fatal("the directory this binary runs from was deleted")
	}
}

// S-5: an unreadable record means the active version is unknown, and
// unknown is never a licence to delete. Both shapes of unreadable, because
// "absent" and "corrupt" reach the failure by different paths.
//
// proved by: ActiveVersion's read-error branch made to `return "", nil`
// (Compute proceeds with an empty active version and sweeps everything).
func TestAnUnreadableRegistryRefusesAndRemovesNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		break_ func(t *testing.T, home string)
	}{
		{"absent", func(t *testing.T, home string) {
			if err := os.Remove(filepath.Join(home, ".claude", "plugins", "installed_plugins.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"unparseable", func(t *testing.T, home string) {
			p := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
			if err := os.WriteFile(p, []byte("{not json at all"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"procoder not listed", func(t *testing.T, home string) {
			p := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
			if err := os.WriteFile(p, []byte(`{"version":2,"plugins":{"other@other":[{"version":"1.0.0"}]}}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := cache(t, "3.1.0", map[string]int{"3.1.0": 10, "3.0.0": 10, "2.0.1": 10})
			tc.break_(t, home)
			if _, err := Compute(home, ""); err == nil {
				t.Fatal("procoder computed a sweep against a record it could not read")
			}
			for _, v := range []string{"3.1.0", "3.0.0", "2.0.1"} {
				if !exists(t, home, v) {
					t.Errorf("%s was removed despite the refusal", v)
				}
			}
		})
	}
}

// S-6: the reclaimed total is what actually went, checked against
// arithmetic on a fixture of known sizes rather than against the figure
// the code produced.
//
// proved by: `reclaimed += size` moved before the RemoveAll error check
// (a directory that failed to go still counts toward the total).
func TestTheReclaimedFigureIsWhatActuallyWent(t *testing.T) {
	const kb = 1024
	home := cache(t, "3.1.0", map[string]int{
		"3.1.0": 5 * kb, "3.0.0": 5 * kb, "2.0.1": 3 * kb, "1.4.0": 7 * kb,
	})
	plan, err := Compute(home, "")
	if err != nil {
		t.Fatal(err)
	}
	removed, reclaimed, _ := Apply(home, plan)
	if len(removed) != 2 {
		t.Fatalf("removed %v, want 2.0.1 and 1.4.0", removed)
	}
	// 3 KB + 7 KB, and nothing from the two that were kept.
	if want := int64(10 * kb); reclaimed != want {
		t.Errorf("reclaimed %d bytes, want %d", reclaimed, want)
	}
}

// S-1: no cache directory is not an error. procoder may be installed from
// a release binary rather than the marketplace, and a cache that was never
// created is not a problem to report.
//
// proved by: the os.IsNotExist branch in Compute made to return a Refusal
// (a normal install reads as broken).
func TestAMissingCacheDirectoryIsNotAnError(t *testing.T) {
	home := t.TempDir()
	writeRegistry(t, home, "3.1.0")
	plan, err := Compute(home, "")
	if err != nil {
		t.Fatalf("a missing cache directory was treated as an error: %v", err)
	}
	if len(plan.Removable) != 0 {
		t.Errorf("something was removable from a cache that does not exist: %v", plan.Removable)
	}
}

// An unrecognised directory cannot be ranked, so the window has no opinion
// about it. It is kept and named — guessing what it is worth is exactly
// what a delete path must not do.
//
// proved by: `parseVersion(e.Name()) == nil` branch removed, so the
// directory falls through into the version list and gets swept.
func TestAnUnrecognisedDirectoryIsKeptAndNamed(t *testing.T) {
	home := cache(t, "3.1.0", map[string]int{
		"3.1.0": 10, "3.0.0": 10, "2.0.1": 10, "tmp-download": 10, "backup~": 10,
	})
	plan, err := Compute(home, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range plan.Removable {
		if v == "tmp-download" || v == "backup~" {
			t.Fatalf("%s is not a version and was marked removable", v)
		}
	}
	Apply(home, plan)
	for _, v := range []string{"tmp-download", "backup~"} {
		if !exists(t, home, v) {
			t.Errorf("%s was deleted despite not being a version", v)
		}
	}
	joined := strings.Join(plan.Notes, "\n")
	for _, v := range []string{"tmp-download", "backup~"} {
		if !strings.Contains(joined, v) {
			t.Errorf("%s was kept silently — a person cannot audit what they are not told: %q", v, joined)
		}
	}
}

// The active version named by the registry is not on disk: a state this
// does not understand. Removing the rest could leave nothing that works.
//
// proved by: the `!activePresent` early return deleted (the sweep proceeds
// and empties the cache).
func TestAnAbsentActiveVersionStopsTheSweep(t *testing.T) {
	home := cache(t, "9.9.9", map[string]int{"3.1.0": 10, "3.0.0": 10, "2.0.1": 10})
	plan, err := Compute(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Removable) != 0 {
		t.Fatalf("a sweep was planned while the active version was missing: %v", plan.Removable)
	}
	if !strings.Contains(strings.Join(plan.Notes, "\n"), "9.9.9") {
		t.Errorf("the reason was not stated: %v", plan.Notes)
	}
}

// Only the active version cached: nothing to do, and it must not read as a
// failure.
//
// proved by: covered by the Removable check — an implementation that swept
// the sole version would fail here before any report is rendered.
func TestASoleCachedVersionIsLeftAlone(t *testing.T) {
	home := cache(t, "3.1.0", map[string]int{"3.1.0": 10})
	plan, err := Compute(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Removable) != 0 {
		t.Fatalf("the only cached version was marked removable: %v", plan.Removable)
	}
	if !exists(t, home, "3.1.0") {
		t.Fatal("the only cached version is gone")
	}
}
