// Package gitx is the git-hygiene half of the GitOps domain: everything a
// senior developer glances at before calling work finished, computed
// deterministically from the repository itself. Every function answers with
// information; acting on it is the agent's job (P-CONTROL).
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Finding is one hygiene problem: where it is and what it is. This is the
// shape every non-formatter domain reports in — a diagnosis the agent
// investigates, not content it can paste.
type Finding struct {
	File    string
	Line    int // 0 when the finding is about the file as a whole
	Message string
	// Blocking findings fail the gate; the rest are information.
	Blocking bool
}

// gitRaw is git without the whitespace trim: output whose separators are
// bytes rather than lines cannot afford to have its edges tidied.
func gitRaw(root string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	return string(out), err
}

func git(root string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// ChangedFiles is the set a commit is about to contain: modified, added,
// renamed, untracked — deleted files are not checkable. Paths are absolute.
func ChangedFiles(root string) ([]string, error) {
	// gitRaw, not git: porcelain encodes the status in the first two
	// COLUMNS, and a worktree-modified file's line begins with a space.
	// Trimming the whole output ate that space on the first line only, so
	// the first changed file came back with its first character missing —
	// `cmd/procoder/main.go` as `md/procoder/main.go`. Everything
	// downstream then looked at a path that does not exist: the gate
	// skipped it, and golangci-lint answered "directory not found", which
	// procoder reported as NOT checked rather than as a bug in itself.
	out, err := gitRaw(root, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if len(line) < 4 || strings.Contains(line[:2], "D") {
			continue
		}
		name := strings.TrimSpace(line[3:])
		if i := strings.Index(name, " -> "); i >= 0 {
			name = name[i+4:]
		}
		name = strings.Trim(name, `"`)
		files = append(files, filepath.Join(root, name))
	}
	return files, nil
}

// FilesUnder lists the gate's file set beneath dir: tracked plus
// untracked-but-not-ignored. Paths are absolute.
func FilesUnder(root, dir string) []string {
	out, err := git(root, "ls-files", "--cached", "--others", "--exclude-standard", "--", dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			files = append(files, filepath.Join(root, line))
		}
	}
	return files
}

// CurrentBranch, or "" when detached.
func CurrentBranch(root string) string {
	b, err := git(root, "branch", "--show-current")
	if err != nil {
		return ""
	}
	return b
}

// DefaultBranch asks the remote first (origin/HEAD), then falls back to the
// conventional names. "" means it could not be determined, and the caller
// treats that as "not on it" — a hygiene check must not block on ignorance.
func DefaultBranch(root string) string {
	if ref, err := git(root, "symbolic-ref", "refs/remotes/origin/HEAD", "--short"); err == nil {
		return strings.TrimPrefix(ref, "origin/")
	}
	for _, name := range []string{"main", "master"} {
		if _, err := git(root, "rev-parse", "--verify", "refs/heads/"+name); err == nil {
			return name
		}
	}
	return ""
}

// GrepOn lists the files on a ref whose content matches an extended regular
// expression, as repo-relative paths. It is how a report asks what another
// branch holds without checking it out; err is a git that could not answer,
// which is never the same as a match of none.
func GrepOn(root, ref, pattern, pathspec string) ([]string, error) {
	// -z, not because of newlines in file names but because git quotes any
	// path with a non-ASCII byte ("caf\303\251-story.md") unless told not
	// to: the quoted form does not exist on disk, so a caller that stats
	// what it reads here would call every accented path missing. -E keeps
	// the pattern in the flavour a Go caller writes, not git's default BRE.
	out, err := gitRaw(root, "grep", "-lzE", "-e", pattern, ref, "--", pathspec)
	if err != nil {
		// git grep exits 1 for no matches, which is an answer, not a failure.
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, rec := range strings.Split(out, "\x00") {
		// "<ref>:<path>" — a ref name cannot contain a colon, so the first
		// one ends the ref and a path may carry as many as it likes.
		if _, path, found := strings.Cut(rec, ":"); found && path != "" {
			files = append(files, path)
		}
	}
	return files, nil
}

// HasRef reports whether a ref resolves in this repository.
func HasRef(root, ref string) bool {
	_, err := git(root, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

// UnpushedMessages returns the full message of every commit not yet on the
// upstream — the set whose wording the gate can still save. No upstream means
// no basis for "unpushed", so the last commit alone is checked: better one
// honest data point than a guess.
func UnpushedMessages(root string) []string {
	var msgs []string
	for _, c := range UnpushedCommits(root) {
		msgs = append(msgs, c.Message)
	}
	return msgs
}

// Commit is one unpushed commit: its abbreviated sha and its message.
//
// The sha exists so a finding can say WHICH commit it is about. Without it
// the attribution check reported "commit message carries an
// AI-attribution line" while the commit being written was clean and the
// offending one was three back — unarguable, and unfixable by the
// `--amend` the finding suggested. The only way past was `--no-verify`,
// which switches off everything else too (#185).
type Commit struct {
	SHA     string
	Subject string
	Message string
}

// UnpushedCommits is every commit this branch has that its upstream does
// not. With no upstream it is the last commit alone — the same fallback
// UnpushedMessages has always used, because a branch with no upstream has
// no answerable range.
func UnpushedCommits(root string) []Commit {
	rangeSpec := "@{upstream}..HEAD"
	if _, err := git(root, "rev-parse", "--verify", "@{upstream}"); err != nil {
		rangeSpec = "-1"
	}
	// %h and %B, separated by a unit byte so a message containing anything
	// at all cannot be mistaken for the delimiter.
	out, err := git(root, "log", "--format=%h%x1f%B%x00", rangeSpec)
	if err != nil {
		return nil
	}
	var commits []Commit
	for _, rec := range strings.Split(out, "\x00") {
		if strings.TrimSpace(rec) == "" {
			continue
		}
		sha, msg, found := strings.Cut(strings.TrimLeft(rec, "\n"), "\x1f")
		if !found {
			continue
		}
		if msg = strings.TrimSpace(msg); msg == "" {
			continue
		}
		commits = append(commits, Commit{
			SHA: strings.TrimSpace(sha), Subject: firstLineOf(msg), Message: msg,
		})
	}
	return commits
}

// --- the checks -------------------------------------------------------------

// Conflict markers. `<<<<<<< ` and `>>>>>>> ` at line start are unambiguous;
// a bare `=======` is a legitimate Markdown underline and is deliberately not
// matched on its own — the two ends of the conflict are always present when
// the middle is.
var conflictMarker = regexp.MustCompile(`(?m)^(<{7}( |$)|>{7}( |$))`)

// conflictAllow exempts a whole file from the conflict-marker check, and
// only with a reason after the token: a document whose subject IS merge
// conflicts cannot show a reader one otherwise, and the alternatives are
// all worse — mangle the example so a copy of it is broken, drop the
// topic, or turn the check off everywhere.
//
// File-scoped and explicit rather than "skip fenced code blocks", because
// a real conflict lands inside a fence often enough that skipping them
// would be a silent miss — and a check that quietly misses things is the
// failure this product argues against. Same spirit as gitleaks:allow: one
// greppable line, a stated reason, and nothing implicit.
var conflictAllow = regexp.MustCompile(`procoder:allow-conflict-markers(.*)`)

// conflictAllowed reports whether the file carries the exemption WITH a
// reason. The comment terminator is stripped first, so the reason in
// `<!-- procoder:allow-conflict-markers -->` reads as empty — a bare
// token is someone silencing the check, not documenting an exception.
func conflictAllowed(data []byte) bool {
	m := conflictAllow.FindSubmatch(data)
	if m == nil {
		return false
	}
	reason := strings.TrimSpace(string(m[1]))
	for _, close := range []string{"-->", "*/", "#}", "-}"} {
		reason = strings.TrimSpace(strings.TrimSuffix(reason, close))
	}
	return reason != ""
}

// ConflictMarkers finds merge-conflict markers left in changed files.
func ConflictMarkers(files []string) []Finding {
	var out []Finding
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil || !utf8ish(data) {
			continue
		}
		if conflictAllowed(data) {
			continue
		}
		for _, idx := range conflictMarker.FindAllIndex(data, -1) {
			line := 1 + bytes.Count(data[:idx[0]], []byte("\n"))
			out = append(out, Finding{File: f, Line: line, Blocking: true,
				Message: "merge conflict marker left in the file"})
		}
	}
	return out
}

// Junk: OS droppings, merge leftovers, caches, editor swap — the garbage a
// senior developer never lets into history. Deliberate build artifacts a repo
// ships on purpose (this repo's dist/ binaries) are NOT junk and NOT listed.
var junkNames = map[string]bool{
	".DS_Store": true, "Thumbs.db": true, "desktop.ini": true,
	".lycheecache": true, "npm-debug.log": true, "yarn-error.log": true,
}

var junkExts = map[string]bool{
	".orig": true, ".rej": true, ".pyc": true, ".pyo": true,
	".swp": true, ".swo": true, ".tmp": true, ".log": true,
}

// junkDirs anywhere in the path mean the whole entry is cache, not source.
var junkDirs = map[string]bool{
	"__pycache__": true, "node_modules": true, ".venv": true, "venv": true,
	".cache": true, ".pytest_cache": true, ".mypy_cache": true,
	".ruff_cache": true, ".tox": true, ".gradle": true,
}

// JunkFiles flags staged garbage: OS droppings, caches, editor swap, logs.
func JunkFiles(files []string) []Finding {
	var out []Finding
	for _, f := range files {
		base := filepath.Base(f)
		if junkNames[base] || junkExts[filepath.Ext(base)] || inJunkDir(f) {
			out = append(out, Finding{File: f, Blocking: true,
				Message: "junk file staged — caches and garbage never belong in a commit"})
		}
	}
	return out
}

func inJunkDir(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if junkDirs[part] {
			return true
		}
	}
	return false
}

// gitignoreNeeds maps an ecosystem marker present in the repo to the ignore
// entry its cache/artifact garbage requires. Reported, not blocked: the fix
// is one line in .gitignore and the finding says which.
var gitignoreNeeds = []struct{ marker, entry string }{
	{"go.mod", "*.test"},
	{"package.json", "node_modules"},
	{"pyproject.toml", "__pycache__"},
	{"requirements.txt", "__pycache__"},
	{"Cargo.toml", "target"},
	{"mkdocs.yml", "site"},
	// procoder's own derived state: per-machine, rebuilt on demand, and
	// written into the repository by procoder itself — so a repository
	// that follows this advice exactly must not still end up staging
	// procoder's droppings. These three are what this repository's own
	// .gitignore carries, and ProcoderArtifacts holds the two together.
	{".procoder", ".procoder/state/"},
	{".procoder", ".procoder/index/"},
	{".procoder", ".lycheecache"},
}

// ProcoderArtifacts is what procoder writes into a repository it governs:
// the code index, the per-machine state, and lychee's link cache from
// `procoder docs --external`. Every one of them needs an ignore entry, and
// this list is what a test compares against both the advice above and this
// repository's own .gitignore — because the advice went stale exactly once,
// by naming one of the three.
var ProcoderArtifacts = []string{".procoder/state/", ".procoder/index/", ".lycheecache"}

// IgnoreCoverage checks the repo has a .gitignore covering the garbage its
// present ecosystems generate.
func IgnoreCoverage(root string) []Finding {
	var present []struct{ marker, entry string }
	for _, n := range gitignoreNeeds {
		if _, err := os.Stat(filepath.Join(root, n.marker)); err == nil {
			present = append(present, n)
		}
	}
	if len(present) == 0 {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return []Finding{{Message: ".gitignore is missing — every repo ignores its garbage; add one covering the ecosystems present"}}
	}
	text := string(data)
	var out []Finding
	for _, n := range present {
		if !strings.Contains(text, n.entry) {
			out = append(out, Finding{File: filepath.Join(root, ".gitignore"),
				Message: fmt.Sprintf(".gitignore has no %q entry, but %s is present — its garbage will end up staged", n.entry, n.marker)})
		}
	}
	return out
}

// Oversized files about to be committed.
func Oversized(files []string, maxMB int) []Finding {
	limit := int64(maxMB) << 20
	var out []Finding
	for _, f := range files {
		if info, err := os.Stat(f); err == nil && info.Size() > limit {
			out = append(out, Finding{File: f, Blocking: true,
				Message: fmt.Sprintf("%d MB staged — over the %d MB limit; large artifacts belong in releases or LFS, not history", info.Size()>>20, maxMB)})
		}
	}
	return out
}

// AI attribution in commit messages. The work is the author's; a line crediting
// the tool is noise at best and a policy violation at worst, and it BLOCKS.
//
// aiIdentity is one machine author the gate recognises: the name the finding
// reports, the alternation that spots it inside a Co-Authored-By trailer, and
// the shapes it takes outside one. Both the pattern and the wording come from
// here, so a host is added in a single place.
type aiIdentity struct {
	name string
	// inTrailer matches only on a co-author line, never on prose: the
	// Co-Authored-By header predates AI coders by a decade and is correct for
	// pair programming, patches carried on someone's behalf and squashed
	// contributions. A gate that blocked every one of those is a gate its
	// users learn to route around, so the header alone is never the signal —
	// the identity named on it is.
	inTrailer string
	// anywhere matches the forms that are not trailers at all: a "generated
	// with" sentence, a vendor's noreply address in a body, a robot emoji.
	anywhere string
}

// aiIdentities is the whole rule. Vendor addresses are matched as the literal
// noreply mailbox rather than by domain, because a human employed by an AI lab
// co-authors commits from that same domain and is a person, not a tool.
//
// debt: a named list is stale the moment a host nobody has heard of ships a
// trailer of its own, and every miss is silent — the user has the policy
// without the enforcement, which is the failure this list exists to fix.
// Revisit when a host outside it is reported writing one, or when keeping it
// current stops being a matter of reading the host's own settings docs.
var aiIdentities = []aiIdentity{
	// Claude Code writes the trailer, the "Generated with" line and the
	// session emoji; `attribution` in settings.json turns all three off.
	// `anthropic` is deliberately NOT in the trailer pattern: it would match
	// a person at that company co-authoring from their own address, which is
	// the rule this list states — the vendor's noreply mailbox is the tool,
	// the domain is an employer.
	{name: "Claude", inTrailer: `claude`, anywhere: `generated with[^\n]*\bclaude\b|noreply@anthropic\.com`},
	// Codex injects the trailer from an account-level policy, so a user who
	// cannot administer the account meets it on every commit.
	{name: "Codex", inTrailer: `codex|noreply@openai\.com`},
	// VS Code's git extension, when git.addAICoAuthor is anything but "off".
	{name: "Copilot", inTrailer: `copilot`, anywhere: `\bcopilot@github\.com\b`},
	// Cursor's agent attributes with a "Made with Cursor" trailer rather than
	// a co-author line; its cloud agent adds a co-author on top.
	{name: "Cursor", inTrailer: `cursor`, anywhere: `made[ -]with:?[ \t]*cursor`},
	// The bot handle, not the bare first name: "Devin" belongs to plenty of
	// human co-authors and blocking them would teach users to distrust the gate.
	{name: "Devin", inTrailer: `devin-ai-integration`},
	{name: "Gemini", inTrailer: `gemini`},
	{name: "aider", inTrailer: `aider|noreply@aider\.chat`},
	// Not an identity but the sign of one: the emoji no human types into a
	// commit subject.
	{name: "a robot emoji", anywhere: `🤖`},
}

// pattern is the identity's two halves as one expression: case-insensitive,
// multi-line, so the trailer half anchors per line instead of per message.
func (a aiIdentity) pattern() string {
	var alts []string
	if a.inTrailer != "" {
		// The trailing run to end-of-line is what makes the finding quote the
		// whole trailer: without it the match stops mid-address and the reader
		// is shown half the line they are being asked to delete.
		alts = append(alts, `^[ \t]*co-authored-by:.*\b(?:`+a.inTrailer+`)\b[^\n]*`)
	}
	if a.anywhere != "" {
		alts = append(alts, a.anywhere)
	}
	return `(?im)` + strings.Join(alts, "|")
}

var aiMatchers = func() []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(aiIdentities))
	for i, a := range aiIdentities {
		out[i] = regexp.MustCompile(a.pattern())
	}
	return out
}()

// Where the per-host settings that stop the trailer at its source are written
// down. The finding carries it because amending is only half the fix: the host
// that added the trailer adds it again on the next commit, and a user who only
// ever reads the gate's output would hit the same wall every time.
const attributionRemedyURL = "https://procoder.azrty.com/portability/#the-trailer-your-host-adds"

// Attribution finds AI-attribution lines in the given commit messages.
//
// Kept for callers that have only text. Where the commits are available,
// AttributionIn says which one — see the note on Commit.
func Attribution(messages []string) []Finding {
	commits := make([]Commit, 0, len(messages))
	for _, m := range messages {
		commits = append(commits, Commit{Message: m, Subject: firstLineOf(m)})
	}
	return AttributionIn(commits)
}

// AttributionIn finds AI-attribution lines and names the commit carrying
// each one.
//
// Naming it is the whole point. The check runs over every unpushed commit,
// so a trailer three commits back blocks every commit after it — and the
// finding used to open "commit message carries an AI-attribution line",
// which reads as being about the commit you are writing. People read that
// against a clean message, could not argue with it, and could not fix it
// with the `--amend` it suggested, because the offending commit was not
// the one being amended. `--no-verify` was the only way through, and it
// switches off everything else too (#185).
func AttributionIn(commits []Commit) []Finding {
	var out []Finding
	for _, c := range commits {
		// The finding names the identity as well as the line, so someone who
		// believes it is wrong can say which entry is wrong and argue with it,
		// instead of reading a blocked commit as the gate being mysterious.
		name, loc := matchAIIdentity(c.Message)
		if loc == "" {
			continue
		}
		where := "the commit message"
		fix := "remove it (git commit --amend)"
		if c.SHA != "" {
			where = fmt.Sprintf("commit %s (%q)", c.SHA, strings.TrimSpace(c.Subject))
			fix = fmt.Sprintf("rewrite that commit (git rebase -i %s^) — amending HEAD will not clear it unless HEAD is the one named", c.SHA)
		}
		out = append(out, Finding{Blocking: true,
			Message: fmt.Sprintf("%s carries an AI-attribution line crediting %s: %q — the work is the author's; %s. If the host added it, it will add it again on the next commit — turn it off at the source: %s",
				where, name, strings.TrimSpace(firstLineOf(loc)), fix, attributionRemedyURL)})
	}
	return out
}

// matchAIIdentity reports the first recognised identity in the text and the
// line that carried it.
func matchAIIdentity(text string) (name, line string) {
	for i, re := range aiMatchers {
		if loc := re.FindString(text); loc != "" {
			return aiIdentities[i].name, loc
		}
	}
	return "", ""
}

// AttributionInMessage checks the message a commit is ABOUT to be made
// with, rather than the messages of commits already made.
//
// Both halves are needed and they catch different things. The unpushed
// range catches a trailer that already landed; this catches one before it
// does, which is the only moment it can still be removed by editing the
// message rather than by rewriting history.
//
// Missing this is what made #185 as bad as it was: the trailer was never
// stopped on the way in, so it landed, and then blocked every subsequent
// commit from the far side. Raised in review on #187.
func AttributionInMessage(message string) []Finding {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	name, loc := matchAIIdentity(message)
	if loc == "" {
		return nil
	}
	// Its own wording rather than AttributionIn's. `git commit --amend` is
	// the right advice for a commit that exists and the wrong advice for
	// one that does not — and telling somebody to amend a commit they have
	// not made is the kind of unfollowable instruction that teaches
	// `--no-verify` (#185).
	return []Finding{{Blocking: true,
		Message: fmt.Sprintf("the commit message being written carries an AI-attribution line crediting %s: %q — the work is the author's; remove the line from the message before committing. If the host added it, it will add it again on the next commit — turn it off at the source: %s",
			name, strings.TrimSpace(firstLineOf(loc)), attributionRemedyURL)}}
}

// ScrubText applies the same attribution patterns to arbitrary text — the PR
// skill runs a drafted body through this before anything is sent.
func ScrubText(text string) []Finding {
	return Attribution([]string{text})
}

// Commit subject shape: non-empty, <=72 chars, blank line before any body.
// Reported, never blocking — the commit template makes this mostly automatic.
func SubjectShape(messages []string) []Finding {
	var out []Finding
	for _, m := range messages {
		lines := strings.Split(m, "\n")
		subject := strings.TrimSpace(lines[0])
		switch {
		case subject == "":
			out = append(out, Finding{Message: "commit has an empty subject line"})
		case len(subject) > 72:
			out = append(out, Finding{Message: fmt.Sprintf("commit subject is %d chars (aim for <=72): %q", len(subject), subject[:40]+"...")})
		}
		if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
			out = append(out, Finding{Message: fmt.Sprintf("no blank line between subject and body: %q", subject)})
		}
	}
	return out
}

// OnDefaultBranch reports (or blocks, per config) work happening directly on
// the default branch.
func OnDefaultBranch(root string, block bool) []Finding {
	current, def := CurrentBranch(root), DefaultBranch(root)
	if current == "" || def == "" || current != def {
		return nil
	}
	return []Finding{{Blocking: block,
		Message: fmt.Sprintf("working directly on the default branch (%s) — a senior habit is a branch per change; set [git] default_branch_policy in .procoder/config.toml to choose report or block", def)}}
}

func utf8ish(data []byte) bool {
	return !bytes.Contains(data[:min(len(data), 8000)], []byte{0})
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RepoRel makes a path repo-relative, whether it arrived absolute or
// already relative to the repository root.
//
// filepath.Rel refuses to compare an absolute base with a relative
// target, so a caller that hands it whatever it was given silently drops
// every relative path. That is how `procoder check somefile.py` stopped
// being scanned while `procoder check` on the same file — which resolves
// to an absolute path through git — was scanned normally: the same file,
// two verdicts, decided by how it was named.
//
// The second return is false when the path genuinely leaves the
// repository. A directory named "..foo" is inside it.
func RepoRel(root, path string) (string, bool) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
