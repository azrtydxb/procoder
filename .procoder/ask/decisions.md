# Decisions waiting on a human

Written by the agent, read by `procoder ask`. One `## ` heading per
decision; the lines under it are its options.

## Rescope #198 and #191, and merge #200 with #201 and #204 with #208?

Today's release overtook two of them, and two pairs are the same idea filed
twice. Left alone, somebody rebuilds what already exists.

- rescope and merge now: #198 loses its unfalsifiable-criteria half (shipped
  as `CriteriaWithoutFalsifiers`) and keeps fixed-output, hedgy vocabulary
  and unmeasured thresholds; #191 keeps board visibility and shared
  blockers and drops "there is no decisions file", because there is one now.
- leave them: the overlap is discoverable by whoever picks one up, at the
  cost of them finding out after starting.

## Which is the next piece of work?

- #193, merge-conflict discipline: the failure happened here today — git
  split a conflict through a function, "keep both sides" truncated a test,
  and only the compiler caught it. Concrete evidence, and the fix is prose.
- #201 + #200, the execute path: verified, not assumed —
  `internal/runcmd/runcmd.go:172` execs argv the repository declares, so an
  agent writing a launch command under injection is a live path. The only
  security-shaped items in the set.
- #195, the context.md glossary: small, self-contained, pays offevery session.

## Do the four large features stay open as a roadmap?

#189 SKILL.md redesign, #190 `procoder learn`, #192 `procoder wizard`, #194
docs hardening. Each is a release of its own; two add new commands.

- keep open: a roadmap that says what procoder might become.
- close until wanted: an open issue nobody is going to start reads as a
  commitment, and twenty-three of them read as a plan.
