package lint

import (
	"fmt"
	"path/filepath"
	"strings"

	"procoder/internal/gitx"
)

// unlinted names the languages procoder formats but has no linter for, and
// what a person would install if they wanted one. The entry exists so the
// gate says "nothing linted this" out loud instead of counting the file as
// clean — the whole point of this rule is that a green gate must mean the
// code was checked, not that procoder had nothing to check it with.
//
// A language leaves this table by gaining a linter, not by being removed
// from it.
var unlinted = map[string]string{
	".cs":   "C#: procoder has no linter for it yet — the analysers ship with a build (`dotnet build`), not as a per-file tool",
	".dart": "Dart: procoder has no linter for it yet — `dart analyze` covers a project, and wiring it is not done",
}

// lintUnlinted reports, for each language procoder formats and cannot lint,
// that no linting happened.
//
// This is not notChecked's case. A missing gitleaks or golangci-lint is a
// machine that has not run `procoder init` yet — install it and the check
// runs, so that finding blocks regardless of policy the same way every
// "the check did not run" finding does. A language with no linter at all
// has no install to run; the repository cannot fix its way out, and a
// finding that always blocks would leave it unable to land any commit
// touching that language. [lint] policy governs this one, like every
// other judgement call in this file.
func lintUnlinted(files []string, block bool) []gitx.Finding {
	seen := map[string]bool{}
	var out []gitx.Finding
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		why, ok := unlinted[ext]
		if !ok || seen[ext] {
			continue
		}
		seen[ext] = true
		out = append(out, gitx.Finding{Blocking: block, File: f,
			Message: fmt.Sprintf("NOT linted — %s (lint)", why)})
	}
	return out
}
