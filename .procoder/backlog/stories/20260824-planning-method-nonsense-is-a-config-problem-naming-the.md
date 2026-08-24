# `[planning] method = "nonsense"` is a config Problem naming the line, and the run continues on the default.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: -

## Description

Every other key in config.toml names the line when its value means
nothing, because a writer who mistypes a setting believes it is set. This
one is no different, and it is more consequential than most: a typo here
silently decides which methodology governs the repository.

Done means an unrecognised value is a Problem naming the line, and the
run continues on the default.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] `[planning] method = "nonsense"` is a config Problem naming the line, and the run continues on the default.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
