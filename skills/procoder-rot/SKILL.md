---
name: procoder-rot
# procoder: literal alone/deprecated-no-trigger the line below names the rot skill
description: Find dead, stale, and deprecated code left behind — rung 4, ALONE. Use when the user says "procoder rot", "find dead code", "what can I delete", "unused exports", "stale code", "deprecated code", "unused dependencies", "what's rotting in here", or invokes /procoder-rot. Reports deletions with reachability evidence; never deletes.
---

# procoder-rot

A change isn't done until the thing it replaced is gone. This skill finds what
previous changes left behind.

## Procedure

**Step 0 — run the engine.** `node <plugin>/bin/procoder.js check <scope>`.
Its `alone/*` rules (`commented-code`, `deprecated-no-trigger`, `orphan-todo`,  <!-- procoder: literal alone/deprecated-no-trigger the doctrine names this pattern, it is not an instance of it -->
`debug-leftover`, `blanket-suppression`, `unexplained-suppression`) are
deterministic. Report them as-is. Never re-derive them by reading files, and
never drop one because it looks minor. Everything below is judgment the engine
cannot compute.

Then one pass per category. Each row names the search that finds candidates.

| # | Category | How to find candidates |
|---|---|---|
| 1 | **Dead exports** | List exported symbols (`git grep -nE '^export (const\|function\|class\|type)'`, `pub fn`, `public static`, `__all__`, …). For each: `git grep -nw "<symbol>"`. Zero hits outside the definition file = candidate. |
| 2 | **Commented-out code** | From the engine's `alone/commented-code` findings. Do not re-scan by hand. |
| 3 | **Settled feature flags** | Find flag reads (`git grep -nE 'featureFlag\|isEnabled\|FLAG_\|LaunchDarkly\|unleash'`). For each flag: `git log -S "<flag>" --oneline` — if its value hasn't moved in a release, it is settled. The dead branch goes. |
| 4 | **Deprecations with no removal trigger** | Engine's `alone/deprecated-no-trigger`. Add age: `git log -1 --format=%ar -S "<marker>"`. A deprecation *with* a trigger (version, date, ticket) is doing its job — leave it. |  <!-- procoder: literal alone/deprecated-no-trigger the doctrine names this pattern, it is not an instance of it -->
| 5 | **Version twins** | `git grep -nlE '(_old\|_new\|_v[0-9]\|_final\|Legacy\|Copy)\b'` plus any `v2` living beside a `v1`. Both alive with callers split = migration stalled; name which side to finish. |
| 6 | **Stale documentation** | For each doc block or README section describing behavior: `git log -1 --format=%at` on the doc vs. on the code file it describes. Code newer than the doc = read both and check the doc still tells the truth. |
| 7 | **Unused dependencies** | Every entry in the manifest (`package.json`, `pyproject.toml`, `go.mod`, `Cargo.toml`, `*.csproj`): `git grep -nw "<name>"` in source. Zero import sites = candidate. Check build/config files and plugin-style deps (formatters, type plugins, test runners) before calling it. |
| 8 | **Orphaned fixtures and config keys** | Files under `tests/fixtures`, `testdata`, `__fixtures__` with no loader referencing the filename. Config keys declared in schema/defaults and read nowhere. |

## Verification — required before recommending any deletion

A symbol with zero direct call sites can still be reachable. Before you call
anything dead, run every check below and say which one cleared it:

1. **Bare-name string search.** `git grep -n "'<name>'"` and `git grep -n '"<name>"'` —
   catches dynamic dispatch, reflection, DI container registration, and route
   tables keyed by name.
2. **Constructed names.** Search for the stem, not the full name: a handler called
   `userCreateHandler` may be reached as `` `${entity}CreateHandler` ``. Grep the
   stem and any prefix/suffix that appears in template literals or `getattr`,
   `Reflect`, `importlib`, `Class.forName`, `Activator.CreateInstance`.
3. **Entry points declared outside source.** Check `package.json` (`bin`, `exports`,
   `main`, `scripts`), plugin manifests, CI workflows, Dockerfiles, `setup.py`
   entry_points, framework auto-discovery directories (routes, migrations,
   handlers loaded by convention).
4. **Public API contract.** If the package is published or the symbol is in a
   documented public surface, it has callers you cannot see. Not rot.
5. **Cross-language and cross-repo callers.** FFI, gRPC/GraphQL schema names, SQL
   function names, front-end calling a back-end route string.

If any check leaves reachability uncertain, report the item as **needs
confirmation**, not as a deletion. Recommending a deletion that breaks a caller
is the one failure that makes this command untrustworthy.

## Output

Grouped by category, one line per finding, rung fixed at `[4 ALONE]`, with an age
column where git can supply it:

```
[4 ALONE]   api/users.ts:6      createUserV1 exported, 0 non-def hits, 14mo → delete (dead export)
[4 ALONE]   api/flags.ts:20     flag NEW_CHECKOUT unchanged 8mo → delete flag + dead branch (settled flag)
[4 ALONE]   package.json:31     lodash declared, 0 imports → remove (unused dependency)
[4 ALONE]   docs/auth.md:44     describes session cookies; code moved to JWT 5mo ago → rewrite (stale doc)
[4 ALONE]   src/registry.ts:12  loadPlugin, only string-built call sites → needs confirmation
```

Close with two lines: total lines removable, and the single highest-value
deletion with why it is worth the most.

## Do not

- Do not delete anything. This skill reports; the user decides.
- Do not flag a published package's exports as dead just because this repo has no
  caller.
- Do not treat a deprecation *with* a removal trigger as rot — it is doing its job.
- Do not report a deletion whose reachability you did not verify. "Needs
  confirmation" is always available and always cheaper than a broken caller.
- Do not re-derive engine findings by eye, and do not write essays. One line each.
