<!-- procoder:allow-conflict-markers this tutorial shows a learner what a conflict looks like -->

# Getting started

**A tutorial.** By the end you will have Procoder installed, and you will
have watched its commit gate refuse a change and then accept it. Ten
minutes. Follow it exactly; explanations come later.

## 1. Install the plugin

In Claude Code:

```
/plugin marketplace add azrtydxb/procoder
/plugin install procoder
```

Then run `/reload-plugins`.

The hooks are now live. Every file the agent writes is checked in the
same turn, and every session starts with the engineering principles.

## 2. Put the binary on PATH

The plugin ships the binary, and this tutorial calls it directly so you
can see what the agent sees.

```
git clone https://github.com/azrtydxb/procoder
export PATH="$PWD/procoder/dist/darwin-arm64:$PATH"
procoder version
```

Replace `darwin-arm64` with your platform: `darwin-amd64`,
`linux-amd64`, `linux-arm64`, or `windows-amd64`.

You will see the version number:

```
0.32.8
```

## 3. Make a repository to break

```
mkdir checkout-notes && cd checkout-notes
git init -q .
```

Create `NOTES.md` with a merge conflict left in it, and trailing
whitespace after `cents.`:

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
procoder check
```

```
unformatted  /tmp/checkout-notes/NOTES.md  (run `procoder format "/tmp/checkout-notes/NOTES.md"` for the result)
BLOCKING     /tmp/checkout-notes/NOTES.md:5  merge conflict marker left in the file
BLOCKING     /tmp/checkout-notes/NOTES.md:9  merge conflict marker left in the file
info         .procoder/github/PULL_REQUEST_TEMPLATE.md is missing — run `procoder templates` and write it
info         .procoder/github/COMMIT_TEMPLATE.md is missing — run `procoder templates` and write it
info         .procoder/github/WORKFLOW.md is missing — run `procoder templates` and write it
procoder gate: 0 clean, 1 unformatted, 0 unchecked, 0 out of scope, 5 hygiene finding(s) (2 blocking)
```

Check the exit code:

```
echo $?
```

```
1
```

**BLOCKING** lines fail. **info** lines do not.

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
procoder format NOTES.md
```

```
== NOTES.md — formatted result (prettier), review and write it:
# Checkout notes

The checkout total is computed in cents.

Refunds reuse the same path.
```

The trailing whitespace is gone from the printed result — and still
present in your file. **Procoder printed the fix; it did not apply it.**
Copy that content into `NOTES.md` yourself.

## 7. Run the gate again

```
procoder check
```

```
info         .procoder/github/PULL_REQUEST_TEMPLATE.md is missing — run `procoder templates` and write it
info         .procoder/github/COMMIT_TEMPLATE.md is missing — run `procoder templates` and write it
info         .procoder/github/WORKFLOW.md is missing — run `procoder templates` and write it
procoder gate: 1 clean, 0 unformatted, 0 unchecked, 0 out of scope, 3 hygiene finding(s) (0 blocking)
```

```
echo $?
```

```
0
```

The gate passes. You can commit.

## What you just did

You ran the same gate CI runs, watched it refuse a commit over two
conflict markers, and saw `format` hand back a fix for you to apply
rather than editing your file. That last part is the whole design: **the binary computes and
reports, you decide and write.**

Next:

- [Ship a change](workflow.md) — the daily sequence, start to tag.
- [Onboard an existing codebase](how-to-onboard.md) — for a repository
  that predates Procoder.
- [The quality chain](quality-chain.md) — why every controller refuses
  instead of advising.
