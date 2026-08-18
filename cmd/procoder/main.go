// procoder — a harness that gives AI coders the tools and skills to work like
// a senior developer. This binary is the whole engine; hooks and skills are
// thin callers into it. See the design contract for what each command owes:
// the agent always stays in control, tools compute results and hand them over,
// and a file that could not be checked is never reported as clean.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"procoder/internal/actions"
	"procoder/internal/docs"
	"procoder/internal/doctor"
	"procoder/internal/format"
	"procoder/internal/gate"
	"procoder/internal/gitcmd"
	"procoder/internal/gitx"
	"procoder/internal/hook"
	"procoder/internal/initcmd"
	"procoder/internal/tools"
)

func init() {
	// actionlint is required when the repo carries workflow files; gh when it
	// pushes to GitHub. Wired here to keep doctor free of import cycles.
	doctor.ExtraTools = func(root string) []*tools.Tool {
		var out []*tools.Tool
		if entries, err := os.ReadDir(root + "/.github/workflows"); err == nil && len(entries) > 0 {
			out = append(out, actions.Actionlint)
		}
		// asked of git itself, so worktrees (where .git is a file) answer too
		if raw, err := exec.Command("git", "-C", root, "config", "--get", "remote.origin.url").Output(); err == nil && strings.Contains(string(raw), "github.com") {
			out = append(out, actions.Gh)
		}
		if md := docs.MarkdownFiles(root); len(md) > 0 {
			out = append(out, docs.Lychee)
			for _, f := range md {
				if raw, err := os.ReadFile(f); err == nil && strings.Contains(string(raw), "```mermaid") {
					out = append(out, docs.Mmdc)
					break
				}
			}
		}
		if _, err := os.Stat(root + "/mkdocs.yml"); err == nil {
			out = append(out, docs.Mkdocs)
		}
		return out
	}
}

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
  init [--yes]         print the install commands for the missing formatters;
                       --yes runs them and re-checks that every tool answers
  git                  the pre-finish status: branch vs default, hygiene
                       findings (conflict markers, junk, oversized), message
                       checks, workflow lint, template state
  docs [--external]    the documentation report: broken references, diagrams,
                       drift, API doc comments, required docs, badges, README
                       structure; --external adds link checking and Pages health
  templates            print the default content for any missing template
                       under .procoder/github/ — the agent reviews and writes it
  scrub <file|->       check text (a commit message, a drafted PR body) for
                       AI-attribution lines; exits 1 when any are found
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
	case "init":
		execute := len(args) > 1 && args[1] == "--yes"
		return initcmd.Run(doctor.Root(), execute, os.Stdout)
	case "git":
		return gitcmd.Status(doctor.Root(), os.Stdout)
	case "docs":
		external := len(args) > 1 && args[1] == "--external"
		root := doctor.Root()
		changed, _ := gitx.ChangedFiles(root)
		return docs.Run(root, changed, external, os.Stdout)
	case "templates":
		return gitcmd.Templates(doctor.Root(), os.Stdout)
	case "scrub":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return gitcmd.Scrub(args[1], os.Stdin, os.Stdout)
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
