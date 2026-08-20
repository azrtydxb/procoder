---
description: Run the formatting gate over the changed files, as a commit or CI would.
---

The user invoked /procoder:check with arguments: $ARGUMENTS

Run:

    procoder check $ARGUMENTS

(With no arguments it checks the repository's changed files.)

Report the summary line to the user. If anything is unformatted, fetch each
file's formatted result with `procoder format <file>`, review it, and write
it — then re-run the check to confirm the gate passes. If anything is
UNCHECKED, the gate fails until the missing formatter is installed; point the
user at /procoder:doctor.
