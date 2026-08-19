package backlog

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

func writeItem(t *testing.T, root, kind, id, body string) string {
	t.Helper()
	dir := filepath.Join(root, Dir, kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, id+".md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCreationPrintsAndRefusesCollisions(t *testing.T) {
	root := t.TempDir()
	out, lines := collect()
	if code := Milestone(root, "MVP Launch", out); code != 0 {
		t.Fatalf("milestone: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "milestones/mvp-launch.md") || !strings.Contains(joined, "Status: open") {
		t.Fatalf("must print path and template: %v", *lines)
	}
	// the binary printed but wrote nothing — P-CONTROL
	if _, err := os.Stat(filepath.Join(root, Dir)); !os.IsNotExist(err) {
		t.Fatal("creation must not write files")
	}
	// a written file with the same slug refuses the second creation
	writeItem(t, root, KindMilestone, "mvp-launch", "# MVP Launch\n\nStatus: open\n")
	out2, lines2 := collect()
	if code := Milestone(root, "MVP launch", out2); code != 2 {
		t.Fatalf("slug collision must refuse: exit %d %v", code, *lines2)
	}
}

func TestStoryRequiresAnEpic(t *testing.T) {
	out, lines := collect()
	if code := Story(t.TempDir(), "login form", "", out); code != 2 {
		t.Fatalf("story without epic must refuse: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "--epic") {
		t.Fatalf("refusal must name the flag: %v", *lines)
	}
}

func TestLoadAllParsesEveryKindAndFlagsUnreadable(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindMilestone, "v1", "# V1\n\nStatus: open\n")
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\nMilestone: v1\nSpec: auth @ abc123def456\n")
	writeItem(t, root, KindStory, "20260819-login", "# Login\n\nStatus: done 2026-08-19\nEpic: auth\nSprint: 001-mvp\n")
	writeItem(t, root, KindSprint, "001-mvp", "# mvp\n\nStatus: active\n")
	items, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("want 4 items, got %d: %+v", len(items), items)
	}
	byID := map[string]Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	e := byID["auth"]
	if e.Milestone != "v1" || e.SpecName != "auth" || e.SpecPrint != "abc123def456" {
		t.Fatalf("epic fields wrong: %+v", e)
	}
	s := byID["20260819-login"]
	if s.Epic != "auth" || s.Sprint != "001-mvp" || !s.Done() {
		t.Fatalf("story fields wrong: %+v", s)
	}
	if byID["001-mvp"].Status != "active" || byID["001-mvp"].Done() {
		t.Fatalf("sprint status wrong: %+v", byID["001-mvp"])
	}

	if runtime.GOOS != "windows" {
		p := writeItem(t, root, KindStory, "broken", "# broken\n")
		os.Chmod(p, 0o000)
		defer os.Chmod(p, 0o644)
		items, err = LoadAll(root)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, it := range items {
			if it.ID == "broken" && it.Status == "unreadable" {
				found = true
			}
		}
		if !found {
			t.Fatal("an unreadable file must surface as unreadable, never vanish")
		}
	}
}

func TestLoadAllEmptyBacklog(t *testing.T) {
	items, err := LoadAll(t.TempDir())
	if err != nil || items != nil {
		t.Fatalf("missing backlog dir must be empty, not an error: %v %v", items, err)
	}
}

func TestItemFileGuardsTraversal(t *testing.T) {
	for _, id := range []string{"", "a/b", "..", "../x"} {
		if _, err := ItemFile(t.TempDir(), KindStory, id); err == nil {
			t.Fatalf("id %q must be refused", id)
		}
	}
}

// Story ids are born from whole acceptance criteria; uncapped they blow
// past Windows' 260-character path limit on CI checkouts.
func TestSlugifyCapsLongTitles(t *testing.T) {
	long := strings.Repeat("very long words about acceptance ", 8)
	slug := slugify(long)
	if len(slug) > 60 {
		t.Fatalf("slug must be capped at 60 chars, got %d: %q", len(slug), slug)
	}
	if strings.HasSuffix(slug, "-") || !strings.Contains(slug, "very-long") {
		t.Fatalf("cap must cut at a word boundary and keep the head: %q", slug)
	}
	if slugify("short title") != "short-title" {
		t.Fatal("short slugs must pass through untouched")
	}
}
