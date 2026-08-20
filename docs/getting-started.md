<!-- procoder:allow-conflict-markers this tutorial shows a learner what a conflict looks like -->

# Getting started

**A tutorial**, for Claude Code. By the end you will have Procoder
installed and its tools in place, and you will have watched the commit
gate refuse a change and then accept it. Ten minutes.

Follow it exactly. Explanations come later.

Not using Claude Code? [Install the binary
manually](how-to-install-manually.md) — every other agent reads the same
`AGENTS.md` contract.

## 1. Install

In Claude Code:

```
/plugin marketplace add azrtydxb/procoder
/plugin install procoder
```

Then run `/reload-plugins`.

**That is the whole install.** The plugin carries the binary for your
platform, so there is nothing to clone, compile, or put on `PATH`. The
hooks are live now: every file the agent writes is checked in the same
turn, and every session starts with the engineering principles.

## 2. Let it install the tools this repository needs

Open the repository you want to work in and run:

```
/procoder:init
```

Procoder surveys what **this** repository needs — formatters, linters,
scanners, index builders, chosen by the files you actually have — and
prints one install command per gap. Every command is visible before it
runs.

With nothing missing you will see:

```
procoder init: every formatter this repository needs is installed
```

This step matters more than it looks. A missing tool is never silently
skipped: files it would have checked are reported **unchecked, and
unchecked fails the gate**.

## 3. Give the gate something to catch

Still in that repository, create `NOTES.md` with a merge conflict left
in it and trailing whitespace after `cents.`:

```
# Checkout notes

The checkout total is computed in cents.

<<<<<<< HEAD
Refunds reuse the same path.
=======
Refunds have their own path.
>>>>>>> feature/refunds
```

Stage it:

```
git add NOTES.md
```

## 4. Run the gate

```
/procoder:check
```

```
unformatted  NOTES.md  (run `procoder format "NOTES.md"` for the result)
BLOCKING     NOTES.md:5  merge conflict marker left in the file
BLOCKING     NOTES.md:9  merge conflict marker left in the file
procoder gate: 0 clean, 1 unformatted, 0 unchecked, 0 out of scope, 2 hygiene finding(s) (2 blocking)
```

**BLOCKING** lines fail. **info** lines do not.

Try committing and the gate stops you — this is the same check CI runs,
wired into the commit itself.

## 5. Resolve the conflict

Edit `NOTES.md`, keep one side, and leave the trailing whitespace alone
for now:

```markdown
# Checkout notes

The checkout total is computed in cents.

Refunds reuse the same path.
```

## 6. Ask for the formatted result

```
/procoder:format NOTES.md
```

```
== NOTES.md — formatted result (prettier), review and write it:
# Checkout notes

The checkout total is computed in cents.

Refunds reuse the same path.
```

The trailing whitespace is gone from the printed result — and still
present in your file. **Procoder printed the fix; it did not apply it.**
The agent reviews that content and writes it.

## 7. Run the gate again

```
/procoder:check
```

```
procoder gate: 1 clean, 0 unformatted, 0 unchecked, 0 out of scope, 0 hygiene finding(s) (0 blocking)
```

The gate passes. You can commit.

## What you just did

You installed Procoder in one step, let it close the tool gaps for your
repository, and watched the gate refuse a commit and then accept it. And
you saw `format` hand back a fix rather than editing your file — that
last part is the whole design: **the binary computes and reports, you
decide and write.**

Next:

- [Ship a change](workflow.md) — the daily sequence, start to tag.
- [Onboard an existing codebase](how-to-onboard.md) — for a repository
  that predates Procoder and needs a full sweep.
- [The quality chain](quality-chain.md) — why every controller refuses
  instead of advising.
