package docs

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMD(t *testing.T, body string) (string, string) {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "page.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, p
}

// The defect: a paragraph opens a span and never closes it, so Markdown
// renders a literal backtick and the rest of the sentence loses its
// formatting. A bot reviewer caught one of these in a pull request whose
// rubric names the case in words and whose fresh-context review applied
// that rubric — the eye reads the intent and skips the missing character.
// proved by: dropped the %2 test — the unclosed paragraph then reports
// nothing and the check exists for decoration.
func TestAnUnclosedSpanIsReported(t *testing.T) {
	root, p := writeMD(t, "# page\n\nthe finding is `delete: speculative\nplaceholder and nothing closes it.\n")
	got := UnclosedSpans(root, p)
	if len(got) != 1 {
		t.Fatalf("one unclosed span, got %d: %+v", len(got), got)
	}
	if got[0].Line != 3 {
		t.Errorf("the finding points at the line that opened it, got %d", got[0].Line)
	}
	if got[0].Blocking {
		t.Error("a deliberate literal backtick is legitimate — this informs, it does not block")
	}
}

// The false positive that would have made the check worthless: CommonMark
// lets a span wrap, and a wrapped name is one span rendered with a space.
// Counting per line flagged 46 of these in this repository, every one of
// them correct — which is how a check earns its way into being ignored.
// proved by: counted per line instead of per paragraph — every wrapped
// span in the tree becomes a finding.
func TestAWrappedSpanIsNotUnclosed(t *testing.T) {
	root, p := writeMD(t, "# page\n\nrun `procoder\ntemplates` to print the defaults.\n\nand `one` more.\n")
	if got := UnclosedSpans(root, p); len(got) != 0 {
		t.Errorf("a span may wrap: %+v", got)
	}
}

// Fenced code is not prose, and a fence boundary ends the paragraph before
// it — otherwise a shell snippet full of backticks reports the page.
func TestFencedCodeIsNotProse(t *testing.T) {
	root, p := writeMD(t, "# page\n\n```sh\necho `date`\nunbalanced ` here\n```\n\nplain `text` after.\n")
	if got := UnclosedSpans(root, p); len(got) != 0 {
		t.Errorf("a fence is not a span: %+v", got)
	}
}

// Doubled backticks are a span that contains a backtick, not two spans.
func TestDoubledBackticksAreOneSpan(t *testing.T) {
	root, p := writeMD(t, "# page\n\nwrite ``a `b` c`` when the name has one.\n")
	if got := UnclosedSpans(root, p); len(got) != 0 {
		t.Errorf("doubled backticks close themselves: %+v", got)
	}
}
