package config

import (
	"fmt"
	"io"

	"procoder/internal/gitx"
	"procoder/internal/store"
)

// Report is `procoder config`: every effective setting, its value, and
// where that value came from.
//
// This exists because configurability without visibility is worse than
// none. Once a repository can change this much, a person reading an
// unfamiliar one has to be able to ask which of procoder's defaults are
// still in force — and get an answer that names the source of each rather
// than a file they have to diff against a version they do not have.
func Report(root string, stdout io.Writer) int {
	cfg := Load(root)
	fmt.Fprintln(stdout, "setting                     value        source")
	for _, s := range cfg.Settings {
		mark := ""
		if s.Relaxed {
			mark = "  ← relaxed from " + s.Default
		}
		fmt.Fprintf(stdout, "%-27s %-12s %s%s\n", s.Key, s.Value, s.Source, mark)
	}
	// Not a row in the table above, deliberately. The table is settings:
	// each has a default and can be relaxed from it. The identity has
	// neither — it is computed, or stated, and listing it there would
	// invite somebody to read it as a knob that had been loosened.
	id := store.IdentityFor(root, cfg.ServiceRepo)
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "repo identity  %s  (%s)\n", id.Key, id.Source())

	if len(cfg.Problems) == 0 {
		return 0
	}
	fmt.Fprintln(stdout)
	for _, p := range cfg.Problems {
		fmt.Fprintln(stdout, "NOT applied  "+p.String())
	}
	return 1
}

// Findings are the configuration's contribution to the gate: what could
// not be applied, and what was deliberately loosened.
//
// A problem blocks. A relaxation does not — the repository chose it, and
// blocking would make the setting useless — but it prints on every run,
// because a green gate must never be able to mean "the config was
// loosened" without saying so.
func (c Config) Findings() []gitx.Finding {
	var out []gitx.Finding
	for _, p := range c.Problems {
		out = append(out, gitx.Finding{Blocking: true, File: ".procoder/config.toml", Line: p.Line,
			Message: "NOT applied — " + p.Reason + ": " + p.Text + " (config)"})
	}
	for _, s := range c.Settings {
		if !s.Relaxed {
			continue
		}
		out = append(out, gitx.Finding{File: ".procoder/config.toml",
			Message: fmt.Sprintf("relaxed: %s = %s, weaker than the default %s — %s (config)",
				s.Key, s.Value, s.Default, s.Why)})
	}
	return out
}
