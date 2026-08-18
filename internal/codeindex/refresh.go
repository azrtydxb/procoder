package codeindex

import (
	"os"
	"path/filepath"
)

// RebuildIfStale rebuilds both tiers when an index exists and its commit no
// longer matches HEAD — the gate calls this so the natural lifecycle moment
// (finishing a change) also refreshes what per-write updates cannot reach:
// editor edits outside the hook and the precise tier. A repo that never
// built an index is left alone, so CI and index-less users pay nothing.
// Returns what happened as one line, or "" when there was nothing to do.
func RebuildIfStale(root string) string {
	if _, err := os.Stat(filepath.Join(root, Dir, tagsFile)); err != nil {
		return ""
	}
	if loadMeta(root).Commit == head(root) {
		return ""
	}
	var notes []string
	if err := Build(root, func(s string) { notes = append(notes, s) }); err != nil {
		return "index: refresh failed — " + err.Error()
	}
	return "index: refreshed (" + notes[0] + ")"
}
