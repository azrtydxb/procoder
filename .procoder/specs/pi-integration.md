# pi-integration

Status: complete

## Problem

pi is listed as a supported host and is not one. `package.json` declares a pi
package and points it at `pi-extension/index.mjs`, which does one thing: append
AGENTS.md to the system prompt on every turn. pi already loads AGENTS.md as a
context file, so the contract arrives twice — measured in a live pi session at
17,385 characters without the adapter and 29,845 with it, the 12,458-byte
difference being one whole copy of the file, per turn, for the length of the
session. The declared skill path makes things worse rather than better:
`pi.skills: ["./commands"]` yields one skill named "commands" because pi falls
back to the parent directory name when a skill has no name field, so all 34
command files collide and 33 of them are invisible. Of the 34, 33 invoke
"${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh", and pi exports no plugin-root
variable at all, so the line expands to /hooks/launcher.sh and fails. The net
effect for a coder working in pi inside a repository procoder governs is no
commit gate, no post-write findings, no handoff note, and no commands — while
`docs/portability.md` tells the next reader that pi is covered.

## Users

- A coder who works in pi. They need the same four enforcement points every
  other host gets, and they need the price of them paid by the binary rather
  than by their patience.
- procoder maintainers. One command source, one gate implementation, and a host
  table where each row means what the neighbouring row means.
- Adopters installing procoder for pi. One install per machine, nothing
  committed into their repository, and no dependence on what happens to be on
  their PATH.

## In scope

- [S-1] Stop the duplicate contract: the adapter injects AGENTS.md only when pi
  has not already loaded it, judged from the loaded context files rather than
  from a guess about the host.
- [S-2] Post-write enforcement on write and edit results, fed by the same
  `hook post-tool-use` entry point Claude Code calls, so each finding comes
  from the place it already comes from: `format.Check`, `lint.Files`,
  `security.SecretsChangedFiles`, `docs.Drift`, `actions.Lint`,
  `codeindex.Refresh`, `ask.Pending`.
- [S-3] The commit gate at the shell boundary: a commit inside a bash call is
  judged by `hook pre-tool-use` before it runs, and blocked with the gate's own
  reason.
- [S-4] The handoff note and the unasked-decision block, run per turn rather
  than only at session end, and before a compaction.
- [S-5] All 34 commands registered at load as slash commands under the
  procoder namespace, with their real descriptions, argument completion, and
  the launcher path resolved from the adapter's own module location.
- [S-6] The reporting half of procoder exposed as one callable tool, so the
  agent can run the gate without a shell, with output truncated to pi's limits.
- [S-7] The manifest corrected: the command directory leaves every skill path,
  skills/ becomes the skill path, and the package carries the pi-package
  keyword.
- [S-8] The command-text transform that turns a Claude command into a host
  command moves out of `internal/portability/portability_test.go` into a
  function the parity test and the adapter both call.
- [S-9] Documentation and drift coverage: the pi row in `docs/portability.md`
  states the surfaces wired and names the places where pi does more than
  Claude Code, and the portability tests pin the new wiring.
- [S-10] pi becomes an explicit answer from `host.Detect` rather than an
  accident of the Claude default, with the Claude envelope kept and a test
  case added.

## Out of scope

- The PATH version skew this work surfaced — the OpenCode and Kilo twins shell
  out to a bare procoder, which on the machine this spec was written on
  resolves to 1.0.2 while the package is 3.4.0. Real defect, separate fix;
  routing it through this spec would hide it inside a feature.
- Registering any pi slash command, or generating a per-host copy of the
  command markdown. Both alternatives were put to a human and declined.
- Changing what Claude Code runs: `commands/*.md` keeps its plugin-root launcher
  path.
- update.md under pi. It drives the Claude plugin self-update flow; a pinned pi
  install moves with a new install ref, not with a slash command.
- Mutating operations as callable tools. Closing a story, seeding a backlog and
  preparing a release stay human-invoked slash commands.
- Custom footers, themes, widgets and editor replacements. pi can have them;
  this spec does not.

## Constraints

- The adapter stays dependency-free: it imports node built-ins only. pi runs an
  install step for packages, and pulling `@earendil-works/pi-coding-agent` into
  the import list would make that step load-bearing for the first time.
- The adapter stays ES module source with a default export. `TestEveryHostAdapterIsESMSource`
  reads the file as text and rejects CommonJS, because a CommonJS adapter loads
  fine locally through Node's interop and is still refused by pi's validator at
  install time — which is how issue #105 shipped.
- Every verdict comes from the binary. The adapter wires events to processes and
  formats what comes back; it decides nothing that `hook pre-tool-use` and
  `hook post-tool-use` do not already decide for Claude Code.
- A hook that cannot run must not break the session, the same split the launcher
  already implements: silence for a hook, refusal for a command.
- The launcher is resolved from the adapter's own directory, never from PATH, so
  the gate that judges a commit is the gate that shipped with the adapter.
- The unasked-decision block must stay idempotent per message. pi can start a
  new turn from a follow-up message, and a block the agent cannot satisfy loops
  the session — which is worse than the failure it prevents.
- No UI await on any enforcement path: pi runs headless in print and JSON modes,
  where dialogs are unavailable.
- The behaviour tested against is pi 0.84.4.

## Interfaces

- pi events wired: before_agent_start, tool_call, tool_result, agent_settled,
  session_before_compact, session_shutdown.
- Slash commands: `/procoder:<name>` for every file in commands/ except
  update.md, which is 33 today. Descriptions come from each file's frontmatter;
  argument completion is offered for the subcommand words each command names.
- One tool, named procoder, with a subcommand parameter restricted to reporting
  commands — `procoder check`, `procoder test`, `procoder lint`,
  `procoder security`, `procoder debt`, `procoder status`, `procoder review`,
  `procoder doctor`, `procoder index`. Anything that mutates state, closes
  work, or tags a release is refused with the slash command to use instead.
- `internal/hook` payloads: unchanged. The adapter synthesises the
  tool_name/tool_input shape the OpenCode adapter already synthesises.
- host.Pi becomes a named host returning the raw-stdout envelope, identical to
  `host.Claude`.
- package.json: pi.skills points at ./skills, pi.prompts is absent, and
  keywords carries pi-package. The `pi` key itself keeps its name; it is what
  pi documents.

## Data

No new store. Two files already owned by procoder under `.procoder/state/` stay
the only mutable state, and both keep their current owners:

- handoff.md — the facts block rewritten by the stop path, the Notes section
  left untouched. pi calls the same code path, so the marker contract is
  unchanged and a session that alternates between pi and Claude Code overwrites
  nothing it does not already overwrite.
- last-unasked-decision — the dedupe record, now also consulted per turn.

Tool results carry the findings text; nothing about the pi session is persisted
beyond what pi itself persists.

## Edge cases

- A repository with no AGENTS.md, or pi started with context files disabled:
  the adapter supplies the contract rather than staying silent about it.
- A resumed, reloaded or forked session must not receive the principles text
  twice — the same reason `host.SessionSource` exists for #175, where a day of
  resumed sessions cost 187k tokens of repeated injection.
- pi runs write and edit calls in parallel by default: two results patched in
  the same turn must not interleave into each other's text.
- A file written outside the repository, or deleted between the write and the
  check.
- A project pi has not been granted trust for: a global install still loads,
  and nothing in this spec depends on project trust.
- Another extension owning /procoder:check or a colliding name: pi keeps both
  and assigns invocation suffixes, and the adapter must not assume the name it
  registered is the name the user types.
- Windows, where hooks/launcher.cmd answers instead of the shell script, and a
  path containing spaces.
- An aborted turn: the gate is judged for 120 seconds and the user can press
  escape inside that window.
- A repository procoder has never governed: every hook stays silent.

## Failure modes

- No binary and no network: the session starts, every write and commit
  proceeds, and one notification says the gate did not run for this action. A
  hook that cannot judge is not a hook that passes.
- The binary answers with something unparseable: no verdict, the commit is
  allowed, and the reason says the gate printed no verdict. This is the
  OpenCode adapter's existing "unavailable" decision and pi copies it.
- Tool output past pi's 2,000-line or 50 KB guidance is truncated with the
  head kept, the full text written to a temp file, and that path named in the
  result — the LLM is never left with a partial list it believes is complete.
- A missing or unreadable AGENTS.md degrades to no injection, never to a crash.
- The handoff note cannot be written: the note is lost, the turn still ends.
- Compaction that fails or is cancelled still leaves the handoff it wrote,
  because the note is written before the block is decided.

## Decisions

Three forks in this design were not theirs to take; each went to
`.procoder/ask/decisions.md` and came back answered.

- **The command set registers at load; it is never copied.** Reading
  commands/ from the adapter, transform applied in memory, launcher resolved
  from the module location. A generated twin set was rejected as 33 more files
  and a second thing to keep in step, and rewording the canonical markdown was
  rejected because it changes what Claude Code runs.
- **Global install, user scope, pinned ref.** Nothing is committed into a
  governed repository, and the adapter never resolves the binary through PATH.
  The pin moves only with a new install ref, which is what makes the version
  skew in Out of scope impossible on this host rather than merely unlikely.
- **Advantage where pi offers it, parity otherwise.** `tool_result` carries the
  post-write findings and a per-turn boundary runs the handoff, with the host
  table saying so instead of leaving the rows looking equivalent.

## Acceptance criteria

- [x] [S-1] `TestPiAdapterInjectsContractOnce` measures the system prompt with
      and without the adapter: the lengths differ by less than 1 KB, and a marker
      string unique to AGENTS.md occurs exactly once. Breaks if the injection is
      made unconditional again.
- [x] [S-1] The same `TestPiAdapterInjectsContractOnce` runs with context files
      disabled and finds the marker once rather than zero times. Fails if the guard
      keys on the host name instead of on the loaded context files.
- [x] [S-2] `TestPiWriteResultCarriesFormatVerdict`: a write that leaves a file
      unformatted returns the formatted text inside that write's own tool result,
      and `procoder check` over the file passes immediately after. Fails if the
      verdict arrives as a separate injected message.
- [ ] [S-2] `TestPiWriteResultCarriesSecretFinding`: a secret written to a file
      is named in that write's result, at `security.SecretsChangedFiles`' line, before
      any commit is attempted.
      NOT built as a pi test. The adapter forwards whatever `additionalContext`
      the binary returns and asserts nothing about its kind — the format case
      above holds that path. The secret line itself is `internal/hook`'s claim
      and is tested there; a second copy of that assertion on this side of the
      launcher would test the transport twice and the rule once.
- [x] [S-3] `TestPiCommitGateBlocksThenAllows`: a bash commit is blocked with
      the gate's own reason while `procoder check` reports a blocking finding, and
      is allowed once that finding is fixed.
- [x] [S-3] `TestPiGateSpawnsOnlyOnCommits` counts child processes across
      ordinary shell calls including git log --abbrev-commit and asserts zero.
      Regresses if the pre-filter in the adapter is dropped.
- [ ] [S-4] `TestPiTurnWritesHandoffKeepingNotes` inspects `handoff.md`: the
      facts block is regenerated after a turn and the Notes text below the closing
      marker survives the next turn byte for byte. Fails if the marker contract is
      reimplemented in the adapter instead of reused.
      NOT built as a pi test. The adapter writes no handoff at all — it spawns
      `hook stop` and asserts only that the last assistant message reached it;
      the facts/Notes contract is `internal/hook`'s and is tested over a real
      repository there. Proven live as well: a nested pi session left
      `.procoder/state/handoff.md` carrying facts read from its own git state.
- [x] [S-4] `TestPiUnaskedDecisionBlocksOnce`: a final message that defers a
      decision produces exactly one follow-up. Fails if the dedupe lives in adapter
      memory rather than in the `last-unasked-decision` record.
- [x] [S-5] `TestPiCommandRegistryMatchesCommandDir` asserts one registered
      command per file in commands/ except update.md, each carrying that file's
      frontmatter description. Fails if a command file is added and not registered,
      or a removed one still is.
- [x] [S-5] `TestPiLauncherResolvesFromModuleLocation` asserts the launcher path
      starts at the installed package directory, so the 1.0.2 binary on PATH is
      never reached. Fails if PATH is consulted first.
- [x] [S-5] `TestPiLauncherPathPerPlatform` asserts the win32 branch ends in
      launcher.cmd.
- [x] [S-6] `TestPiToolTruncatesWithTempFile`: output past 2,000 lines or 50 KB
      comes back truncated with the full text's path named in the result. Breaks if
      raw stdout is forwarded to the model.
- [x] [S-6] `TestPiToolRefusesMutatingSubcommand`: closing work or tagging a
      release through the tool is refused, naming the slash command instead. Fails
      if the allowlist becomes a denylist.
- [x] [S-7] `TestPiSkillPathIsSkillsNotCommands` loads the package manifest and
      finds `skills/procoder/SKILL.md` as the skill named procoder, with nothing
      from commands/ in any skill path.
- [x] [S-8] `TestOpenCodeCommandParity` and the adapter both apply the one
      shared transform, and `TestEveryHostAdapterIsESMSource` still names the pi
      file. Fails if the rule is duplicated rather than shared.
- [x] [S-9] `TestPiHostRowStatesItsSurfaces` reads `docs/portability.md` and
      requires the pi row to name all four hook surfaces and the places pi exceeds
      Claude Code.
- [x] [S-2] [S-3] `TestPiHooksDegradeToAWarning`: with no binary and no network
      a session starts, writes and commits proceed, and exactly one notice says the
      gate did not run. Fails if a gate that could not run is reported as a pass.
- [x] [S-10] `TestHostDetection` with PI_CODING_AGENT set returns the pi host
      while the gate output keeps the raw-stdout envelope.

## Open questions

<!-- None. The three forks this design turned on are answered in Decisions
     above and recorded in .procoder/ask/answers.md. Any line written here as
     a question holds this spec not ready, so "nothing is open" has to be
     punctuation rather than prose. -->
