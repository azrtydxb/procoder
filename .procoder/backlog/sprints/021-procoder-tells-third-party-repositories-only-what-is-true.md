# procoder tells third-party repositories only what is true anywhere

Status: done
Created: 2026-08-26

## Goal

Stop procoder answering for code it was never given.

A contributor cloned an upstream project, changed two files, and was handed
nineteen findings — seventeen about procoder's conventions and one about a
line 2,500 rows from their diff. The project's own gate was green. The only
way through was `--no-verify`, and that is the damage: not the noise, but
that the noise switches off the checks worth having.

Two rules. A repository that never adopted procoder gets only what is true
anywhere — no credential, no 12MB blob, no conflict marker, no junk file,
no trailer nobody wrote. And in that repository the checks that read file
content see only the lines this commit wrote, because in somebody else's
code the diff is the only part that is mine to answer for.

The constraint that shapes every story: **an adopting repository loses
nothing.** Same findings, same words, same blocking. That failure would be
silent, so it gets its own story and its own test over the same fixture
with `.procoder/` added — the two runs differing in adoption and nothing
else.

And the reduced gate says it is reduced. A gate that quietly checks less is
the failure this project exists to prevent, and being deliberate about it
changes nothing.
