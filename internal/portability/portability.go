// Package portability keeps procoder working across every AI coding
// agent, not just Claude Code. One canonical AGENTS.md carries the
// always-on instructions; each rule-file host gets a byte-identical copy
// under its own path (plus host frontmatter where required); plugin-tier
// hosts get thin manifests. This package is the drift guard: copies and
// manifest versions are pinned to the master, and drift blocks the gate —
// the same discipline as the PR-template mirror. The binary reports and
// prints content; the agent writes files (P-CONTROL).
package portability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"procoder/internal/gitx"
)

// Master is the canonical always-on instruction file every copy derives
// from. Hosts that read AGENTS.md natively (Zed, Amp, Jules, Devin,
// Qoder instruction tier, VS Code agents…) need nothing else.
const Master = "AGENTS.md"

// Copy is one host's rule-file: the AGENTS.md body under the host's path,
// with host-specific frontmatter where the host requires it.
type Copy struct {
	Host        string
	Path        string
	Frontmatter string // empty when the host reads plain markdown
}

// Copies is every rule-file host served. Order is the docs order.
var Copies = []Copy{
	{Host: "Cursor", Path: ".cursor/rules/procoder.mdc",
		Frontmatter: "---\ndescription: procoder engineering rules\nalwaysApply: true\n---\n\n"},
	{Host: "Windsurf", Path: ".windsurf/rules/procoder.md"},
	{Host: "Cline", Path: ".clinerules/procoder.md"},
	{Host: "Kilo Code", Path: ".kilocode/rules/procoder.md"},
	{Host: "Roo Code", Path: ".roo/rules/procoder.md"},
	{Host: "Kiro", Path: ".kiro/steering/procoder.md",
		Frontmatter: "---\ninclusion: always\n---\n\n"},
	{Host: "Antigravity", Path: ".agents/rules/procoder.md"},
	{Host: "Qoder", Path: ".qoder/rules/procoder.md"},
	{Host: "Copilot (editors)", Path: ".github/copilot-instructions.md"},
	{Host: "OpenAI Codex (repo docs)", Path: ".codex/AGENTS.md"},
}

// versionedManifests are the plugin-tier manifests whose version field
// must match .claude-plugin/plugin.json — a release where every manifest
// is stale TOGETHER passes mutual-agreement checks, so all pin to one.
var versionedManifests = []string{
	".codex-plugin/plugin.json",
	".github/plugin/marketplace.json", // version under metadata.version
	".github/plugin/plugin.json",
	"gemini-extension.json",
	"package.json",
	"plugin.yaml",
}

// forbiddenPaths are files a host would auto-discover with incompatible
// semantics — their EXISTENCE is the bug (Gemini auto-loads a root
// hooks/hooks.json with different event names).
var forbiddenPaths = []string{"hooks/hooks.json"}

// Check pins every copy and manifest to the master. Wired into the shared
// Collect so gate, git, and CI can never disagree.
func Check(root string) []gitx.Finding {
	master, err := os.ReadFile(filepath.Join(root, Master))
	if os.IsNotExist(err) {
		return nil // repos procoder governs need not ship an agent layer
	}
	if err != nil {
		return []gitx.Finding{{File: filepath.Join(root, Master), Blocking: true,
			Message: Master + " exists but cannot be read (" + err.Error() + ") — the agent layer is NOT checked"}}
	}
	// the same derivation Agents uses — frontmatter stripped — so the gate
	// and the command can never disagree about what canonical means
	want := normalize(stripFrontmatter(string(master)))
	// missing copies are only worth reporting once the repo has adopted
	// the layer (at least one copy present) — an AGENTS.md alone is a file
	// many repos carry for unrelated reasons, and ten nag lines per gate
	// run would be noise; drifted or unreadable copies always report
	adopted := false
	for _, c := range Copies {
		if _, err := os.Stat(filepath.Join(root, c.Path)); err == nil {
			adopted = true
			break
		}
	}
	var out []gitx.Finding
	for _, c := range Copies {
		raw, err := os.ReadFile(filepath.Join(root, c.Path))
		if os.IsNotExist(err) {
			if adopted {
				out = append(out, gitx.Finding{File: filepath.Join(root, c.Path),
					Message: c.Path + " is missing — " + c.Host + " gets no rules; run `procoder agents` and write it"})
			}
			continue
		}
		if err != nil {
			out = append(out, gitx.Finding{File: filepath.Join(root, c.Path), Blocking: true,
				Message: c.Path + " exists but cannot be read (" + err.Error() + ") — NOT checked (" + c.Host + ")"})
			continue
		}
		if normalize(stripFrontmatter(string(raw))) != want {
			out = append(out, gitx.Finding{File: filepath.Join(root, c.Path), Blocking: true,
				Message: "out of sync with " + Master + " — edit the master, then run `procoder agents` and rewrite this copy (" + c.Host + ")"})
		}
	}
	if version := pluginVersion(root); version != "" {
		for _, m := range versionedManifests {
			v, err := manifestVersion(filepath.Join(root, m))
			if err != nil || v == "" {
				continue // absence is the agents command's report, not the gate's
			}
			if v != version {
				out = append(out, gitx.Finding{File: filepath.Join(root, m), Blocking: true,
					Message: fmt.Sprintf("manifest version %s != plugin version %s — stale-together releases pass mutual checks, so every manifest pins to the plugin", v, version)})
			}
		}
	}
	for _, p := range forbiddenPaths {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			out = append(out, gitx.Finding{File: filepath.Join(root, p), Blocking: true,
				Message: "must not exist — Gemini auto-loads this path with incompatible event names; hooks live in host-specific files"})
		}
	}
	return out
}

// Agents prints the per-host status and the content for anything missing
// or drifted, so the agent can write it.
func Agents(root string, out func(string)) int {
	master, err := os.ReadFile(filepath.Join(root, Master))
	if err != nil {
		out("no " + Master + " — this repo ships no agent layer (the procoder plugin repo does; a governed repo may too)")
		return 0
	}
	body := stripFrontmatter(string(master))
	want := normalize(body)
	bad := 0
	for _, c := range Copies {
		raw, rerr := os.ReadFile(filepath.Join(root, c.Path))
		switch {
		case rerr != nil:
			bad++
			out("== " + c.Host + ": missing — write this to " + c.Path + ":")
			out(c.Frontmatter + body)
		case normalize(stripFrontmatter(string(raw))) != want:
			bad++
			out("== " + c.Host + ": DRIFTED — rewrite " + c.Path + " with:")
			out(c.Frontmatter + body)
		default:
			out("   " + c.Host + ": ok (" + c.Path + ")")
		}
	}
	if bad == 0 {
		out("every agent rule file matches " + Master)
		return 0
	}
	out(fmt.Sprintf("%d file(s) to write — the gate blocks on drift", bad))
	return 1
}

// normalize levels the differences that are not drift: trailing spaces
// and CRLF endings.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// stripFrontmatter drops a leading --- ... --- block (host metadata).
// CRLF is leveled first so a Windows-written copy still strips.
func stripFrontmatter(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	rest := s[4:]
	i := strings.Index(rest, "\n---")
	if i < 0 {
		return s
	}
	rest = rest[i+4:]
	if j := strings.IndexByte(rest, '\n'); j >= 0 {
		rest = rest[j+1:]
	}
	return rest
}

func pluginVersion(root string) string {
	v, err := manifestVersion(filepath.Join(root, ".claude-plugin/plugin.json"))
	if err != nil {
		return ""
	}
	return v
}

func manifestVersion(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		for _, line := range strings.Split(string(raw), "\n") {
			if rest, ok := strings.CutPrefix(line, "version:"); ok {
				return strings.TrimSpace(rest), nil
			}
		}
		return "", nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	if v, ok := m["version"].(string); ok {
		return v, nil
	}
	if meta, ok := m["metadata"].(map[string]any); ok {
		if v, ok := meta["version"].(string); ok {
			return v, nil
		}
	}
	return "", nil
}
