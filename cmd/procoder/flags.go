package main

import (
	"fmt"
	"io"
	"strings"
)

// knownFlags is every flag each command accepts. A command absent from the
// table accepts none.
//
// It exists because the arms read their flags positionally — `args[1] ==
// "--deep"` — which means a flag nobody implemented was not refused, it was
// ignored, and for the arms that take paths it was worse: `procoder check
// --staged` handed "--staged" to the formatter, which has no formatter for
// that file type, counted it out of scope, found nothing else to look at,
// and exited 0. A gate that reports clean because it checked a filename
// somebody typed by mistake is the exact failure this project calls a
// silent green, sitting in the command that enforces it.
//
// `version` refused unrecognised flags from the start. This is that rule
// applied to the other seventy-seven.
var knownFlags = map[string][]string{
	"ask":          {"--file"},
	"backlog":      {"--epic", "--milestone", "--severity"},
	"bench":        {"--save"},
	"ci":           {"--runs"},
	"copilot-leak": {"--since", "--quiet", "--from-copilot"},
	"docs":         {"--ack", "--external"},
	"dispatch":     {"--task"},
	"claims":       {"--by"},
	"env":          {"--sync"},
	"index":        {"--at"},
	"init":         {"--yes"},
	"lint":         {"--types"},
	"principles":   {"--hook"},
	"prune":        {"--apply"},
	"release":      {"--credits"},
	"review":       {"--lens", "--perspectives"},
	"run":          {"--exec"},
	"security":     {"--deep"},
	"self-upgrade": {"--force"},
	"test":         {"--coverage", "--name"},
	"version":      {"--check"},
}

// checkFlags refuses a flag the command does not implement, and says which
// flags it does. It returns the arguments the command should see, and false
// when the caller should stop.
//
// Scanning stops at the first positional argument, and at an explicit `--`.
// That is deliberate rather than lazy: the tail of several commands is free
// text a person wrote — a commit message for `docs --ack`, a reason for
// `sprint carry` — and a word there beginning with a dash is a word, not a
// flag procoder should reject. Every flag the arms actually read is read
// from the front, so the front is where the check belongs.
//
// A `--` that ends the scan is removed. Leaving it in would hand the arms
// a separator to treat as an argument, which is how `procoder check --
// file.go` came to report a file called "--" as out of scope: the same
// swallowed-token bug in a different coat.
func checkFlags(args []string, stderr io.Writer) ([]string, bool) {
	if len(args) == 0 {
		return args, true
	}
	allowed := knownFlags[args[0]]
	for i, a := range args[1:] {
		if a == "--" {
			return append(args[:i+1:i+1], args[i+2:]...), true
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			return args, true
		}
		if !contains(allowed, a) {
			fmt.Fprintf(stderr, "procoder %s: %s is not a flag of this command — %s\n",
				args[0], a, flagsSentence(allowed))
			return args, false
		}
	}
	return args, true
}

func contains(set []string, s string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}

func flagsSentence(allowed []string) string {
	if len(allowed) == 0 {
		return "it takes no flags"
	}
	return "it takes " + strings.Join(allowed, ", ")
}

// currentCmd is the top-level command being dispatched, kept so a usage
// error can print that command's own help. A package variable rather than
// a parameter threaded through forty-five call sites: the process runs one
// command and exits.
var currentCmd string

// usageErr prints the usage of the command that was actually typed, and
// returns the usage exit code.
//
// It replaces printing the whole usage text, which is 159 lines. Somebody
// who typed `procoder adr` and forgot the subcommand had to find one line
// of the seventy-eight to learn what they were missing, which is a poor
// answer to a question procoder knows the answer to.
func usageErr(stderr io.Writer) int {
	fmt.Fprint(stderr, usageFor(currentCmd))
	return 2
}

// usageFor cuts one command's block out of the usage text: the line that
// names it, plus every continuation line under it. A command it cannot
// find falls back to the whole text, because a person who mistyped the
// command name does want the list.
func usageFor(cmd string) string {
	lines := strings.Split(usage, "\n")
	start := -1
	for i, l := range lines {
		if !strings.HasPrefix(l, "  ") || strings.HasPrefix(l, "   ") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimSpace(l), " ")
		if name == cmd {
			start = i
			break
		}
	}
	if start < 0 {
		return usage
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "  ") && !strings.HasPrefix(lines[i], "   ") {
			end = i
			break
		}
	}
	// the block keeps its own indentation so the continuation lines stay
	// aligned under the description, which is what makes it readable
	return "usage:\n" + strings.Join(lines[start:end], "\n") + "\n"
}

// flagValue reads `--name value` out of an argument list. Absent is "",
// which every caller treats as "not given" and refuses on when it matters.
func flagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
		if v, ok := strings.CutPrefix(a, name+"="); ok {
			return v
		}
	}
	return ""
}
