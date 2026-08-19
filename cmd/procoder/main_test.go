package main

import (
	"strings"
	"testing"

	"procoder/internal/docs"
)

// The usage text and the docs-coverage command list are pinned to each
// other: a command added to one without the other fails here, so the
// documentation completeness check can never silently drift from reality.
func TestUsageAndCoverageListAgree(t *testing.T) {
	for _, cmd := range docs.Commands {
		if !strings.Contains(usage, "\n  "+cmd) {
			t.Errorf("usage text does not list %q", cmd)
		}
	}
	// and the reverse: a command added to usage must join the coverage list
	listed := map[string]bool{}
	for _, cmd := range docs.Commands {
		listed[cmd] = true
	}
	for _, line := range strings.Split(usage, "\n") {
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if !listed[fields[0]] {
			t.Errorf("usage lists %q but docs.Commands does not — CommandCoverage would miss it", fields[0])
		}
	}
}
