# the launcher fetches, verifies and caches its own binary

Status: closed 2026-08-25
Created: 2026-08-25

## Goal

Make the launcher able to stand alone, before anything is taken away from
it.

Everything in this sprint is the mechanism: read the version from
`plugin.json`, fetch the asset for this platform, verify it against the
published checksums, install it atomically, exec it — and on the second
run, do none of that and exec what is already there.

Two halves of one rule decide every failure. A hook that cannot fetch
warns and lets the session continue, because a hook that fails hard breaks
the thing procoder exists to help. A command that cannot fetch refuses,
because a launcher that exits 0 having run nothing is a silent green
sitting underneath every check in the tool. SessionStart is a hook even
though it is spelled `principles --hook`, and that is tested rather than
remembered.

Nothing is removed here. `dist/` still exists, CI still publishes what it
publishes, and the launcher prefers a cached binary — so this sprint can
land without a single user noticing, which is the point. The removals in
sprint 020 are safe only once this is proven.

The bar: every failure path is exercised against a stub server, not
against GitHub. A test that needs the network is a test that will be
skipped.

## Result

committed: 11
done: 11 (20260825-a-binary-whose-sha256-does-not-match-the-manifest-is-not, 20260825-a-checksum-mismatch-under-a-hook-exits-0-and-executes, 20260825-a-download-that-is-interrupted-leaves-no-file-at-the-cache, 20260825-a-plugin-json-that-is-missing-unparseable-or-naming-a, 20260825-a-second-invocation-inside-the-failure-window-makes-no, 20260825-launcher-sh-check-in-the-same-conditions-exits-non-zero-and, 20260825-launcher-sh-hook-post-tool-use-with-no-binary-and-no, 20260825-launcher-sh-principles-hook-with-no-binary-and-no-network, 20260825-the-version-fetched-is-the-one-in-claude-plugin-plugin-json, 20260825-with-no-cached-binary-and-a-reachable-network-the-launcher, 20260825-with-the-binary-already-cached-the-launcher-makes-no)
carried: 0

## Retro

**Designing the traps out before writing code was worth more than the code
was.** Seven were found reading the design against reality, and the worst
would have shipped: SessionStart is spelled `principles --hook`, not `hook
<sub>`, so the split written to keep sessions alive would have refused at
session start on any machine that could not fetch. It was found by listing
how hooks are ACTUALLY invoked in claude-hooks.json rather than trusting a
summary written an hour earlier — including one written by me.

**Two of eight mutations did not fail, and both were the test's fault.**
The windows/arm64 refusal asserted that "windows-arm64" appeared in the
message, which the asset URL contains anyway, so deleting the sentence
that names the platform left it green. And swapping the atomic rename for
a copy leaves the race test passing, because copying a small file wins a
two-way race almost every time. The first was fixed. The second cannot be
fixed with a filesystem this suite controls, so the limit is written into
the test — the third time this campaign that stating what a check does not
prove was the only honest option.

**An assertion that passes for the wrong reason is the recurring defect of
this whole effort.** Empty greps, a pipefail exit code, a URL containing
the string the message was supposed to carry. They are all one shape: the
assertion is true, and true for a reason that has nothing to do with the
behaviour. Running the mutation is the only thing that has ever caught it;
reading the assertion never has.

**Landing a change that nobody can notice is what makes the next one
safe.** Nothing was removed here. dist/ still ships, a cached binary still
wins, and every user is on exactly the path they were on yesterday. The
removals in 020 are reversible right up until they are not, and this
sprint is the evidence that they can be made at all.

**The adaptation worth keeping: a stub server, never the real one.** Every
failure path — offline, no such release, bad checksum, no line for this
platform, unwritable cache, two racers — is exercised locally and
deterministically. A test that reached GitHub would answer about the
network, and would be skipped the first time CI ran offline, which is
exactly when it would matter.
