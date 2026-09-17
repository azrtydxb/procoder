package host

import (
	"fmt"
	"strings"
)

// SetupNames are explicit setup targets, not guesses about installed editors.
var SetupNames = []string{"claude", "codex", "copilot", "qoder", "pi", "kilo", "opencode", "cursor", "windsurf", "cline", "roo", "kiro", "antigravity", "gemini", "grok", "devin", "hermes", "agents", "skills"}

// ValidSetup reports whether a name identifies a supported setup target.
func ValidSetup(name string) bool {
	for _, h := range SetupNames {
		if h == name {
			return true
		}
	}
	return false
}

// Setup uses only explicit adapter context. Hook envelope detection is a
// separate legacy interface: its Claude fallback and path heuristic are not
// evidence of which integration a repository should acquire.
func Setup(args []string, env Env, allowYes bool) (hosts []string, execute bool, err error) {
	all := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--yes" && allowYes && !execute:
			execute = true
		case a == "--all" && !all:
			all = true
		case a == "--host" || strings.HasPrefix(a, "--host="):
			name := strings.TrimPrefix(a, "--host=")
			if a == "--host" {
				i++
				if i >= len(args) {
					return nil, false, fmt.Errorf("--host needs a host name")
				}
				name = args[i]
			}
			if !ValidSetup(name) {
				return nil, false, fmt.Errorf("unknown host %q; choose %s", name, strings.Join(SetupNames, ", "))
			}
			for _, h := range hosts {
				if h == name {
					return nil, false, fmt.Errorf("duplicate --host %s", name)
				}
			}
			hosts = append(hosts, name)
		default:
			return nil, false, fmt.Errorf("unexpected setup argument %q", a)
		}
	}
	if all && len(hosts) > 0 {
		return nil, false, fmt.Errorf("--all and --host cannot be combined")
	}
	if all {
		return []string{"all"}, execute, nil
	}
	if len(hosts) > 0 {
		return hosts, execute, nil
	}
	if h := env.Read("PROCODER_HOST"); h != "" {
		if !ValidSetup(h) {
			return nil, false, fmt.Errorf("invalid PROCODER_HOST %q; use --host to override", h)
		}
		return []string{h}, execute, nil
	}
	// These are host-specific plugin signals. CLAUDE_PLUGIN_ROOT is shared
	// by several frameworks and therefore deliberately not sufficient.
	for _, signal := range []struct{ key, name string }{{"COPILOT_PLUGIN_DATA", "copilot"}, {"PLUGIN_DATA", "codex"}, {"QODER_SESSION_ID", "qoder"}, {"PI_CODING_AGENT", "pi"}} {
		if env.Read(signal.key) != "" {
			hosts = append(hosts, signal.name)
		}
	}
	if len(hosts) == 1 {
		return hosts, execute, nil
	}
	if len(hosts) > 1 {
		return nil, false, fmt.Errorf("conflicting host context; choose --host explicitly")
	}
	return nil, false, fmt.Errorf("which AI host should procoder set up? Re-run with --host <name> (%s), or deliberately choose --all", strings.Join(SetupNames, ", "))
}
