package docs

import (
	"os"
	"path/filepath"
	"strings"

	"procoder/internal/gitx"
)

// EmptyDocs reports documentation files that have no content.
//
// This exists because it happened: `procoder format` prints a single header
// line for a file that is ALREADY FORMATTED and nothing after it, so a
// pipeline that strips the header and writes the rest empties the file — on
// the success path, silently, exit 0. docs/commands.md was destroyed that
// way, 551 lines, and shipped: it passed review, the gate, `mkdocs build
// --strict` and a release, because nothing anywhere asked whether a
// documentation file still had documentation in it.
//
// Worse, the documentation obligation was SATISFIED by the destruction. It
// asks whether a doc changed in this diff; emptying one is a change.
//
// Blocking, and not a matter of policy: an empty doc is not a style
// opinion, it is a page that used to say something and now says nothing.
func EmptyDocs(root string, files []string) []gitx.Finding {
	var out []gitx.Finding
	for _, f := range files {
		// IsMarkdownFile, not an .md check: this package already treats
		// .markdown as Markdown, and a guard that covers one spelling is a
		// guard anybody can step around by renaming.
		if !IsMarkdownFile(f) {
			continue
		}
		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, f)
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Whitespace only counts as empty: a file holding one newline is
		// not a page anybody can read, and that is the shape the clobber
		// leaves behind.
		if strings.TrimSpace(string(data)) == "" {
			// The path is repo-relative, not the base name: `git checkout
			// <ref> -- commands.md` finds nothing when the file is
			// docs/commands.md, and a refusal whose remedy does not work
			// is worse than one that offers none.
			rel := path
			if r, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(r, "..") {
				rel = filepath.ToSlash(r)
			}
			out = append(out, gitx.Finding{Blocking: true, File: path,
				Message: "this documentation file is empty — if it was emptied by accident, `git checkout <ref> -- " +
					rel + "` restores it; if it is meant to go, delete it (docs)"})
		}
	}
	return out
}
