# Procoder brand

Senior developer wisdom. Packed into one plugin.

This directory holds the identity masters. The files the README and the
documentation site actually load are the right-sized derivatives in
[`docs/assets/`](../docs/assets) — regenerate them from here rather than
editing them, with the commands under [Assets](#assets).

## Positioning

Procoder is the senior developer sitting beside your AI coding agent: the
engineering judgment, not another code generator. Best practices,
architecture, clean code, security, testing, maintainability, review
thinking, proven patterns, performance, production-readiness.

|                   |                                                                                     |
| ----------------- | ----------------------------------------------------------------------------------- |
| Brand promise     | Build Like a Senior.                                                                |
| Primary tagline   | Senior Developer Wisdom. Packed into One Plugin.                                    |
| Short tagline     | Best Practices. Every Time.                                                         |
| Technical tagline | Senior Dev Wisdom. Automated.                                                       |
| In one sentence   | Procoder gives your AI coding agent the engineering judgment of a senior developer. |
| In three words    | Engineer. Review. Ship.                                                             |

## Logo

The mark combines three ideas: the **P** for Procoder and professional
software engineering, the **`</>`** that says software without needing a
word, and the **cape** — the plugin giving the agent capabilities it did
not have. Together: professional coding, supercharged.

| Asset  | Master             | Use                                            |
| ------ | ------------------ | ---------------------------------------------- |
| Banner | `brand/banner.png` | README header, marketing, social cards         |
| Logo   | `brand/logo.png`   | documentation site, avatars, plugin listings   |
| Icon   | `brand/icon.png`   | favicon, CLI branding, app icon, social avatar |

Below roughly 32 × 32 px, clarity beats detail: drop the cape and keep
**P + `</>`**. Never place the full lockup at favicon size.

Leave clear space around the logo on every side — at least **25% of the
logo's height** — and put no text, border, or other element inside it.

**Do not** stretch or rotate the mark, change the gradient, add competing
colors, place it over busy imagery, add heavy shadows or glow, alter the
code symbol, or swap the cape for something else.

## Color

A dark developer interface with an energetic violet → blue → cyan
spectrum running through it.

| Name            | Hex       | Role                                                    |
| --------------- | --------- | ------------------------------------------------------- |
| Procoder Black  | `#05070F` | primary background                                      |
| Midnight        | `#0B1020` | cards, navigation, code panels                          |
| Procoder Purple | `#7C3AED` | **primary brand color** — actions, links, active states |
| Electric Violet | `#9333EA` | secondary accent, gradients                             |
| Procoder Blue   | `#2563EB` | technical accent, diagrams, information states          |
| Electric Cyan   | `#06B6D4` | high-energy accent, used sparingly                      |
| White           | `#F8FAFC` | primary text on dark                                    |
| Slate           | `#94A3B8` | secondary text                                          |
| Muted Slate     | `#64748B` | metadata, low-priority information                      |

The signature gradient is violet → purple → blue → cyan:

```css
background: linear-gradient(
  135deg,
  #9333ea 0%,
  #7c3aed 35%,
  #2563eb 70%,
  #06b6d4 100%
);
```

Use it for logo treatments, headings, primary buttons, hero graphics,
dividers, and documentation accents — never across large areas. It should
read as energy flowing through an otherwise restrained dark interface.

## Typography

**Inter** for everything human: site, documentation, README graphics, UI,
marketing. Weights 400 body, 500 UI, 600 subheadings, 700 headings, 800
hero.

**JetBrains Mono** for everything machine: code, CLI commands, technical
labels, feature tags, version numbers. Inter reads as a professional
software company; JetBrains Mono carries the engineering credibility.

Headings are short and certain. **Build Like a Senior.** — not _Improve
Your Development Experience Using Best Practices_. In the same register:
Code With Confidence. Best Practices. Built In. Architecture Matters.
Ship Better Code. Think Before You Code.

## Voice

Procoder sounds like an experienced engineer, because that is the whole
product.

- **Confident** — "Keep business logic out of your controller," not "You
  might perhaps want to consider…"
- **Concise** — give the reason, skip the lecture.
- **Opinionated** — "Use dependency injection here," not "there are
  several possible approaches."
- **Pragmatic** — good engineering appropriate to the problem, not
  maximum abstraction everywhere.
- **Educational** — where it is practical, say why the practice matters.

If Procoder were a person: fifteen years in, has shipped systems that
failed, has inherited terrible codebases, knows why the patterns exist,
does not chase trends, dislikes unnecessary complexity, automates the
repetitive discipline. Most of all, wants you to ship.

## Visual language

Near-black backgrounds, generous negative space, purple-blue gradients,
thin glowing lines, subtle grids, code fragments, terminal elements,
restrained glow. Senior engineering tool with a little cyberpunk energy —
not a gaming product. The logo can be playful; everything around it stays
professional.

Icons are thin-line, geometric, slightly rounded, mostly monochrome, with
purple, blue, or cyan for active states: `</>` development, shield
security, puzzle patterns, rocket delivery, check quality, git branch
workflow, layers architecture, flask testing, gauge performance.

Components: cards 12px radius on `#0B1020` with a
`1px solid rgba(148, 163, 184, 0.15)` border; buttons 8px; badges 6px.
Primary buttons take the purple → blue gradient with white text;
secondary buttons stay dark with a thin slate border. Installation
commands get a terminal treatment.

## Naming

The product name is **Procoder** in prose and **`procoder`** wherever a
machine reads it — the binary, the package, the plugin id, the commands.
Avoid `PROCODER`, `Pro Coder`, `Pro-coder`, and `proCoder`.

## Assets

The derivatives under `docs/assets/` are generated from the masters here.
The banner is quantized because it is mostly smooth dark gradient, which
costs 350 KB in true color and shows no banding at 256:

```sh
magick brand/banner.png -strip -resize 1600x -colors 256 \
  -dither FloydSteinberg -define png:compression-level=9 docs/assets/banner.png
magick brand/logo.png -strip -resize 512x512 \
  -define png:compression-level=9 docs/assets/logo.png
magick brand/icon.png -strip -resize 256x256 \
  -define png:compression-level=9 docs/assets/favicon.png
```

The palette above is applied to the documentation site in
[`docs/stylesheets/brand.css`](../docs/stylesheets/brand.css); change a
color there and here together.
