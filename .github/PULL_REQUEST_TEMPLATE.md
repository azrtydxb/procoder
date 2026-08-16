## What and why

<!-- One or two sentences. Link an issue if there is one. -->

## Checklist

- [ ] `npm test` passes (521+ tests, including the self-scan in `tests/dogfood.test.js`)
- [ ] `npm run sync:check` is clean (doctrine edited only in `skills/procoder/SKILL.md`, then `npm run sync`, both changes committed)
- [ ] No new runtime dependency, or the PR description argues for one
- [ ] No rule/check id renamed (breaks the ratchet baseline silently) — or the PR says so explicitly and explains the baseline impact
- [ ] Any new regex over a whole line has a timing test proving it stays linear on a long adversarial line
