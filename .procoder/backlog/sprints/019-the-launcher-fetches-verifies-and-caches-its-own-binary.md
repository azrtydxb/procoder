# the launcher fetches, verifies and caches its own binary

Status: active
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
