---
name: procoder-statusline
description: Install, inspect, or remove procoder's Claude Code statusline badge. Use when the user says "install the statusline", "procoder statusline", "show the procoder level in my status bar", "remove the statusline", or invokes /procoder-statusline.
---

# procoder-statusline

Thin wrapper over `node <plugin>/bin/procoder.js statusline`. The CLI owns the
file handling; this skill only decides which subcommand to run and reports what
it printed.

## Procedure

1. **Look first.** Run `statusline status` and report both lines it prints: the
   settings file, and what is configured there today.

   | Status says | Do |
   |---|---|
   | no statusLine configured | run `statusline install` |
   | installed | say so, stop — install would be a no-op |
   | a statusLine that is not procoder's | step 2 |
   | removal was asked for | run `statusline uninstall` |

2. **Foreign statusLine — ask, do not replace.** Run `statusline install`
   without flags. It refuses, printing what is configured and what procoder
   would set. Show the user both, ask whether to replace it, and only re-run
   with `--force` if they say yes. That statusLine is their work.

3. **Report the result.** Name the settings file written, and the timestamped
   `.backup-<ms>` copy the CLI printed. Add one line: the badge appears at the
   next session start, not in this one.

## Output

- What was configured before, and what is configured now.
- The backup path, verbatim from the CLI.
- `procoder statusline uninstall` as the way back.

## Do not

- Do not pass `--force` on your own initiative — only after an explicit yes.
- Do not edit `settings.json` by hand; the CLI backs up and writes atomically.
- Do not claim the badge is visible yet — it starts with the next session.
