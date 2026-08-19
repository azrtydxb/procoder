package tools

import "testing"

// The language matrix: every popular language procoder claims maps to its
// canonical formatter. A registry edit that drops a language fails here
// before the docs can overclaim.
func TestFormatterMatrixCoversTheClaimedLanguages(t *testing.T) {
	want := map[string]string{
		".go":    "gofmt",
		".py":    "ruff",
		".js":    "prettier",
		".ts":    "prettier",
		".json":  "prettier",
		".md":    "prettier",
		".html":  "prettier",
		".css":   "prettier",
		".yaml":  "prettier",
		".rs":    "rustfmt",
		".c":     "clang-format",
		".cpp":   "clang-format",
		".h":     "clang-format",
		".sh":    "shfmt",
		".java":  "google-java-format",
		".kt":    "ktfmt",
		".kts":   "ktfmt",
		".swift": "swiftformat",
		".rb":    "rubocop",
		".dart":  "dart",
		".cs":    "csharpier",
	}
	for ext, name := range want {
		tool := ByExtension[ext]
		if tool == nil {
			t.Errorf("%s has no formatter — the matrix regressed", ext)
			continue
		}
		if tool.Name != name {
			t.Errorf("%s -> %s, want %s", ext, tool.Name, name)
		}
	}
}

// Every registered tool can be installed or explained: a human fallback
// line always exists, and stdin tools produce argv without a file.
func TestEveryFormatterHasAnInstallStory(t *testing.T) {
	seen := map[string]bool{}
	for ext, tool := range ByExtension {
		if seen[tool.Name] {
			continue
		}
		seen[tool.Name] = true
		if tool.Install == "" {
			t.Errorf("%s (%s) has no human install line", tool.Name, ext)
		}
		if tool.Args == nil {
			t.Errorf("%s (%s) builds no argv", tool.Name, ext)
		}
	}
}
