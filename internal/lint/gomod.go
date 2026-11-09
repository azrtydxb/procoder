package lint

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// golangciConfigNames are the config files golangci-lint itself accepts.
var golangciConfigNames = []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}

// sortedKeys gives the module directories in a stable order, so the order
// of findings does not depend on map iteration.
func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// goPatterns are the package patterns for files, relative to modDir:
// "./" for the module root, "./x" for a subpackage. Sorted and unique.
func goPatterns(modDir string, files []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		rel, err := filepath.Rel(modDir, filepath.Dir(f))
		if err != nil {
			continue
		}
		pat := "./"
		if rel != "." {
			pat = "./" + filepath.ToSlash(rel)
		}
		if !seen[pat] {
			seen[pat] = true
			out = append(out, pat)
		}
	}
	sort.Strings(out)
	return out
}

// repoRelPath maps a path golangci-lint printed — relative to the
// directory it ran in, or absolute — to a slash path relative to root,
// which is what the wanted filter and gitx.Finding carry. A path outside
// root is returned as printed.
func repoRelPath(root, modDir, p string) string {
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(modDir, p)
	}
	if rel, ok := under(root, abs); ok {
		return rel
	}
	return filepath.ToSlash(p)
}

// golangciConfigIn is the golangci config file sitting directly in dir, or
// "" when there is none.
func golangciConfigIn(dir string) string {
	for _, name := range golangciConfigNames {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// under is abs relative to base as a slash path, false when it is outside.
func under(base, abs string) (string, bool) {
	rel, err := filepath.Rel(base, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// pathModeSupport caches, per binary, whether `golangci-lint run` knows
// --path-mode: asked once per process, not once per module.
var pathModeSupport sync.Map // bin path -> bool

// supportsPathMode reports whether bin's `run --help` lists --path-mode.
// A binary that cannot even print its help is treated as not supporting it;
// the real run then fails (or not) on its own terms and says so.
func supportsPathMode(bin string) bool {
	if v, ok := pathModeSupport.Load(bin); ok {
		return v.(bool)
	}
	raw, _ := execute(filepath.Dir(bin), bin, []string{"run", "--help"})
	ok := strings.Contains(raw, "--path-mode")
	pathModeSupport.Store(bin, ok)
	return ok
}
