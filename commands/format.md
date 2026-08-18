---
description: Show the formatted result for files, so you can review and write it.
---

The user invoked /procoder:format with arguments: $ARGUMENTS

Run the harness formatter over the requested files:

    "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh" format <files...>

If no files were named in the arguments, use the files you have written or
edited in this session.

The command prints each file's formatted result — it never modifies anything;
you stay in control. For each file:
- "already formatted": nothing to do.
- a formatted result: review it, then write it to the file yourself.
- "NOT checked": the formatter is missing or failed — tell the user what the
  output says, and suggest /procoder:doctor. Do not treat the file as clean.
- "out of scope": no formatter covers this file type; say so and move on.
