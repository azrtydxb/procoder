# procoder against a real GitHub repository, CI and a release included

Status: closed 2026-08-24
Created: 2026-08-24

## Goal

Answer the four questions a local fixture cannot.

`ci --runs` reads workflow history, `copilot-leak` reads review comments,
`docs --external` checks published Pages and live links, and the release
path tags, builds and publishes. Every one of them reaches the GitHub API,
and calling them covered on the strength of a local run would be this
campaign committing the exact failure it was built to find.

So the fixture gets pushed to a throwaway public repository, its CI runs
for real, and a version is tagged and released end to end. Public rather
than private because Actions minutes are free there and this will run more
than once. Named as a fixture, so anyone who finds it knows what it is.

The bar is the same as every other sprint: a command that could not reach
GitHub says so and is recorded NOT RUN, never counted among the passes. An
API that answers with an empty list is not an API that answered clean —
`TestEmptyRunListIsNeverReadAsGreen` already exists because that
distinction was got wrong once.

The repository is deleted when the campaign closes, and its absence is
verified rather than asserted. That is sprint 018's story, and it is the
reason this one may create something at all.

## Result

committed: 1
done: 1 (20260824-ci-runs-copilot-leak-docs-external-and-a-tagged-release-are)
carried: 0

## Retro

**A real repository found what a local fixture could not, immediately.**
The release controller pronounced an already-tagged, already-published
version ready and printed a `git tag` command that fails. Nothing local
could have shown that: it needs a tag that exists, which needs a release
that happened. The decision to spend a public throwaway repo rather than
mock the API paid for itself in one command.

**The instructive test was the failing one.** A green CI run tells you
almost nothing — it is the state everything defaults to looking like.
Pushing a deliberately broken test and watching `ci --runs` come back
"failure — failing job(s): test" is the assertion with content, and the
same is true of "HEAD is not pushed — CI cannot have seen it": a verdict
that qualifies itself is worth more than one that is merely correct.

**A false finding cost a minute; not checking would have cost credibility.**
The first dead-link plant used a `.invalid` domain, came back unreported,
and looked exactly like a silent green in link checking. lychee excludes
reserved TLDs by RFC 2606 on purpose. Asking lychee directly, and noticing
`.lycheecache` existed at all, settled it before anything was written
down. Third time this campaign the interesting finding was the instrument.

**Redundancy defeats single-mutation proof, and that has to be said rather
than hidden.** Two independent mechanisms stop an unrelated tag blocking a
release, so neither mutation alone fails the test. Both were run, both
left it passing, and the pair together fails it. The choice was to state
the proof as the pair or to quietly claim a proof that did not hold — and
the second is exactly the failure the mutation discipline exists to stop.

**The adaptation worth keeping: record what a sprint did NOT cover, in the
story, at close time.** Pages health has an "enabled but stale" branch this
campaign never reached, because enabling Pages on a throwaway repo to test
one branch was out of proportion. Written into the evidence beside what
was covered, that is a known gap; left out, it would read as tested.
