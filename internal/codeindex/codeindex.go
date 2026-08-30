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

	"procoder/internal/store"
	"procoder/internal/textutil"
	"procoder/internal/tools"
)

// Dir is where the index lives, under D-HOME.
const Dir = ".procoder/index"

const (
	tagsFile = "tags.jsonl"
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

// swiftDartOptlib defines regex parsers for the two matrix languages
// universal-ctags does not ship: Swift and Dart. Kinds mirror what the
// broad tier needs (definitions with names and lines); signatures and
// scopes stay best-effort, which is the broad tier's contract anyway.
var swiftDartOptlib = []string{
	"--langdef=procoderswift",
	"--map-procoderswift=+.swift",
	`--regex-procoderswift=/^[[:space:]]*(public |private |internal |open |fileprivate )*(final )?class[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)/\3/c,class/`,
	`--regex-procoderswift=/^[[:space:]]*(public |private |internal |open |fileprivate )*struct[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)/\2/s,struct/`,
	`--regex-procoderswift=/^[[:space:]]*(public |private |internal |open |fileprivate )*enum[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)/\2/e,enum/`,
	`--regex-procoderswift=/^[[:space:]]*(public |private |internal |open |fileprivate )*protocol[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)/\2/p,protocol/`,
	`--regex-procoderswift=/^[[:space:]]*(public |private |internal |open |fileprivate |static |class |override |mutating )*func[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)/\2/f,function/`,
	"--langdef=procoderdart",
	"--map-procoderdart=+.dart",
	`--regex-procoderdart=/^[[:space:]]*(abstract[[:space:]]+)?(base[[:space:]]+|final[[:space:]]+|sealed[[:space:]]+)?class[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)/\3/c,class/`,
	`--regex-procoderdart=/^[[:space:]]*enum[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)/\1/e,enum/`,
	`--regex-procoderdart=/^[[:space:]]*mixin[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)/\1/m,mixin/`,
	`--regex-procoderdart=/^[[:space:]]*(static[[:space:]]+)?[A-Za-z_<>,\[\]? ]+[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)\(/\2/f,function/`,
}

// RustAnalyzer emits SCIP for Rust workspaces (`rust-analyzer scip .`).
var RustAnalyzer = &tools.Tool{
	Name:        "rust-analyzer",
	Install:     "rustup component add rust-analyzer",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "rustup", Args: []string{"component", "add", "rust-analyzer"}},
		{Manager: "brew", Args: []string{"install", "rust-analyzer"}},
	},
}

// ScipJava indexes Java, Kotlin, and Scala builds (Maven/Gradle).
var ScipJava = &tools.Tool{
	Name:        "scip-java",
	Install:     "brew install coursier && cs install --contrib scip-java",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "cs", Args: []string{"install", "--contrib", "scip-java"}},
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
	args := []string{"--output-format=json", "--fields=+nSl", "-L", "-", "-o", "-"}
	// universal-ctags ships no Swift or Dart parser; procoder supplies
	// regex-based definitions so the broad tier covers them too (top-level
	// symbols — approximate by nature, and the precise tier stays honest
	// about not covering these languages)
	args = append(args, swiftDartOptlib...)
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- resolved from the fixed tool table, never user input
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
	if err := store.SaveIn(root, Dir, tagsFile, tags); err != nil {
		return err
	}
	stdout(fmt.Sprintf("broad tier: %d symbols from %d files (universal-ctags)", count, len(files)))

	precise := buildPrecise(root, dir, stdout)

	meta := Meta{Commit: head(root), BuiltAt: time.Now().UTC().Format(time.RFC3339),
		Files: len(files), Tags: count, Precise: precise}
	enc, _ := json.Marshal(meta)
	return store.SaveIn(root, Dir, metaFile, enc)
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

// detectIndexers lists every SCIP indexer the repository's layout calls
// for, in manifest order — a polyglot monorepo gets them all, not just the
// first match. The generic package.json only counts when no tsconfig
// already claimed TypeScript, so the indexer never runs twice.
func detectIndexers(root string) []string {
	var out []string
	if exists(root, "go.mod") {
		out = append(out, "scip-go")
	}
	if exists(root, "Cargo.toml") {
		out = append(out, "rust-analyzer")
	}
	if exists(root, "pom.xml") || exists(root, "build.gradle") || exists(root, "build.gradle.kts") {
		out = append(out, "scip-java")
	}
	if exists(root, "tsconfig.json") || exists(root, "package.json") {
		out = append(out, "scip-typescript")
	}
	if exists(root, "pyproject.toml") || exists(root, "requirements.txt") {
		out = append(out, "scip-python")
	}
	return out
}

// buildPrecise runs every SCIP indexer the layout calls for and merges the
// results into one refs.json, converted while the scip CLI is at hand so
// queries never need it again. Partial success is said per indexer.
func buildPrecise(root, dir string, stdout func(string)) bool {
	indexers := detectIndexers(root)
	if len(indexers) == 0 {
		stdout("precise tier: no SCIP indexer covers this repository's layout — refs stay textual")
		return false
	}
	scipBin := tools.Resolve(ScipCLI, root)
	if scipBin == "" {
		stdout("precise tier: the scip CLI is missing to convert indexer output — run `procoder init` (refs stay textual)")
		return false
	}
	var docs []json.RawMessage
	var built []string
	for _, indexer := range indexers {
		out, ok := runIndexer(root, dir, indexer, scipBin, stdout)
		if !ok {
			continue
		}
		docs = append(docs, out...)
		built = append(built, indexer)
	}
	if len(built) == 0 {
		return false
	}
	merged, err := json.Marshal(map[string]any{"documents": docs})
	if err != nil {
		return false
	}
	if err := store.SaveIn(root, Dir, refsFile, merged); err != nil {
		return false
	}
	if len(built) < len(indexers) {
		stdout(fmt.Sprintf("precise tier: built with %s — %d of %d indexers answered (the rest stay textual)",
			strings.Join(built, ", "), len(built), len(indexers)))
	} else {
		stdout("precise tier: built with " + strings.Join(built, ", ") + " (refs are precise)")
	}
	return true
}

// runIndexer runs one SCIP indexer and returns its converted documents.
// Failures are reported and skipped — one broken ecosystem must not cost
// the others their precise tier.
func runIndexer(root, dir, indexer, scipBin string, stdout func(string)) ([]json.RawMessage, bool) {
	tool := map[string]*tools.Tool{"scip-go": ScipGo, "scip-typescript": ScipTypescript,
		"scip-python": ScipPython, "rust-analyzer": RustAnalyzer, "scip-java": ScipJava}[indexer]
	bin := tools.Resolve(tool, root)
	if bin == "" {
		stdout("precise tier: " + indexer + " is not installed; run `procoder init` (its ecosystem stays textual)")
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel()
	scipPath := filepath.Join(dir, "index-"+indexer+".scip")
	var cmd *exec.Cmd
	switch indexer {
	case "scip-go":
		cmd = exec.CommandContext(ctx, bin, "-o", scipPath) // nosemgrep -- resolved from the fixed tool table, never user input
	case "scip-typescript":
		cmd = exec.CommandContext(ctx, bin, "index", "--output", scipPath) // nosemgrep -- resolved from the fixed tool table, never user input
	case "scip-python":
		cmd = exec.CommandContext(ctx, bin, "index", "--output", scipPath, ".") // nosemgrep -- resolved from the fixed tool table, never user input
	case "rust-analyzer":
		// rust-analyzer writes index.scip into the current dir; point it home
		cmd = exec.CommandContext(ctx, bin, "scip", ".", "--output", scipPath) // nosemgrep -- resolved from the fixed tool table, never user input
	case "scip-java":
		cmd = exec.CommandContext(ctx, bin, "index", "--output", scipPath) // nosemgrep -- resolved from the fixed tool table, never user input
	}
	cmd.Dir = root
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		stdout("precise tier: " + indexer + " failed — " + textutil.FirstLine(errb.String()) + " (its ecosystem stays textual)")
		return nil, false
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel2()
	conv := exec.CommandContext(ctx2, scipBin, "print", "--json", scipPath) // nosemgrep -- resolved from the fixed tool table, never user input
	var convErr bytes.Buffer
	conv.Stderr = &convErr
	out, err := conv.Output()
	if err != nil {
		stdout("precise tier: scip print failed for " + indexer + " — " + textutil.FirstLine(convErr.String()+err.Error()) + " (its ecosystem stays textual)")
		return nil, false
	}
	var idx struct {
		Documents []json.RawMessage `json:"documents"`
	}
	if json.Unmarshal(out, &idx) != nil {
		stdout("precise tier: " + indexer + " output unreadable (its ecosystem stays textual)")
		return nil, false
	}
	return idx.Documents, true
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
