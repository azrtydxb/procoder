package release

import (
	"strings"
	"testing"
)

// A bot credit is written exactly the way the documentation tells it to be
// — `[@github-actions[bot]](https://github.com/apps/github-actions)` — and
// counts: the capture takes the whole `[bot]` suffix, not a login
// truncated at the bracket.
// proved by: stopped the capture at the first bracket — the bot's credit
// parsed without its suffix and every bot contributor read as uncredited,
// which this test's whole-handle assertion is there to catch.
func TestABotCreditIsRecognisedWhole(t *testing.T) {
	entry := "_summary_\n\n**Changed — pins moved.**\n([#9](https://github.com/azrtydxb/procoder/pull/9)), contributed by\n[@github-actions[bot]](https://github.com/apps/github-actions).\n"
	got := CreditsIn(entry)
	if len(got) != 1 || got[0].Handle != "github-actions[bot]" || len(got[0].Cites) != 1 || got[0].Cites[0] != 9 {
		t.Fatalf("a bot credit names its handle whole and pairs it with the paragraph's cite: %+v", got)
	}
}

// A credit belongs to the thing its own paragraph cites. A handle in one
// paragraph and a number in another are unrelated, and pairing them would
// invent a claim the entry never made.
// proved by: gathered handles and numbers across the whole entry instead
// of per paragraph — every contributor appears to have opened everything,
// so no misattribution is detectable at all.
func TestACreditIsPairedWithWhatItsOwnParagraphCites(t *testing.T) {
	entry := `_summary_

**Fixed — one thing.**
([#152](https://github.com/azrtydxb/procoder/pull/152)) Reported by
[@codixio](https://github.com/codixio) in
[#150](https://github.com/azrtydxb/procoder/issues/150).

**Fixed — another.**
([#148](https://github.com/azrtydxb/procoder/pull/148)) Contributed by
[@Acroaticum](https://github.com/Acroaticum).
`
	got := CreditsIn(entry)
	if len(got) != 2 {
		t.Fatalf("one credit per named contributor: %+v", got)
	}
	if got[0].Handle != "codixio" || len(got[0].Cites) != 2 {
		t.Errorf("the first credit cites its own paragraph's two numbers: %+v", got[0])
	}
	if got[1].Handle != "Acroaticum" || len(got[1].Cites) != 1 || got[1].Cites[0] != 148 {
		t.Errorf("the second cites only 148, not the first paragraph's: %+v", got[1])
	}
}

// A contributor named in a paragraph that links nothing cannot be checked
// by anyone — reader or controller. Saying so is the point: an unverifiable
// credit is the shape the wrong name hides in.
// proved by: skipped paragraphs with no citations instead of reporting
// them — a credit attached to nothing passes, which is exactly where a
// misattribution would sit unnoticed.
func TestACreditThatCitesNothingIsReported(t *testing.T) {
	entry := "_summary_\n\n**Fixed — a thing.** Thanks to [@somebody](https://github.com/somebody).\n"
	got := VerifyCredits(t.TempDir(), entry)
	if len(got) != 1 {
		t.Fatalf("a credit with nothing to check against is reported: %v", got)
	}
	if !strings.Contains(got[0], "@somebody") || !strings.Contains(got[0], "links no issue") {
		t.Errorf("and says why: %q", got[0])
	}
}

// An entry naming nobody has nothing to verify, and a controller that
// reached for GitHub anyway would make every release depend on the network
// for a question it does not need to ask.
// proved by: returned a problem for an entry with no credits — every
// release with no contributors to thank is blocked on a check with no
// subject.
func TestAnEntryWithNoCreditsAsksGitHubNothing(t *testing.T) {
	entry := "_summary_\n\n**Added — a thing.**\n([#1](https://github.com/azrtydxb/procoder/pull/1)) Done.\n"
	if got := VerifyCredits(t.TempDir(), entry); len(got) != 0 {
		t.Errorf("nothing to verify is not a problem: %v", got)
	}
}

// GitHub not answering is not a pass. This is the one check in the
// controller that reaches the network, so it is the one most able to fail
// open — and a credit nothing verified is exactly how the wrong name
// ships, which is the failure this exists to prevent.
// proved by: returned nil when the lookup errors — an offline release
// verifies nothing, says nothing, and publishes whatever handle was
// written.
func TestGitHubNotAnsweringBlocksRatherThanPasses(t *testing.T) {
	// PATH emptied so `gh` cannot be found: the same shape as a machine
	// without it, a revoked token, or no network.
	t.Setenv("PATH", t.TempDir())

	entry := `_summary_

**Fixed — a thing.**
([#1](https://github.com/azrtydxb/procoder/pull/1)) Reported by
[@somebody](https://github.com/somebody).
`
	got := VerifyCredits(t.TempDir(), entry)
	if len(got) == 0 {
		t.Fatal("a lookup that could not run must not read as a credit that checked out")
	}
	if !strings.Contains(got[0], "NOT verified") {
		t.Errorf("and must say it did not run, not that it failed: %q", got[0])
	}
	if !strings.Contains(got[0], "#1") {
		t.Errorf("naming what it could not check: %q", got[0])
	}
}

// A cited number too large to be an int is skipped, not misread: the
// regex already promised digits, so the only parse failure is a range
// error, and an ignored Sscanf would have left n at its zero value —
// crediting a #0 that the entry never cited, or owing a credit for it.
// proved by: appending the parsed value anyway — the credit then carries a
// zero citation (or the owed list asks GitHub who opened #0).
func TestACitedNumberThatDoesNotFitAnIntIsSkipped(t *testing.T) {
	big := "999999999999999999999999999999999999999999"
	entry := "_summary_\n\n**Fixed — a thing.**\n([#" + big + "](https://github.com/azrtydxb/procoder/pull/1)), contributed by\n[@somebody](https://github.com/somebody).\n"
	got := CreditsIn(entry)
	if len(got) != 1 || got[0].Handle != "somebody" || len(got[0].Cites) != 0 {
		t.Fatalf("a citation too large to be a number is skipped, not reinterpreted: %+v", got)
	}

	called := map[int]bool{}
	gaps := missingCreditsWith(entry,
		func() ([]string, error) { return []string{"the-releaser"}, nil },
		func(n int) (origin, error) { called[n] = true; return origin{login: "x"}, nil },
	)
	if called[0] || len(gaps) != 0 {
		t.Fatalf("an unparseable number is not a credit nor an owed one (resolve asked for: %v, gaps: %v)", called, gaps)
	}
}
