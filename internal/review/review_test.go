package review

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// treeDigest is every file's path and content, hashed — the evidence that
// a command which only prints has only printed.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		fmt.Fprintf(h, "%s\x00%x\x00", path, sha256.Sum256(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("the tree must be readable for the digest to mean anything: %v", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func writeLens(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The whole point of the command: five distinct stances over the content,
// and nothing written. The binary judges nothing — it prints the lens and
// the scope, the agent judges — so a review that modified the tree would
// have broken the contract that makes it safe to run on anything.
// proved by: had Print write its output to a file under root as well as to
// the writer — the digest changes and this fails, where the printed output
// is byte-identical and every other assertion still passes.
func TestReviewPrintsEveryLensAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, root)

	lenses, problems := Resolve(root)
	if len(problems) != 0 {
		t.Fatalf("a repository with no overrides has no problems: %+v", problems)
	}
	if len(lenses) != 5 {
		t.Fatalf("five shipped lenses, got %d: %v", len(lenses), Names(lenses))
	}

	var buf bytes.Buffer
	Print(&buf, []string{filepath.Join(root, "a.go")}, lenses)
	out := buf.String()

	for _, name := range []string{"adversarial", "edge-case", "verification-gap", "structure", "prose"} {
		var l Lens
		for _, c := range lenses {
			if c.Name == name {
				l = c
			}
		}
		if l.Body == "" {
			t.Fatalf("lens %s is missing from the shipped set", name)
		}
		if !strings.Contains(out, l.Body) {
			t.Errorf("lens %s did not reach the output", name)
		}
	}
	if !strings.Contains(out, "a.go") {
		t.Errorf("the content in scope must be named:\n%s", out)
	}

	if after := treeDigest(t, root); after != before {
		t.Errorf("review prints and never writes — the tree changed")
	}
}

// Each lens is a distinct stance, not five wordings of one. A set where
// two lenses say the same thing costs the reader five passes and returns
// one perspective.
// proved by: pointed two entries in Lenses at the same body — this fails
// naming the pair, where every other test still passes because the count
// and the printing are unaffected.
func TestEveryLensIsADistinctStance(t *testing.T) {
	seen := map[string]string{}
	for _, l := range Lenses {
		if l.Body == "" {
			t.Errorf("lens %s has no instruction", l.Name)
			continue
		}
		if prior, dup := seen[l.Body]; dup {
			t.Errorf("lenses %s and %s are the same stance", prior, l.Name)
		}
		seen[l.Body] = l.Name
	}
}

// A reader who already knows what they are worried about asks one
// question rather than five. A name that is not a lens is returned rather
// than skipped: silently running the other four leaves them believing they
// got the lens they asked for.
// proved by: dropped the unknown return from Select and skipped unknown
// names — `--lens nope` then prints a full review and exits 0.
func TestSelectNarrowsAndNamesWhatItDoesNotKnow(t *testing.T) {
	all, _ := Resolve(t.TempDir())

	got, unknown := Select(all, []string{"edge-case"})
	if len(unknown) != 0 {
		t.Fatalf("edge-case is a lens: %v", unknown)
	}
	if len(got) != 1 || got[0].Name != "edge-case" {
		t.Fatalf("exactly the lens asked for: %v", Names(got))
	}

	// Order follows the caller, not the shipped order — someone who asks
	// for prose then adversarial gets them that way round.
	got, _ = Select(all, []string{"prose", "adversarial"})
	if len(got) != 2 || got[0].Name != "prose" || got[1].Name != "adversarial" {
		t.Errorf("selection keeps the caller's order: %v", Names(got))
	}

	if _, unknown = Select(all, []string{"nope"}); len(unknown) != 1 || unknown[0] != "nope" {
		t.Errorf("an unknown name comes back by name: %v", unknown)
	}
}

// D-OVERRIDE: a repository that disagrees with a lens replaces it, without
// forking procoder or giving up the other four. The source is reported so
// a reader knows whose words they are reading.
// proved by: made Resolve ignore the override directory — the shipped
// adversarial lens comes back under the repository's nose, and the review
// claims a stance the repository deliberately replaced.
func TestAnOverrideReplacesTheShippedLens(t *testing.T) {
	root := t.TempDir()
	const mine = "# Ours\n\nLook for the thing our last outage taught us.\n"
	writeLens(t, root, "adversarial", mine)

	lenses, problems := Resolve(root)
	if len(problems) != 0 {
		t.Fatalf("a readable override is not a problem: %+v", problems)
	}
	if len(lenses) != 5 {
		t.Fatalf("an override replaces a lens, it does not remove one: %v", Names(lenses))
	}
	for _, l := range lenses {
		switch l.Name {
		case "adversarial":
			if l.Body != mine {
				t.Errorf("the override's content must be what runs:\n%s", l.Body)
			}
			if !strings.Contains(l.Source, "adversarial.md") {
				t.Errorf("the source must name the override: %q", l.Source)
			}
		default:
			if l.Source != "default" {
				t.Errorf("%s has no override and must stay procoder's: %q", l.Name, l.Source)
			}
		}
	}
}

// An override that cannot be read blocks, and procoder does NOT fall back
// to its own — deliberately unlike templates.Resolve. A lens shapes a
// judgment, and `procoder review` is not gated by the commit gate: an
// agent reading printed output may act on it whatever the exit code says.
// A review under the repository's lens name running procoder's words is
// worse than no review, so there must be nothing printed to act on.
// proved by: returned the shipped lens alongside the finding, the way
// templates.Resolve does — the count goes back to five and procoder's
// adversarial stance is printed under the repository's name.
func TestAnUnreadableOverrideBlocksAndDoesNotFallBack(t *testing.T) {
	root := t.TempDir()
	writeLens(t, root, "adversarial", "   \n\t\n")

	lenses, problems := Resolve(root)
	if len(problems) != 1 {
		t.Fatalf("an empty override is exactly one problem: %+v", problems)
	}
	if !problems[0].Blocking {
		t.Error("a lens that could not load is a refusal, and must block")
	}
	if !strings.Contains(problems[0].File, "adversarial.md") {
		t.Errorf("the finding must name the file: %q", problems[0].File)
	}

	for _, l := range lenses {
		if l.Name == "adversarial" {
			t.Fatal("procoder must not substitute its own lens for one the repository replaced")
		}
	}
	if len(lenses) != 4 {
		t.Errorf("the other four are unaffected: %v", Names(lenses))
	}
}

// Perspectives are who is reading, where lenses are how. A lens is a
// method — enumerate paths, trace verification; a perspective is a set
// of concerns brought before any method is applied, and the architect and
// the implementer reach different conclusions about the same correct
// code.
// proved by: pointed PerspectiveSet at Lenses — the two sets become one,
// and asking for a perspective returns a method instead of a stance.
func TestPerspectivesAreTheirOwnSetAndTheirOwnStances(t *testing.T) {
	root := t.TempDir()
	got, problems := ResolvePerspectives(root)
	if len(problems) != 0 {
		t.Fatalf("a repository with no overrides has no problems: %+v", problems)
	}
	if len(got) != 4 {
		t.Fatalf("four shipped perspectives, got %d: %v", len(got), Names(got))
	}

	// Distinct from each other, for the reason the lenses are: four
	// wordings of one stance costs four passes and returns one read.
	seen := map[string]string{}
	for _, p := range got {
		if prior, dup := seen[p.Body]; dup {
			t.Errorf("perspectives %s and %s are the same stance", prior, p.Name)
		}
		seen[p.Body] = p.Name
	}

	// And distinct from the lenses: sharing a body would mean one of the
	// two sets is not what it claims to be.
	lensBodies := map[string]bool{}
	for _, l := range Lenses {
		lensBodies[l.Body] = true
	}
	for _, p := range got {
		if lensBodies[p.Body] {
			t.Errorf("perspective %s is a lens wearing another name", p.Name)
		}
	}
}

// The override contract is the same one lenses hold, and for the same
// reason: a read under your own perspective's name running procoder's
// words is worse than no read, so nothing is printed to act on.
// proved by: had ResolvePerspectives read the lens directory instead —
// a repository's perspective override is ignored and its lens override
// silently governs a different command.
func TestAPerspectiveOverrideBehavesLikeALensOverride(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, PerspectiveDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const mine = "# Ours\n\nAsk what our last incident would have asked.\n"
	if err := os.WriteFile(filepath.Join(dir, "architect.md"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}

	got, problems := ResolvePerspectives(root)
	if len(problems) != 0 {
		t.Fatalf("a readable override is not a problem: %+v", problems)
	}
	for _, p := range got {
		if p.Name == "architect" && p.Body != mine {
			t.Errorf("the override is what runs:\n%s", p.Body)
		}
	}

	// Empty refuses and does not fall back, and the refusal says
	// "perspective" rather than calling everything a lens.
	if err := os.WriteFile(filepath.Join(dir, "architect.md"), []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, problems = ResolvePerspectives(root)
	if len(problems) != 1 || !problems[0].Blocking {
		t.Fatalf("an empty override blocks: %+v", problems)
	}
	if !strings.Contains(problems[0].Message, "perspective architect") {
		t.Errorf("the refusal names what it is: %q", problems[0].Message)
	}
	for _, p := range got {
		if p.Name == "architect" {
			t.Error("procoder must not substitute its own for one the repository replaced")
		}
	}
}
