---
applyTo: "**/*_test.go"
---

# Tests in procoder

A test here is not finished when it passes. It is finished when it has
been **watched to fail**.

## Every test carries its mutation

Each test has a `// proved by:` line naming a specific change to the
source that must make it fail:

```go
// proved by: made the unmarshal failure return nil — a crashed semgrep
// then reads as a clean SAST pass.
func TestSastUnreadableOutputIsNotRun(t *testing.T) {
```

Then actually apply that change, build, and run the test. Three
discoveries came only from doing it, never from reading the assertion:

- A test exercised a helper directly and never the dispatch, so deleting
  the call from `run()` left it passing. It goes through the real entry
  point now.
- A P-CONTROL test ran `procoder format` only over files that were
  already formatted, so the branch that prints a rewritten file never
  executed — and a mutation making that branch write to disk passed. **A
  mutation must reach the branch**; check the fixture can provoke it.
- A mutation that does not compile is not a mutation, and neither is one
  that produces no diff. Verify it applied.

Where two redundant mechanisms guard one behaviour, no single mutation
will fail the test. Say so in the comment and name the pair, rather than
claiming a proof that does not hold.

## Assertions

- Assert the **verdict**, not the subject. `grep "unformatted <file>"`,
  not `grep "<file>"` — the file's name also appears in "UNCHECKED
  <file> — csharpier is not installed", which reports the opposite of a
  finding.
- Never assert on a word that appears either way. Checking for "deny" or
  "block" in a hook envelope passes on an allow, because "block" sits
  inside "1 blocking finding(s)". Read the decision field.
- An assertion that cannot fail is worse than none. `if before >= after`
  passes exactly when the knob under test did nothing.

## Portability

Windows runs `go test ./internal/...` in CI, and two habits break there:

- `exec.LookPath` needs an executable extension, so an extensionless
  shell-script stub is invisible. Skip on Windows like the existing stubs
  do, or use a real binary.
- `os.Chmod(dir, 0o000)` does not deny reads on Windows, so a test that
  provokes an unreadable-directory branch that way never enters it.
- Never use a rooted literal such as `/repo` as a test repository root:
  macOS and Linux call it absolute and Windows does not.

The behaviour under test is rarely platform-specific. The way of
provoking it often is — skip the provocation, not the coverage.

## Fixtures

- `t.TempDir()` for anything written. A test must not touch the working
  tree.
- Derive credential-shaped test data at run time rather than writing a
  literal: the repository's own secret scanner reads test files too, and a
  hand-invented string may not look like a key to the scanner at all,
  which makes the test pass for the wrong reason.
- Do not plant a value scanners deliberately allowlist. AWS's documented
  example key is ignored on purpose, so planting it tests the allowlist.
