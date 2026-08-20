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

### Four kinds of document, never mixed

Documentation is not one thing; it is four, and a page trying to be two of
them serves neither. This is the Divio documentation system
(https://docs.divio.com/documentation-system/). Decide which one you are
writing BEFORE the first line, and let the page's opening sentence say so.

| Kind | Serves | Answers | Form |
| --- | --- | --- | --- |
| Tutorial | a newcomer learning | "teach me" | a lesson that must succeed end to end |
| How-to guide | a competent user working | "how do I X?" | numbered steps toward one stated goal |
| Reference | someone looking a thing up | "what are the options?" | flat, complete, boring description |
| Explanation | someone wanting to understand | "why is it like this?" | prose on context and trade-offs |

The failures are predictable, and each is a real page someone abandoned:

- A tutorial that stops to explain trade-offs loses the learner. Explain
  later — the lesson only has to work.
- A how-to that teaches from scratch wastes a reader who already knows.
  Assume competence; put the goal in the title.
- Reference that argues for an approach cannot be trusted to describe it.
  Describe, never persuade.
- Explanation carrying steps rots, because the steps then live in two
  places. Link to the how-to.

A tutorial must run exactly as written on a clean machine. A reference must
cover everything, not the interesting parts.

### Writing craft

- Answer first. Open every section with the command or the fact; context
  comes after it, never before.
- Show rather than tell. A copy-pasteable example beats a paragraph about
  one. Prose has to earn its space; working code already has.
- Real data. ` + "`payments-api`" + `, ` + "`POSTGRES_URL`" + `, ` + "`checkout_total`" + ` — never foo, bar,
  baz. Placeholder names hide the shape of real input.
- Short sentences, under fifteen words where the meaning allows. One idea
  each.
- Scannable: subheadings, bullets, bold on the term the eye should land on.
  Readers scan before they read.
- Findable: name a feature the way a reader searches for it, and give the
  synonym they might use instead — "the write hook (PostToolUse)".
- Say the don'ts. Where a feature has a common misuse, a short "Common
  pitfalls" list is worth three paragraphs of careful prose.

### Mechanics

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
