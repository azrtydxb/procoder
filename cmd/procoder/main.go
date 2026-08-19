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
	"procoder/internal/adr"
	"procoder/internal/audit"
	"procoder/internal/backlog"
	"procoder/internal/bench"
	"procoder/internal/ciops"
	"procoder/internal/codeindex"
	"procoder/internal/config"
	"procoder/internal/debt"
	"procoder/internal/deps"
	"procoder/internal/docs"
	"procoder/internal/doctor"
	"procoder/internal/envsync"
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
	"procoder/internal/release"
	"procoder/internal/runcmd"
	"procoder/internal/security"
	"procoder/internal/spec"
	"procoder/internal/status"
	"procoder/internal/testrun"
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
			// gopls rides along: it computes the `index rename` diffs for Go
			out = append(out, lint.GolangciLint, codeindex.Gopls)
		}
		// the --types checkers, where the canonical linter does not compile:
		// tsc under a project tsconfig, pyright where Python is a project
		if _, err := os.Stat(root + "/tsconfig.json"); err == nil {
			out = append(out, lint.Tsc)
		}
		for _, m := range []string{"/pyproject.toml", "/requirements.txt"} {
			if _, err := os.Stat(root + m); err == nil {
				out = append(out, lint.Pyright)
				break
			}
		}
		if exts[".sh"] {
			out = append(out, lint.Shellcheck)
		}
		if lint.HasEslintConfig(root) {
			out = append(out, lint.Eslint)
		}
		// the wider language matrix: each linter only where its files exist
		// (the FORMATTERS for these languages ride doctor's extension
		// survey automatically via the formatter registry)
		if exts[".kt"] || exts[".kts"] {
			out = append(out, lint.Ktlint)
		}
		if exts[".swift"] {
			out = append(out, lint.Swiftlint)
		}
		if exts[".rb"] || exts[".rake"] {
			out = append(out, lint.RubocopLint)
		}
		if _, err := os.Stat(root + "/Cargo.toml"); err == nil {
			out = append(out, lint.Cargo)
		}
		if exts[".java"] {
			out = append(out, lint.Checkstyle)
		}
		// domain 1: gitleaks guards every repo; SAST and dependency scans
		// where there is code and manifests to scan
		out = append(out, security.Gitleaks)
		out = append(out, security.Semgrep)
		// one shared list with Deps(), so doctor requires the scanner for
		// exactly the repos the scan would cover
		for _, m := range security.DepManifests {
			if _, err := os.Stat(root + "/" + m); err == nil {
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
		if _, err := os.Stat(root + "/Cargo.toml"); err == nil {
			out = append(out, codeindex.RustAnalyzer, codeindex.ScipCLI)
		}
		for _, m := range []string{"/pom.xml", "/build.gradle", "/build.gradle.kts"} {
			if _, err := os.Stat(root + m); err == nil {
				out = append(out, codeindex.ScipJava, codeindex.ScipCLI)
				break
			}
		}
		return out
	}
}

// version is stamped by the release build via -ldflags.
var version = "dev"

const usage = `usage: procoder <command> [args]

  agents               the universal agent layer: per-host rule files
                       derived from AGENTS.md (Cursor, Windsurf, Cline,
                       Kilo Code, Roo, Kiro, Antigravity, Qoder, Copilot,
                       Codex) — prints content for anything missing or
                       drifted; the gate blocks on drift
  audit                every domain's checks over the WHOLE tracked tree —
                       the onboarding sweep for a repository procoder has
                       not governed before; exit 1 if it would fail the gate
  backlog <sub>        the project layer: milestones, epics, and user
                       stories under .procoder/backlog/ —
                       milestone <title> | epic <title> [--milestone <id>] |
                       story <title> --epic <id> | seed <spec> [--milestone
                       <id>] | list | board | close story <id>... (several
                       share one gate and suite run) | close epic|milestone
                       <id>; stories carry todo-task rigor, closes REFUSE
                       until the controller is satisfied (todo itself stays
                       the standalone list for non-spec work)
  adr <sub>            architecture decision records under .procoder/adr/:
                       new <title> | list | check — durable decisions with
                       their date and context; check refuses hollow records,
                       bad statuses, and dangling supersede references
  bench [--save]       run the repository's Go benchmarks and compare against
                       the saved baseline (.procoder/bench/baseline.txt);
                       regressions beyond [bench] threshold (default 10%) are
                       marked and exit 1; --save records a new baseline
  check [paths...]     the commit gate: changed files (or the given paths) must
                       be formatted; unchecked counts as failing, skipped file
                       types are counted out loud
  ci [--runs]          workflow hygiene: pinned actions, job timeouts,
                       concurrency cancellation, tests exist; --runs asks gh
                       for this branch's newest run per workflow instead, and
                       says when the newest run predates your latest push
  debt                 harvest deliberate-simplification markers (comment
                       marker from [debt] in config.toml, default "debt:")
                       into a ledger; markers with no revisit trigger are
                       flagged
  deps                 the freshness report: outdated dependencies per
                       ecosystem via each one's native tool, licenses where a
                       tool exists — report-only, judgment stays yours
  env [--sync]         what changed in the project's environment since you
                       last synced: lockfile digests, migrations added, new
                       .env keys (names only, never values); --sync records
                       the current tree as the baseline
  docs [--external | --ack <reason>]
                       the documentation report: broken references, diagrams,
                       drift, API doc comments, required docs, badges, README
                       structure; --external adds link checking and Pages health
  doctor               which formatters this repository needs, which are
                       installed, and how to install the missing ones
  format <files...>    print each file's formatted result to stdout, so the
                       agent can review and write it
  git                  the pre-finish status: branch vs default, hygiene
                       findings (conflict markers, junk, oversized), message
                       checks, workflow lint, template state
  hook <sub>           the host hooks: post-tool-use reads a PostToolUse
                       payload and hands back the formatted result plus lint
                       and secret findings (the file is never modified);
                       pre-tool-use intercepts a git commit and stops it
                       when the gate has blocking findings ([git]
                       commit_gate = block|report|off, default block);
                       install-git prints a .git/hooks/pre-commit script so
                       the gate also holds outside any agent
  index <sub> [arg]    the code index (built from universal-ctags + SCIP):
                       build | find <symbol> | search <text> | refs <symbol> |
                       outline <file> | impact | callers <symbol> | unused |
                       entrypoints | graph | stats |
                       impls <symbol> — what implements an interface or its
                       methods (precise tier only) |
                       rename <symbol> <new> [--at path:line] — the rename
                       as a reviewable diff (Go via gopls); nothing is written
  infra                DevOps hygiene where the files exist: Dockerfiles
                       (hadolint), Terraform (fmt/validate/tflint),
                       Kubernetes manifests (kubeconform), Helm charts
  init [--yes]         print the install commands for the missing formatters;
                       --yes runs them and re-checks that every tool answers
  lessons              the self-learning ledger (.procoder/github/LESSONS.md):
                       findings that escaped our gates, each with the
                       adaptation that closes its class; unlearned lessons
                       (no adaptation) exit 1
  lint [--types] [paths...]
                       the canonical linter per ecosystem over the changed
                       files (or the given paths); report by default,
                       blocking when [lint] policy = "block"; --types adds
                       the type-checker where the linter does not compile
                       (tsc --noEmit for TypeScript, pyright for Python)
  maintain             the maintainability report: dead-code candidates from
                       the index, complexity and function length — judgment
                       calls, never blocking
  plan <sub> [arg]     implementation plans under .procoder/plans/, the
                       spec → plan → todo middle link:
                       template <name> | list | check [name|all] — check
                       blocks on placeholders and on tasks without files
                       or steps
  principles [--hook]  print the engineering principles the session starts
                       with — .procoder/PRINCIPLES.md wins over the default;
                       --hook answers in the running host's SessionStart
                       JSON shape (claude/codex/copilot/qoder)
  release [<version>]  the pre-tag controller: version-sync across [release]
                       files, the changelog entry, a clean tree, the gate, and
                       the suite under [test] policy — every failure listed,
                       and on success the tag command printed, never run
  run [--exec]         how to run this project: the launch command(s) it
                       declares, with the file that declared each; --exec runs
                       a single non-server candidate for you
  scrub <file|->       check text (a commit message, a drafted PR body) for
                       AI-attribution lines; exits 1 when any are found
  security [--deep]    secrets over the changed files (gitleaks — blocking);
                       --deep adds SAST (semgrep) and dependency vulns
                       (osv-scanner) over the whole repository
  sprint <sub>         scope-boxed sprints over the backlog: open <goal> |
                       pull <story-id>... | carry <story-id> <reason> |
                       status | close — one active sprint at a time, close
                       refuses while a committed story is neither done nor
                       explicitly carried back with a reason
  status               the state of play, computed fresh: branch, dirty
                       files, the active sprint and its open stories, open
                       tasks, unlearned lessons, index freshness
  spec <sub> [arg]     spec-first design under .procoder/specs/:
                       template <name> | list | check [name|all] — check
                       blocks while sections are missing, OPEN: questions
                       remain, or acceptance criteria are untestable
  templates            print the default content for any missing template
                       under .procoder/github/ — the agent reviews and writes it
  test [--coverage] [--name <pattern>] [paths...]
                       run the repository's actual test suite: every detected
                       ecosystem's canonical runner (go test, cargo test, the
                       package.json test script, pytest, gradle/maven), each
                       reported honestly — NOT run is never the same as green;
                       --coverage reports the percentage where the runner
                       measures it natively; with [test] policy = "block" the
                       todo and story closes refuse while the suite is red
  todo <sub> [arg]     the quality-gated task list under .procoder/todo/:
                       add <title> | list | show <id> | close <id> — close
                       REFUSES until every acceptance criterion is checked,
                       evidence is recorded, and the gate is clean
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
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		switch args[1] {
		case "post-tool-use":
			return hook.Run(os.Stdin, os.Stdout)
		case "pre-tool-use":
			return hook.PreToolUse(os.Stdin, os.Stdout)
		case "install-git":
			return hook.InstallGit(doctor.Root(), func(s string) { fmt.Println(s) })
		case "stop":
			return hook.Stop(os.Stdin, doctor.Root())
		default:
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
	case "format":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return formatCmd(args[1:])
	case "audit":
		return audit.Run(doctor.Root(), func(s string) { fmt.Println(s) })
	case "status":
		return status.Run(doctor.Root(), func(s string) { fmt.Println(s) })
	case "env":
		sync := len(args) > 1 && args[1] == "--sync"
		if len(args) > 1 && !sync {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return envsync.Run(doctor.Root(), sync, func(s string) { fmt.Println(s) })
	case "run":
		exec := len(args) > 1 && args[1] == "--exec"
		if len(args) > 1 && !exec {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return runcmd.Run(doctor.Root(), exec, func(s string) { fmt.Println(s) })
	case "adr":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		root := doctor.Root()
		out := func(s string) { fmt.Println(s) }
		switch args[1] {
		case "new":
			if len(args) < 3 {
				fmt.Fprint(os.Stderr, usage)
				return 2
			}
			return adr.New(root, strings.Join(args[2:], " "), out)
		case "list":
			return adr.List(root, out)
		case "check":
			return adr.Run(root, out)
		default:
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
	case "bench":
		root := doctor.Root()
		save := len(args) > 1 && args[1] == "--save"
		if len(args) > 1 && !save {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return bench.Run(root, save, config.Load(root).BenchThreshold, func(s string) { fmt.Println(s) })
	case "deps":
		return deps.Run(doctor.Root(), func(s string) { fmt.Println(s) })
	case "release":
		root := doctor.Root()
		version := ""
		if len(args) > 1 {
			version = args[1]
		}
		return release.Run(root, version, func() bool {
			return gate.Run(nil, root, io.Discard) == 0
		}, suiteCheck(root), func(s string) { fmt.Println(s) })
	case "backlog":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return backlogCmd(args[1:])
	case "sprint":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return sprintCmd(args[1:])
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
		if len(args) > 1 && args[1] == "--runs" {
			return ciops.Runs(root, func(s string) { fmt.Println(s) })
		}
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
		types := false
		var paths []string
		for _, a := range args[1:] {
			if a == "--types" {
				types = true
				continue
			}
			paths = append(paths, a)
		}
		if len(paths) == 0 {
			changed, err := gitx.ChangedFiles(root)
			if err != nil {
				fmt.Println(err)
				return 1
			}
			paths = changed
		} else {
			// a directory argument means its files — lint switches on file
			// extension, so an unexpanded directory would silently lint nothing
			var expanded []string
			for _, p := range paths {
				abs := p
				if !filepath.IsAbs(abs) {
					abs = filepath.Join(root, abs)
				}
				if info, err := os.Stat(abs); err == nil && info.IsDir() {
					expanded = append(expanded, gitx.FilesUnder(root, p)...)
				} else {
					expanded = append(expanded, p)
				}
			}
			paths = expanded
		}
		cfg := config.Load(root)
		findings := lint.Files(root, paths, cfg.LintBlock)
		if types {
			findings = append(findings, lint.Types(root, paths, cfg.LintBlock)...)
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
			fmt.Printf("%s %s  %s\n", mark, loc, f.Message)
		}
		fmt.Printf("procoder lint: %d finding(s) (%d blocking)\n", len(findings), blocking)
		if blocking > 0 {
			return 1
		}
		return 0
	case "docs":
		root := doctor.Root()
		// --ack prints the line that clears the documentation obligation;
		// the agent places it in the commit message, where a reviewer sees
		// the decision instead of a silent skip
		if len(args) > 1 && args[1] == "--ack" {
			if len(args) < 3 || strings.TrimSpace(strings.Join(args[2:], " ")) == "" {
				fmt.Fprint(os.Stderr, usage)
				return 2
			}
			fmt.Println(docs.AckLine(strings.Join(args[2:], " ")))
			return 0
		}
		external := len(args) > 1 && args[1] == "--external"
		changed, _ := gitx.ChangedFiles(root)
		return docs.RunFor(root, changed, "", external, config.Load(root).DocsBlock, os.Stdout)
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
	case "test":
		root := doctor.Root()
		coverage := false
		name := ""
		var paths []string
		for i := 1; i < len(args); i++ {
			switch {
			case args[i] == "--coverage":
				coverage = true
			case args[i] == "--name" && i+1 < len(args):
				name = args[i+1]
				i++
			default:
				paths = append(paths, args[i])
			}
		}
		return testrun.Report(testrun.Run(root, paths, coverage, name), func(s string) { fmt.Println(s) })
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
	case "find", "search", "refs", "outline", "impls":
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
		case "impls":
			return codeindex.Impls(root, args[1], out)
		default:
			return codeindex.Outline(root, args[1], out)
		}
	case "rename":
		// rename <symbol> <new> [--at path:line] — --at picks one
		// definition when the name is defined more than once
		at := ""
		var pos []string
		for i := 1; i < len(args); i++ {
			if args[i] == "--at" && i+1 < len(args) {
				at = args[i+1]
				i++
				continue
			}
			pos = append(pos, args[i])
		}
		if len(pos) != 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return codeindex.Rename(root, pos[0], pos[1], at, out)
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

// backlogCmd dispatches the project layer. Creation subcommands parse
// their flag in the loop style of `index rename --at`; closes route to
// the refusing controllers.
func backlogCmd(args []string) int {
	root := doctor.Root()
	out := func(s string) { fmt.Println(s) }
	// positional args with one optional flag, per subcommand
	flagVal := func(name string, rest []string) (string, []string) {
		val := ""
		var pos []string
		for i := 0; i < len(rest); i++ {
			if rest[i] == name && i+1 < len(rest) {
				val = rest[i+1]
				i++
				continue
			}
			pos = append(pos, rest[i])
		}
		return val, pos
	}
	switch args[0] {
	case "milestone":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return backlog.Milestone(root, strings.Join(args[1:], " "), out)
	case "epic":
		ms, pos := flagVal("--milestone", args[1:])
		if len(pos) == 0 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return backlog.Epic(root, strings.Join(pos, " "), ms, out)
	case "story":
		epic, pos := flagVal("--epic", args[1:])
		if len(pos) == 0 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return backlog.Story(root, strings.Join(pos, " "), epic, out)
	case "bug":
		epic, pos := flagVal("--epic", args[1:])
		severity, pos := flagVal("--severity", pos)
		if len(pos) == 0 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return backlog.Bug(root, strings.Join(pos, " "), epic, severity, out)
	case "seed":
		ms, pos := flagVal("--milestone", args[1:])
		if len(pos) != 1 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return backlog.Seed(root, pos[0], ms, out)
	case "list":
		return backlog.List(root, out)
	case "board":
		return backlog.Board(root, out)
	case "close":
		if len(args) < 3 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		switch args[1] {
		case "story":
			// several ids share one verification: the gate and the suite
			// judge the tree, not a story
			return backlog.CloseStories(root, args[2:], func() bool {
				return gate.Run(nil, root, io.Discard) == 0
			}, suiteCheck(root), out)
		case "epic":
			if len(args) != 3 {
				fmt.Fprint(os.Stderr, usage)
				return 2
			}
			return backlog.CloseEpic(root, args[2], out)
		case "milestone":
			if len(args) != 3 {
				fmt.Fprint(os.Stderr, usage)
				return 2
			}
			return backlog.CloseMilestone(root, args[2], out)
		}
		fmt.Fprint(os.Stderr, usage)
		return 2
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

// sprintCmd dispatches the sprint lifecycle over the backlog.
func sprintCmd(args []string) int {
	root := doctor.Root()
	out := func(s string) { fmt.Println(s) }
	switch args[0] {
	case "open":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return backlog.SprintOpen(root, strings.Join(args[1:], " "), out)
	case "pull":
		if len(args) < 2 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return backlog.SprintPull(root, args[1:], out)
	case "carry":
		if len(args) < 3 {
			fmt.Fprint(os.Stderr, usage)
			return 2
		}
		return backlog.SprintCarry(root, args[1], strings.Join(args[2:], " "), out)
	case "status":
		return backlog.SprintStatus(root, out)
	case "close":
		return backlog.SprintClose(root, out)
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

// suiteCheck returns the test-suite verdict closure when the repo opted
// into `[test] policy = "block"`, nil otherwise — closes then behave
// exactly as before the policy existed.
func suiteCheck(root string) func() (bool, string) {
	if !config.Load(root).TestBlock {
		return nil
	}
	return testrun.Suite(root)
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
		return todo.CloseWith(root, args[1], func() bool {
			return gate.Run(nil, root, io.Discard) == 0
		}, suiteCheck(root), out)
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
