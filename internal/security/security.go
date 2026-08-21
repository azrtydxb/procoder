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
	"path/filepath"
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

func Sast(root string) []gitx.Finding {
	blocksAt := config.Load(root).SastBlocksAt
	bin := tools.Resolve(Semgrep, root)
	if bin == "" {
		return []gitx.Finding{{Blocking: true,
			Message: "NOT checked — semgrep is not installed; run `procoder init` (security)"}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), deepTimeout)
	defer cancel()
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
			Message: "semgrep output unreadable — SAST was NOT run: " + firstLine(errb.String()+errStr(err)) + " (security)"}}
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
	"requirements.txt", "poetry.lock", "Pipfile.lock", "pyproject.toml",
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
	for _, m := range DepManifests {
		if _, err := os.Stat(filepath.Join(root, m)); err == nil {
			margs = append(margs, "-L", m)
		}
	}
	// a bare package.json is not scannable by osv (it needs a lockfile's
	// pinned versions); one that DECLARES dependencies without any
	// lockfile is an honest gap, one without dependencies has nothing to
	// check and stays silent
	var out []gitx.Finding
	if hasNpmDepsWithoutLockfile(root) {
		out = append(out, gitx.Finding{Blocking: true,
			Message: "npm dependencies NOT checked — package.json declares dependencies but no lockfile exists for osv-scanner; generate package-lock.json (security)"})
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
			Message: "osv-scanner output unreadable — dependencies were NOT checked: " + firstLine(errb.String()+errStr(err)) + " (security)"})
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
func hasNpmDepsWithoutLockfile(root string) bool {
	for _, lock := range []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"} {
		if _, err := os.Stat(filepath.Join(root, lock)); err == nil {
			return false
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
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
	return len(m.Dependencies)+len(m.DevDependencies) > 0
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
