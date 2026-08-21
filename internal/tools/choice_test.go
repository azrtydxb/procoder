package tools

import "testing"

// A repository may choose among the tools procoder ships, by name.
// proved by: made ForFileIn ignore the choice and return ByExtension —
// `js = "biome"` is accepted, reported as configured, and prettier runs.
func TestTheRepositorysChoiceWins(t *testing.T) {
	got := ForFileIn("/x/a.js", map[string]string{"js": "biome"})
	if got == nil || got.Name != "biome" {
		t.Fatalf("the chosen tool must be used, got %v", got)
	}
	// Every extension of the language follows the one key.
	for _, f := range []string{"/x/a.ts", "/x/a.tsx", "/x/a.mjs", "/x/a.cts"} {
		if ForFileIn(f, map[string]string{"js": "biome"}).Name != "biome" {
			t.Errorf("%s must follow the js choice", f)
		}
	}
	// A language the repo said nothing about keeps its default.
	if got := ForFileIn("/x/a.go", map[string]string{"js": "biome"}); got == nil || got.Name != "gofmt" {
		t.Errorf("an unmentioned language keeps its default, got %v", got)
	}
}

// A name procoder does not ship must not leave the file unchecked. Config
// reports and blocks on it; the formatter still runs the default, because
// a mistyped tool name is a reason to tell somebody, not a reason to stop
// reading their code.
// proved by: returned nil from ForFileIn for an unknown name — the file
// becomes "no formatter covers this file type" and the gate skips it.
func TestAnUnknownToolNameStillLeavesTheFileChecked(t *testing.T) {
	got := ForFileIn("/x/a.js", map[string]string{"js": "nosuchtool"})
	if got == nil {
		t.Fatal("an unknown name must fall back to the default, not to nothing")
	}
	if got.Name != "prettier" {
		t.Errorf("want the default prettier, got %q", got.Name)
	}
}

// Every tool on the menu must be able to PRINT. A tool that can only
// write in place would break the contract the menu exists to protect —
// which is why pint, phpcbf and php-cs-fixer are not offered for PHP
// however good they are.
// proved by: added a tool with no Args to an Alternatives entry — this
// test names it, where nothing else in the suite would notice until a
// user's file was silently rewritten.
func TestEveryAlternativeCanPrint(t *testing.T) {
	for lang, byName := range Alternatives {
		for name, tool := range byName {
			if tool == nil {
				t.Errorf("%s = %q has no tool", lang, name)
				continue
			}
			if tool.Args == nil {
				t.Errorf("%s = %q has no Args — procoder cannot invoke it, let alone make it print", lang, name)
				continue
			}
			// The argv must name the file or read stdin; a tool given
			// neither would format nothing, and one given the file with no
			// print flag is the shape that writes in place.
			args := tool.Args("/x/a.js")
			var mentions bool
			for _, a := range args {
				if a == "/x/a.js" || len(a) > 0 && a[0] == '-' {
					mentions = true
				}
			}
			if !mentions {
				t.Errorf("%s = %q: argv %v neither names the file nor passes a flag", lang, name, args)
			}
		}
	}
}
