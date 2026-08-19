# Documentation rules

Repo-level documentation rules the procoder harness reads and follows. Edit
freely — what is written here wins over the built-in defaults. The three list
sections below are machine-read (one `- item` per line); everything else is
guidance for the agent.

## Required docs

- README.md
- CHANGELOG.md
- SECURITY.md

## Required badges

- ci
- license

## README first screen

- usp
- badges
- quick start

## Version-tracked docs

- README.md
- docs/index.md

## README must mention

- commit gate
- code index
- security
- lint
- maintain
- performance
- documentation
- ci
- infra
- audit
- spec
- plan
- todo
- debt
- lessons
- principles
- every agent
- self-learning

## Guidance

Documentation voice is professional product documentation: third person,
present tense, no personal names, no project-history anecdotes, no
first-person diary ("this burned us", "we learned"). History belongs in
the changelog and the lessons ledger; the docs state what the product
does and why, as if written for a stranger evaluating it today. Real
command output shown in docs is captured verbatim from actual runs,
never fabricated.

The README's first screen must sell the project: lead with the one-line value
proposition, then badges, then a quick start a stranger can paste. Diagrams
are Mermaid (they render on GitHub and in the docs site) with the shared
theme in .procoder/docs/mermaid.json. Broken relative links and diagrams
that do not compile are blocking; external links are verified by
`procoder docs --external` and CI — never skipped, never in the write hook.
Keep CHANGELOG.md current: every release gets an entry a user can read.
