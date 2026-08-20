package copilot

import (
	"strings"
	"testing"
	"time"
)

// fakeToken builds a credential-shaped string at runtime. Never write one as
// a literal: this repository's own gate scans its source for secrets, and a
// fixture that trips it turns a passing test into a blocked commit.
func fakeToken(prefix string, n int) string {
	return prefix + strings.Repeat("aB3dE5fG7h", n/10+1)[:n]
}

func body(f Finding, root string) string {
	return Sanitise(f, root).Body
}

// proved by: dropping the fence branch from stripCodeBlocks, so the block's
// lines fall through to the output.
func TestFencedCodeBlocksNeverSurvive(t *testing.T) {
	// a closing fence must be at least as long as its opener, which is why
	// the pairs are spelled out rather than derived
	for _, fence := range [][2]string{{"```", "```"}, {"```go", "```"}, {"~~~", "~~~"}, {"~~~~ go", "~~~~"}} {
		f := Finding{Body: "Nil pointer on the second call.\n\n" + fence[0] +
			"\nfunc secretSauce() { proprietary() }\n" + fence[1] + "\n\nUse a guard clause."}
		got := body(f, "/repo")
		if strings.Contains(got, "proprietary") || strings.Contains(got, "secretSauce") {
			t.Fatalf("%s block leaked the user's code: %q", fence[0], got)
		}
		if !strings.Contains(got, "Nil pointer") || !strings.Contains(got, "guard clause") {
			t.Fatalf("%s block took the defect description with it: %q", fence[0], got)
		}
	}
}

// proved by: deleting stripHTMLCode — GitHub renders HTML in issue bodies,
// so <pre> is a fence by another name and nothing else was catching it.
func TestHTMLCodeBlocksAreCodeToo(t *testing.T) {
	f := Finding{Body: "The handler is wrong:\n<pre><code>proprietaryThing(secretSauce)</code></pre>\nGuard the nil case.\n"}
	got := body(f, "/repo")
	if strings.Contains(got, "proprietaryThing") || strings.Contains(got, "secretSauce") {
		t.Fatalf("an HTML code block leaked the user's code: %q", got)
	}
	if !strings.Contains(got, "Guard the nil case") {
		t.Fatalf("the fix must survive: %q", got)
	}
}

// proved by: returning false instead of true for an unclosed tag, which
// publishes an unread body as if it had been verified.
func TestAnUnclosedHTMLCodeTagIsIncompleteAndDropsTheRest(t *testing.T) {
	f := Finding{Body: "A defect.\n<pre>proprietaryThing()\n"}
	got := body(f, "/repo")
	if strings.Contains(got, "proprietaryThing") {
		t.Fatalf("everything after an unclosed code tag is code: %q", got)
	}
	if !strings.HasPrefix(got, incompleteNote) {
		t.Fatalf("an unclosed code tag must be declared: %q", got)
	}
}

// proved by: removing the quoteRe unquoting from stripCodeBlocks' loop, so a
// fence that arrives quoted is not seen as a fence at all. This is the shape
// that matters most: Copilot's review annotation IS a quote block, so the
// source it copied out of the user's file arrives behind "> ".
func TestCodeInsideAQuotedFenceIsStillCode(t *testing.T) {
	for _, quote := range []string{"> ", "> > ", ">"} {
		f := Finding{Body: quote + "The call is unchecked.\n" + quote + "```go\n" +
			quote + "proprietaryThing(secretSauce)\n" + quote + "```\n" + quote + "Check the error.\n"}
		got := body(f, "/repo")
		if strings.Contains(got, "proprietaryThing") || strings.Contains(got, "secretSauce") {
			t.Fatalf("a quoted fence (%q) leaked the user's code: %q", quote, got)
		}
		if !strings.Contains(got, "Check the error") {
			t.Fatalf("a quoted fence (%q) took the fix with it: %q", quote, got)
		}
	}
}

// proved by: unquoting more than one space per marker, so `>     code` loses
// its four-space indent and an indented block reads as prose.
func TestAQuotedIndentedBlockIsStillCode(t *testing.T) {
	f := Finding{Body: "> The loop is wrong:\n>\n>     proprietaryAlgorithm()\n>\n> Iterate a copy.\n"}
	got := body(f, "/repo")
	if strings.Contains(got, "proprietaryAlgorithm") {
		t.Fatalf("a quoted indented block leaked: %q", got)
	}
	if !strings.Contains(got, "Iterate a copy") {
		t.Fatalf("the fix must survive: %q", got)
	}
}

// proved by: deleting the `indented && prevBlank` case, so a four-space block
// is treated as prose and copied out verbatim.
func TestIndentedCodeBlockIsStripped(t *testing.T) {
	f := Finding{Body: "The loop is wrong:\n\n    for i := range proprietaryThing {\n        mutate(i)\n    }\n\nIterate a copy."}
	got := body(f, "/repo")
	if strings.Contains(got, "proprietaryThing") || strings.Contains(got, "mutate") {
		t.Fatalf("indented code leaked: %q", got)
	}
	if !strings.Contains(got, "Iterate a copy") {
		t.Fatalf("the suggested fix must survive: %q", got)
	}
}

// proved by: dropping the `prevBlank` requirement, which then eats the
// continuation line of a list item — the defect description itself.
func TestIndentedListContinuationIsProseNotCode(t *testing.T) {
	f := Finding{Body: "- the mutex is released twice\n    which panics on the second unlock\n"}
	if got := body(f, "/repo"); !strings.Contains(got, "panics on the second unlock") {
		t.Fatalf("an indented list continuation is prose: %q", got)
	}
}

// proved by: removing any one entry from secretPatterns — that shape's token
// then reaches the output whole.
func TestEverySecretShapeIsRedacted(t *testing.T) {
	cases := map[string]string{
		"github classic": fakeToken("ghp"+"_", 36),
		"github oauth":   fakeToken("gho"+"_", 36),
		"github user":    fakeToken("ghu"+"_", 36),
		"github server":  fakeToken("ghs"+"_", 36),
		"github refresh": fakeToken("ghr"+"_", 36),
		"github fine":    fakeToken("github"+"_pat"+"_", 40),
		"openai":         fakeToken("sk"+"-", 32),
		"google":         fakeToken("AIza", 35),
		"aws":            "AKIA" + strings.Repeat("Z7Q", 6)[:16],
		"bearer":         "Bearer " + fakeToken("", 30),
		"password kv":    "password=" + fakeToken("", 18),
		"api key kv":     "api_key: " + fakeToken("", 24),
	}
	for name, secret := range cases {
		f := Finding{Body: "The handler logs the credential: " + secret + " on every request."}
		got := body(f, "/repo")
		if strings.Contains(got, secret) {
			t.Fatalf("%s survived sanitisation: %q", name, got)
		}
		if !strings.Contains(got, secretMarker) {
			t.Fatalf("%s was removed without leaving the marker: %q", name, got)
		}
	}
}

// proved by: changing any secret pattern's replacement to keep a prefix (e.g.
// "$0[:8]...") — a redaction that shows part of the value is not a redaction.
func TestRedactionRevealsNoPartOfTheSecret(t *testing.T) {
	secret := fakeToken("ghp"+"_", 36)
	got := body(Finding{Body: "leaked " + secret + " here"}, "/repo")
	for i := 0; i+10 <= len(secret); i += 5 {
		if strings.Contains(got, secret[i:i+10]) {
			t.Fatalf("a 10-character run of the secret survived: %q", got)
		}
	}
}

// proved by: removing the relativiseRoot call from scrubText, which then
// publishes the user's home directory (and their name) in the issue body.
func TestAbsolutePathsUnderRootBecomeRelativeAndForwardSlashed(t *testing.T) {
	root := "/Users/someone/Development/widget"
	f := Finding{Body: "Bug at " + root + "/internal/store/db.go:42 — the transaction is never rolled back."}
	got := body(f, root)
	if strings.Contains(got, root) {
		t.Fatalf("the absolute path must never be emitted: %q", got)
	}
	if !strings.Contains(got, "internal/store/db.go:42") {
		t.Fatalf("the file:line position must survive as a relative path: %q", got)
	}
}

// proved by: replacing forwardSlash with filepath.ToSlash, which is a no-op
// on Unix and therefore emits backslashes for a Windows-shaped finding.
func TestWindowsPathsAreEmittedForwardSlashed(t *testing.T) {
	root := `C:\Users\someone\widget`
	got := body(Finding{Body: `Bug at ` + root + `\internal\store\db.go:42 here.`}, root)
	if strings.Contains(got, `\`) {
		t.Fatalf("every emitted path is forward-slashed: %q", got)
	}
	if !strings.Contains(got, "internal/store/db.go:42") {
		t.Fatalf("the position must survive the conversion: %q", got)
	}
}

// proved by: deleting the redactAbsolutePaths call — a path in a sibling
// directory names the machine and its owner just as loudly as one under root.
func TestPathsOutsideRootAreNeverEmitted(t *testing.T) {
	root := "/Users/someone/Development/widget"
	f := Finding{Body: "The key is read from /Users/someone/.ssh/id_rsa at startup."}
	got := body(f, root)
	if strings.Contains(got, "someone") || strings.Contains(got, "id_rsa") {
		t.Fatalf("a path above root must not be emitted: %q", got)
	}
	if !strings.Contains(got, pathMarker) {
		t.Fatalf("the removed path must leave a marker: %q", got)
	}
}

// proved by: deleting the boundary check in replaceRootPrefix, which then
// treats `<root>-old/x.go` as being inside the root and emits `-old/x.go` —
// a mangled path that silently confirms the directory above it.
func TestASiblingThatStartsWithTheRootsNameIsNotInsideIt(t *testing.T) {
	root := "/Users/someone/Development/widget"
	got := body(Finding{Body: "Copied from " + root + "-old/internal/x.go today."}, root)
	if strings.Contains(got, "someone") || strings.Contains(got, "-old") {
		t.Fatalf("a sibling directory is outside the root and must be redacted: %q", got)
	}
	if !strings.Contains(got, pathMarker) {
		t.Fatalf("the removed path must leave a marker: %q", got)
	}
}

// proved by: dropping the empty-tail case in replaceRootPrefix, so the root
// mentioned on its own is emitted whole instead of as the project's ".".
func TestTheRootOnItsOwnBecomesDot(t *testing.T) {
	root := "/Users/someone/Development/widget"
	got := body(Finding{Body: "Run the tool from " + root + " and retry."}, root)
	if strings.Contains(got, "someone") {
		t.Fatalf("the root's own path must not be emitted: %q", got)
	}
	if !strings.Contains(got, "from . and retry") {
		t.Fatalf("the project root reads as \".\": %q", got)
	}
}

// proved by: dropping the "://" guard in redactAbsolutePaths, which then
// redacts the link back to Copilot's own issue and destroys traceability.
func TestTheLinkBackToCopilotSurvives(t *testing.T) {
	url := "https://github.com/acme/widget/issues/12"
	got := body(Finding{Body: "See " + url + " for the original."}, "/repo")
	if !strings.Contains(got, url) {
		t.Fatalf("a URL is not a filesystem path: %q", got)
	}
}

// proved by: removing the copilotRe exemption in stripIdentities, which
// erases the one name the record exists to keep.
func TestPeopleGoAndCopilotStays(t *testing.T) {
	f := Finding{Body: "Reported by @copilot[bot], assigned to @realperson (someone@example.com)."}
	got := body(f, "/repo")
	if strings.Contains(got, "realperson") || strings.Contains(got, "example.com") {
		t.Fatalf("a person's handle or email must not be emitted: %q", got)
	}
	if !strings.Contains(got, "@copilot[bot]") {
		t.Fatalf("the bot's name is safe and is the point of the record: %q", got)
	}
}

// proved by: anchoring copilotRe to the exact string "copilot[bot]", so the
// copilot-preview[bot] spelling the spec's Q3 names is scrubbed away.
func TestBothCopilotBotSpellingsAreKept(t *testing.T) {
	got := body(Finding{Body: "From @copilot-preview[bot] and @copilot."}, "/repo")
	if !strings.Contains(got, "@copilot-preview[bot]") || !strings.Contains(got, "@copilot.") {
		t.Fatalf("both Copilot spellings are safe: %q", got)
	}
}

// proved by: returning the raw body unscrubbed on the incomplete path — the
// note would then be an announcement that a secret is about to follow.
func TestUnparseableBodyKeepsTheNoteAndIsStillScrubbed(t *testing.T) {
	root := "/Users/someone/widget"
	secret := fakeToken("ghp"+"_", 36)
	f := Finding{Body: "Token " + secret + " is logged from " + root +
		"/internal/log.go:9.\n\n```go\nfunc leak() { proprietary() }\n"} // fence never closed
	got := body(f, root)
	if !strings.HasPrefix(got, incompleteNote) {
		t.Fatalf("an unparseable body must say so up front: %q", got)
	}
	if strings.Contains(got, secret) || strings.Contains(got, root) || strings.Contains(got, "proprietary") {
		t.Fatalf("the incomplete path still redacts everything: %q", got)
	}
	if !strings.Contains(got, "internal/log.go:9") {
		t.Fatalf("the position must survive even here: %q", got)
	}
}

// proved by: making stripCodeBlocks close the fence at end of input instead
// of reporting it — the finding would then be published as if verified.
func TestClosedFenceRaisesNoIncompleteNote(t *testing.T) {
	f := Finding{Body: "A defect.\n\n```\ncode\n```\n\nA fix."}
	if got := body(f, "/repo"); strings.Contains(got, incompleteNote) {
		t.Fatalf("a well-formed body must not be flagged: %q", got)
	}
}

// proved by: returning the note (or a placeholder) when nothing is left —
// the caller is specified to skip a finding whose body sanitises to nothing,
// and it can only do that if the body is actually empty.
func TestABodyOfNothingButCodeSanitisesToEmpty(t *testing.T) {
	f := Finding{Body: "```go\nfunc proprietary() {}\n```\n"}
	if got := body(f, "/repo"); got != "" {
		t.Fatalf("nothing but code must sanitise to an empty body, got %q", got)
	}
	if got := body(Finding{Body: "```go\nfunc proprietary() {}\n"}, "/repo"); got != "" {
		t.Fatalf("nothing but an unterminated block must sanitise to empty too, got %q", got)
	}
}

// proved by: stripping Copilot's quote furniture before the scrub instead of
// after, or dropping the blockquote unwrapping — the body then arrives with
// "> " on every line and reads as a quote of nobody.
func TestCopilotsQuoteWrapperComesOffAndTheFindingStays(t *testing.T) {
	f := Finding{Body: "---\n> **Copilot**\n>\n> The error from Close() is discarded.\n> Check it and wrap it.\n"}
	got := body(f, "/repo")
	if strings.Contains(got, ">") || strings.Contains(got, "**Copilot**") {
		t.Fatalf("Copilot's wrapper is furniture, not finding: %q", got)
	}
	if !strings.Contains(got, "discarded") || !strings.Contains(got, "Check it and wrap it") {
		t.Fatalf("the defect and the fix must both survive: %q", got)
	}
}

// proved by: leaving Sanitise's line fallback out, so a position that only
// ever existed in the prose is lost.
func TestTheLineIsRecoveredFromTheBodyWhenUnknown(t *testing.T) {
	got := Sanitise(Finding{Body: "internal/store/db.go:42 leaks a handle."}, "/repo")
	if got.Line != 42 {
		t.Fatalf("the position in the prose is the position, got %d", got.Line)
	}
	known := Sanitise(Finding{Body: "internal/store/db.go:42 leaks a handle.", Line: 7}, "/repo")
	if known.Line != 7 {
		t.Fatalf("a line the caller already knew must not be overwritten, got %d", known.Line)
	}
}

// proved by: passing f.Title straight through instead of scrubbing it — an
// issue title is published exactly as widely as its body.
func TestTheTitleIsScrubbedToo(t *testing.T) {
	root := "/Users/someone/widget"
	secret := fakeToken("ghp"+"_", 36)
	got := Sanitise(Finding{Title: "Leak of " + secret + " in " + root + "/main.go", Body: "x"}, root)
	if strings.Contains(got.Title, secret) || strings.Contains(got.Title, root) {
		t.Fatalf("the title must be scrubbed like the body: %q", got.Title)
	}
	if !strings.Contains(got.Title, "main.go") {
		t.Fatalf("the title must keep what is safe: %q", got.Title)
	}
	long := Sanitise(Finding{Title: strings.Repeat("word ", 60)}, root)
	if len(long.Title) > titleMax {
		t.Fatalf("a title is one line: %d chars", len(long.Title))
	}
}

// proved by: dropping OriginalURL or Created from the returned struct — the
// traceability back to the Copilot issue is metadata, never user code, and
// must pass through untouched.
func TestMetadataPassesThroughUntouched(t *testing.T) {
	when := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	f := Finding{OriginalURL: "https://github.com/acme/widget/issues/12", Created: when, Body: "A defect."}
	got := Sanitise(f, "/repo")
	if got.OriginalURL != f.OriginalURL || !got.Created.Equal(when) {
		t.Fatalf("metadata must survive verbatim: %+v", got)
	}
}

// proved by: any single weakening of the pipeline — this is the spec's
// acceptance criterion, one body carrying all three kinds of leak at once.
func TestSecretPathAndCodeAreAllGoneTogether(t *testing.T) {
	root := "/Users/someone/Development/widget"
	secret := fakeToken("ghp"+"_", 36)
	f := Finding{
		Title: "Credential logged in the request handler",
		Body: "The handler at " + root + "/internal/http/handler.go:88 logs the token.\n\n" +
			"```go\nlog.Printf(\"auth %s\", " + secret + ")\nfunc proprietarySecretSauce() {}\n```\n\n" +
			"It also reads " + root + "/config.yaml where " + secret + " is stored.\n\n" +
			"Redact the credential before logging.\n",
	}
	got := Sanitise(f, root)
	for _, forbidden := range []string{secret, root, "proprietarySecretSauce", "log.Printf"} {
		if strings.Contains(got.Body, forbidden) {
			t.Fatalf("%q reached the sanitised body: %q", forbidden, got.Body)
		}
	}
	if !strings.Contains(got.Body, "internal/http/handler.go:88") {
		t.Fatalf("the file:line must survive: %q", got.Body)
	}
	if !strings.Contains(got.Body, "Redact the credential before logging") {
		t.Fatalf("the suggested fix must survive: %q", got.Body)
	}
	if got.Line != 88 {
		t.Fatalf("the position must be recovered, got %d", got.Line)
	}
}
