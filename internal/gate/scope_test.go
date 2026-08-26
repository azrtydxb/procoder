package gate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gitRepo is a real repository, because the universal gate's whole job is
// to tell this commit's lines from everybody else's, and only git knows.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

func gitDo(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// S-4, and the sentence that opened #172: procoder called a file
// "drifted" that it had never written. "Drift" means moved away from what
// procoder generated; against somebody else's AGENTS.md it is simply a
// false statement about a file, and it is the finding the reporter led
// with.
//
// proved by: the universal branch made to call gitcmd.CollectFor
// (AgentsDrift runs, and "drift" is back in the report).
func TestANonAdoptingRepositoryIsNeverToldItsFilesDrifted(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	// Somebody else's agent instructions, and their own Copilot file.
	writeFile(t, root, "AGENTS.md", "# Agents\n\nThis project uses its own review bot.\n")
	writeFile(t, root, ".github/copilot-instructions.md", "# Copilot\n\nUse two-space indent.\n")
	note := writeFile(t, root, "NOTES.txt", "a note\n")
	gitDo(t, root, "add", "-A")

	var out bytes.Buffer
	RunWith([]string{note}, root, "docs: a note", &out)
	if strings.Contains(strings.ToLower(out.String()), "drift") {
		t.Fatalf("procoder claimed a file it never wrote had drifted:\n%s", out.String())
	}
}

// S-3: none of procoder's house-rule domains speak in somebody else's
// repository. Named individually rather than counted, because a count
// passes for the wrong reason the moment a domain is added or renamed.
//
// proved by: same mutation as above — the template and planning findings
// reappear (want none, got "PULL_REQUEST_TEMPLATE.md is missing"). The
// houseRules half has its own test below.
func TestNoHouseRulesInANonAdoptingRepository(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	writeFile(t, root, "AGENTS.md", "# Agents\n\nSomebody else's.\n")
	note := writeFile(t, root, "NOTES.txt", "a note\n")
	gitDo(t, root, "add", "-A")

	var out bytes.Buffer
	RunWith([]string{note}, root, "docs: a note", &out)
	report := out.String()
	for _, house := range []string{
		"PULL_REQUEST_TEMPLATE.md", // procoder's own templates
		"COMMIT_TEMPLATE.md",
		"WORKFLOW.md",
		"drift",               // the agent layer
		"documentation oblig", // the docs domain
		"default branch",      // the branch habit
		".gitignore",          // ignore coverage
	} {
		if strings.Contains(strings.ToLower(report), strings.ToLower(house)) {
			t.Errorf("house rule %q was applied to a repository that never adopted procoder:\n%s", house, report)
		}
	}
}

// S-2: quiet is not toothless. A conflict marker the commit just wrote is
// wrong in anybody's repository, and must still block.
//
// proved by: drop gitx.NarrowToDiff(... ConflictMarkers ...) from the
// universal branch (exit 0 instead of 1 — the gate stops blocking).
func TestUniversalStillBlocksOnAConflictMarkerTheCommitWrote(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	writeFile(t, root, "seed.txt", "seed\n")
	gitDo(t, root, "add", "-A")
	gitDo(t, root, "commit", "-q", "-m", "seed")

	bad := writeFile(t, root, "merged.txt", "ok\n<<<<<<< HEAD\nmine\n=======\ntheirs\n>>>>>>> other\n")
	gitDo(t, root, "add", "-A")

	var out bytes.Buffer
	code := RunWith([]string{bad}, root, "fix: merge", &out)
	if code != 1 {
		t.Fatalf("exit %d in a non-adopting repository with a fresh conflict marker, want 1\n%s", code, out.String())
	}
	if s, _ := ScopeFor(root, ""); s != Universal {
		t.Fatalf("the fixture was not the mode under test: %s", s)
	}
}

// S-6, the file-level half: an oversized file or a junk file is about the
// file's PRESENCE, and this commit is adding it whole. Those must not
// narrow to the diff, or a committed .DS_Store passes because it has no
// interesting lines.
//
// What makes that safe is already in place: a JunkFiles finding carries no
// line, and NarrowToDiff keeps line-less findings by construction
// (TestNarrowToDiffKeepsWholeFileFindings). Routing junk through the
// narrower is therefore a no-op — verified, not assumed — so the mutation
// here is the blunter one.
//
// proved by: `gitx.JunkFiles(changed)` dropped from CollectUniversal
// (exit 0 — a committed .DS_Store sails through).
func TestAJunkFileStillBlocksInANonAdoptingRepository(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	writeFile(t, root, "seed.txt", "seed\n")
	gitDo(t, root, "add", "-A")
	gitDo(t, root, "commit", "-q", "-m", "seed")

	junk := writeFile(t, root, ".DS_Store", "\x00\x01binary\n")
	gitDo(t, root, "add", "-A", "-f")

	var out bytes.Buffer
	code := RunWith([]string{junk}, root, "chore: oops", &out)
	if code != 1 {
		t.Fatalf("a junk file did not block in a non-adopting repository: exit %d\n%s", code, out.String())
	}
}

// S-5: a gate that has halved itself says so. A quiet gate and a clean
// repository must not look the same — that is the silent green this
// project exists to prevent, arriving as a UX bug instead of a logic one.
//
// proved by: delete the `fmt.Fprintf(stdout, "gate scope: ...")` line
// (the mode is never announced).
func TestTheGateAnnouncesItsScope(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	note := writeFile(t, root, "NOTES.txt", "a note\n")
	gitDo(t, root, "add", "-A")

	var out bytes.Buffer
	RunWith([]string{note}, root, "docs: a note", &out)
	report := out.String()
	if !strings.Contains(report, "gate scope: universal") {
		t.Fatalf("the gate did not say which mode it ran in:\n%s", report)
	}
	if !strings.Contains(report, "NOT checked here") {
		t.Fatalf("the gate did not say that procoder's conventions were skipped:\n%s", report)
	}
	// And the summary must not undo the warning. "0 clean, 0 unformatted"
	// over a file nothing looked at reads as a formatting pass — the same
	// silent green, arriving one line later.
	if strings.Contains(report, "clean,") {
		t.Fatalf("the summary claimed a formatting verdict the gate never reached:\n%s", report)
	}
	if !strings.Contains(report, "not formatting-checked") {
		t.Fatalf("the summary did not say the files went unchecked:\n%s", report)
	}
}

// S-2 read the other way: an adopting repository loses nothing. The same
// fixture, with .procoder/ present, gets the full set back.
//
// proved by: `if s, ok := parseScope(cfgScope); ok` in ScopeFor made to
// return Universal always (the templates finding disappears, want it).
func TestAnAdoptingRepositoryStillGetsTheHouseRules(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	writeFile(t, root, ".procoder/config.toml", "")
	note := writeFile(t, root, "NOTES.txt", "a note\n")
	gitDo(t, root, "add", "-A")

	var out bytes.Buffer
	RunWith([]string{note}, root, "docs: a note", &out)
	report := out.String()
	if !strings.Contains(report, "gate scope: adopted") {
		t.Fatalf("a repository with .procoder/ did not get the full gate:\n%s", report)
	}
	if !strings.Contains(report, "PULL_REQUEST_TEMPLATE.md") {
		t.Fatalf("an adopting repository lost a house rule it had before:\n%s", report)
	}
}

// S-3, the other half: the house-rule DOMAINS are silent too. A `debt:`
// marker without a revisit condition is procoder's own convention about
// how to record a shortcut, and a project that never adopted it has not
// agreed to write its TODOs that way.
//
// proved by: `if scope == Adopted { hygiene = append(..., houseRules...) }`
// made unconditional (the debt finding reappears).
func TestHouseRuleDomainsAreSilentInANonAdoptingRepository(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	// A shortcut recorded the way this project happens to record them,
	// with no revisit condition — which procoder's debt domain flags.
	code := writeFile(t, root, "worker.go", "package worker\n\n// debt: single global lock\nvar mu int\n")
	gitDo(t, root, "add", "-A")

	var out bytes.Buffer
	RunWith([]string{code}, root, "feat: a worker", &out)
	if strings.Contains(strings.ToLower(out.String()), "debt") {
		t.Fatalf("procoder's debt convention was applied to a repository that never adopted it:\n%s", out.String())
	}
}

// stubGitleaksFinding puts a gitleaks on PATH that always reports one
// secret, at a line this test chooses. Real gitleaks would find nothing in
// a fixture whose "credential" is a made-up string, and planting a real
// pattern in a test file is how a repository ends up with a secret in its
// history.
func stubGitleaksFinding(t *testing.T, file string, line int) {
	t.Helper()
	bin := t.TempDir()
	report := fmt.Sprintf(`[{"File":%q,"StartLine":%d,"RuleID":"generic-api-key"}]`,
		filepath.ToSlash(file), line)
	if runtime.GOOS == "windows" {
		body := "@echo off\r\n>%6 echo " + strings.ReplaceAll(report, "%", "%%") + "\r\n"
		if err := os.WriteFile(filepath.Join(bin, "gitleaks.cmd"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		body := "#!/bin/sh\ncat > \"$6\" <<'EOF'\n" + report + "\nEOF\n"
		if err := os.WriteFile(filepath.Join(bin, "gitleaks"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// manyLines is a file long enough that "the finding is nowhere near the
// change" is a fact about the fixture rather than a hope.
func manyLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

// S-6, and the finding that opened the issue: a constant on line 4,423 of
// somebody else's file, flagged against a change 2,500 lines away. Not a
// credential, not written by this commit, and blocking it anyway.
//
// proved by: the universal branch made to call security.SecretsChangedFiles
// instead of SecretsInDiff (exit 1 — the pre-existing line blocks again).
func TestAPreExistingSecretDoesNotBlockInANonAdoptingRepository(t *testing.T) {
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	big := writeFile(t, root, "vendor.txt", manyLines(3000))
	gitDo(t, root, "add", "-A")
	gitDo(t, root, "commit", "-q", "-m", "upstream")

	// The change is at the top; the flagged line is 2,500 rows below it.
	body := strings.Split(manyLines(3000), "\n")
	body[1] = "the change this commit actually made"
	writeFile(t, root, "vendor.txt", strings.Join(body, "\n"))
	gitDo(t, root, "add", "-A")
	stubGitleaksFinding(t, big, 2500)

	var out bytes.Buffer
	code := RunWith([]string{big}, root, "fix: a small change", &out)
	if code != 0 {
		t.Fatalf("a line this commit never touched blocked it: exit %d\n%s", code, out.String())
	}
}

// The half that keeps it a gate: the same repository, the same scanner,
// but the flagged line is one this commit wrote. That blocks.
//
// proved by: `if f.Line == 0 || touched[key]` → `if false` in NarrowToDiff
// (exit 0 — a credential the commit is adding sails through).
func TestASecretOnALineTheCommitWroteStillBlocks(t *testing.T) {
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	big := writeFile(t, root, "vendor.txt", manyLines(3000))
	gitDo(t, root, "add", "-A")
	gitDo(t, root, "commit", "-q", "-m", "upstream")

	body := strings.Split(manyLines(3000), "\n")
	body[2499] = "the credential this commit is adding"
	writeFile(t, root, "vendor.txt", strings.Join(body, "\n"))
	gitDo(t, root, "add", "-A")
	stubGitleaksFinding(t, big, 2500)

	var out bytes.Buffer
	code := RunWith([]string{big}, root, "feat: a change", &out)
	if code != 1 {
		t.Fatalf("a credential on a line this commit wrote did not block: exit %d\n%s", code, out.String())
	}
}

// The constraint that shapes the sprint: an adopting repository loses
// nothing. Same fixture, same untouched line, `.procoder/` present — and
// there the answer is the one it has always been, because in your own
// repository that credential is yours whoever wrote the line.
//
// proved by: `if scope == Adopted` → `if false` at the secrets branch
// (exit 0 — an adopting repository stops hearing about its own code).
func TestAPreExistingSecretStillBlocksInAnAdoptingRepository(t *testing.T) {
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	writeFile(t, root, ".procoder/config.toml", "")
	big := writeFile(t, root, "vendor.txt", manyLines(3000))
	gitDo(t, root, "add", "-A")
	gitDo(t, root, "commit", "-q", "-m", "base")

	body := strings.Split(manyLines(3000), "\n")
	body[1] = "the change this commit actually made"
	writeFile(t, root, "vendor.txt", strings.Join(body, "\n"))
	gitDo(t, root, "add", "-A")
	stubGitleaksFinding(t, big, 2500)

	var out bytes.Buffer
	code := RunWith([]string{big}, root, "fix: a small change", &out)
	if code != 1 {
		t.Fatalf("an adopting repository stopped hearing about a secret in its own file: exit %d\n%s", code, out.String())
	}
}

// The same shape for conflict markers: pre-existing is silent, and the
// story lists it separately because it is a different check reading the
// same file.
//
// proved by: `gitx.NarrowToDiff(root, paths, gitx.ConflictMarkers(paths))`
// → `gitx.ConflictMarkers(paths)` (exit 1 — somebody else's marker blocks).
func TestAPreExistingConflictMarkerIsSilentInANonAdoptingRepository(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	// Upstream committed a conflict marker long ago. Not this commit's doing.
	body := "ok\n<<<<<<< HEAD\nmine\n=======\ntheirs\n>>>>>>> other\n" + manyLines(50)
	f := writeFile(t, root, "old.txt", body)
	gitDo(t, root, "add", "-A")
	gitDo(t, root, "commit", "-q", "-m", "upstream")

	writeFile(t, root, "old.txt", body+"one line this commit adds\n")
	gitDo(t, root, "add", "-A")

	var out bytes.Buffer
	code := RunWith([]string{f}, root, "docs: a line", &out)
	if code != 0 {
		t.Fatalf("somebody else's conflict marker blocked this commit: exit %d\n%s", code, out.String())
	}
}

// S-5 at the gate rather than at ScopeFor: forcing universal in a
// repository that HAS adopted procoder really does reduce the gate. The
// override is worthless if it only changes the printed word.
//
// proved by: `ScopeFor(root, cfg.GateScope)` → `ScopeFor(root, "")`
// (the config is ignored and the house rules come back).
func TestForcingUniversalInAnAdoptingRepositoryReducesTheGate(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	writeFile(t, root, ".procoder/config.toml", "[gate]\nscope = \"universal\"\n")
	note := writeFile(t, root, "NOTES.txt", "a note\n")
	gitDo(t, root, "add", "-A")

	var out bytes.Buffer
	RunWith([]string{note}, root, "docs: a note", &out)
	report := out.String()
	if !strings.Contains(report, "gate scope: universal") {
		t.Fatalf("the forced mode was not honoured:\n%s", report)
	}
	if strings.Contains(report, "PULL_REQUEST_TEMPLATE.md") {
		t.Fatalf("the forced mode changed the word but not the checks:\n%s", report)
	}
}

// S-6's file-level half, stated for size rather than junk: an oversized
// file is about the blob this commit is adding, so it does not narrow.
//
// proved by: `gitx.Oversized(changed, cfg.MaxFileMB)` dropped from
// CollectUniversal (exit 0 — the blob sails through).
func TestAnOversizedFileStillBlocksInANonAdoptingRepository(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root := gitRepo(t)
	writeFile(t, root, "seed.txt", "seed\n")
	gitDo(t, root, "add", "-A")
	gitDo(t, root, "commit", "-q", "-m", "seed")

	blob := filepath.Join(root, "big.bin")
	if err := os.WriteFile(blob, bytes.Repeat([]byte{0}, 6<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	gitDo(t, root, "add", "-A")

	var out bytes.Buffer
	code := RunWith([]string{blob}, root, "chore: a blob", &out)
	if code != 1 {
		t.Fatalf("an oversized blob did not block in a non-adopting repository: exit %d\n%s", code, out.String())
	}
}

// houseRuleFixture is one tree exercising every domain that encodes a
// procoder convention. Built by one function and used by both tests below,
// so "the same tree apart from .procoder/" is structural rather than a
// claim in a comment — the two runs cannot drift apart without the
// function changing under both.
func houseRuleFixture(t *testing.T, adopting bool) (root string, paths []string) {
	t.Helper()
	root = gitRepo(t)
	if adopting {
		writeFile(t, root, ".procoder/config.toml", "")
	}
	// Formatting: gofmt has an opinion about this and it is not met.
	paths = append(paths, writeFile(t, root, "bad.go", "package main\nfunc  main( ){}\n"))
	// The agent layer: an AGENTS.md that is somebody else's.
	writeFile(t, root, "AGENTS.md", "# Agents\n\nThis project has its own bot.\n")
	// Debt: a marker with no revisit condition.
	paths = append(paths, writeFile(t, root, "worker.go",
		"package worker\n\n// debt: one global lock\nvar mu int\n"))
	// Documentation: an exported symbol added with no doc change.
	writeFile(t, root, "README.md", "# A project\n")
	gitDo(t, root, "add", "-A")
	return root, paths
}

// The seventeen findings that should never have appeared. Enumerated
// rather than counted: a domain added later and forgotten here fails this
// test instead of leaking into somebody else's repository.
//
// proved by: `if scope != Adopted { break }` removed from the format loop
// (the unformatted file is reported, want silence).
func TestNoHouseRuleDomainSpeaksInANonAdoptingRepository(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root, paths := houseRuleFixture(t, false)

	var out bytes.Buffer
	code := RunWith(paths, root, "feat: a change", &out)
	report := strings.ToLower(out.String())
	for domain, marker := range map[string]string{
		"formatting":       "unformatted",
		"format-failed":    "unchecked",
		"the agent layer":  "drift",
		"debt":             "(debt)",
		"documentation":    "documentation oblig",
		"templates":        "template",
		"planning":         "planning",
		"the suite":        "tests: failed",
		"linting":          "(lint)",
		"maintainability":  "complexity",
		"the branch habit": "default branch",
		"ignore coverage":  ".gitignore",
	} {
		if strings.Contains(report, marker) {
			t.Errorf("%s spoke in a repository that never adopted procoder (%q):\n%s", domain, marker, out.String())
		}
	}
	if code != 0 {
		t.Errorf("a non-adopting repository was blocked by procoder's conventions: exit %d\n%s", code, out.String())
	}
}

// The constraint the epic is built around, over the same fixture with
// `.procoder/` added. The two runs differ in adoption and nothing else.
//
// proved by: `if scope == Adopted` at the houseRules branch → `if false`
// (the adopting repository silently loses its own checks).
func TestTheSameFixtureKeepsItsHouseRulesWhenAdopting(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	stubLinter(t)
	root, paths := houseRuleFixture(t, true)

	var out bytes.Buffer
	code := RunWith(paths, root, "feat: a change", &out)
	report := strings.ToLower(out.String())
	// Not every domain fires on every fixture — this tree has no lockfile,
	// no test suite, and no built index, so the vulnerable-dependency,
	// suite and documentation legs have nothing to say here either way.
	// These five do fire, and each is one whose silence in the run above
	// was the point.
	for domain, marker := range map[string]string{
		"formatting":      "unformatted",
		"debt":            "(debt)",
		"the agent layer": "(agents)",
		"templates":       "template",
		"ignore coverage": ".gitignore",
	} {
		if !strings.Contains(report, marker) {
			t.Errorf("an adopting repository lost %s (%q):\n%s", domain, marker, out.String())
		}
	}
	if code != 1 {
		t.Errorf("the adopting run did not block on findings it reported: exit %d\n%s", code, out.String())
	}
}
