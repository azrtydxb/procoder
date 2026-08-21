// Package format runs one formatter over one file and answers with the
// formatted bytes — never by writing anything. The three possible answers are
// kept distinct on purpose, because collapsing them is how a gate starts
// lying:
//
//	clean       the tool ran and the file already matches its output
//	unformatted the tool ran and the formatted bytes differ — they are returned
//	unchecked   the tool could not answer (missing, no required config, error,
//	            timeout) and the reason is returned; NEVER reported as clean
package format

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"procoder/internal/tools"
)

// Verdict says what happened for one file.
type Verdict int

const (
	// OutOfScope: no formatter claims this file type. Counted, not judged.
	OutOfScope Verdict = iota
	// Clean: the formatter ran and the file already matches its output.
	Clean
	// Unformatted: the formatter ran and disagrees; the formatted bytes are
	// in hand for the agent to review and write.
	Unformatted
	// Unchecked: the formatter could not answer — missing, failed, or timed
	// out. Counted as failing, never as clean: a gate that cannot look is a
	// gate with a hole in it.
	Unchecked
)

// Result of checking one file.
type Result struct {
	File    string
	Verdict Verdict
	// Formatted holds the tool's output when Verdict is Unformatted.
	Formatted []byte
	// Reason explains Unchecked and OutOfScope in one sentence.
	Reason string
	// Tool is the formatter's name when one was responsible for the file.
	Tool string
}

// A formatter that has not answered in this long is wedged, not slow — the
// slowest healthy run observed (prettier cold start on a large file) is under
// two seconds. Reaching this is reported as Unchecked, never absorbed.
const hungToolTimeout = 30 * time.Second

// Check runs the file's formatter and compares output with the file's bytes.
func Check(file string) Result {
	tool := tools.ForFile(file)
	if tool == nil {
		return Result{File: file, Verdict: OutOfScope,
			Reason: "no formatter covers this file type"}
	}

	// The commit template's leading blank lines are the writing surface the
	// commit editor opens on — prettier strips them, which would break the
	// file's function. procoder's own dogfood CI caught this: the template is
	// content with meaning in its whitespace, not prose to normalise.
	if filepath.Base(filepath.Dir(file)) == "github" && filepath.Base(file) == "COMMIT_TEMPLATE.md" {
		return Result{File: file, Verdict: OutOfScope,
			Reason: "the commit template's leading blank lines are functional; formatting would remove them"}
	}

	src, err := os.ReadFile(file)
	if err != nil {
		return Result{File: file, Verdict: Unchecked, Tool: tool.Name,
			Reason: fmt.Sprintf("cannot read the file: %v", err)}
	}

	if !tools.HasProjectConfig(tool, file) {
		reason := fmt.Sprintf("no %s in the project — procoder imposes no style of its own",
			tool.NeedsProjectConfig)
		if tool.ConfigMissing != "" {
			reason = tool.ConfigMissing
		}
		return Result{File: file, Verdict: OutOfScope, Tool: tool.Name, Reason: reason}
	}

	bin := tools.Resolve(tool, tools.RepoRoot(mustDir(file)))
	if bin == "" {
		return Result{File: file, Verdict: Unchecked, Tool: tool.Name,
			Reason: fmt.Sprintf("%s is not installed — %s", tool.Name, tool.Install)}
	}

	formatted, err := run(bin, tool, file, src)
	if err != nil {
		return Result{File: file, Verdict: Unchecked, Tool: tool.Name,
			Reason: fmt.Sprintf("%s did not answer: %v", tool.Name, err)}
	}

	if bytes.Equal(normalizeEOF(src), normalizeEOF(formatted)) {
		return Result{File: file, Verdict: Clean, Tool: tool.Name}
	}
	return Result{File: file, Verdict: Unformatted, Tool: tool.Name, Formatted: formatted}
}

// run invokes the tool so it PRINTS the formatted result. Stdout is the whole
// answer; a non-zero exit or an empty answer on non-empty input is "did not
// answer", because an empty result is byte-for-byte what a clean empty file
// looks like.
func run(bin string, tool *tools.Tool, file string, src []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hungToolTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, tool.Args(file)...) // nosemgrep -- resolved from the fixed tool table, never user input
	if tool.Stdin {
		cmd.Stdin = bytes.NewReader(src)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("no answer after %s — the process was killed", hungToolTimeout)
		}
		// some tools (rubocop) exit 1 on success whenever they DETECTED
		// something; with a printed result on stdout that is the answer
		var exit *exec.ExitError
		if tool.ExitOneIsAnswer && errors.As(err, &exit) && exit.ExitCode() == 1 && out.Len() > 0 {
			return out.Bytes(), nil
		}
		return nil, fmt.Errorf("%v: %s", err, firstLine(errb.Bytes()))
	}
	if out.Len() == 0 && len(src) > 0 {
		return nil, fmt.Errorf("printed nothing for a non-empty file")
	}
	return out.Bytes(), nil
}

// normalizeEOF: a single trailing-newline difference is the one disagreement
// between formatters and editors that carries no information; comparing raw
// would flag files no human or agent considers unformatted.
func normalizeEOF(b []byte) []byte {
	return append(bytes.TrimRight(b, "\n"), '\n')
}

func firstLine(b []byte) string {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		b = b[:i]
	}
	if len(b) > 200 {
		b = b[:200]
	}
	return string(b)
}

func mustDir(file string) string {
	if abs, err := os.Getwd(); err == nil && !bytes.ContainsRune([]byte(file), os.PathSeparator) {
		return abs
	}
	return dirOf(file)
}

func dirOf(file string) string {
	for i := len(file) - 1; i >= 0; i-- {
		if os.IsPathSeparator(file[i]) {
			return file[:i]
		}
	}
	return "."
}
