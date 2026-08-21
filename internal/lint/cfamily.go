package lint

import (
	"fmt"
	"path/filepath"
	"strings"

	"procoder/internal/gitx"
	"procoder/internal/textutil"
	"procoder/internal/tools"
)

// ClangTidy is the C and C++ linter. C and C++ were formatted and linted by
// nothing: the gate looked at a .cpp file, formatted it, and reported clean
// without any analysis having happened.
var ClangTidy = &tools.Tool{
	Name:        "clang-tidy",
	Install:     "brew install llvm   (or: apt install clang-tidy)",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "llvm"}},
		{Manager: "apt-get", Args: []string{"install", "-y", "clang-tidy"}},
		{Manager: "dnf", Args: []string{"install", "-y", "clang-tools-extra"}},
	},
}

// clangTidyBaseline is procoder's curated check set for a project with no
// .clang-tidy of its own. The analyser and bug-prone families plus cert:
// the classes that are bugs in any codebase — uninitialised reads, garbage
// values, use-after-move — and not one style check, because clang-format
// already owns style and two tools arguing about braces is how a gate
// starts contradicting itself.
//
// clang-tidy refuses to run at all with no checks enabled ("Error: no
// checks enabled"), so this is not an optional flourish: without it the
// tool exits non-zero and every C++ file reports NOT checked.
const clangTidyBaseline = "clang-analyzer-*,bugprone-*,cert-*"

// cFamilyStandard is passed after `--`, which tells clang-tidy to use these
// compiler flags instead of looking for a compilation database. Without it
// a project that has no compile_commands.json gets a warning on every file
// and analyses against whatever default the driver picks.
func cFamilyStandard(file string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".c", ".h":
		return "-std=c17"
	default:
		return "-std=c++17"
	}
}

// lintCFamily runs clang-tidy over C and C++ files.
//
// One file per run: clang-tidy takes several, but the compiler flags after
// `--` apply to all of them, and a C header analysed as C++ reports errors
// that are artefacts of the invocation rather than facts about the code.
func lintCFamily(root string, files []string, block bool) []gitx.Finding {
	bin := tools.Resolve(ClangTidy, root)
	if bin == "" {
		return notChecked(files[0], ClangTidy.Name)
	}
	var out []gitx.Finding
	for _, f := range files {
		args := []string{"--quiet"}
		// The project's own .clang-tidy wins wherever it has one; the
		// curated set is only for a project that chose nothing.
		if hasAny(root, f, []string{".clang-tidy"}) == "" {
			args = append(args, "--checks="+clangTidyBaseline)
		}
		args = append(args, f, "--", cFamilyStandard(f))
		raw, err := execute(root, bin, args)
		// clang-tidy prints the finding and then a trail of "note:" lines
		// explaining how it got there. They carry the same file and line
		// shape, so the shared parser reads each as its own finding and one
		// uninitialised variable arrives as three. The notes are detail,
		// not additional problems.
		found := parse(raw, block)[:0:0]
		for _, f := range parse(raw, block) {
			if strings.HasPrefix(f.Message, "note:") {
				continue
			}
			found = append(found, f)
		}
		if len(found) == 0 && err != nil {
			out = append(out, gitx.Finding{Blocking: true, File: f,
				Message: fmt.Sprintf("NOT checked — clang-tidy failed: %s (lint)",
					textutil.FirstLine(raw+err.Error()))})
			continue
		}
		out = append(out, found...)
	}
	return out
}
