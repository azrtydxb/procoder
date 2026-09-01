# Procoder

You are working in a repository governed by Procoder — a harness that
gives AI coders the tools and discipline of a senior developer. The
`procoder` binary computes; you act. It never modifies code behind your
back, and a file it could not check is never reported as clean.

## The contract

- Before calling any work finished, run `procoder check` — the commit
  gate. Blocking findings (unformatted files, conflict markers, junk,
  secrets, attribution lines) must be fixed, not argued with.
- `procoder format <file>` prints the formatted result; you review and
  write it. The binary never touches the file.
- Never add AI-attribution lines (Co-Authored-By, "generated with") to
  commits or PRs — `procoder scrub` verifies. If the gate blocks one you
  did not write, the host appended it and will append it again next
  commit: turn it off at the source rather than amending forever
  (docs/portability.md, "The trailer your host adds").
- Deliberate corner-cuts carry a `debt:` comment naming the ceiling and
  the revisit condition; `procoder debt` harvests the ledger.
- Specs live in `.procoder/specs/`, plans in `.procoder/plans/`, tasks
  in `.procoder/todo/` — each has a quality controller (`spec check`,
  `plan check`, `todo close`) that blocks until the work is actually
  complete. Do not game the checkboxes; the controllers ask for evidence.
- Run `procoder test` before claiming anything works. NOT run is never
  green. Where `[test] policy = "block"`, the closes refuse on a red or
  unverifiable suite.
- A file an agent session could have written is never executed
  automatically. procoder reads plenty of it — `.procoder/ask/`, the
  handoff note, the backlog, the specs — and hooks run unattended on every
  write and every commit. Display it, and require a separate step a human
  invokes before anything from it runs. `procoder run` is the shape: it
  prints the declared launch commands, executes only under `--exec`, and
  refuses even then when more than one candidate exists rather than
  guessing which you meant.
- A merge conflict is resolved hunk by hunk, by what each side was trying
  to do. `git merge --abort` and `git rebase --abort` are not resolutions —
  they erase the attempt. Being stuck is a thing to say, not a thing to
  undo. Read the resolved file rather than trusting its shape: git splits a
  conflict wherever the texts diverge, including through the middle of a
  function, so "keep both sides" can leave one side without its closing
  lines and still look plausible.
- Before calling a piece of work finished, four passes in order, each a
  different question. Implement what was scoped, with nothing quietly
  deferred. Reread the diff as a reviewer who did not write it. Hunt
  defects deliberately — `procoder review` is that pass, and its
  `adversarial` and `edge-case` lenses are pointed at exactly it. Then the
  cheap polish: a name, a comment, a small robustness gap, and stop there.
  Thoroughness comes from asking four different questions, not from
  asking the same one harder.
- Splitting work does not divide the care. The eleventh story in an epic
  gets the same four passes as the first, and a task decomposed three
  levels deep gets them at every leaf — not a share of them. "I am nine
  stories in, I know this codebase now, I can go faster" is the feeling
  that precedes the bug that took the longest to find. Depth is where
  attention leaks: the work looks familiar, the pieces left look small,
  and each one is still somebody's afternoon spent reading what you wrote.
- A decision that is not yours to make — commit or hold, merge now or
  after, which of two approaches — goes in `.procoder/ask/decisions.md`,
  one `## ` heading per decision with its options beneath, and then you
  ask. `procoder ask` collects it with everything else. Asking without
  recording means the question dies at the next compaction; recording
  without asking means nobody answers it.

## The work chain

Non-trivial work starts above the code, and each link refuses to advance
until its own gap is closed.

- `procoder spec <sub>` — `template <name> | list | check` in
  `.procoder/specs/`. Check blocks while a section is empty, a question
  in Open questions is unanswered (`procoder ask` records answers), or a
  criterion is untestable.
- `procoder plan <sub>` — `template | list | check` in
  `.procoder/plans/`. Check blocks on placeholders and on tasks without
  files or steps. Write the plan for a stranger; never say "same as
  task N".
- `procoder backlog <sub>` — the project layer in `.procoder/backlog/`:
  `milestone | epic | story | bug | seed <spec> | list | board | close`.
  Seed decomposes a COMPLETE spec into an epic and its stories. Story
  closes carry todo rigor; epic and milestone closes refuse while a
  child is open.
- `procoder sprint <sub>` — `open`, `pull`, `carry`, `status`, `close`.
  One active sprint at a time. Close refuses while a committed story is
  neither done nor carried back with a reason, and scaffolds the retro
  the next `open` requires.
- `procoder todo <sub>` — `add | list | show | close`. The standalone
  list for work not born from a spec; `close` refuses without checked
  criteria, recorded evidence, and a clean gate.
- `procoder adr <sub>` — `new <title> | list | check` in
  `.procoder/adr/`. Records are immutable: a changed mind supersedes,
  never rewrites. Check refuses hollow records and dangling supersedes.
- `procoder release [<version>]` — the pre-tag controller: version sync
  across `[release] files`, the changelog entry, a clean tree, the
  gate, and the suite. It prints the `git tag` command; it never tags.

## Parallel work

- Fan out when the work decomposes: independent units — research, separate
  files, separate verification — run in parallel. Subagents where the host
  has them, parallel sessions where it does not. The ceiling is the unit, not
  the headcount: an independent fifteen-minute fix stays serial, and more
  agents than independent work is not parallelism, it is a pileup.
- Writers are fenced by the tree, not by politeness: two agents that will
  touch the same files or the same feature each get their own `git worktree`
  on their own branch — the parent creates the worktree and names each agent
  its path. Read-only, file-disjoint work shares the tree; a worktree is a
  fence, not a tax, so no fence when nothing collides.
- Convergence is the chain, not a shortcut: each writer lands a mergeable
  diff on its own branch — one branch, one writer — and the merges go back
  through the normal way: the gate on every merge, the four passes over the
  merged result, not a share of them. The worktree is scratch; the branch
  is what outlives it. Subagents inherit every rule in this file the way a
  session does: the fence and the gate apply to their work, and their
  "done" carries the same five answers.

## Which command, right now

Read down; take the first row that matches — and run it. The trigger is the
situation, not a request: "mid-change, about to say it is done" means the
gate runs on its own, not when told and not when convenient. The hosts offer
the same commands through different surfaces — slash command, plugin tool,
hook — and a command the host offers is workflow, not a feature to ask about;
the verdict is the same whichever surface carried it, and a blocked gate
outranks a finished-sounding turn.

| Where you are                                | Start here               |
| -------------------------------------------- | ------------------------ |
| A repo procoder has never governed           | `procoder audit`         |
| An idea not yet worth a spec                 | `procoder analyze brief` |
| Non-trivial work, no spec yet                | `procoder spec template` |
| A spec that checks COMPLETE                  | `procoder backlog seed`  |
| Work already committed to, no spec behind it | `procoder todo add`      |
| A story to build, no plan yet                | `procoder plan template` |
| Mid-change, about to say it is done          | `procoder check`         |
| A decision that is not yours                 | `procoder ask`           |
| A durable choice worth keeping               | `procoder adr new`       |
| Ready to tag                                 | `procoder release`       |
| Lost in an unfamiliar codebase               | `procoder index find`    |

## What you talk yourself into

Every row is a thing that has actually been said. The left column is the
sentence; the right is what is true when it is said.

| The thought                                         | What is true                                                                                                                       |
| --------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| "Small fix, I will skip `procoder check` this once" | The gate exists for changes too small to look worth checking. That is precisely when it gets skipped and something ships broken.   |
| "I know what they meant, I will answer this myself" | An answer the user never saw is not a decision, it is a guess wearing one's clothes — and they never learn they were not asked.    |
| "The debt comment is self-explanatory"              | `procoder debt` harvests the ceiling and the revisit condition. A marker without them is unharvestable, which is to say invisible. |
| "Tests are slow, I will run the gate without them"  | NOT run is never green. "I will add tests after" is how untested code ships permanently.                                           |
| "The suite was green before my change"              | Before is not after. The one run that matters is the one over what you are about to commit.                                        |
| "It is only a docs change"                          | Documentation that is wrong is worse than documentation that is missing, because somebody acts on it.                              |
| "I will fix the conflict by keeping my side"        | Both sides were somebody's work. Keeping one silently is how a feature vanishes between two green runs.                            |
| "I am nearly out of context, I will wrap up here"   | Stopping is fine; saying it is finished is not. Say where you stopped.                                                             |

## Before you call it done

Not prose to agree with — five things with an answer.

- [ ] `procoder check` ran clean over this change, this turn
- [ ] `procoder test` ran, and passed, over what is about to be committed
- [ ] every `debt:` marker added names a ceiling AND a revisit condition
- [ ] `.procoder/ask/decisions.md` has no heading the user has not answered
- [ ] every claim made in the report traces to a command that produced it

## Build principles

Climb this ladder and stop at the first rung that holds: does it need to
exist at all → does this codebase already have it → stdlib → platform →
an installed dependency → one line → only then the minimum code that
works. The ladder runs AFTER you understand the problem — read every
file the change touches first. Bug fix = root cause: find every caller
before editing. Never simplify away input validation, error handling
that prevents data loss, security, or accessibility. Non-trivial logic
leaves one runnable check behind. A repo overrides these wholesale with
`.procoder/PRINCIPLES.md` (`procoder principles` prints the effective
text).

## The toolbox

- `procoder doctor` / `procoder init` — which tools this repo needs and
  how to install the gaps.
- `procoder index <sub>` — the code map: find, search, refs, outline,
  callers, impact, unused, entrypoints. Reach for it before grepping.
- `procoder lint [--types]` / `security [--deep]` / `ci` / `infra` /
  `docs [--external]` / `maintain` — the domain reports; blocking beats
  advisory, honesty beats convenience.
- `procoder test [--coverage]` — every detected ecosystem's canonical
  runner. Coverage is reported, never enforced.
- `procoder bench [--save]` — Go benchmarks against the saved baseline
  (`.procoder/bench/baseline.txt`); regressions past `[bench] threshold`
  exit 1. Go only in this version. `--save` is a deliberate decision.
- `procoder deps` — outdated dependencies per ecosystem, licenses where
  a tool exists. Report-only: the judgment stays yours.
- `procoder audit` — the whole-tree onboarding sweep for a repo procoder
  has not governed before.
- `procoder git` and `procoder templates` — pre-finish status and the
  repo's template files under `.procoder/`.
- `procoder ask` — the questions no domain can answer for itself. When
  you are handed one, STOP and put it to the user: an invented answer is
  indistinguishable from a decision. Record theirs with
  `procoder ask --file <path>`.
- `procoder agents` — the per-host rule files derived from this file.
  Regenerate after editing it; drift blocks the gate.
- `procoder lessons` — the ledger of what escaped the gates. A lesson
  with no adaptation is UNLEARNED and exits 1.
- `procoder copilot-leak` — what Copilot's auto-review caught that our
  gates did not: sanitised, filed as issues only if you say yes, and
  recorded as unlearned. `--from-copilot` reads that ledger back.
- `procoder hook post-tool-use` — the write hook's entry point, wired by
  the plugin. You do not call it by hand.
- `procoder version` — the version, when a report needs to name it.
- `procoder version --check` and `procoder self-upgrade` — what is newer
  than this binary, and the install, after an explicit yes. The upgrade
  refuses to move backwards and steps aside from a package manager's
  install. When a session start reports a newer version, say so and ask
  the user rather than upgrading on their behalf.

Install: the binary ships per platform in `dist/` of the procoder repo
(github.com/azrtydxb/procoder); put the one for your platform on PATH,
or use the Claude Code plugin which wires everything automatically.
