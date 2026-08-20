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
   - a page serving two purposes at once: split it. See below.
   - Pages findings: if Pages is off or stale, check the docs CI job and
     `gh api repos/{owner}/{repo}/pages`; fix the pipeline, not the symptom.
4. Re-run `procoder docs --external` and show the user the report. Repeat
   until no BLOCK lines remain and every info line is fixed or explained.

Never mark a doc updated without reading what it actually says (P-CONTROL:
the tools diagnose, you verify and write).

## Writing, not just checking

When you WRITE documentation — a new page, a rewrite, a section — the rules
file governs, and its core is the Divio documentation system: there are four
kinds of document, and a page that is two of them serves neither.

Name the kind before the first line:

- **Tutorial** — teaches a newcomer. It must run exactly as written on a
  clean machine. No trade-offs, no alternatives; they lose the learner.
- **How-to guide** — one stated goal, numbered steps, competence assumed.
  Put the goal in the title.
- **Reference** — describes, never persuades. Flat, complete, boring. All
  the options, not the interesting ones.
- **Explanation** — why it is like this: context, trade-offs, the road not
  taken. No steps; link to the how-to instead.

Then write it the way it gets read: answer first, a copy-pasteable example
over a paragraph about one, real names (`payments-api`, not `foo`), short
sentences, bold on the term the eye should land on, the searchable synonym
included, and an explicit "Common pitfalls" list wherever a feature has a
known misuse.

If the reader would have to be two people at once, the page is two pages.
