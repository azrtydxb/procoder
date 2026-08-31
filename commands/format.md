---
description: Show the formatted result for files, so you can review and write it.
---

The user invoked /procoder:format with arguments: $ARGUMENTS

Run the harness formatter over the requested files:

    "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh" format <files...>

If no files were named in the arguments, use the files you have written or
edited in this session.

The command prints each file's formatted result — it never modifies anything;
you stay in control.

For ONE file, stdout is exactly the bytes that belong in that file, whatever
the verdict: the formatter's output when it disagrees, the file's own bytes
when it does not, when no formatter covers the type, or when the formatter
could not answer. The verdict line goes to stderr. Capture that output to
another path and write it yourself, and the round trip is loss-free on every
verdict: an empty stdout means the file could not be read, never that it came
out clean.

Never redirect the output over the file it just read. The shell truncates the
target before the command runs, so that file is already empty by the time the
formatter could look at it, and nothing printed afterwards can put it back. The
command notices this shape and refuses with the name of the file and how to
restore it — that is a loud failure, not a recovery.

With MORE THAN ONE file, stdout carries a header naming each one, because five
files cannot share one stream otherwise. Read that output; never redirect a
multi-file run over any file it names.

For each file, the verdict says:

- "already formatted": the bytes on stdout are the file unchanged, and there is
  nothing to do.
- a formatted result: review it, then write it to the file yourself.
- "NOT checked": the formatter is missing or failed — the exit is non-zero. Tell
  the user what the output says, and suggest /procoder:doctor. Do not treat the
  file as clean.
- "out of scope": no formatter covers this file type; say so and move on.
