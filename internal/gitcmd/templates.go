package gitcmd

// Default template contents. Per P-CONTROL the binary only PRINTS these —
// `procoder templates` — and the agent writes the files under
// .procoder/github/ (D-HOME). Plain Markdown, made to be edited.

const PRTemplate = `## What

<!-- One paragraph: what this change does, in the reader's terms. -->

## Why

<!-- The problem or need. Link the issue if one exists. -->

## How

<!-- The approach, and any decision a reviewer would want to question. -->

## Testing

<!-- What was run, and what proved it works. "Tests pass" needs the command. -->

## Notes for the reviewer

<!-- Anything that saves the reviewer time: where to start, what to skip. -->
`

const CommitTemplate = `

# --- commit message guide (lines starting with # are dropped) ---------------
# Subject: imperative, <=72 chars, no trailing period.
#   Good: "add oversized-file check to the gate"
# Blank line, then the body: WHY the change exists, then what it does.
# No AI attribution lines - the work is the author's.
`

const WorkflowTemplate = `# Workflow rules

Repo-level rules the procoder skills read and follow. Edit freely — what is
written here wins over the skills' built-in defaults.

## Worktrees

Feature work happens in a git worktree (one per branch/agent) so the default
branch checkout stays clean and parallel agents never collide. Use the
harness's native worktree support when available, ` + "`git worktree add`" + ` otherwise.

## Merge watching

While a PR's checks and reviews run, delegate the waiting to a background
agent that only watches and reports. The main agent keeps working and acts on
the report: every fix, every reply, and the merge itself stay with the main
agent — watchers gather information, the agent acts.

## After a successful merge

Clean up: delete the remote branch (merge with ` + "`--delete-branch`" + `), delete the
local branch, remove the worktree if one was used, run ` + "`git fetch --prune`" + `,
and return to an updated default branch.
`
