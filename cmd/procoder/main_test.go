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
}
