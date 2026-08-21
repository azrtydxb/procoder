package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

// writePHPProject makes a directory holding one .php file, and the prettier
// PHP plugin's entry module when withPlugin.
func writePHPProject(t *testing.T, withPlugin bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.php"), []byte("<?php\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withPlugin {
		dir := filepath.Join(root, "node_modules", "@prettier", "plugin-php", "src")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.mjs"), []byte("export default {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// A .php file must reach a formatter at all. Before this it reached none,
// so the gate reported clean over PHP it had never looked at.
// proved by: unregistered ".php" — ForFile returns nil and every PHP file
// is "no formatter covers this file type", which the gate counts as out of
// scope and a reader reads as fine.
func TestPHPFilesReachAFormatter(t *testing.T) {
	tool := tools.ForFile("/x/a.php")
	if tool == nil {
		t.Fatal("no formatter is registered for .php")
	}
	if tool.Name != "prettier" {
		t.Errorf("PHP formats through prettier, got %q", tool.Name)
	}
}

// Without the plugin, PHP formatting cannot run. That is a MISSING TOOL,
// so the verdict is unchecked — which fails the gate — and not out of
// scope, which passes it. The plugin is the thing that parses PHP; nothing
// about its absence is a style opinion, and modelling it as one meant a
// PHP repository without it was formatted by nothing and called fine.
// proved by: returned OutOfScope here again — a PHP project with no plugin
// then passes `procoder check` with the file counted under "skipped".
func TestWithoutThePluginPHPIsUncheckedWithTheInstallLine(t *testing.T) {
	root := writePHPProject(t, false)
	got := Check(filepath.Join(root, "a.php"))
	if got.Verdict != Unchecked {
		t.Fatalf("without the plugin PHP must be unchecked, got verdict %v (%s)", got.Verdict, got.Reason)
	}
	if !strings.Contains(got.Reason, "npm i -D") {
		t.Errorf("the reason must carry the install line: %q", got.Reason)
	}
}

// The plugin is named to prettier by absolute path. prettier resolves a
// bare plugin name against the WORKING DIRECTORY, and procoder's working
// directory is not the project's — so a bare name works when procoder
// happens to run from the project root and fails everywhere else.
// proved by: passed "--plugin=@prettier/plugin-php" instead — formatting
// works from the project root and fails from any other directory, which is
// the hardest kind of bug to be told about.
func TestThePluginIsNamedByAbsolutePath(t *testing.T) {
	root := writePHPProject(t, true)
	file := filepath.Join(root, "a.php")
	args := tools.ForFile(file).Args(file)
	if len(args) == 0 || !strings.HasPrefix(args[0], "--plugin=") {
		t.Fatalf("the plugin must be passed to prettier: %v", args)
	}
	path := strings.TrimPrefix(args[0], "--plugin=")
	if !filepath.IsAbs(path) {
		t.Errorf("the plugin path must be absolute, got %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the plugin path must point at something real: %v", err)
	}
	// prettier refuses a directory ("Directory import is not supported"),
	// and the package declares its entry through "exports" with no "main"
	// to fall back to, so the path has to be the entry module itself.
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		t.Error("prettier cannot import a directory — the path must be the entry module")
	}
}

// P-CONTROL, asserted on the one language whose usual formatters cannot
// honour it: the file's bytes are the same after a format as before. It
// needs the real plugin, because a stub would prove only that a fake
// formatter does not write, which proves nothing — so CI installs it on
// the Linux runner rather than letting this skip everywhere. A guard that
// can only skip looks like coverage and is not.
// proved by: made Check write the formatted result back — the digest moves
// and this test names it. (php-cs-fixer and pint do exactly that, which is
// why neither is the formatter here.)
func TestFormattingPHPNeverTouchesTheFile(t *testing.T) {
	root := realPHPProject(t)
	file := filepath.Join(root, "messy.php")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	got := Check(file)
	if got.Verdict == Unchecked {
		t.Fatalf("the formatter must run where the plugin is installed: %s", got.Reason)
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("formatting must print the result, never write it")
	}
	// And it must have produced something to write: an "unformatted"
	// verdict with no formatted bytes leaves the agent nothing to act on.
	if got.Verdict == Unformatted && len(got.Formatted) == 0 {
		t.Error("an unformatted verdict must carry the formatted result")
	}
}

// realPHPProject copies a messy PHP file next to a genuinely installed
// prettier PHP plugin, or skips.
func realPHPProject(t *testing.T) string {
	t.Helper()
	root := tools.RepoRoot(mustDir("."))
	plugin := filepath.Join(root, "node_modules", "@prettier", "plugin-php", "src", "index.mjs")
	if _, err := os.Stat(plugin); err != nil {
		t.Skip("the prettier PHP plugin is not installed here: ", err)
	}
	if err := os.WriteFile(filepath.Join(root, "messy.php"),
		[]byte("<?php\nfunction   greet($n){\n return $n; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join(root, "messy.php")) })
	return root
}
