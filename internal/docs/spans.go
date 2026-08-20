package docs

import (
	"os"
	"strings"

	"procoder/internal/gitx"
)

// UnclosedSpans finds inline code spans a paragraph opens and never closes.
//
// Markdown renders an unmatched backtick as a literal backtick, so the text
// after it loses its formatting silently and the reader sees prose where a
// name was meant. It escaped a rubric that names the case in words ("code
// spans unbroken") and a fresh-context review applying that rubric: the eye
// reads the intent and skips the missing character. Counting is the one
// thing a machine does better here.
//
// The unit is the PARAGRAPH, not the line: CommonMark lets a span wrap, and
// `procoder\ntemplates` is one span rendered with a space, not a defect.
// Counting per line would flag every wrapped name in the repository — 46 of
// them here, all correct — which is how a check earns its way into being
// ignored. Fenced blocks are skipped whole: a fence is not a span, and the
// code inside one is nobody's prose.
//
// Information, not blocking: a deliberate literal backtick is legitimate,
// if rare.
func UnclosedSpans(root, file string) []gitx.Finding {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var out []gitx.Finding
	inFence := false
	ticks, start := 0, 0
	flush := func() {
		if ticks%2 == 1 {
			out = append(out, gitx.Finding{
				File: file, Line: start,
				Message: "code span opened and never closed in this paragraph — Markdown renders the rest as prose (docs)",
			})
		}
		ticks, start = 0, 0
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			// A fence boundary also ends the paragraph before it.
			if !inFence {
				flush()
			} else {
				ticks, start = 0, 0
			}
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		// Doubled backticks open and close a span that itself contains one,
		// so they play no part in the odd/even question.
		if n := strings.Count(strings.ReplaceAll(line, "``", ""), "`"); n > 0 {
			if ticks == 0 {
				start = i + 1
			}
			ticks += n
		}
	}
	flush()
	return out
}
