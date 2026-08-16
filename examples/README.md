# Worked examples

One before/after pair per rung. Each `before.ts` trips its rung through
`node bin/procoder.js check`, and each `after.ts` produces zero findings — the
test suite (`tests/examples.test.js`) asserts exactly that, so these are
executable documentation rather than prose.

| Rung | Pair | Finding it demonstrates | The fix |
|---|---|---|---|
| SAFE | [`safe/`](safe/) | authorization decided from client-controlled input, and a query built by string interpolation | look the role up server-side; bind the query's parameters instead of interpolating them |
| TRUE | [`true/`](true/) | an unhandled empty-array edge, and an error swallowed by an empty `catch` | guard the empty case; log the failure with a correlation id and rethrow |
| OBVIOUS | [`obvious/`](obvious/) | a 60-line function, six positional parameters, and a nested ternary | split into three named functions, an options object, and guard clauses |
| ALONE | [`alone/`](alone/) | a `@deprecated` function with no removal trigger, and a commented-out block | delete the old path; when something genuinely must stay, say when it can go (`// procoder: remove after v3.0`) |

Run any pair yourself:

```bash
node bin/procoder.js check examples/safe/before.ts   # reports SAFE findings
node bin/procoder.js check examples/safe/after.ts    # exits 0, no output
```
