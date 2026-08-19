package textutil

import "strings"

import "testing"

func TestFirstLineSkipsBlanksTrimsAndCaps(t *testing.T) {
	if got := FirstLine("\n\n  hello  \nsecond\n"); got != "hello" {
		t.Fatalf("want the first line with content, got %q", got)
	}
	if got := FirstLine("   \n\t\n"); got != "no output" {
		t.Fatalf("nothing to report must say so, got %q", got)
	}
	long := strings.Repeat("x", 300)
	if got := FirstLine(long); len(got) != maxLine {
		t.Fatalf("want a %d-char cap, got %d", maxLine, len(got))
	}
}

func TestSlugCutsAtAWordBoundary(t *testing.T) {
	if got := Slug("Hello, World!"); got != "hello-world" {
		t.Fatalf("want hello-world, got %q", got)
	}
	if got := Slug("!!!"); got != "" {
		t.Fatalf("a title with nothing usable slugs to empty, got %q", got)
	}
	long := Slug(strings.Repeat("very long words about acceptance ", 8))
	if len(long) > maxSlug {
		t.Fatalf("slug must be capped at %d, got %d: %q", maxSlug, len(long), long)
	}
	if strings.HasSuffix(long, "-") || !strings.HasPrefix(long, "very-long") {
		t.Fatalf("the cut must land on a word boundary and keep the head: %q", long)
	}
}

func TestSectionStopsAtTheNextHeading(t *testing.T) {
	doc := "# T\n\n## One\n\nfirst body\n\n## Two\n\nsecond body\n"
	if got := strings.TrimSpace(Section(doc, "One")); got != "first body" {
		t.Fatalf("want the first section only, got %q", got)
	}
	if got := Section(doc, "Missing"); got != "" {
		t.Fatalf("an absent section is empty, got %q", got)
	}
}

func TestStripCommentsRemovesGuidanceIncludingUnterminated(t *testing.T) {
	if got := strings.TrimSpace(StripComments("a <!-- hint --> b")); got != "a  b" {
		t.Fatalf("want the comment gone, got %q", got)
	}
	if got := StripComments("kept <!-- never closed"); got != "kept " {
		t.Fatalf("an unterminated comment truncates rather than returning everything: %q", got)
	}
}
