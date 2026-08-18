// procoder — a harness that gives AI coders the tools and skills to work like
// a senior developer. This binary is the whole engine; hooks and skills are
// thin callers into it. See the design contract for what each command owes:
// the agent always stays in control, tools compute results and hand them over,
// and a file that could not be checked is never reported as clean.
package main

import (
	"fmt"
	"os"

	"procoder/internal/doctor"
	"procoder/internal/format"
	"procoder/internal/gate"
	"procoder/internal/hook"
)

// version is stamped by the release build via -ldflags.
var version = "dev"

const usage = `usage: procoder <command> [args]

  hook post-tool-use   read a PostToolUse payload on stdin; if the written file
                       is unformatted, hand the agent the formatted result
                       (the file itself is never modified)
  format <files...>    print each file's formatted result to stdout, so the
                       agent can review and write it
  check [paths...]     the commit gate: changed files (or the given paths) must
                       be formatted; unchecked counts as failing, skipped file
                       types are counted out loud
  doctor               which formatters this repository needs, which are
                       installed, and how to install the missing ones
  version              print the version
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	switch args[0] {
	case "hook":
		if len(args) < 2 || args[1] != "post-tool-use" {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return hook.Run(os.Stdin, os.Stdout)
	case "format":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return formatCmd(args[1:])
	case "check":
		return gate.Run(args[1:], doctor.Root(), os.Stdout)
	case "doctor":
		return doctor.Run(doctor.Root(), os.Stdout)
	case "version":
		fmt.Println(version)
		return 0
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

// formatCmd prints each file's formatted result — it writes nothing, per
// P-CONTROL. Multiple files are separated by headers so the agent can tell
// which output belongs to which file.
func formatCmd(files []string) int {
	code := 0
	for _, f := range files {
		res := format.Check(f)
		switch res.Verdict {
		case format.Clean:
			fmt.Printf("== %s — already formatted\n", f)
		case format.OutOfScope:
			fmt.Printf("== %s — out of scope: %s\n", f, res.Reason)
		case format.Unchecked:
			fmt.Printf("== %s — NOT checked: %s\n", f, res.Reason)
			code = 1
		case format.Unformatted:
			fmt.Printf("== %s — formatted result (%s), review and write it:\n", f, res.Tool)
			os.Stdout.Write(res.Formatted)
		}
	}
	return code
}
