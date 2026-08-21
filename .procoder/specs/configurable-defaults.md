# configurable-defaults

Status: COMPLETE

## Problem

Procoder decides a great deal on a repository's behalf and lets it argue
about very little. An audit of the tree found four override mechanisms
already in place and a long tail of decisions reachable by none of them.

Overridable today: `.procoder/config.toml` carries sixteen typed settings,
`.procoder/PRINCIPLES.md` replaces the response style wholesale,
`.procoder/{docs,security}/RULES.md` carry prose with machine-readable
list sections that replace defaults, and `.procoder/github/` holds five
templates. The RULES.md pattern is the good one and it exists in three of
roughly forty packages.

Hardcoded: which tool answers for a language, because `tools.ByExtension`
fixes one formatter per extension and the lint dispatch is a switch. Eight
embedded rule sets — golangci's curated list, phpstan's level, the eslint
baseline, clang-tidy's check families, clang-format's fallback style,
checkstyle's bundled config. Security severities, as literals: SAST blocks
at ERROR and nothing else. Nine of twelve templates, which is the entire
quality chain — spec, plan, ADR, todo, milestone, epic, story, sprint,
bug. Nineteen timeouts. And the changelog layout, which lives in a comment
and one test.

The cost is not theoretical. A team that has chosen pint, or biome, or
wants WARNING-severity SAST findings to block, cannot express any of it,
and their only escape is to stop using the domain.

## Users

- **A team with existing tooling** wants Procoder to drive the tools they
  already chose rather than a second set beside them.
- **A team with a stricter bar than the default** wants to raise it and
  have the gate enforce the higher one.
- **A team with a looser bar** wants to lower it deliberately and be able
  to see, later, that they did.
- **A maintainer reading someone else's repo** needs to know at a glance
  which of Procoder's defaults are still in force.

## In scope

- A `[tools]` table selecting among tools Procoder ships definitions for.
- Security severities, thresholds and timeouts as typed config.
- The `RULES.md` pattern extended to the domains that hold rule sets.
- `.procoder/templates/` for the nine templates that have no override,
  and a changelog template.
- Every setting that WEAKENS a default reported in the gate output.
- `procoder config` printing the effective configuration and its source.

## Out of scope

- Arbitrary user-defined tools: a repo cannot name a binary and an argv.
  Procoder owns the invocation so the print-don't-write contract stays a
  guarantee rather than a hope (D-1).
- Turning a domain off entirely. Policies already govern whether findings
  block; silence is not a setting.
- Per-file or per-directory configuration. One file at the repository
  root, as today.
- Changing any default. This is about who may override them.

## Constraints

- **D-OVERRIDE.** The repository's file wins, always.
- **No silent green.** A weakened check must not look like a strict one
  that passed — the rule the gate already lives by.
- Every existing `.procoder/` file keeps working untouched: a repository
  that upgrades and changes nothing behaves exactly as it did.
- No new dependency, and no new top-level command beyond `config`.
- A missing or malformed setting is reported and the default used; a
  config file that cannot be parsed is NOT a reason to check nothing.

## Interfaces

- `.procoder/config.toml` gains `[tools]`, `[security]`, `[timeouts]` and
  per-domain baseline keys.
- `.procoder/templates/<name>.md` overrides an embedded template. For
  anything missing, the binary prints the default — the behaviour
  `procoder templates` already has for `.procoder/github/`.
- `.procoder/<domain>/RULES.md` gains list sections for rule sets.
- `procoder config` prints every effective setting, its value, and where
  it came from — default, config.toml, or a rules file.
- No change to any existing command's arguments.

## Data

- No new state directory. Everything lives under `.procoder/`, which is
  already the contract.
- Templates are read at use time, not cached.

## Edge cases

- A `[tools]` entry naming a tool Procoder does not ship: reported by name
  as unknown, with the list of what it does ship, and the default used.
- A `[tools]` entry naming a tool that is not installed: the existing
  NOT-checked path, which already names the install command.
- A template file that exists but is empty: the empty-documentation guard
  already blocks this, and it must keep doing so rather than silently
  falling back to the default.
- A severity set to a value that is not a severity: named, default used.
- A repository with no config.toml at all: every default in force, and
  `procoder config` says so.
- Two settings that contradict each other — a strengthened threshold and a
  weakened policy — are both honoured; they are not in conflict.

## Failure modes

- A weakened setting that nobody can see turns a green gate into a claim
  about the config rather than the code. Every weakening prints.
- A malformed config that silently reverts to defaults would let a team
  believe a setting is in force when it is not: unparseable settings are
  named individually, not swallowed.
- A tool override that Procoder cannot invoke correctly would corrupt
  files. Only tools with shipped definitions can be selected.
- Extending RULES.md carelessly would let a missing section mean "no
  rules" rather than "defaults" — absent must keep meaning default.

## Acceptance criteria

- [ ] A repository setting `[tools] js = "biome"` is formatted by biome,
      and `procoder doctor` lists biome rather than prettier.
- [ ] A `[tools]` entry naming a tool Procoder does not ship is reported
      by name, lists what is available, and the default is used — the file
      is still checked.
- [ ] `[security] sast_blocks_at = "WARNING"` makes a WARNING finding
      block, where the default blocks only at ERROR.
- [ ] Setting a severity Procoder does not recognise names it and uses the
      default, and the run still reports findings.
- [ ] A repository lowering any default gets a line in the gate output
      naming the setting, its value, and the default it replaced.
- [ ] Raising a default prints no such line — strengthening is not a
      warning.
- [ ] `.procoder/templates/story.md` replaces the embedded story template
      in `backlog story` output, and a repository without the file gets
      the embedded one unchanged.
- [ ] An empty template file blocks rather than falling back to the
      default, asserted against the existing empty-documentation guard.
- [ ] A `## checks` list in a domain's RULES.md replaces that domain's
      baseline check set; a RULES.md with no such section keeps the
      default.
- [ ] `procoder config` prints every effective setting with its source,
      and a repository with no config.toml shows every source as default.
- [ ] A config.toml that cannot be parsed reports the failure, blocks, and
      does not silently run on defaults.
- [ ] A repository upgrading with no config changes produces byte-identical
      gate output to the previous version, asserted on a fixture.

## Open questions

<!-- none — decisions below -->

## Decisions

- **D-1: only tools Procoder ships definitions for can be selected.** A
  repository names `pint` or `biome`; it does not name a binary and an
  argv. Procoder owns the invocation, which is what keeps the
  print-don't-write contract a guarantee — a user-supplied args line that
  writes in place would corrupt files while looking configured. Adding a
  tool is a change to Procoder, and a cheap one.
- **D-2: weakening is allowed and visible.** Any setting that lowers a
  default prints a line in the gate output naming what was relaxed.
  Strengthening prints nothing. This follows the rule the gate already
  lives by: a green verdict must not be able to mean "the config was
  loosened" without saying so.
- **D-3: route each kind to the mechanism that already fits, and add no
  fifth.** Typed knobs to config.toml, rule sets to the domain's RULES.md,
  templates to a directory of files, response style to PRINCIPLES.md.
  Three of these already exist and work; the audit found no kind of
  setting that needs a mechanism they do not cover.
- **D-4: absent keeps meaning default, everywhere.** A missing config key,
  a missing RULES.md section and a missing template file all mean "use the
  default". Only an EMPTY file is an error, because that is a file
  somebody destroyed rather than never wrote.
- **D-6 (corrects D-1's example): the menu holds only tools that can
  PRINT.** D-1 said a repository may select any tool Procoder ships a
  definition for, and used `php = "pint"` as the example. Building it
  showed the example was impossible: pint writes in place, as do phpcbf
  and php-cs-fixer, so none of them can be offered without breaking the
  contract D-1 exists to protect. The rule is therefore narrower than it
  first read — a tool reaches the menu by being able to emit the formatted
  source on stdout, which was tested for each candidate rather than
  assumed. biome does it through `--stdin-file-path`, which is why it is
  the first alternative shipped.
- **D-5: `procoder config` exists because configurability without
  visibility is worse than none.** A reader of an unfamiliar repository
  must be able to ask which defaults are still in force and get an answer
  naming the source of each.
