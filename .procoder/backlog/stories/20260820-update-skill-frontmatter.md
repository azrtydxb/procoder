# Update skills/procoder/SKILL.md with proper frontmatter

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

The skills/procoder/SKILL.md file currently has YAML frontmatter but needs to be fully compliant with the Agent Skills specification. Update the frontmatter fields (name, description, license, metadata) to meet all spec requirements: name 1-64 chars with lowercase alphanumerics, description 1-1024 chars describing both WHAT and WHEN, proper metadata category and version.

## Acceptance criteria

- [ ] Frontmatter YAML parses without errors
- [ ] `name` is 1-64 chars, lowercase alphanumerics + hyphens, no leading/trailing hyphen
- [ ] `description` is 1-1024 chars and describes both WHAT the skill does AND WHEN to use it
- [ ] `license` field present (Apache-2.0)
- [ ] `metadata.category` present (development)
- [ ] `metadata.version` present (1.0.1)
- [ ] Body is under 500 lines (progressive disclosure)
- [ ] `allowed-tools` added to frontmatter

## Evidence

