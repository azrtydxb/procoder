package codeindex

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"procoder/internal/textutil"
	"procoder/internal/tools"
)

// Gopls is Go's language server; its command line computes cross-file
// renames. procoder only ever asks it for the diff (-d), never the write.
var Gopls = &tools.Tool{
	Name:        "gopls",
	Install:     "go install golang.org/x/tools/gopls@latest",
	VersionArgs: []string{"version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "go", Args: []string{"install", "golang.org/x/tools/gopls@latest"}},
		{Manager: "brew", Args: []string{"install", "gopls"}},
	},
}

// Rename computes a cross-file rename of symbol to newName and prints it as
// a unified diff for the agent to review and apply — per P-CONTROL nothing
// is written. Go has a real engine (gopls); every other language answers
// honestly with the reference worksheet instead of a half-right rewrite.
// at ("path:line") picks one definition when the name is defined more than
// once.
func Rename(root, symbol, newName, at string, out func(string)) int {
	tags, err := loadTags(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	var defs []Tag
	for _, t := range tags {
		if t.Name == symbol {
			defs = append(defs, t)
		}
	}
	if at != "" {
		defs = filterAt(defs, at)
	}
	if len(defs) == 0 {
		out(fmt.Sprintf("no definition of %q in the index — try `procoder index search %s`", symbol, symbol))
		return 1
	}
	if len(defs) > 1 {
		out(fmt.Sprintf("%q is defined %d times — rerun with --at <path:line> to pick one:", symbol, len(defs)))
		for _, t := range defs {
			printTag(out, t)
		}
		return 2
	}
	def := defs[0]
	if strings.ToLower(filepath.Ext(def.Path)) != ".go" {
		out(fmt.Sprintf("no rename engine for %s — procoder computes renames only where the language ships one (Go, via gopls)", def.Path))
		out("the reference worksheet instead — edit each site yourself, then verify with `procoder index refs`:")
		return Refs(root, symbol, out)
	}
	return goplsRename(root, def, symbol, newName, out)
}

// filterAt keeps the definitions matching a "path:line" pick.
func filterAt(defs []Tag, at string) []Tag {
	path, lineStr, ok := strings.Cut(at, ":")
	line, err := strconv.Atoi(lineStr)
	if !ok || err != nil {
		return nil
	}
	var out []Tag
	for _, t := range defs {
		if t.Path == filepath.ToSlash(path) && t.Line == line {
			out = append(out, t)
		}
	}
	return out
}

// goplsRename asks gopls for the rename diff at the definition's position.
func goplsRename(root string, def Tag, symbol, newName string, out func(string)) int {
	col, err := symbolColumn(filepath.Join(root, def.Path), def.Line, symbol)
	if err != nil {
		out(fmt.Sprintf("%s:%d no longer holds %q (%v) — run `procoder index build` and retry", def.Path, def.Line, symbol, err))
		return 1
	}
	bin := tools.Resolve(Gopls, root)
	if bin == "" {
		out("rename NOT computed — gopls is not installed (" + Gopls.Install + ")")
		return 1
	}
	pos := fmt.Sprintf("%s:%d:%d", def.Path, def.Line, col)
	ctx, cancel := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "rename", "-d", pos, newName) // nosemgrep -- resolved from the fixed tool table, never user input
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			out(fmt.Sprintf("rename NOT computed — gopls gave no answer in %s", hungToolTimeout))
			return 1
		}
		out("rename FAILED — nothing was changed: " + textutil.FirstLine(stderr.String()+err.Error()))
		return 1
	}
	diff := strings.TrimRight(stdout.String(), "\n")
	if diff == "" {
		out(fmt.Sprintf("gopls found nothing to rename at %s — the index may be stale; run `procoder index build`", pos))
		return 1
	}
	for _, line := range strings.Split(diff, "\n") {
		out(line)
	}
	out(fmt.Sprintf("review and apply this diff yourself — procoder never writes code; then rebuild the index and verify with `procoder index refs %s`", newName))
	if n := staleNote(root); n != "" {
		out(n)
	}
	return 0
}

// symbolColumn finds the 1-based column of symbol on the given line, on a
// word boundary — the position gopls resolves the rename from.
func symbolColumn(path string, line int, symbol string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for n := 1; sc.Scan(); n++ {
		if n != line {
			continue
		}
		text := sc.Text()
		for from := 0; ; {
			i := strings.Index(text[from:], symbol)
			if i < 0 {
				break
			}
			start := from + i
			end := start + len(symbol)
			if (start == 0 || !isWordByte(text[start-1])) && (end == len(text) || !isWordByte(text[end])) {
				return start + 1, nil
			}
			from = end
		}
		return 0, fmt.Errorf("symbol not on the line")
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("file has fewer than %d lines", line)
}

func isWordByte(b byte) bool {
	return b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9')
}
