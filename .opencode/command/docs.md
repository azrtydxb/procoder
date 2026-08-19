---
description: "The documentation report: references, diagrams, drift, API docs, badges, README structure, links, Pages."
---

The user invoked /procoder:docs with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

First read .procoder/docs/RULES.md and follow it — the repo's rules win over
any defaults. If it is missing, get the default via `procoder templates`,
write it (and .procoder/docs/mermaid.json), then continue.

1. Run `procoder docs --external` and read the whole report.
2. BLOCK lines are yours to fix before the work is finished: broken relative
   references, Mermaid diagrams that do not compile, dead external links,
   files NOT checked because a tool is missing (run `procoder init`).
3. info lines are judgment calls, judged honestly:
   - drift ("doc mentions changed file X"): open the doc, verify the prose is
     still true; update it if not, and say so either way.
   - missing doc comments: write real doc comments for the symbols that are
     genuinely public API; say why for any you leave.
   - missing required docs / badges / README structure: write them. The
     README's first screen must sell the project — one-line USP, badges,
     quick start a stranger can paste.
   - Pages findings: if Pages is off or stale, check the docs CI job and
     `gh api repos/{owner}/{repo}/pages`; fix the pipeline, not the symptom.
4. Re-run `procoder docs --external` and show the user the report. Repeat
   until no BLOCK lines remain and every info line is fixed or explained.

Never mark a doc updated without reading what it actually says (P-CONTROL:
the tools diagnose, you verify and write).
