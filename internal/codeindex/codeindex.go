// Package codeindex is the shared platform layer beneath the domains: who
// defines what, where, and who refers to it. Per D-INDEX it integrates real
// indexers — universal-ctags for the broad tier, SCIP for the precise tier —
// and never parses code itself. The index lives in .procoder/index/
// (gitignored: derived, per-machine) as plain files any domain can read.
package codeindex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"procoder/internal/tools"
)

// Dir is where the index lives, under D-HOME.
const Dir = ".procoder/index"

const (
	tagsFile = "tags.jsonl"
	scipFile = "index.scip"
	refsFile = "refs.json"
	metaFile = "meta.json"
)

const hungToolTimeout = 120 * time.Second

// Ctags is universal-ctags. The Probe matters: macOS ships a BSD ctags that
// cannot do JSON output, so presence on PATH is not enough — the binary must
// answer as Universal Ctags or it does not count as installed.
var Ctags = &tools.Tool{
	Name:        "ctags",
	Install:     "brew install universal-ctags   (or: apt install universal-ctags)",
	VersionArgs: []string{"--version"},
	Probe:       probeUniversal,
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "universal-ctags"}},
		{Manager: "apt-get", Args: []string{"install", "-y", "universal-ctags"}},
		{Manager: "dnf", Args: []string{"install", "-y", "ctags"}},
		{Manager: "winget", Args: []string{"install", "--id", "UniversalCtags.Ctags", "-e"}},
	},
}

// ScipGo builds the precise tier for Go repositories. The project moved to
// the scip-code org; the sourcegraph path no longer go-installs.
var ScipGo = &tools.Tool{
	Name:        "scip-go",
	Install:     "go install github.com/scip-code/scip-go/cmd/scip-go@latest",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "go", Args: []string{"install", "github.com/scip-code/scip-go/cmd/scip-go@latest"}},
	},
}

// ScipTypescript and ScipPython are the precise tier for their ecosystems.
var ScipTypescript = &tools.Tool{
	Name:        "scip-typescript",
	Install:     "npm i -g @sourcegraph/scip-typescript",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "npm", Args: []string{"install", "-g", "@sourcegraph/scip-typescript"}},
	},
}

// ScipPython indexes Python for the precise tier.
var ScipPython = &tools.Tool{
	Name:        "scip-python",
	Install:     "npm i -g @sourcegraph/scip-python",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "npm", Args: []string{"install", "-g", "@sourcegraph/scip-python"}},
	},
}

// ScipCLI converts .scip protobuf to JSON at build time. It ships only as a
// pinned release tarball, so the install candidate is a shell one-liner.
var ScipCLI = &tools.Tool{
	Name:        "scip",
	Install:     "curl -sL https://github.com/sourcegraph/scip/releases/download/v0.9.0/scip-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz | tar xz -C ~/.local/bin scip",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "sh", Args: []string{"-c",
			"mkdir -p $HOME/.local/bin && curl -sL https://github.com/sourcegraph/scip/releases/download/v0.9.0/scip-" +
				runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz | tar xz -C $HOME/.local/bin scip"}},
	},
}

func probeUniversal(bin string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--version").Output() // nosemgrep -- resolved from the fixed tool table, never user input
	return err == nil && bytes.Contains(out, []byte("Universal Ctags"))
}

// Tag is one ctags entry — the broad tier's unit.
type Tag struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Kind      string `json:"kind"`
	Signature string `json:"signature,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Language  string `json:"language,omitempty"`
}

// Meta records what the index was built from, so staleness is a fact the
// index can state instead of a surprise the user discovers.
type Meta struct {
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
	Files   int    `json:"files"`
	Tags    int    `json:"tags"`
	Precise bool   `json:"precise"`
}

// sourceExts limits indexing to code; docs and data have their own domains.
var skipExts = map[string]bool{
	".md": true, ".markdown": true, ".json": true, ".lock": true,
	".svg": true, ".png": true, ".jpg": true, ".gif": true, ".ico": true,
	".sum": true, ".txt": true, ".csv": true,
}

func codeFiles(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", root,
		"ls-files", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, fmt.Errorf("the index needs a git repository: %v", err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" || skipExts[strings.ToLower(filepath.Ext(line))] {
			continue
		}
		files = append(files, line)
	}
	return files, nil
}

// Build creates both tiers. The broad tier (ctags) is required; the precise
// tier is built when the repo's language has an installed SCIP indexer, and
// its absence is said, never silent.
func Build(root string, stdout func(string)) error {
	files, err := codeFiles(root)
	if err != nil {
		return err
	}
	bin := tools.Resolve(Ctags, root)
	if bin == "" {
		return fmt.Errorf("universal-ctags is not installed (a BSD ctags on PATH does not count) — run `procoder init`")
	}
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, // nosemgrep -- resolved from the fixed tool table, never user input
		"--output-format=json", "--fields=+nSl", "-L", "-", "-o", "-")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(strings.Join(files, "\n"))
	raw, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("ctags gave no answer in %s — the index was NOT built", hungToolTimeout)
	}
	if err != nil {
		return fmt.Errorf("ctags failed: %v", err)
	}
	tags, count := normalizeTags(raw)
	if err := os.WriteFile(filepath.Join(dir, tagsFile), tags, 0o644); err != nil {
		return err
	}
	stdout(fmt.Sprintf("broad tier: %d symbols from %d files (universal-ctags)", count, len(files)))

	precise := buildPrecise(root, dir, stdout)

	meta := Meta{Commit: head(root), BuiltAt: time.Now().UTC().Format(time.RFC3339),
		Files: len(files), Tags: count, Precise: precise}
	enc, _ := json.Marshal(meta)
	return os.WriteFile(filepath.Join(dir, metaFile), enc, 0o644)
}

// normalizeTags keeps only real tag lines and re-emits them in our stable
// shape, so consumers never depend on ctags' full output format.
func normalizeTags(raw []byte) ([]byte, int) {
	var buf bytes.Buffer
	count := 0
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var entry struct {
			Type      string `json:"_type"`
			Name      string `json:"name"`
			Path      string `json:"path"`
			Line      int    `json:"line"`
			Kind      string `json:"kind"`
			Signature string `json:"signature"`
			Scope     string `json:"scope"`
			Language  string `json:"language"`
		}
		if json.Unmarshal(sc.Bytes(), &entry) != nil || entry.Type != "tag" {
			continue
		}
		t := Tag{Name: entry.Name, Path: filepath.ToSlash(entry.Path), Line: entry.Line,
			Kind: entry.Kind, Signature: entry.Signature, Scope: entry.Scope, Language: entry.Language}
		enc, _ := json.Marshal(t)
		buf.Write(enc)
		buf.WriteByte('\n')
		count++
	}
	return buf.Bytes(), count
}

// buildPrecise runs the language's SCIP indexer and converts to JSON while
// the scip CLI is at hand, so queries never need it again.
func buildPrecise(root, dir string, stdout func(string)) bool {
	indexer := ""
	switch {
	case exists(root, "go.mod"):
		indexer = "scip-go"
	case exists(root, "tsconfig.json") || exists(root, "package.json"):
		indexer = "scip-typescript"
	case exists(root, "pyproject.toml") || exists(root, "requirements.txt"):
		indexer = "scip-python"
	default:
		stdout("precise tier: no SCIP indexer covers this repository's layout — refs stay textual")
		return false
	}
	tool := map[string]*tools.Tool{"scip-go": ScipGo, "scip-typescript": ScipTypescript, "scip-python": ScipPython}[indexer]
	bin := tools.Resolve(tool, root)
	if bin == "" {
		stdout("precise tier: NOT built — " + indexer + " is not installed; run `procoder init` (refs stay textual)")
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel()
	scipPath := filepath.Join(dir, scipFile)
	var cmd *exec.Cmd
	switch indexer {
	case "scip-go":
		cmd = exec.CommandContext(ctx, bin, "-o", scipPath) // nosemgrep -- resolved from the fixed tool table, never user input
	case "scip-typescript":
		cmd = exec.CommandContext(ctx, bin, "index", "--output", scipPath) // nosemgrep -- resolved from the fixed tool table, never user input
	case "scip-python":
		cmd = exec.CommandContext(ctx, bin, "index", "--output", scipPath, ".") // nosemgrep -- resolved from the fixed tool table, never user input
	}
	cmd.Dir = root
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		stdout("precise tier: " + indexer + " failed — " + firstLine(errb.String()) + " (refs stay textual)")
		return false
	}

	scipBin := tools.Resolve(ScipCLI, root)
	if scipBin == "" {
		stdout("precise tier: index.scip built, but the scip CLI is missing to convert it — run `procoder init` (refs stay textual)")
		return false
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel2()
	conv := exec.CommandContext(ctx2, scipBin, "print", "--json", scipPath) // nosemgrep -- resolved from the fixed tool table, never user input
	var convErr bytes.Buffer
	conv.Stderr = &convErr
	out, err := conv.Output()
	if err != nil {
		stdout("precise tier: scip print failed — " + firstLine(convErr.String()+err.Error()) + " (refs stay textual)")
		return false
	}
	if err := os.WriteFile(filepath.Join(dir, refsFile), out, 0o644); err != nil {
		return false
	}
	stdout("precise tier: built with " + indexer + " (refs are precise)")
	return true
}

func exists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

func head(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			if len(t) > 160 {
				t = t[:160]
			}
			return t
		}
	}
	return "no output"
}
