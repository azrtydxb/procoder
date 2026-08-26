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
	"strconv"
	"strings"
	"time"

	"procoder/internal/actions"
	"procoder/internal/adr"
	"procoder/internal/analysis"
	"procoder/internal/ask"
	"procoder/internal/audit"
	"procoder/internal/backlog"
	"procoder/internal/bench"
	"procoder/internal/ciops"
	"procoder/internal/codeindex"
	"procoder/internal/config"
	"procoder/internal/copilot"
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
	"procoder/internal/plugincache"
	"procoder/internal/portability"
	"procoder/internal/principles"
	"procoder/internal/release"
	"procoder/internal/releases"
	"procoder/internal/review"
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
		// TypeScript is linted against procoder's baseline when the project
		// configured nothing, and that baseline needs the parser eslint core
		// does not carry. Required wherever TypeScript is, so the refusal
		// this produces when it is absent has a remedy init can run.
		if exts[".ts"] || exts[".tsx"] || exts[".mts"] || exts[".cts"] {
			out = append(out, lint.Eslint, lint.TypescriptEslint)
		}
		// C and C++ were formatted and linted by nothing at all.
		if exts[".c"] || exts[".h"] || exts[".cpp"] || exts[".cc"] || exts[".cxx"] || exts[".hpp"] {
			out = append(out, lint.ClangTidy)
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
		// PHP names the linters the PROJECT configured, and php itself as
		// the floor. Listing phpstan on a repository that chose phpcs would
		// tell a reader to install a tool procoder is never going to run
		// there — doctor's job is what this repository needs, not what PHP
		// has available.
		// phpstan is procoder's default PHP linter, so it is required
		// wherever PHP is — not only where the project already configured
		// it. Listing it conditionally was the bug: a repository with no
		// PHP tooling at all was told nothing was missing, `procoder init`
		// installed nothing, and the lint domain fell back to a syntax
		// check forever. phpcs stays conditional because it is the
		// project's choice, not procoder's default.
		if exts[".php"] {
			out = append(out, lint.PHP, lint.Phpstan)
			if lint.HasPhpcsConfig(root) {
				out = append(out, lint.Phpcs)
			}
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

  ask [--file <path>]  the questions no domain can answer for itself —
                       unresolved spec questions, documentation gaps,
                       flagged secrets, lint findings that need judgement.
                       Asks a person when there is one, writes them to
                       .procoder/ask/QA.md when there is not. --file
                       records the answers a human wrote back
  agents               the universal agent layer: per-host rule files
                       derived from AGENTS.md (Cursor, Windsurf, Cline,
                       Kilo Code, Roo, Kiro, Antigravity, Qoder, Copilot,
                       Codex) — prints content for anything missing or
                       drifted; the gate blocks on drift
  analyze <sub>        the question before the spec, under .procoder/analysis/:
                       brief <name> | list | where | check [name|all] — what
                       is known, what is not, the options and a
                       recommendation; "where" right-sizes a piece of work
                       to the stage it actually needs
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
  config               every effective setting, its value, and where it came
                       from — a config.toml line or procoder's default —
                       with any policy relaxed below the default marked
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
  copilot-leak [--since <dur>] [--quiet] [--from-copilot]
                       what Copilot's auto-review found that our gates did
                       not: sanitised of every trace of your code, then —
                       only if you say yes — filed as issues and recorded in
                       .procoder/github/COPILOT-LEAKS.md as unlearned; asks
                       nothing when there is no terminal to ask.
                       --from-copilot reads that ledger back instead, and
                       exits 1 while any finding is still unclassified
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
  review [--lens <name>[,<name>]] [--perspectives] [paths...]
                       the judgment half of the gate: prints the review
                       lenses and the scope for the agent to judge against,
                       and writes nothing. --perspectives reads the change
                       as analyst, architect, implementer and reviewer in
                       turn; a lens that could not be loaded is a refusal,
                       never a review that silently happened
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
                       blocks while sections are missing, questions
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
  version [--check]    print the version; --check asks GitHub what the
                       newest release is and offers to install it when this
                       build is behind. Silenced by [version] check = "off"
  self-upgrade [--force]
                       install the newest release over this binary, after
                       an explicit yes. Refuses to move backwards, and
                       steps aside from a package manager's install unless
                       --force says the file is really yours
  prune [--apply]      the cached plugin versions that can go. Reports and
                       removes nothing; --apply removes them. Keeps the
                       active version and one previous, protects the
                       version in use twice, and refuses rather than
                       guesses when the install record cannot be read
`

func main() {
	// The principles hook reports what is newer than this build; the version
	// is stamped here at link time, so this is the only place that knows it.
	principles.Version = version
	releases.Running = version

	os.Exit(run(os.Args[1:]))
}

// printLine is the reporting sink for a command that writes to stdout —
// the domains all take an `out func(string)`, and this is the one every
// caller here was spelling out inline.
func printLine(s string) { fmt.Println(s) }

// printFindings renders a findings list the way every reporting command
// renders one — the mark, the location relative to the repository, the
// message, then the count line — and answers the exit code: blocking
// findings fail, information does not. Four commands each had their own
// copy of this loop, which is how three of them drift while the fourth
// gets fixed.
func printFindings(root, label string, findings []gitx.Finding, out func(string)) int {
	blocking := 0
	for _, f := range findings {
		mark := "  info "
		if f.Blocking {
			mark = "  BLOCK"
			blocking++
		}
		loc := f.File
		// a path outside the repository is printed as given: relativising it
		// produces ../../.. noise that helps nobody locate anything
		if rel, err := filepath.Rel(root, loc); err == nil && !strings.HasPrefix(rel, "..") {
			loc = rel
		}
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", loc, f.Line)
		}
		if loc != "" {
			loc += "  "
		}
		out(fmt.Sprintf("%s %s%s", mark, loc, f.Message))
	}
	out(fmt.Sprintf("procoder %s: %d finding(s) (%d blocking)", label, len(findings), blocking))
	if blocking > 0 {
		return 1
	}
	return 0
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	currentCmd = args[0]
	// A flag the command does not implement is a usage error (exit 2 per
	// ADR 0003), never a path and never silence.
	args, ok := checkFlags(args, os.Stderr)
	if !ok {
		return 2
	}
	switch args[0] {
	case "hook":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		switch args[1] {
		case "post-tool-use":
			return hook.Run(os.Stdin, os.Stdout)
		case "pre-tool-use":
			return hook.PreToolUse(os.Stdin, os.Stdout)
		case "install-git":
			return hook.InstallGit(doctor.Root(), printLine)
		case "stop":
			return hook.Stop(os.Stdin, doctor.Root())
		default:
			return usageErr(os.Stderr)
		}
	case "format":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return formatCmd(args[1:])
	case "review":
		return reviewCmd(args[1:])
	case "analyze":
		return analyzeCmd(args[1:])
	case "audit":
		return audit.Run(doctor.Root(), printLine)
	case "status":
		return status.Run(doctor.Root(), printLine)
	case "env":
		sync := len(args) > 1 && args[1] == "--sync"
		if len(args) > 1 && !sync {
			return usageErr(os.Stderr)
		}
		return envsync.Run(doctor.Root(), sync, printLine)
	case "run":
		exec := len(args) > 1 && args[1] == "--exec"
		if len(args) > 1 && !exec {
			return usageErr(os.Stderr)
		}
		return runcmd.Run(doctor.Root(), exec, printLine)
	case "adr":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return adrCmd(args[1:])
	case "bench":
		root := doctor.Root()
		save := len(args) > 1 && args[1] == "--save"
		if len(args) > 1 && !save {
			return usageErr(os.Stderr)
		}
		return bench.Run(root, save, config.Load(root).BenchThreshold, printLine)
	case "deps":
		return deps.Run(doctor.Root(), printLine)
	case "release":
		root := doctor.Root()
		version := ""
		if len(args) > 1 {
			version = args[1]
		}
		return release.Run(root, version, func() bool {
			return gate.Run(nil, root, io.Discard) == 0
		}, suiteCheck(root), printLine)
	case "backlog":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return backlogCmd(args[1:])
	case "sprint":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return sprintCmd(args[1:])
	case "check":
		return gate.Run(args[1:], doctor.Root(), os.Stdout)
	case "config":
		return config.Report(doctor.Root(), os.Stdout)
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
			return ciops.Runs(root, printLine)
		}
		return printFindings(root, "ci", ciops.Check(root, config.Load(root).PinActions), printLine)
	case "infra":
		root := doctor.Root()
		if infra.Detect(root).Empty() {
			fmt.Println("procoder infra: no infrastructure files in this repository")
			return 0
		}
		return printFindings(root, "infra", infra.Check(root), printLine)
	case "maintain":
		return maintain.Run(doctor.Root(), printLine)
	case "security":
		root := doctor.Root()
		deep := len(args) > 1 && args[1] == "--deep"
		// Positional paths were read as nothing at all: `procoder security
		// scripts/` scanned the change set and answered about that, so a
		// person who named a directory was told "0 finding(s)" about files
		// nobody had looked at. Worse, the empty-change-set message added
		// earlier in this campaign ends "(pass paths to check them anyway)"
		// — advice this command did not take. `lint` and `check` have
		// always honoured their paths; this is the same.
		// A directory is expanded to its files, exactly as lintCmd does:
		// SecretsChangedFiles drops anything that is not a regular file, so
		// an unexpanded directory scans nothing and reports clean.
		paths := expandDirs(root, positional(args[1:]))
		changed := paths
		if len(changed) == 0 {
			var err error
			changed, err = gitx.ChangedFiles(root)
			if err != nil {
				// "I could not find out what changed" is not "nothing
				// changed". Swallowing this reported a clean-looking
				// verdict outside a git repository, or whenever git could
				// not run — the exact silent green this domain polices.
				fmt.Printf("procoder security: NOT checked — the changed files could not be listed (%v); pass paths to scan them explicitly\n", err)
				return 1
			}
			if len(changed) == 0 && !deep {
				fmt.Println(nothingChanged("security"))
				return 0
			}
		}
		findings := security.SecretsChangedFiles(root, changed)
		if deep {
			findings = append(findings, security.Sast(root)...)
			findings = append(findings, security.Deps(root)...)
		} else {
			// The gate runs the SAST leg over the changed files as well as
			// the secret scanner, and this command has to answer the same
			// question or it is not a preview of the gate — it is a weaker
			// check wearing the same name. A hardcoded AWS key that blocks
			// at commit was reported here as zero findings, because
			// gitleaks does not fire on it and semgrep, which does, was
			// never asked.
			findings = append(findings, security.SastChanged(root, changed)...)
		}
		return printFindings(root, "security", findings, printLine)
	case "lint":
		return lintCmd(args[1:])
	case "docs":
		root := doctor.Root()
		// --ack prints the line that clears the documentation obligation;
		// the agent places it in the commit message, where a reviewer sees
		// the decision instead of a silent skip
		if len(args) > 1 && args[1] == "--ack" {
			if len(args) < 3 || strings.TrimSpace(strings.Join(args[2:], " ")) == "" {
				return usageErr(os.Stderr)
			}
			fmt.Println(docs.AckLine(strings.Join(args[2:], " ")))
			return 0
		}
		external := len(args) > 1 && args[1] == "--external"
		changed, _ := gitx.ChangedFiles(root)
		return docs.RunFor(root, changed, "", external, config.Load(root).DocsBlock, os.Stdout)
	case "index":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return indexCmd(args[1:])
	case "todo":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return todoCmd(args[1:])
	case "spec":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return specCmd(args[1:])
	case "plan":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return planCmd(args[1:])
	case "debt":
		return debt.Run(doctor.Root(), printLine)
	case "lessons":
		return lessons.Run(doctor.Root(), printLine)
	case "copilot-leak":
		return copilotLeakCmd(args[1:])
	case "ask":
		return askCmd(args[1:])
	case "agents":
		return portability.Agents(doctor.Root(), printLine)
	case "principles":
		if len(args) > 1 && args[1] == "--hook" {
			return principles.RunHook(doctor.Root(), printLine)
		}
		return principles.Run(doctor.Root(), printLine)
	case "templates":
		return gitcmd.Templates(doctor.Root(), os.Stdout)
	case "test":
		return testCmd(args[1:])
	case "scrub":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return gitcmd.Scrub(args[1], os.Stdin, os.Stdout)
	case "version":
		// An unrecognised flag is refused rather than ignored: `version
		// --chekc` printing the version and exiting 0 tells the user their
		// check ran when nothing checked anything.
		if len(args) > 2 {
			printLine("version takes at most --check")
			return 2
		}
		if len(args) == 2 {
			if args[1] != "--check" {
				printLine("version: unknown argument " + args[1])
				return 2
			}
			return versionCheckCmd()
		}
		// N-05: bare `version` stays one line and asks nobody anything.
		fmt.Println(version)
		return 0
	case "self-upgrade":
		force := false
		for _, a := range args[1:] {
			if a != "--force" {
				printLine("self-upgrade: unknown argument " + a)
				return 2
			}
			force = true
		}
		return releases.Upgrade(version, force, upgradeConsent, printLine)
	case "prune":
		apply := false
		for _, a := range args[1:] {
			if a != "--apply" {
				printLine("prune: unknown argument " + a)
				return 2
			}
			apply = true
		}
		return pruneCmd(apply, printLine)
	default:
		return usageErr(os.Stderr)
	}
}

// pruneCmd reports which cached plugin versions could go, and removes them
// only when asked twice — once by typing the command, once by --apply.
//
// Somebody typing `procoder prune` to find out what it does must not lose
// a gigabyte discovering the answer. Deletion is the one thing here that
// cannot be undone, so it is the one thing that is not the default (#181).
func pruneCmd(apply bool, out func(string)) int {
	home, err := os.UserHomeDir()
	if err != nil {
		out("prune: your home directory could not be determined (" + err.Error() + ") — refusing to guess where the plugin cache is")
		return 2
	}
	// The running binary's own directory: the second, independent
	// protection on the version in use. An error here is not fatal — it
	// makes this check contribute nothing, and the registry check still
	// stands — so it is deliberately not a refusal.
	running := ""
	if exe, err := os.Executable(); err == nil {
		running = filepath.Dir(exe)
	}

	plan, err := plugincache.Compute(home, running)
	if err != nil {
		out("prune: " + err.Error())
		return 2
	}
	for _, n := range plan.Notes {
		out("  note  " + n)
	}
	if len(plan.Removable) == 0 {
		out("prune: nothing to remove — " + strconv.Itoa(len(plan.Kept)) + " cached version(s), all of them kept")
		return 0
	}
	if !apply {
		for _, v := range plan.Removable {
			out("  would remove  " + v)
		}
		out("prune: " + strconv.Itoa(len(plan.Removable)) + " version(s) can go, reclaiming " + plugincache.Human(plan.Bytes))
		out("  keeping " + strings.Join(plan.Kept, ", ") + " (" + plan.Active + " is active)")
		out("  run `procoder prune --apply` to remove them")
		return 0
	}

	removed, reclaimed, failures := plugincache.Apply(home, plan)
	for _, v := range removed {
		out("  removed  " + v)
	}
	for _, f := range failures {
		out("  " + f)
	}
	// The reclaimed figure is summed from what actually went, never from
	// the plan: a sweep that removed nothing must not report a number.
	out("prune: removed " + strconv.Itoa(len(removed)) + " version(s), reclaimed " + plugincache.Human(reclaimed))
	out("  keeping " + strings.Join(plan.Kept, ", ") + " (" + plan.Active + " is active)")
	if len(failures) > 0 {
		return 1
	}
	return 0
}

// indexCmd dispatches the index subcommands; every query prints through one
// line-writer so output stays uniform.
func adrCmd(args []string) int {
	root := doctor.Root()
	switch args[0] {
	case "new":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return adr.New(root, strings.Join(args[1:], " "), printLine)
	case "list":
		return adr.List(root, printLine)
	case "check":
		return adr.Run(root, printLine)
	default:
		return usageErr(os.Stderr)
	}
}

// nothingChanged is what a change-scoped command says when the change set
// is empty. Printing "0 finding(s)" there is indistinguishable from
// printing it over a clean diff, so a person on an untouched tree was told
// their repository linted clean by a linter that never opened a file.
// `procoder check` has always said "no changed files"; this is the same
// sentence for the commands that forgot it.
// expandDirs replaces each directory argument with the repository files
// under it, and returns every path rooted. Two silences depended on this:
// a check that works file by file gets nothing from an unexpanded
// directory, and SecretsChangedFiles resolves what it is handed against
// the PROCESS's directory while gitx.ChangedFiles hands it absolute paths
// — so a relative path scanned nothing whenever procoder was run from a
// subdirectory, and said "0 finding(s)" about it.
func expandDirs(root string, paths []string) []string {
	var out []string
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, p)
		}
		info, err := os.Stat(abs)
		if err != nil {
			// Not resolvable from the root either; hand it on unchanged so
			// the check that receives it can say so about the name the
			// person actually typed.
			out = append(out, p)
			continue
		}
		if !info.IsDir() {
			out = append(out, abs)
			continue
		}
		for _, f := range gitx.FilesUnder(root, p) {
			if filepath.IsAbs(f) {
				out = append(out, f)
				continue
			}
			out = append(out, filepath.Join(root, f))
		}
	}
	return out
}

// positional drops the flags a command has already read, leaving the paths.
// Flag parsing happens at the front (see checkFlags), so anything that is
// not a leading dash is a path.
func positional(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func nothingChanged(cmd string) string {
	return "procoder " + cmd + ": no changed files — nothing was checked (pass paths to check them anyway)"
}

func lintCmd(args []string) int {
	root := doctor.Root()
	types := false
	var paths []string
	for _, a := range args {
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
		if len(changed) == 0 {
			fmt.Println(nothingChanged("lint"))
			return 0
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
	return printFindings(root, "lint", findings, printLine)
}

func testCmd(args []string) int {
	coverage := false
	name := ""
	var paths []string
	for i := 0; i < len(args); i++ {
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
	return testrun.Report(testrun.Run(doctor.Root(), paths, coverage, name), printLine)
}

// askCmd is `procoder ask`: the questions no domain can answer for itself,
// put to a person. --file is the way an answer gets back in when there was
// no terminal to ask on — the coder relays the questions, writes what the
// human said, and hands the file over.
func askCmd(args []string) int {
	root := doctor.Root()
	if len(args) > 0 {
		switch {
		case args[0] == "--file" && len(args) > 1:
			return ask.FromFile(root, args[1], printLine)
		case args[0] == "--file":
			printLine("ask: --file wants the path of the file holding the answers")
		default:
			printLine("ask: unknown argument " + args[0])
		}
		return 2
	}
	return ask.Run(root, os.Stdin, printLine)
}

// versionCheckCmd is `procoder version --check`: the version, then what
// GitHub says is newest, then the offer. It performs exactly one query and
// makes exactly one decision, both inside releases.Upgrade — asking here and
// deciding there meant two round trips, so a release published between them
// installed a version the user was never shown, and a package-manager
// refusal arrived only after they had already said yes.
func versionCheckCmd() int {
	printLine(version)
	if config.Load(doctor.Root()).VersionCheckOff {
		printLine("version check is off in .procoder/config.toml ([version] check)")
		return 0
	}
	return releases.Upgrade(version, false, announceThenAsk, printLine)
}

// announceThenAsk is the consent function `--check` hands to the upgrade: it
// says what is newer before asking, because a bare prompt does not tell the
// reader what they are agreeing to.
func announceThenAsk(latest string) bool {
	printLine(releases.WarningLine(version, latest))
	return upgradeConsent(latest)
}

// upgradeConsent asks before anything is downloaded or run (N-08). No
// terminal is not a yes: it is a question nobody was asked.
func upgradeConsent(latest string) bool {
	if !copilot.CanAsk(os.Stdin) {
		printLine("no terminal to ask on — an upgrade needs an explicit yes, so nothing was installed")
		return false
	}
	printLine("Upgrade procoder " + version + " → " + latest + "? [y/N]")
	return copilot.ReadYes(os.Stdin)
}

// copilotLeakCmd is the capture path: find, sanitise, ask, record. The exit
// codes follow the spec's own Exit 2 line — declining, or having nobody to
// ask, is a reporting exit rather than an error, and so is a check that could
// not run. Only a real answer exits 0.
func copilotLeakCmd(args []string) int {
	since := 24 * time.Hour
	quiet, report := false, false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--quiet":
			quiet = true
		case args[i] == "--from-copilot":
			report = true
		case args[i] == "--since" && i+1 < len(args):
			d, ok := parseWindow(args[i+1])
			if !ok {
				printLine("copilot-leak: --since wants a duration like 6h, 90m, or 2d — got " + args[i+1])
				return 2
			}
			since, i = d, i+1
		case args[i] == "--since":
			printLine("copilot-leak: --since wants a duration like 6h, 90m, or 2d — none was given")
			return 2
		default:
			printLine("copilot-leak: unknown argument " + args[i])
			return 2
		}
	}
	root := doctor.Root()
	if report {
		// The ledger half of the loop: what was captured, and what nobody has
		// turned into an adaptation yet. It reaches no network and asks
		// nothing — reading back what the capture wrote is a different job
		// from going to look for more.
		return lessons.RunCopilotLeaks(root, printLine)
	}
	finds, ok := copilot.Find(root, since, printLine)
	if !ok {
		return 2 // Find has already said what it could not check
	}
	if len(finds) == 0 {
		printLine("copilot-leak: no findings since " + since.String())
		return 0
	}
	// Sanitising before the count is printed, not after the answer: nothing
	// the user is shown has been through anything less than the filter that
	// protects the code itself.
	var safe []copilot.Sanitised
	for _, f := range finds {
		safe = append(safe, copilot.Sanitise(f, root))
	}
	if quiet {
		printLine(fmt.Sprintf("copilot-leak: %d finding(s) since %s — run without --quiet to capture them", len(safe), since))
		return 0
	}
	if !copilot.Prompt(os.Stdin, printLine, len(safe), since) {
		if copilot.CanAsk(os.Stdin) {
			printLine("copilot-leak: skipped — nothing was published and nothing was recorded")
		} else {
			printLine(fmt.Sprintf("copilot-leak: %d finding(s) since %s, and no terminal to ask on — publishing needs an explicit yes, so nothing was captured", len(safe), since))
		}
		return 2
	}
	issues, entries, notes := copilot.Capture(safe, root)
	for _, n := range notes {
		printLine(n)
	}
	printLine(fmt.Sprintf("copilot-leak: %d issue(s) created, %d entry(ies) recorded in %s — each unlearned until you write its adaptation",
		issues, entries, lessons.CopilotLeaksPath))
	return 0
}

// parseWindow accepts what time.ParseDuration accepts, plus the plain day
// suffix the interface advertises — Go rejects "2d" and a user who typed it
// deserves the window they asked for rather than a lecture.
func parseWindow(v string) (time.Duration, bool) {
	if days, ok := strings.CutSuffix(v, "d"); ok {
		if n, err := strconv.Atoi(days); err == nil && n > 0 {
			return time.Duration(n) * 24 * time.Hour, true
		}
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, false
	}
	return d, true
}

func indexCmd(args []string) int {
	root := doctor.Root()
	out := printLine
	switch args[0] {
	case "build":
		if err := codeindex.Build(root, out); err != nil {
			fmt.Println(err)
			return 1
		}
		return 0
	case "callers":
		if len(args) < 2 {
			return usageErr(os.Stderr)
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
			return usageErr(os.Stderr)
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
			return usageErr(os.Stderr)
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
		return usageErr(os.Stderr)
	}
}

// backlogCmd dispatches the project layer. Creation subcommands parse
// their flag in the loop style of `index rename --at`; closes route to
// the refusing controllers.
func backlogCmd(args []string) int {
	root := doctor.Root()
	out := printLine
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
			return usageErr(os.Stderr)
		}
		return backlog.Milestone(root, strings.Join(args[1:], " "), out)
	case "epic":
		ms, pos := flagVal("--milestone", args[1:])
		if len(pos) == 0 {
			return usageErr(os.Stderr)
		}
		return backlog.Epic(root, strings.Join(pos, " "), ms, out)
	case "story":
		epic, pos := flagVal("--epic", args[1:])
		if len(pos) == 0 {
			return usageErr(os.Stderr)
		}
		return backlog.Story(root, strings.Join(pos, " "), epic, out)
	case "bug":
		epic, pos := flagVal("--epic", args[1:])
		severity, pos := flagVal("--severity", pos)
		if len(pos) == 0 {
			return usageErr(os.Stderr)
		}
		return backlog.Bug(root, strings.Join(pos, " "), epic, severity, out)
	case "seed":
		ms, pos := flagVal("--milestone", args[1:])
		if len(pos) != 1 {
			return usageErr(os.Stderr)
		}
		return backlog.Seed(root, pos[0], ms, out)
	case "list":
		return backlog.List(root, out)
	case "board":
		return backlog.Board(root, out)
	case "close":
		if len(args) < 3 {
			return usageErr(os.Stderr)
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
				return usageErr(os.Stderr)
			}
			return backlog.CloseEpic(root, args[2], out)
		case "milestone":
			if len(args) != 3 {
				return usageErr(os.Stderr)
			}
			return backlog.CloseMilestone(root, args[2], out)
		}
		return usageErr(os.Stderr)
	default:
		return usageErr(os.Stderr)
	}
}

// sprintCmd dispatches the sprint lifecycle over the backlog.
func sprintCmd(args []string) int {
	root := doctor.Root()
	out := printLine
	switch args[0] {
	case "open":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return backlog.SprintOpen(root, strings.Join(args[1:], " "), out)
	case "pull":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return backlog.SprintPull(root, args[1:], out)
	case "carry":
		if len(args) < 3 {
			return usageErr(os.Stderr)
		}
		return backlog.SprintCarry(root, args[1], strings.Join(args[2:], " "), out)
	case "status":
		return backlog.SprintStatus(root, out)
	case "close":
		return backlog.SprintClose(root, out)
	default:
		return usageErr(os.Stderr)
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
	out := printLine
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
			return usageErr(os.Stderr)
		}
		return todo.Add(root, strings.Join(args[1:], " "), out)
	case "show":
		if len(args) < 2 {
			return usageErr(os.Stderr)
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
			return usageErr(os.Stderr)
		}
		return todo.CloseWith(root, args[1], func() bool {
			return gate.Run(nil, root, io.Discard) == 0
		}, suiteCheck(root), out)
	default:
		return usageErr(os.Stderr)
	}
}

// planCmd dispatches the plan quality controller.
func planCmd(args []string) int {
	root := doctor.Root()
	out := printLine
	switch args[0] {
	case "list":
		return plan.List(root, out)
	case "template":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return plan.PrintTemplate(args[1], out)
	case "check":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		return plan.Check(root, name, out)
	default:
		return usageErr(os.Stderr)
	}
}

// specCmd dispatches the spec quality controller.
func specCmd(args []string) int {
	root := doctor.Root()
	out := printLine
	switch args[0] {
	case "list":
		return spec.List(root, out)
	case "template":
		if len(args) < 2 {
			return usageErr(os.Stderr)
		}
		return spec.PrintTemplate(args[1], out)
	case "check":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		return spec.Check(root, name, out)
	default:
		return usageErr(os.Stderr)
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

// reviewCmd is the judgment half of the gate: it prints the lenses and the
// scope, and the agent judges. The binary writes nothing, which is what
// makes it safe to run on anything.
//
// Exit codes follow ADR 0003: 0 when every selected lens resolved, 1 when
// one could not — a lens that failed to load is a refusal, and a review
// that did not happen must not exit 0 — and 2 for a name that is not a
// lens, which is a usage error rather than a finding about the code.
func reviewCmd(args []string) int {
	root := doctor.Root()

	var want []string
	var paths []string
	perspectives := false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--lens" && i+1 < len(args):
			i++
			want = append(want, strings.Split(args[i], ",")...)
		case strings.HasPrefix(args[i], "--lens="):
			want = append(want, strings.Split(strings.TrimPrefix(args[i], "--lens="), ",")...)
		case args[i] == "--perspectives":
			perspectives = true
		case strings.HasPrefix(args[i], "-"):
			fmt.Fprintf(os.Stderr, "procoder review: unknown flag %q\n", args[i])
			return 2
		default:
			paths = append(paths, args[i])
		}
	}

	// Perspectives are who is reading; lenses are how. A spec or a plan
	// wants the first — the architectural question is cheapest to answer
	// before there is a diff to answer it against.
	resolve := review.Resolve
	if perspectives {
		resolve = review.ResolvePerspectives
	}
	lenses, problems := resolve(root)
	// The refusal comes before anything is printed. A lens that could not
	// be read must not be discovered halfway down a review the reader has
	// already started acting on.
	if len(problems) > 0 {
		printFindings(root, "review", problems, printLine)
		return 1
	}

	if len(want) > 0 {
		selected, unknown := review.Select(lenses, want)
		if len(unknown) > 0 {
			// Named for the mode the caller asked for: under
			// --perspectives these are perspectives, and an error saying
			// "no such lens" contradicts the flag they just passed while
			// listing names that are not lenses either.
			kind := "lens"
			if perspectives {
				kind = "perspective"
			}
			fmt.Fprintf(os.Stderr, "procoder review: no such %s: %s — procoder has %s\n",
				kind, strings.Join(unknown, ", "), strings.Join(review.Names(lenses), ", "))
			return 2
		}
		lenses = selected
	}

	if len(paths) == 0 {
		changed, err := gitx.ChangedFiles(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "procoder review: cannot list changed files (%v) — pass paths explicitly\n", err)
			return 2
		}
		paths = changed
	}

	if perspectives {
		review.PrintPerspectives(os.Stdout, paths, lenses)
	} else {
		review.Print(os.Stdout, paths, lenses)
	}
	return 0
}

// analyzeCmd is the phase before the spec: it prints a document for the
// agent to write, lists what exists, and refuses one nobody filled in.
// The binary writes nothing here either.
func analyzeCmd(args []string) int {
	root := doctor.Root()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: procoder analyze brief <name> | where | list | check [name|all]")
		return 2
	}
	switch args[0] {
	case "brief":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: procoder analyze brief <name>")
			return 2
		}
		name := args[1]
		if name != filepath.Base(name) || strings.Contains(name, "..") {
			fmt.Fprintf(os.Stderr, "invalid analysis name %q — names are plain file names\n", name)
			return 2
		}
		fmt.Printf("== write this to %s/%s.md and fill it through the interview:\n",
			analysis.Dir, name)
		fmt.Printf(analysis.Template, name, time.Now().UTC().Format("2006-01-02"))
		return 0
	case "list":
		files := analysis.Files(root)
		if len(files) == 0 {
			printLine("no analysis yet — `procoder analyze brief <name>` starts one")
			return 0
		}
		for _, f := range files {
			printLine(strings.TrimSuffix(filepath.Base(f), ".md"))
		}
		return 0
	case "where":
		return analysis.Where(root, printLine)
	case "check":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		return analysis.Check(root, name, printLine)
	default:
		fmt.Fprintln(os.Stderr, "usage: procoder analyze brief <name> | where | list | check [name|all]")
		return 2
	}
}
