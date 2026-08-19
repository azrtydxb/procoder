---
description: "Test-driven development with tests that actually catch breaks: red before green, name the break each test catches, and the mutation check before done."
---

The user invoked /procoder:tdd with arguments:

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

The iron law: **no production code without a failing test first.** Wrote
code before the test? Delete it and start over — don't keep it "as
reference". Violating the letter of this rule is violating its spirit.
The arguments (if any) name the feature or fix to drive; apply the cycle
to it.

The cycle, every behaviour:

1. **RED** — write one minimal test for one behaviour, then RUN it and
   watch it fail. This step is mandatory evidence, not ceremony: a test
   that passes immediately is testing existing behaviour (fix the test);
   a test that errors isn't failing for the right reason (fix it until it
   fails on the assertion you meant).
2. **GREEN** — the simplest code that passes. No options, no flags, no
   flexibility nobody asked for. Run the test (pass), run the suite
   (pass), output pristine — a new warning is a finding.
3. **REFACTOR** — only on green, no new behaviour, suite green after.

Writing tests that earn their keep:

- **Name the break**: before writing the body, answer "what production
  change makes this test fail — and is that change a bug?" No answer →
  redesign the test around observable behaviour. Asserting a constant
  equals itself, or that source text contains a line, catches decisions,
  not bugs.
- **Derive expectations independently**: literal expected values, not
  values computed by the same code path being tested — mirror assertions
  always pass.
- **Test your code, not the framework**: constructors, getters, and
  trivial forwarding earn tests only when they validate, normalise,
  default, or cause side effects.
- **Mocks**: mock the slow or external edge, keep what the test actually
  depends on real; a mock mirrors the real data completely (a partial
  mock passes the test and fails the integration); mock setup outgrowing
  the test logic means you want an integration test.
- **The mutation check** (before calling the work done): mentally break
  the production code — wrong constant, flipped branch, missing side
  effect, empty return, dropped validation. At least one test must fail
  for each realistic mutation; a mutation nothing catches marks that
  behaviour as unprotected.

Exceptions (throwaway spikes, generated code) exist — but ask the user
first; "skip TDD just this once" arriving mid-task is rationalisation,
not an exception. When stuck: hard to test = hard to use; a test needing
heavy mocking is telling you the design is too coupled.

Evidence discipline: the RED command + failing output and the GREEN
command + passing output go into the task's `## Evidence` section — that
is exactly what `launcher.sh todo close` will ask you for.
