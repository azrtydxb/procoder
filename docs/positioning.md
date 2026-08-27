# Where procoder sits

Procoder is often compared against things it is not, so this page states
the layer it occupies and what it is not trying to be. It is descriptive,
not competitive: the tools named here are good at what they do.

## Four layers, not one

A useful split, and one the ecosystem broadly agrees on:

| Layer          | What it is                               | Example                  |
| -------------- | ---------------------------------------- | ------------------------ |
| **Connection** | How an agent reaches a system at all     | MCP servers              |
| **Actions**    | Individual capabilities an agent invokes | Tools, function calls    |
| **Behaviour**  | What an agent is told to do, in prose    | Agent Skills, rule files |
| **Governance** | What is computed and refused regardless  | Procoder's binary        |

Procoder spans the bottom two rows, and the distinction between them is the
whole point.

## Advice versus computation

A skill pack, a rules file, and an `AGENTS.md` are all **advice**. They
change what an agent is likely to do. They cannot change what happens when
the agent does something else, because nothing reads them at the moment of
the commit.

Procoder ships both halves deliberately:

- **The advisory half** — `AGENTS.md`, `skills/procoder/SKILL.md`, and
  eleven generated host rule files. This is ordinary prose guidance, and it
  works exactly as well as prose guidance ever works.
- **The computed half** — a binary that reads the actual diff, resolves
  real tools, and exits non-zero. `procoder check` does not ask the agent
  whether the files are formatted; it formats them in memory and compares.

The second half is why procoder is a binary rather than a prompt. An agent
under pressure talks itself past a paragraph. It does not talk itself past
a non-zero exit code, because that conversation is not available.

## What procoder does not do

- **It does not write your code**, and outside its own plugin cache it does
  not modify files at all. The binary prints; the agent writes. That is a
  hard rule with one recorded exception (`procoder prune --apply`, which
  touches only procoder's own cache).
- **It does not replace a methodology.** Procoder governs whether work is
  finished and honest. It has no opinion about how you decompose a problem,
  and it defers to BMad's artifacts when `[planning] method = "bmad"` is
  set. See [Influences](influences.md).
- **It does not replace your linters.** Each domain shells out to the
  field's canonical tool — gitleaks, semgrep, osv-scanner, ruff, prettier —
  rather than reimplementing them. Procoder decides what blocks; the tools
  decide what is true.
- **It does not measure its own value.** See
  [Honest limits](honest-limits.md).

## Compared to a skills pack

The closest comparison is a well-built Agent Skills collection. The
difference is not quality; it is enforceability. A skills pack **suggests a
workflow**; procoder **computes a verdict**. A pack can tell an agent to
run the tests. Procoder can refuse to close the task when they did not run,
because `NOT run` is a state it can observe and `I ran them` is a claim it
cannot.

Procoder conforms to the Agent Skills spec and ships as a plugin, so the
two are not alternatives — the skill is procoder's advisory half, and it is
generated from the same source as every other host's rule file so they
cannot drift.

For the specific projects solving adjacent problems, see
[Comparable projects](comparable-projects.md). For the research behind the
premises, see [Research](research.md).
