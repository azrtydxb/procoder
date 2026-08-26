# A missing decisions file produces no finding and no note; a malformed one produces a note naming the file.

Status: done
Created: 2026-08-26
Epic: decisions-reach-the-user
Sprint: 022-a-decision-the-agent-cannot-make-reaches-the-user

## Description

The common case is no decisions file at all, and it must be silent — a
note in front of every user who never writes one is noise that trains
people to ignore notes.

The uncommon case is a file that exists and cannot be read, or one with
content in a shape the parser does not recognise. That must NOT be
silent: the decisions are sitting on disk and would never be asked.

Done when "absent" and "unreadable" do not look the same.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A missing decisions file produces no finding and no note; a malformed one produces a note naming the file.

## Evidence

`TestNoDecisionsFileIsSilent` and `TestAMalformedDecisionsFileSaysSo`.
Killed by making the `os.IsNotExist` branch emit a note, and by making
the no-heading branch return silence.

The second is this project's own rule applied to itself: a check that
could not run must never read as one that passed.
