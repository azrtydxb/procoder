# procoder against a real GitHub repository, CI and a release included

Status: active
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
