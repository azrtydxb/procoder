// Package security is domain 1: secrets, SAST, and dependency vulns, each
// from the field's canonical tool. Secrets block always; SAST blocks at
// ERROR severity; dependency vulns block at high/critical. Everything else
// reports for the agent to judge (P-CONTROL). Rules prose lives in
// .procoder/security/RULES.md (D-OVERRIDE).
package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"procoder/internal/config"
	"procoder/internal/gitx"
	"procoder/internal/tools"
)

// RulesPath is the repo's security rules file.
const RulesPath = ".procoder/security/RULES.md"

// Gitleaks finds committed and about-to-be-committed secrets.
var Gitleaks = &tools.Tool{
	Name:        "gitleaks",
	Install:     "brew install gitleaks",
	VersionArgs: []string{"version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "gitleaks"}},
	},
}

// Semgrep is the SAST pass, community rulesets via --config auto.
var Semgrep = &tools.Tool{
	Name:        "semgrep",
	Install:     "brew install semgrep   (or: pipx install semgrep)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "semgrep"}},
		{Manager: "pipx", Args: []string{"install", "semgrep"}},
		{Manager: "pip3", Args: []string{"install", "--user", "semgrep"}},
	},
}

// OsvScanner checks dependency manifests against the OSV database.
var OsvScanner = &tools.Tool{
	Name:        "osv-scanner",
	Install:     "brew install osv-scanner   (or: go install github.com/google/osv-scanner/v2/cmd/osv-scanner@latest)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "osv-scanner"}},
		{Manager: "go", Args: []string{"install", "github.com/google/osv-scanner/v2/cmd/osv-scanner@latest"}},
	},
}

const (
	secretsTimeout = 60 * time.Second
	deepTimeout    = 10 * time.Minute
	// vulnBlockScore is the CVSS score at and above which a dependency
	// vulnerability blocks (the agreed high/critical line).
	vulnBlockScore = 7.0
)

// Secrets scans the given paths (files or directories) with gitleaks.
// A secret is BLOCKING, always — and its value is never echoed.
func Secrets(root string, paths []string) []gitx.Finding {
	bin := tools.Resolve(Gitleaks, root)
	if bin == "" {
		return []gitx.Finding{{Blocking: true,
			Message: "NOT checked — gitleaks is not installed; run `procoder init` (security)"}}
	}
	var out []gitx.Finding
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		out = append(out, scanOne(bin, root, p)...)
	}
	return out
}

// scanOne runs gitleaks over one source path. src is handed to gitleaks
// verbatim and workdir is where it runs from, so a relative src keeps the
// report's paths — and therefore .gitleaksignore fingerprints — relative.
func scanOne(bin, workdir, src string) []gitx.Finding {
	var out []gitx.Finding
	report := filepath.Join(os.TempDir(), fmt.Sprintf("procoder-gitleaks-%d.json", time.Now().UnixNano()))
	common := []string{"--report-format", "json", "--report-path", report,
		"--no-banner", "--exit-code", "1"}
	ctx, cancel := context.WithTimeout(context.Background(), secretsTimeout)
	cmd := exec.CommandContext(ctx, bin, append([]string{"dir", src}, common...)...) // nosemgrep -- resolved from the fixed tool table, never user input
	cmd.Dir = workdir
	var errb bytes.Buffer
	cmd.Stderr = &errb
	runErr := cmd.Run()
	cancel()
	if ctx.Err() == context.DeadlineExceeded {
		return []gitx.Finding{{File: src, Blocking: true,
			Message: fmt.Sprintf("gitleaks gave no answer in %s — the path was NOT checked (security)", secretsTimeout)}}
	}
	if bytes.Contains(errb.Bytes(), []byte(`unknown command "dir"`)) {
		// gitleaks before v8.19 has no `dir`; the same scan is spelled
		// `detect --no-git --source` there — apt still ships that era
		ctx2, cancel2 := context.WithTimeout(context.Background(), secretsTimeout)
		legacy := exec.CommandContext(ctx2, bin, append([]string{"detect", "--no-git", "--source", src}, common...)...) // nosemgrep -- resolved from the fixed tool table, never user input
		legacy.Dir = workdir
		errb.Reset()
		legacy.Stderr = &errb
		runErr = legacy.Run()
		cancel2()
		if ctx2.Err() == context.DeadlineExceeded {
			return []gitx.Finding{{File: src, Blocking: true,
				Message: fmt.Sprintf("gitleaks gave no answer in %s — the path was NOT checked (security)", secretsTimeout)}}
		}
	}
	raw, readErr := os.ReadFile(report)
	os.Remove(report)
	if readErr != nil {
		return []gitx.Finding{{File: src, Blocking: true,
			Message: "gitleaks produced no report — the path was NOT checked: " + firstLine(errb.String()) + " (security)"}}
	}
	var leaks []struct {
		File      string `json:"File"`
		StartLine int    `json:"StartLine"`
		RuleID    string `json:"RuleID"`
	}
	if json.Unmarshal(raw, &leaks) != nil {
		return []gitx.Finding{{File: src, Blocking: true,
			Message: "gitleaks report unreadable — the path was NOT checked (security)"}}
	}
	for _, l := range leaks {
		// gitleaks echoes the path as given for file scans and prefixes
		// it for directory scans — normalise to one shape
		loc := l.File
		if !filepath.IsAbs(loc) && loc != src {
			loc = filepath.Join(src, loc)
		}
		// the finding names the rule and location — NEVER the secret
		out = append(out, gitx.Finding{File: loc, Line: l.StartLine, Blocking: true,
			Message: fmt.Sprintf("secret detected (%s) — remove it and rotate the credential; committed secrets live in history forever (security)", l.RuleID)})
	}
	_ = runErr // exit 1 just means leaks; they are already collected
	return out
}

// SecretsTree scans the audit's file set — tracked plus untracked,
// gitignored excluded — in ONE gitleaks pass. gitleaks's dir mode walks
// everything under a path with no notion of gitignore, so the files are
// mirrored (hardlink, copy fallback) into a temp tree and THAT is scanned,
// relative to its own root. That is what (1) keeps gitignored content out
// of the scan, which is also what keeps real repos inside the time budget,
// and (2) makes finding fingerprints repo-relative, so a committed
// .gitleaksignore (mirrored along with the rest) works on every checkout.
func SecretsTree(root string, files []string) []gitx.Finding {
	bin := tools.Resolve(Gitleaks, root)
	if bin == "" {
		return []gitx.Finding{{Blocking: true,
			Message: "NOT checked — gitleaks is not installed; run `procoder init` (security)"}}
	}
	tmp, err := os.MkdirTemp("", "procoder-secrets-")
	if err != nil {
		return []gitx.Finding{{Blocking: true,
			Message: "secrets NOT checked — cannot stage the scan tree: " + err.Error() + " (security)"}}
	}
	defer os.RemoveAll(tmp)
	var out []gitx.Finding
	staged, skipped := 0, 0
	for _, f := range files {
		rel, err := filepath.Rel(root, f)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		info, err := os.Lstat(f)
		if err != nil || !info.Mode().IsRegular() {
			continue // symlinks and specials have no content of their own
		}
		dst := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			skipped++
			continue
		}
		if os.Link(f, dst) != nil { // cross-device temp: fall back to a copy
			data, err := os.ReadFile(f)
			if err != nil || os.WriteFile(dst, data, 0o600) != nil {
				skipped++
				continue
			}
		}
		staged++
	}
	if skipped > 0 {
		out = append(out, gitx.Finding{Blocking: true,
			Message: fmt.Sprintf("%d file(s) could not be staged for the secrets scan — they were NOT checked (security)", skipped)})
	}
	if staged == 0 {
		return out
	}
	for _, f := range scanOne(bin, tmp, ".") {
		// map the mirror's relative paths back into the repository
		if f.File == "." || f.File == "" {
			f.File = ""
		} else if !filepath.IsAbs(f.File) {
			f.File = filepath.Join(root, f.File)
		}
		out = append(out, f)
	}
	return out
}

// SecretsChangedFiles is the gate's slice: each changed file scanned.
func SecretsChangedFiles(root string, files []string) []gitx.Finding {
	var existing []string
	for _, f := range files {
		if info, err := os.Stat(f); err == nil && !info.IsDir() {
			existing = append(existing, f)
		}
	}
	if len(existing) == 0 {
		return nil
	}
	return Secrets(root, existing)
}

// Sast runs semgrep with the community rulesets over the repository.
// ERROR severity blocks; WARNING and INFO report.
// severityAtLeast reports whether a finding's severity is at or above the
// bar. An unknown severity from the tool never blocks silently: it is
// ranked below everything, so it still reports and a human still reads it.
func severityAtLeast(found, bar string) bool {
	rank := map[string]int{"INFO": 0, "WARNING": 1, "ERROR": 2}
	f, ok := rank[found]
	if !ok {
		return false
	}
	b, ok := rank[bar]
	if !ok {
		b = rank["ERROR"]
	}
	return f >= b
}

func Sast(root string) []gitx.Finding { return sast(root) }

// SastChanged is the commit gate's SAST leg: the same scan, given the
// files the commit contains instead of the whole tree.
//
// The scan itself is the same whole-tree scan `security --deep` runs, and
// the SCOPING is applied to its findings rather than to its targets.
//
// That is not the obvious way round, and the obvious way is wrong.
// Handing semgrep an explicit list of files makes it scan files it
// otherwise skips — its own default selection is bypassed by naming a
// target — so the gate reported a finding in a _test.go file that
// `security --deep` had never once mentioned. A developer blocked by a
// finding CI does not have is worse than a slower gate.
//
// The cost of doing it this way is about three seconds on this
// repository: measured, the whole tree is 9s against 6.1s for two named
// files. Little of that is scanning. semgrep's time goes on `--config
// auto` loading rules, which is fixed — a single one-line file still
// costs 4.7s — so naming fewer targets was never what made this
// affordable. What makes it affordable is that a commit is not a
// keystroke.
func SastChanged(root string, files []string) []gitx.Finding {
	changed := map[string]bool{}
	for _, f := range files {
		// gitx.RepoRel takes the path however it arrived — absolute from
		// git, or relative because a person typed it — so the same file
		// cannot get two verdicts depending on how it was named.
		if rel, ok := gitx.RepoRel(root, f); ok {
			changed[rel] = true
		}
	}
	if len(changed) == 0 {
		return nil
	}
	var out []gitx.Finding
	for _, f := range sast(root) {
		// A finding that could not be placed — a scan that did not run,
		// unreadable output — has no path and belongs to the commit
		// whatever it touched.
		if f.File == "" || changed[filepath.ToSlash(f.File)] {
			out = append(out, f)
		}
	}
	return out
}

func sast(root string) []gitx.Finding {
	blocksAt := config.Load(root).SastBlocksAt
	bin := tools.Resolve(Semgrep, root)
	if bin == "" {
		return []gitx.Finding{{Blocking: true,
			Message: "NOT checked — semgrep is not installed; run `procoder init` (security)"}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), deepTimeout)
	defer cancel()
	// The tree, always. Naming targets is what made semgrep scan files its
	// own default selection skips, and now that nothing does it there is
	// no argv built from file names — so no filename can be read as a
	// flag, and there is no separator to remember.
	cmd := exec.CommandContext(ctx, bin, "scan", "--config", "auto", "--json", "--quiet", ".") // nosemgrep -- resolved from the fixed tool table, never user input
	cmd.Dir = root
	var buf, errb bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &errb
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return []gitx.Finding{{Blocking: true,
			Message: fmt.Sprintf("semgrep gave no answer in %s — SAST was NOT run (security)", deepTimeout)}}
	}
	var rep struct {
		Results []struct {
			Path  string `json:"path"`
			Start struct {
				Line int `json:"line"`
			} `json:"start"`
			CheckID string `json:"check_id"`
			Extra   struct {
				Severity string `json:"severity"`
				Message  string `json:"message"`
			} `json:"extra"`
		} `json:"results"`
	}
	if json.Unmarshal(buf.Bytes(), &rep) != nil {
		return []gitx.Finding{{Blocking: true,
			Message: "semgrep output unreadable — SAST was NOT run: " + why(errb.String(), err) + " (security)"}}
	}
	var out []gitx.Finding
	for _, r := range rep.Results {
		// The bar is the repository's, not procoder's. ERROR by default —
		// the level semgrep reserves for findings it is confident about —
		// and a team that wants WARNING to stop a commit says so in
		// [security] sast_blocks_at. Lowering it is a strengthening and
		// prints nothing; raising it is a relaxation and prints on every
		// gate run.
		blocking := severityAtLeast(r.Extra.Severity, blocksAt)
		msg := r.Extra.Message
		if len(msg) > 200 {
			msg = msg[:200]
		}
		out = append(out, gitx.Finding{File: r.Path, Line: r.Start.Line, Blocking: blocking,
			Message: fmt.Sprintf("%s [%s %s] (security)", msg, r.Extra.Severity, shortCheck(r.CheckID))})
	}
	return out
}

// DepManifests are the dependency files osv-scanner accepts via -L — the
// ONE list Deps scans and doctor's osv-scanner requirement triggers on, so
// the two can never disagree about which repos need the scanner.
// Podfile.lock is deliberately absent: osv has no extractor for it and one
// bad -L fails the whole invocation (verified against 2.5.1).
var DepManifests = []string{"go.mod", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
	// pyproject.toml is deliberately absent: it declares ranges rather
	// than pinned versions, so osv-scanner has no extractor for it and
	// exits non-zero WITHOUT emitting JSON — which took every other
	// manifest in the same invocation down with it. A repository with a
	// pyproject.toml and no lock file is reported as a gap below, the
	// same way a bare package.json is, rather than scanned and lost.
	"requirements.txt", "poetry.lock", "Pipfile.lock",
	"Cargo.lock", "composer.lock", "Gemfile.lock",
	"pom.xml", "gradle.lockfile", "buildscript-gradle.lockfile",
	"packages.lock.json", "Package.resolved",
	"mix.lock", "pubspec.lock"}

// Deps runs osv-scanner over the repository's manifests. Vulnerabilities at
// or above the high/critical CVSS line block; the rest report.
func Deps(root string) []gitx.Finding {
	bin := tools.Resolve(OsvScanner, root)
	if bin == "" {
		return []gitx.Finding{{Blocking: true,
			Message: "NOT checked — osv-scanner is not installed; run `procoder init` (security)"}}
	}
	// manifests are named explicitly: osv's own directory walker trusts
	// git metadata and comes back empty inside git worktrees
	var margs []string
	for _, m := range manifestsIn(root) {
		margs = append(margs, "-L", m)
	}
	// a bare package.json is not scannable by osv (it needs a lockfile's
	// pinned versions); one that DECLARES dependencies without any
	// lockfile is an honest gap, one without dependencies has nothing to
	// check and stays silent
	var out []gitx.Finding
	for _, gap := range npmGaps(root) {
		out = append(out, gitx.Finding{Blocking: true, File: gap,
			Message: "npm dependencies NOT checked — this package.json declares dependencies but no lockfile exists beside it for osv-scanner; generate package-lock.json (security)"})
	}
	for _, gap := range pythonGaps(root) {
		out = append(out, gitx.Finding{Blocking: true, File: gap,
			Message: "python dependencies NOT checked — this pyproject.toml declares dependencies but no lock file exists beside it for osv-scanner; generate poetry.lock, Pipfile.lock or requirements.txt (security)"})
	}
	if len(margs) == 0 {
		if len(out) > 0 {
			return out
		}
		return []gitx.Finding{{Message: "no dependency manifests found — nothing for osv-scanner to check (security)"}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), deepTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, append([]string{"--format", "json"}, margs...)...) // nosemgrep -- resolved from the fixed tool table, never user input
	cmd.Dir = root
	var buf, errb bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &errb
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return []gitx.Finding{{Blocking: true,
			Message: fmt.Sprintf("osv-scanner gave no answer in %s — dependencies were NOT checked (security)", deepTimeout)}}
	}
	var rep struct {
		Results []struct {
			Packages []struct {
				Package struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"package"`
				Vulnerabilities []struct {
					ID string `json:"id"`
				} `json:"vulnerabilities"`
				Groups []struct {
					MaxSeverity string `json:"max_severity"`
				} `json:"groups"`
			} `json:"packages"`
		} `json:"results"`
	}
	if json.Unmarshal(buf.Bytes(), &rep) != nil {
		return append(out, gitx.Finding{Blocking: true,
			Message: "osv-scanner output unreadable — dependencies were NOT checked: " + why(errb.String(), err) + " (security)"})
	}
	for _, r := range rep.Results {
		for _, p := range r.Packages {
			if len(p.Vulnerabilities) == 0 {
				continue
			}
			max := 0.0
			for _, g := range p.Groups {
				if v, err := strconv.ParseFloat(g.MaxSeverity, 64); err == nil && v > max {
					max = v
				}
			}
			out = append(out, gitx.Finding{Blocking: max >= vulnBlockScore,
				Message: fmt.Sprintf("%s %s has %d known vulnerability(s), max severity %.1f — upgrade it (security)",
					p.Package.Name, p.Package.Version, len(p.Vulnerabilities), max)})
		}
	}
	return out
}

// hasNpmDepsWithoutLockfile: package.json declares dependencies but no
// npm lockfile exists — osv cannot pin versions, so scanning is impossible.
// npmGaps lists every package.json that declares dependencies with no
// lockfile beside it — one per package, because a monorepo has one per
// package and checking only the repository root reports the first and
// stays silent about the rest.
func npmGaps(root string) []string {
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable directory hides its own packages, not the others
		}
		if info.IsDir() {
			if p != root && manifestDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() != "package.json" {
			return nil
		}
		if hasNpmDepsWithoutLockfile(root, filepath.Dir(p)) {
			if rel, ok := gitx.RepoRel(root, p); ok {
				out = append(out, rel)
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// hasNpmDepsWithoutLockfile answers for ONE directory: a package.json
// there declaring dependencies that nothing can pin, so osv cannot scan
// them.
//
// A lockfile beside it answers that. So does a workspace root ABOVE it:
// pnpm, npm and yarn all keep ONE lockfile at the workspace root and none
// in the members, so a member declaring dependencies without a lockfile of
// its own is the normal layout, not a gap - the root lockfile resolves it
// and IS scanned. Reporting it anyway told a pnpm monorepo its packages
// were unchecked when every one of them had been, and the advice that came
// with it ("generate package-lock.json") would have put a second, npm
// lockfile next to the pnpm one.
//
// Walking up without checking membership would be wrong the other way: a
// nested package that the root does NOT list as a member is not covered by
// that lockfile, and staying silent about it would report a scan that never
// happened.
func hasNpmDepsWithoutLockfile(repoRoot, dir string) bool {
	if npmLockfileIn(dir) {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var m struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return true // unparseable manifest: assume deps, stay honest
	}
	if len(m.Dependencies)+len(m.DevDependencies) == 0 {
		return false
	}
	return !npmWorkspaceCovers(repoRoot, dir)
}

// npmLockfileIn: one of the three npm-family lockfiles sits in this
// directory. osv-scanner reads all three.
func npmLockfileIn(dir string) bool {
	for _, lock := range []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, lock)); err == nil {
			return true
		}
	}
	return false
}

// npmWorkspaceCovers: some ancestor of dir, at or below the repository
// root, holds a lockfile AND lists dir among its workspace members. The
// walk stops at repoRoot so a lockfile outside the repository - a parent
// checkout, the user's home - never silences a finding inside it.
func npmWorkspaceCovers(repoRoot, dir string) bool {
	rel, err := filepath.Rel(repoRoot, dir)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false // dir is the root itself, or not under it at all
	}
	for anc := filepath.Dir(dir); ; anc = filepath.Dir(anc) {
		if npmLockfileIn(anc) {
			if member, err := filepath.Rel(anc, dir); err == nil &&
				workspaceGlobsMatch(npmWorkspaceGlobs(anc), filepath.ToSlash(member)) {
				return true
			}
		}
		// stop at the repository root; the second guard is for a repoRoot
		// this walk somehow never meets, which must not spin forever
		if anc == repoRoot || filepath.Dir(anc) == anc {
			return false
		}
	}
}

// npmWorkspaceGlobs reads the member patterns a directory declares, in
// whichever of the three shapes its package manager uses. Nil when the
// directory is not a workspace root.
func npmWorkspaceGlobs(dir string) []string {
	var globs []string
	// pnpm keeps them in their own file, and accepts either extension.
	for _, name := range []string{"pnpm-workspace.yaml", "pnpm-workspace.yml"} {
		if raw, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			globs = append(globs, pnpmWorkspacePackages(string(raw))...)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return globs
	}
	// npm and yarn take the plain array; yarn also takes an object with
	// the array under "packages", which fails the array unmarshal whole,
	// hence two attempts rather than one.
	var flat struct {
		Workspaces []string `json:"workspaces"`
	}
	if json.Unmarshal(raw, &flat) == nil {
		return append(globs, flat.Workspaces...)
	}
	var nested struct {
		Workspaces struct {
			Packages []string `json:"packages"`
		} `json:"workspaces"`
	}
	if json.Unmarshal(raw, &nested) == nil {
		return append(globs, nested.Workspaces.Packages...)
	}
	return globs
}

// pnpmWorkspacePackages pulls the `packages:` entries out of a
// pnpm-workspace.yaml. Hand-rolled because procoder carries no
// dependencies, and this file's shape is a list of strings under one
// known key - both the block form and the inline flow form.
func pnpmWorkspacePackages(text string) []string {
	var out []string
	inList := false
	for _, line := range strings.Split(text, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !inList {
			rest, ok := strings.CutPrefix(trimmed, "packages:")
			if !ok {
				continue
			}
			rest = strings.TrimSpace(rest)
			if strings.HasPrefix(rest, "[") {
				for _, item := range strings.Split(strings.Trim(rest, "[]"), ",") {
					if v := unquoteYAMLScalar(item); v != "" {
						out = append(out, v)
					}
				}
				continue
			}
			inList = true
			continue
		}
		if item, ok := strings.CutPrefix(trimmed, "-"); ok {
			if v := unquoteYAMLScalar(item); v != "" {
				out = append(out, v)
			}
			continue
		}
		// an unindented line is the next top-level key: the list ended
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inList = false
		}
	}
	return out
}

// unquoteYAMLScalar strips the surrounding whitespace and the one layer of
// quotes a workspace pattern may carry.
func unquoteYAMLScalar(s string) string {
	s = strings.TrimSpace(s)
	for _, q := range []string{`"`, "'"} {
		if len(s) >= 2 && strings.HasPrefix(s, q) && strings.HasSuffix(s, q) {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// workspaceGlobsMatch: rel names a member of this workspace. A pattern
// starting with "!" excludes, and excluding wins however the patterns are
// ordered - pnpm and npm both read the negations as a filter over the
// whole set, not as a step in sequence.
func workspaceGlobsMatch(globs []string, rel string) bool {
	matched := false
	for _, g := range globs {
		if neg, ok := strings.CutPrefix(g, "!"); ok {
			if matchWorkspaceGlob(neg, rel) {
				return false
			}
			continue
		}
		if matchWorkspaceGlob(g, rel) {
			matched = true
		}
	}
	return matched
}

// matchWorkspaceGlob matches one workspace pattern against a path relative
// to the workspace root. path.Match cannot do it alone: it has no "**", and
// its "*" would happily match across a separator here.
func matchWorkspaceGlob(pattern, rel string) bool {
	pattern = strings.TrimPrefix(pattern, "./")
	pattern = strings.Trim(pattern, "/")
	if pattern == "" {
		return false
	}
	return matchGlobSegments(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}

func matchGlobSegments(pattern, segments []string) bool {
	if len(pattern) == 0 {
		return len(segments) == 0
	}
	if pattern[0] == "**" {
		// "**" spans zero or more segments, so every split has to be tried
		for i := 0; i <= len(segments); i++ {
			if matchGlobSegments(pattern[1:], segments[i:]) {
				return true
			}
		}
		return false
	}
	if len(segments) == 0 {
		return false
	}
	if ok, err := path.Match(pattern[0], segments[0]); err != nil || !ok {
		return false
	}
	return matchGlobSegments(pattern[1:], segments[1:])
}

// pythonGaps finds every pyproject.toml declaring dependencies with no
// lock file beside it — the Python half of the honest gap npm already
// gets. Without it, dropping pyproject.toml from DepManifests would mean
// a Python repository is told its dependencies are clean when nothing
// looked at them.
func pythonGaps(root string) []string {
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable directory hides its own packages, not the others
		}
		if info.IsDir() {
			if p != root && manifestDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() != "pyproject.toml" {
			return nil
		}
		if hasPythonDepsWithoutLockfile(filepath.Dir(p)) {
			if rel, ok := gitx.RepoRel(root, p); ok {
				out = append(out, rel)
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// hasPythonDepsWithoutLockfile answers for ONE directory: a pyproject.toml
// there declaring dependencies with nothing pinned beside it.
func hasPythonDepsWithoutLockfile(root string) bool {
	for _, lock := range []string{"poetry.lock", "Pipfile.lock", "requirements.txt", "uv.lock", "pdm.lock"} {
		if _, err := os.Stat(filepath.Join(root, lock)); err == nil {
			return false
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		return false
	}
	// A dependency table under any of the layouts in use: PEP 621's
	// `dependencies = [...]`, poetry's `[tool.poetry.dependencies]`, and
	// PEP 735's `[dependency-groups]`. Matching the key rather than
	// parsing TOML keeps this a read, which is what the gate can afford —
	// but the key alone is not the answer. `dependencies = []` declares
	// nothing, and reporting a gap there tells a project with no
	// dependencies that its dependencies were not checked.
	return pyDeps.MatchString(string(raw))
}

// pyDeps matches a dependency declaration that actually has something in
// it: a PEP 621 or PEP 735 list with a first element, or a poetry table
// with a first key. An empty list or an empty table is not a dependency
// set, and neither is the word appearing in prose.
//
// The key half is spelled out rather than left as `[a-zA-Z0-9_-]+`. That
// alternation matched ANY non-empty list assignment, so a project with
// `dependencies = []` and `keywords = ["cli"]` — no dependencies at all —
// was blocked with "python dependencies NOT checked". A blocking false
// positive about a file the reader has done nothing wrong with is worse
// than the gap it was written to close.
var pyDeps = regexp.MustCompile(
	// PEP 621: dependencies = ["x"] / optional-dependencies = ["x"]
	`(?m)^\s*(?:optional-)?dependencies\s*=\s*\[\s*(?:#[^\n]*)?\n?\s*["']` +
		// a table whose entries ARE the dependencies, with a first key
		`|(?s)\[(?:tool\.poetry\.(?:dev-)?dependencies` +
		`|tool\.poetry\.group\.[^.\]]+\.dependencies` +
		`|project\.optional-dependencies` +
		`|dependency-groups)\][^\[]*?\S\s*=`)

// why reports why a tool gave no answer. The tool's own last line of
// stderr is the diagnosis: scanners log their progress first and the
// reason they gave up last, so quoting the FIRST line told the reader
// "dependencies were NOT checked: Starting filesystem walk for root: /" —
// alarming, and not the reason.
//
// The exit status is the fallback, not the answer. It is appended after
// stderr at the call sites, so folding the two together and taking the
// last line would report "exit status 127" every time and lose the
// diagnosis entirely — which is why stderr is asked first, on its own.
func why(stderr string, err error) string {
	if d := lastLine(stderr); d != "" {
		return d
	}
	if s := lastLine(errStr(err)); s != "" {
		return s
	}
	return "no output"
}

// lastLine is the last non-empty line, trimmed and capped. Empty when
// there is none, so callers can fall back rather than print nothing.
func lastLine(s string) string {
	out := ""
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = t
		}
	}
	if len(out) > 160 {
		out = out[:160]
	}
	return out
}

func shortCheck(id string) string {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '.' {
			return id[i+1:]
		}
	}
	return id
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			s = s[:i]
			break
		}
	}
	if len(s) > 160 {
		s = s[:160]
	}
	if s == "" {
		return "no output"
	}
	return s
}

// DepsChanged is the commit gate's dependency-vulnerability leg: the same
// scan, run only when the commit touches something that could change the
// dependency graph.
//
// The scan answers about the manifests, not about the files around them,
// so re-running it on a commit that edits a comment would report the same
// vulnerabilities forever at a cost of nearly a second each time. A commit
// that changes a manifest is exactly the moment the answer can change.
//
// All manifests are scanned, not only the one that changed: a lockfile
// edit moves versions the other manifests resolve against, and a scan of
// half a graph is a scan nobody can trust.
func DepsChanged(root string, files []string) []gitx.Finding {
	if !touchesManifest(root, files) {
		return nil
	}
	return Deps(root)
}

// touchesManifest reports whether any changed file is a dependency
// manifest osv-scanner reads, or the package.json whose absent lockfile
// Deps reports on.
func touchesManifest(root string, files []string) bool {
	watched := map[string]bool{"package.json": true}
	for _, m := range DepManifests {
		watched[m] = true
	}
	for _, f := range files {
		rel, ok := gitx.RepoRel(root, f)
		if !ok {
			continue
		}
		// By base name: a manifest in a subdirectory is still a manifest,
		// and a monorepo keeps one per package.
		if watched[path.Base(rel)] {
			return true
		}
	}
	return false
}

// manifestDirs are directories a dependency manifest may sit in without
// belonging to this repository: vendored copies and installed packages
// carry their own, and scanning them reports vulnerabilities in code
// nobody here can change.
var manifestDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"target": true, "__pycache__": true, ".venv": true,
}

// manifestsIn finds every dependency manifest in the repository, not only
// the ones at its root.
//
// The root-only version was the shape of the whole feature's failure: a
// monorepo keeps one manifest per package, so `security --deep` scanned
// the top level and reported clean over every package beneath it — and
// once the gate began triggering on a nested manifest, a commit paid for
// a scan that could not look at the file it was triggered by.
//
// Paths are returned repo-relative because that is what osv-scanner's -L
// wants alongside cmd.Dir = root.
func manifestsIn(root string) []string {
	names := map[string]bool{}
	for _, m := range DepManifests {
		names[m] = true
	}
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			// An unreadable directory is not a reason to scan nothing; the
			// manifests that ARE readable still get scanned, and osv
			// reports on what it was given.
			return nil //nolint:nilerr // a walk that cannot enter one directory still covers the rest
		}
		if info.IsDir() {
			if p != root && manifestDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !names[info.Name()] {
			return nil
		}
		if rel, ok := gitx.RepoRel(root, p); ok {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// SecretsInDiff is SecretsChangedFiles narrowed to the lines this commit
// wrote — the form the gate uses in a repository that never adopted
// procoder. See gitx.NarrowToDiff for why.
func SecretsInDiff(root string, paths []string) []gitx.Finding {
	return gitx.NarrowToDiff(root, paths, SecretsChangedFiles(root, paths))
}
