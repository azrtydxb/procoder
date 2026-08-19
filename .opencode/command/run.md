---
description: "How to run this project: the launch commands it declares, with the file that declared each."
---

The user invoked /procoder:run with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

1. Run `procoder run`. It prints the launch command(s) this
   repository declares — package.json scripts, Makefile targets, a Go
   main, a Cargo bin, manage.py, docker compose, a Procfile — each with
   the file that declared it, most specific first.
2. Run the command you chose in your OWN shell, not through procoder:
   a server is long-running, and backgrounding plus log capture belong
   to the tool that owns the process. `--exec` exists only for a single
   one-shot command and refuses when there is a choice to make or the
   command looks like a server.
3. No candidates is a real answer: a library has nothing to run.
4. Tests are a different question — use /procoder:test.
