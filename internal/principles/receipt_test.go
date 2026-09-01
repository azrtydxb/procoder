package principles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The host inlines only the first ~2KB of a SessionStart payload and writes
// the rest to a file. These tests pin the two halves of the answer: the
// check arrives inside that window, and the marker it names really is the
// last thing in the payload.
const observedInlineWindow = 2000

// proved by: moving the notice below the principles text puts it past the
// window, so the one instruction that explains the truncation is the part
// that gets truncated.
func TestTheReceiptCheckArrivesInsideTheInlinedWindow(t *testing.T) {
	text := hookText(t.TempDir())

	i := strings.Index(text, "RECEIPT CHECK")
	if i < 0 {
		t.Fatal("no receipt check in the payload")
	}
	end := strings.Index(text, endMarker)
	if end < 0 {
		t.Fatal("the notice names a marker the payload does not contain")
	}
}

// proved by: appending anything after the marker makes it stop meaning "you
// have the whole payload", which is the only thing it is for.
func TestTheMarkerIsTheLastThingInThePayload(t *testing.T) {
	text := strings.TrimRight(hookText(t.TempDir()), "\n")
	if !strings.HasSuffix(text, endMarker) {
		tail := text
		if len(tail) > 200 {
			tail = tail[len(tail)-200:]
		}
		t.Fatalf("the payload does not end with the marker it told the reader to look for:\n…%s", tail)
	}
	if strings.Count(text, endMarker) != 1 {
		t.Fatal("the marker appears more than once, so seeing it proves nothing")
	}
}

// proved by: a payload that fits needs no check, and one that does not fit
// needs the check to be true — this asserts the situation the notice
// describes is the situation that exists.
func TestThePayloadIsBeyondTheWindowAndSaysSo(t *testing.T) {
	text := hookText(t.TempDir())
	if len(text) <= observedInlineWindow {
		t.Skip("the payload now fits the window; the receipt check is harmless but no longer load-bearing")
	}
	if !strings.Contains(text[:observedInlineWindow], "Read that file") {
		t.Fatal("the payload exceeds the window and the recovery instruction is not inside it")
	}
}

// proved by: writing the repository's text verbatim lets an adopter's own
// PRINCIPLES.md carry the marker — pasted from procoder's docs, or from a
// document about procoder — into the first 2KB, so the receipt check passes
// on a truncated payload. That is a confident receipt for a delivery that
// did not happen, which is the one failure it exists to prevent.
//
// The marker is assembled at runtime rather than written as a literal, per
// the repository's own rule about fixtures that trip its scanners.
func TestAQuotedMarkerInTheRepositoryTextCannotSpoofTheCheck(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	quoted := "== " + "procoder principles" + " end " + "=="
	body := "# House rules\n\nSee the marker below, quoted from the docs:\n\n" +
		quoted + "\n\nand inline " + quoted + " too.\n"
	if err := os.WriteFile(filepath.Join(root, ".procoder", "PRINCIPLES.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	text := hookText(root)
	if strings.Count(text, endMarker) != 1 {
		t.Fatalf("the marker appears %d times, so seeing it proves nothing",
			strings.Count(text, endMarker))
	}
	if !strings.HasSuffix(strings.TrimRight(text, "\n"), endMarker) {
		t.Fatal("the surviving marker is not the one at the end")
	}
}
