package portability

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// nodeAdapters are the JS entry points hosts load procoder through. pi's
// path is read from package.json rather than spelled here: the manifest is
// what the host actually follows, so a rename that updates one and not the
// other must fail rather than be invisible.
func nodeAdapters(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pi struct {
			Extensions []string `json:"extensions"`
		} `json:"pi"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Pi.Extensions) != 1 {
		t.Fatalf("package.json declares %d pi extensions, expected 1", len(manifest.Pi.Extensions))
	}
	return map[string]string{
		"pi":       filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(manifest.Pi.Extensions[0], "./"))),
		"opencode": filepath.Join(root, ".opencode/plugins/procoder.mjs"),
		"kilo":     filepath.Join(root, ".kilo/plugin/procoder.js"),
	}
}

// Every adapter a host loads must be ES module source exporting a default
// factory. This is checked against the SOURCE, not against an import: Node's
// CJS interop hands `module.exports` back as `default`, so a CommonJS
// adapter loads perfectly here and is still rejected by the host's own
// validator at install time — which is exactly how procoder shipped a pi
// extension that could not be installed at all (#105).
// proved by: restored `module.exports = function register(pi)` in
// pi-extension/index.mjs — this test then names the file and the CJS marker
// in it, where every load-based check stayed green.
func TestEveryHostAdapterIsESMSource(t *testing.T) {
	for host, path := range nodeAdapters(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: the manifest points at %s, which is not there: %v", host, path, err)
			continue
		}
		src := code(string(raw))
		for _, cjs := range []string{"module.exports", "require("} {
			if strings.Contains(src, cjs) {
				t.Errorf("%s: %s uses %s — hosts validate the export shape and reject CommonJS",
					host, filepath.Base(path), cjs)
			}
		}
		if !strings.Contains(src, "export default") {
			t.Errorf("%s: %s has no default export — the extension contract is a default factory",
				host, filepath.Base(path))
		}
	}
}

// And it must actually load, with a callable default. The source check above
// cannot tell a well-shaped export from a file that throws on import.
// proved by: put a bare `throw new Error("x")` at the top of
// pi-extension/index.mjs — the source check stays green, this one names it.
func TestEveryHostAdapterLoadsWithACallableDefault(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node on PATH")
	}
	for host, path := range nodeAdapters(t) {
		url := "file://" + filepath.ToSlash(path)
		script := `const m = await import(process.argv[1]);
console.log(typeof m.default);`
		cmd := exec.Command(node, "--input-type=module", "-e", script, url)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("%s: %s does not load: %v\n%s", host, filepath.Base(path), err, out)
			continue
		}
		if !strings.Contains(string(out), "function") {
			t.Errorf("%s: %s exports a %s as default, hosts call it", host, filepath.Base(path), strings.TrimSpace(string(out)))
		}
	}
}

// code strips comments so the scan above reads the adapter, not its prose —
// the file explaining why CommonJS was rejected must not be flagged for
// spelling the word. Blunt on purpose: these are three small files with no
// generated content, and a real JS parser here would be a dependency bought
// to check three files nobody minifies. A "//" preceded by ":" is left alone
// so a URL in a string survives.
func code(src string) string {
	for {
		open := strings.Index(src, "/*")
		if open < 0 {
			break
		}
		close := strings.Index(src[open:], "*/")
		if close < 0 {
			break
		}
		src = src[:open] + src[open+close+2:]
	}
	var out []string
	for _, line := range strings.Split(src, "\n") {
		for at := 0; ; {
			i := strings.Index(line[at:], "//")
			if i < 0 {
				break
			}
			i += at
			if i > 0 && line[i-1] == ':' {
				at = i + 2
				continue
			}
			line = line[:i]
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
