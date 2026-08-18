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
	"time"

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
		report := filepath.Join(os.TempDir(), fmt.Sprintf("procoder-gitleaks-%d.json", time.Now().UnixNano()))
		common := []string{"--report-format", "json", "--report-path", report,
			"--no-banner", "--exit-code", "1"}
		ctx, cancel := context.WithTimeout(context.Background(), secretsTimeout)
		cmd := exec.CommandContext(ctx, bin, append([]string{"dir", p}, common...)...) // nosemgrep -- resolved from the fixed tool table, never user input
		cmd.Dir = root
		var errb bytes.Buffer
		cmd.Stderr = &errb
		runErr := cmd.Run()
		cancel()
		if ctx.Err() == context.DeadlineExceeded {
			out = append(out, gitx.Finding{File: p, Blocking: true,
				Message: fmt.Sprintf("gitleaks gave no answer in %s — the path was NOT checked (security)", secretsTimeout)})
			continue
		}
		if bytes.Contains(errb.Bytes(), []byte(`unknown command "dir"`)) {
			// gitleaks before v8.19 has no `dir`; the same scan is spelled
			// `detect --no-git --source` there — apt still ships that era
			ctx2, cancel2 := context.WithTimeout(context.Background(), secretsTimeout)
			legacy := exec.CommandContext(ctx2, bin, append([]string{"detect", "--no-git", "--source", p}, common...)...) // nosemgrep -- resolved from the fixed tool table, never user input
			legacy.Dir = root
			errb.Reset()
			legacy.Stderr = &errb
			runErr = legacy.Run()
			cancel2()
			if ctx2.Err() == context.DeadlineExceeded {
				out = append(out, gitx.Finding{File: p, Blocking: true,
					Message: fmt.Sprintf("gitleaks gave no answer in %s — the path was NOT checked (security)", secretsTimeout)})
				continue
			}
		}
		raw, readErr := os.ReadFile(report)
		os.Remove(report)
		if readErr != nil {
			out = append(out, gitx.Finding{File: p, Blocking: true,
				Message: "gitleaks produced no report — the path was NOT checked: " + firstLine(errb.String()) + " (security)"})
			continue
		}
		var leaks []struct {
			File      string `json:"File"`
			StartLine int    `json:"StartLine"`
			RuleID    string `json:"RuleID"`
		}
		if json.Unmarshal(raw, &leaks) != nil {
			out = append(out, gitx.Finding{File: p, Blocking: true,
				Message: "gitleaks report unreadable — the path was NOT checked (security)"})
			continue
		}
		for _, l := range leaks {
			// gitleaks echoes the path as given for file scans and prefixes
			// it for directory scans — normalise to one shape
			loc := l.File
			if !filepath.IsAbs(loc) && loc != p {
				loc = filepath.Join(p, loc)
			}
			// the finding names the rule and location — NEVER the secret
			out = append(out, gitx.Finding{File: loc, Line: l.StartLine, Blocking: true,
				Message: fmt.Sprintf("secret detected (%s) — remove it and rotate the credential; committed secrets live in history forever (security)", l.RuleID)})
		}
		_ = runErr // exit 1 just means leaks; they are already collected
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
func Sast(root string) []gitx.Finding {
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
			Message: "semgrep output unreadable — SAST was NOT run: " + firstLine(errb.String()+err.Error()) + " (security)"}}
	}
	var out []gitx.Finding
	for _, r := range rep.Results {
		blocking := r.Extra.Severity == "ERROR"
		msg := r.Extra.Message
		if len(msg) > 200 {
			msg = msg[:200]
		}
		out = append(out, gitx.Finding{File: r.Path, Line: r.Start.Line, Blocking: blocking,
			Message: fmt.Sprintf("%s [%s %s] (security)", msg, r.Extra.Severity, shortCheck(r.CheckID))})
	}
	return out
}

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
	for _, m := range []string{"go.mod", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"package.json", "requirements.txt", "poetry.lock", "Pipfile.lock", "pyproject.toml",
		"Cargo.lock", "composer.lock", "Gemfile.lock"} {
		if _, err := os.Stat(filepath.Join(root, m)); err == nil {
			margs = append(margs, "-L", m)
		}
	}
	if len(margs) == 0 {
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
		return []gitx.Finding{{Blocking: true,
			Message: "osv-scanner output unreadable — dependencies were NOT checked: " + firstLine(errb.String()+errStr(err)) + " (security)"}}
	}
	var out []gitx.Finding
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
				Message: fmt.Sprintf("%s %s has %d known vulnerabilit(ies), max severity %.1f — upgrade it (security)",
					p.Package.Name, p.Package.Version, len(p.Vulnerabilities), max)})
		}
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
