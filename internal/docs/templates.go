package docs

// Default contents for the docs domain's repo files. Per P-CONTROL the binary
// only PRINTS these (`procoder templates`); the agent writes them. Per
// D-OVERRIDE the repo's copies win over everything compiled in here.

// DefaultRules is the starting .procoder/docs/RULES.md: prose for the agent
// with three machine-readable list sections the binary parses.
const DefaultRules = `# Documentation rules

Repo-level documentation rules the procoder harness reads and follows. Edit
freely — what is written here wins over the built-in defaults. The three list
sections below are machine-read (one ` + "`- item`" + ` per line); everything else is
guidance for the agent.

## Required docs

- README.md
- CHANGELOG.md

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

<!-- Optional, blocking when filled: the feature families the README's
     narrative must carry. Matching is case-insensitive and whole-word
     (multi-word phrases allowed); badge images and link targets are
     stripped first, so only prose counts. List what your product IS —
     a family the front page stops telling blocks the gate, which is
     how README rot gets caught at commit time. -->

## Guidance

The README's first screen must sell the project: lead with the one-line value
proposition, then badges, then a quick start a stranger can paste. Diagrams
are Mermaid (they render on GitHub and in the docs site) with the shared
theme in .procoder/docs/mermaid.json. Broken relative links and diagrams
that do not compile are blocking; external links are verified by
` + "`procoder docs --external`" + ` and CI — never skipped, never in the write hook.
Keep CHANGELOG.md current: every release gets an entry a user can read.
`

// DefaultMermaidConfig is the shared diagram theme so every diagram in the
// repo looks deliberate and consistent.
const DefaultMermaidConfig = `{
  "theme": "neutral",
  "themeVariables": {
    "fontFamily": "ui-sans-serif, system-ui, sans-serif",
    "primaryColor": "#DCEAED",
    "primaryBorderColor": "#0E5563",
    "primaryTextColor": "#14181D",
    "lineColor": "#4C555F",
    "clusterBkg": "#F4F6F7"
  }
}
`
