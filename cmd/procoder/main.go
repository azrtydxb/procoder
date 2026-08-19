// procoder — a harness that gives AI coders the tools and skills to work like
// a senior developer. This binary is the whole engine; hooks and skills are
// thin callers into it. See the design contract for what each command owes:
// the agent always stays in control, tools compute results and hand them over,
// and a file that could not be checked is never reported as clean.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"procoder/internal/actions"
	"procoder/internal/audit"
	"procoder/internal/ciops"
	"procoder/internal/codeindex"
	"procoder/internal/config"
	"procoder/internal/debt"
	"procoder/internal/docs"
	"procoder/internal/doctor"
	"procoder/internal/format"
	"procoder/internal/gate"
	"procoder/internal/gitcmd"
	"procoder/internal/gitx"
	"procoder/internal/hook"
	"procoder/internal/infra"
	"procoder/internal/initcmd"
	"procoder/internal/lessons"
	"procoder/internal/lint"
	"procoder/internal/maintain"
	"procoder/internal/plan"
	"procoder/internal/portability"
	"procoder/internal/principles"
	"procoder/internal/security"
	"procoder/internal/spec"
	"procoder/internal/todo"
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
		// domain 2: the canonical linters, by ecosystem presence; eslint only
		// where the project carries a config (procoder imposes no rules)
		exts := doctor.ExtensionsIn(root)
		if _, err := os.Stat(root + "/go.mod"); err == nil {
			out = append(out, lint.GolangciLint)
		}
		if exts[".sh"] {
			out = append(out, lint.Shellcheck)
		}
		if lint.HasEslintConfig(root) {
			out = append(out, lint.Eslint)
		}
		// domain 1: gitleaks guards every repo; SAST and dependency scans
		// where there is code and manifests to scan
		out = append(out, security.Gitleaks)
		out = append(out, security.Semgrep)
		for _, m := range []string{"/go.mod", "/package.json", "/pyproject.toml", "/requirements.txt", "/Cargo.toml"} {
			if _, err := os.Stat(root + m); err == nil {
				out = append(out, security.OsvScanner)
				break
			}
		}
		// domain 8: each infra tool only where its files exist
		inv := infra.Detect(root)
		if len(inv.Dockerfiles) > 0 {
			out = append(out, infra.Hadolint)
		}
		if len(inv.TfDirs) > 0 {
			out = append(out, infra.Terraform, infra.Tflint)
		}
		if len(inv.K8sFiles) > 0 {
			out = append(out, infra.Kubeconform)
		}
		if len(inv.ChartDirs) > 0 {
			out = append(out, infra.Helm)
		}
		// the index: universal-ctags always (any code repo), the SCIP tools
		// where the repository's language has an indexer
		out = append(out, codeindex.Ctags)
		if _, err := os.Stat(root + "/go.mod"); err == nil {
			out = append(out, codeindex.ScipGo, codeindex.ScipCLI)
		}
		if _, err := os.Stat(root + "/tsconfig.json"); err == nil {
			out = append(out, codeindex.ScipTypescript, codeindex.ScipCLI)
		}
		if _, err := os.Stat(root + "/pyproject.toml"); err == nil {
			out = append(out, codeindex.ScipPython, codeindex.ScipCLI)
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
  audit                every domain's checks over the WHOLE tracked tree —
                       the onboarding sweep for a repository procoder has
                       not governed before; exit 1 if it would fail the gate
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
  ci                   workflow hygiene: pinned actions, job timeouts,
                       concurrency cancellation, tests exist
  infra                DevOps hygiene where the files exist: Dockerfiles
                       (hadolint), Terraform (fmt/validate/tflint),
                       Kubernetes manifests (kubeconform), Helm charts
  maintain             the maintainability report: dead-code candidates from
                       the index, complexity and function length — judgment
                       calls, never blocking
  security [--deep]    secrets over the changed files (gitleaks — blocking);
                       --deep adds SAST (semgrep) and dependency vulns
                       (osv-scanner) over the whole repository
  lint [paths...]      the canonical linter per ecosystem over the changed
                       files (or the given paths); report by default,
                       blocking when [lint] policy = "block"
  docs [--external]    the documentation report: broken references, diagrams,
                       drift, API doc comments, required docs, badges, README
                       structure; --external adds link checking and Pages health
  index <sub> [arg]    the code index (built from universal-ctags + SCIP):
                       build | find <symbol> | search <text> | refs <symbol> |
                       outline <file> | impact | callers <symbol> | unused |
                       entrypoints | graph | stats
  todo <sub> [arg]     the quality-gated task list under .procoder/todo/:
                       add <title> | list | show <id> | close <id> — close
                       REFUSES until every acceptance criterion is checked,
                       evidence is recorded, and the gate is clean
  plan <sub> [arg]     implementation plans under .procoder/plans/, the
                       spec → plan → todo middle link:
                       template <name> | list | check [name|all] — check
                       blocks on placeholders and on tasks without files
                       or steps
  debt                 harvest deliberate-simplification markers (comment
                       marker from [debt] in config.toml, default "debt:")
                       into a ledger; markers with no revisit trigger are
                       flagged
  agents               the universal agent layer: per-host rule files
                       derived from AGENTS.md (Cursor, Windsurf, Cline,
                       Kilo Code, Roo, Kiro, Antigravity, Qoder, Copilot,
                       Codex) — prints content for anything missing or
                       drifted; the gate blocks on drift
  lessons              the self-learning ledger (.procoder/github/LESSONS.md):
                       findings that escaped our gates, each with the
                       adaptation that closes its class; unlearned lessons
                       (no adaptation) exit 1
  principles [--hook]  print the engineering principles the session starts
                       with — .procoder/PRINCIPLES.md wins over the default;
                       --hook answers in the running host's SessionStart
                       JSON shape (claude/codex/copilot/qoder)
  spec <sub> [arg]     spec-first design under .procoder/specs/:
                       template <name> | list | check [name|all] — check
                       blocks while sections are missing, OPEN: questions
                       remain, or acceptance criteria are untestable
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
	case "audit":
		return audit.Run(doctor.Root(), func(s string) { fmt.Println(s) })
	case "check":
		return gate.Run(args[1:], doctor.Root(), os.Stdout)
	case "doctor":
		return doctor.Run(doctor.Root(), os.Stdout)
	case "init":
		execute := len(args) > 1 && args[1] == "--yes"
		return initcmd.Run(doctor.Root(), execute, os.Stdout)
	case "git":
		return gitcmd.Status(doctor.Root(), os.Stdout)
	case "ci":
		root := doctor.Root()
		findings := ciops.Check(root, config.Load(root).PinActions)
		blocking := 0
		for _, f := range findings {
			mark := "  info "
			if f.Blocking {
				mark = "  BLOCK"
				blocking++
			}
			loc := f.File
			if rel, err := filepath.Rel(root, loc); err == nil && !strings.HasPrefix(rel, "..") {
				loc = rel
			}
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", loc, f.Line)
			}
			if loc != "" {
				loc += "  "
			}
			fmt.Printf("%s %s%s\n", mark, loc, f.Message)
		}
		fmt.Printf("procoder ci: %d finding(s) (%d blocking)\n", len(findings), blocking)
		if blocking > 0 {
			return 1
		}
		return 0
	case "infra":
		root := doctor.Root()
		if infra.Detect(root).Empty() {
			fmt.Println("procoder infra: no infrastructure files in this repository")
			return 0
		}
		findings := infra.Check(root)
		blocking := 0
		for _, f := range findings {
			mark := "  info "
			if f.Blocking {
				mark = "  BLOCK"
				blocking++
			}
			loc := f.File
			if rel, err := filepath.Rel(root, loc); err == nil && !strings.HasPrefix(rel, "..") {
				loc = rel
			}
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", loc, f.Line)
			}
			if loc != "" {
				loc += "  "
			}
			fmt.Printf("%s %s%s\n", mark, loc, f.Message)
		}
		fmt.Printf("procoder infra: %d finding(s) (%d blocking)\n", len(findings), blocking)
		if blocking > 0 {
			return 1
		}
		return 0
	case "maintain":
		return maintain.Run(doctor.Root(), func(s string) { fmt.Println(s) })
	case "security":
		root := doctor.Root()
		changed, _ := gitx.ChangedFiles(root)
		findings := security.SecretsChangedFiles(root, changed)
		if len(args) > 1 && args[1] == "--deep" {
			findings = append(findings, security.Sast(root)...)
			findings = append(findings, security.Deps(root)...)
		}
		blocking := 0
		for _, f := range findings {
			mark := "  info "
			if f.Blocking {
				mark = "  BLOCK"
				blocking++
			}
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", loc, f.Line)
			}
			if loc != "" {
				loc += "  "
			}
			fmt.Printf("%s %s%s\n", mark, loc, f.Message)
		}
		fmt.Printf("procoder security: %d finding(s) (%d blocking)\n", len(findings), blocking)
		if blocking > 0 {
			return 1
		}
		return 0
	case "lint":
		root := doctor.Root()
		paths := args[1:]
		if len(paths) == 0 {
			changed, err := gitx.ChangedFiles(root)
			if err != nil {
				fmt.Println(err)
				return 1
			}
			paths = changed
		}
		cfg := config.Load(root)
		findings := lint.Files(root, paths, cfg.LintBlock)
		blocking := 0
		for _, f := range findings {
			mark := "  info "
			if f.Blocking {
				mark = "  BLOCK"
				blocking++
			}
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", loc, f.Line)
			}
			fmt.Printf("%s %s  %s\n", mark, loc, f.Message)
		}
		fmt.Printf("procoder lint: %d finding(s) (%d blocking)\n", len(findings), blocking)
		if blocking > 0 {
			return 1
		}
		return 0
	case "docs":
		external := len(args) > 1 && args[1] == "--external"
		root := doctor.Root()
		changed, _ := gitx.ChangedFiles(root)
		return docs.Run(root, changed, external, os.Stdout)
	case "index":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return indexCmd(args[1:])
	case "todo":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return todoCmd(args[1:])
	case "spec":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return specCmd(args[1:])
	case "plan":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return planCmd(args[1:])
	case "debt":
		return debt.Run(doctor.Root(), func(s string) { fmt.Println(s) })
	case "lessons":
		return lessons.Run(doctor.Root(), func(s string) { fmt.Println(s) })
	case "agents":
		return portability.Agents(doctor.Root(), func(s string) { fmt.Println(s) })
	case "principles":
		if len(args) > 1 && args[1] == "--hook" {
			return principles.RunHook(doctor.Root(), func(s string) { fmt.Println(s) })
		}
		return principles.Run(doctor.Root(), func(s string) { fmt.Println(s) })
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

// indexCmd dispatches the index subcommands; every query prints through one
// line-writer so output stays uniform.
func indexCmd(args []string) int {
	root := doctor.Root()
	out := func(s string) { fmt.Println(s) }
	switch args[0] {
	case "build":
		if err := codeindex.Build(root, out); err != nil {
			fmt.Println(err)
			return 1
		}
		return 0
	case "callers":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return codeindex.Callers(root, args[1], out)
	case "unused":
		return codeindex.Unused(root, out)
	case "entrypoints":
		return codeindex.Entrypoints(root, out)
	case "graph":
		return codeindex.Graph(root, out)
	case "find", "search", "refs", "outline":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		switch args[0] {
		case "find":
			return codeindex.Find(root, args[1], out)
		case "search":
			return codeindex.Search(root, args[1], out)
		case "refs":
			return codeindex.Refs(root, args[1], out)
		default:
			return codeindex.Outline(root, args[1], out)
		}
	case "impact":
		changed, err := gitx.ChangedFiles(root)
		if err != nil {
			fmt.Println(err)
			return 1
		}
		return codeindex.Impact(root, changed, out)
	case "stats":
		return codeindex.Stats(root, out)
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

// todoCmd dispatches the quality-gated task list. close runs the real gate
// as its final criterion — a task is not done while the tree fails checks.
func todoCmd(args []string) int {
	root := doctor.Root()
	out := func(s string) { fmt.Println(s) }
	switch args[0] {
	case "list":
		tasks, err := todo.List(root)
		if err != nil {
			out(err.Error())
			return 1
		}
		if len(tasks) == 0 {
			out("no tasks — `procoder todo add <title>` starts one")
			return 0
		}
		for _, t := range tasks {
			out(fmt.Sprintf("  [%s]  %s  %s", t.Status, t.ID, t.Title))
		}
		return 0
	case "add":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return todo.Add(root, strings.Join(args[1:], " "), out)
	case "show":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		path, err := todo.File(root, args[1])
		if err != nil {
			out(err.Error())
			return 2
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			out("no task " + args[1] + " — `procoder todo list` shows what exists")
			return 2
		}
		os.Stdout.Write(raw)
		return 0
	case "close":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return todo.Close(root, args[1], func() bool {
			return gate.Run(nil, root, io.Discard) == 0
		}, out)
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

// planCmd dispatches the plan quality controller.
func planCmd(args []string) int {
	root := doctor.Root()
	out := func(s string) { fmt.Println(s) }
	switch args[0] {
	case "list":
		return plan.List(root, out)
	case "template":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return plan.PrintTemplate(args[1], out)
	case "check":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		return plan.Check(root, name, out)
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

// specCmd dispatches the spec quality controller.
func specCmd(args []string) int {
	root := doctor.Root()
	out := func(s string) { fmt.Println(s) }
	switch args[0] {
	case "list":
		return spec.List(root, out)
	case "template":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return spec.PrintTemplate(args[1], out)
	case "check":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		return spec.Check(root, name, out)
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
