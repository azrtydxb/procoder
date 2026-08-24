package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// commandCase matches a command or subcommand name in the CLI dispatch —
// the `case "name":` arms of main.go's switch. These are the names a user
// types, which is what makes them procoder's own rather than a mention.
var commandCase = regexp.MustCompile(`(?m)^\s*case\s+((?:"[a-z0-9-]+"(?:,\s*)?)+):`)

// exportedName matches a Go declaration procoder owns — `func Name`,
// `func (r T) Name`, `type Name` — anchored at column zero, because a
// declaration nested inside another block is not package API surface.
//
// Written this way after the obvious form proved wrong in both
// directions: `\s*(?:func|type)\s+\(?[^)]*\)?\s*([A-Z]...)` captured
// "Status" from `func BmadStatus()`, missing the very prefix this audit
// exists to find, and captured "Unlinted" from `func lintUnlinted`, a
// false positive on an unexported name. The greedy `[^)]*` ate the
// identifier. Both were caught by running the mutation rather than by
// reading the pattern.
var exportedName = regexp.MustCompile(`(?m)^(?:func\s+(?:\([^)]*\)\s*)?|type\s+)([A-Z][A-Za-z0-9_]*)`)

// trademarked are the marks procoder must not name a feature after. Lower
// case, matched against a lower-cased candidate, so BMad, BMAD and bmad are
// one rule rather than three.
var trademarked = []string{"bmad"}

// "BMad" and "BMAD-METHOD" are trademarks of BMad Code, LLC. Naming the
// mark to describe interoperation is fine and necessary — the config value
// `[planning] method = "bmad"`, a doctor line saying which version is
// installed, a sentence of documentation. Naming a procoder FEATURE after
// it is not.
//
// The distinction this audit draws is between a name a user types or a
// symbol procoder exports, and a string procoder prints. The first is
// procoder claiming the name; the second is procoder referring to somebody
// else's product, which is what nominative use means.
//
// It exists because that boundary erodes by accident rather than by
// decision. Nobody sets out to infringe a trademark; somebody adds a
// command and picks the clearest available name for it, and when the
// feature reads BMad's artifacts the clearest available name is the
// trademark. A convention in a spec does not catch that. A test does.
// proved by: add `case "bmad-sync":` to main.go's dispatch, or an exported
// `func BmadStatus()` anywhere under internal/ — each is named with its
// file and line, where the whole suite otherwise stays green.
func TestNoProcoderFeatureIsNamedAfterATrademark(t *testing.T) {
	var offenders []string

	// The command table: what a user types.
	main := filepath.Join("..", "..", "cmd", "procoder", "main.go")
	raw, err := os.ReadFile(main)
	if err != nil {
		t.Fatalf("the command table must be readable for this audit to mean anything: %v", err)
	}
	src := string(raw)
	for _, m := range commandCase.FindAllStringSubmatchIndex(src, -1) {
		arm := src[m[2]:m[3]]
		for _, mark := range trademarked {
			if strings.Contains(strings.ToLower(arm), mark) {
				offenders = append(offenders, "cmd/procoder/main.go:"+
					strconv.Itoa(strings.Count(src[:m[0]], "\n")+1)+" command "+arm)
			}
		}
	}

	// The API surface: what procoder exports.
	err = filepath.Walk(filepath.Join("..", "..", "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		src := string(raw)
		for _, m := range exportedName.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			for _, mark := range trademarked {
				if strings.Contains(strings.ToLower(name), mark) {
					offenders = append(offenders, filepath.ToSlash(path)+":"+
						strconv.Itoa(strings.Count(src[:m[0]], "\n")+1)+" exports "+name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("the tree must be readable for this audit to mean anything: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("a trademark may be referred to, never used as a procoder feature name:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// A command nobody can find is a command nobody uses. `procoder config`
// shipped, worked, printed every effective setting with the config.toml
// line it came from — and appeared in neither `procoder help` nor
// docs/commands.md. It was found by comparing the dispatch against the
// docs during the fixture campaign, which is a thing a test can do every
// time instead of once.
//
// Subcommand arms are excluded: `list`, `check`, `close` and the rest are
// documented under their parent's entry, not as commands of their own.
//
// proved by: deleting the `config` block from the usage text in main.go —
// this test then names it as shipped and undiscoverable.
func TestEveryShippedCommandIsDiscoverable(t *testing.T) {
	main := filepath.Join("..", "..", "cmd", "procoder", "main.go")
	raw, err := os.ReadFile(main)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)

	// the top-level switch only: from run()'s switch to the next
	// function, so the subcommand dispatchers below are not counted
	start := strings.Index(src, "func run(args []string) int {")
	if start < 0 {
		t.Fatal("run() not found in main.go")
	}
	end := strings.Index(src[start+10:], "\nfunc ")
	if end < 0 {
		t.Fatal("could not find the end of run()")
	}
	top := src[start : start+10+end]

	usageText := src[strings.Index(src, "usage = `"):]
	docs, err := os.ReadFile(filepath.Join("..", "..", "docs", "commands.md"))
	if err != nil {
		t.Fatal(err)
	}

	// Exactly one tab: the arms of run()'s own switch. `commandCase`
	// allows any indentation, which would count `post-tool-use` and the
	// rest — subcommands of `hook`, nested inside its arm — as top-level
	// commands and demand a usage block each.
	topLevelCase := regexp.MustCompile(`(?m)^\tcase ((?:"[a-z0-9-]+"(?:, )?)+):`)
	for _, m := range topLevelCase.FindAllStringSubmatch(top, -1) {
		for _, name := range strings.Split(m[1], ",") {
			name = strings.Trim(strings.TrimSpace(name), `"`)
			if name == "" || name == "help" {
				continue
			}
			if !strings.Contains(usageText, "\n  "+name+" ") && !strings.Contains(usageText, "\n  "+name+"\n") {
				t.Errorf("`procoder %s` is dispatched but absent from the usage text — nobody running `procoder help` can find it", name)
			}
			if !strings.Contains(string(docs), "procoder "+name) {
				t.Errorf("`procoder %s` is dispatched but absent from docs/commands.md", name)
			}
		}
	}
}
