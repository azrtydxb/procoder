# the binaries leave the tree and CI builds them

Status: active
Created: 2026-08-25

## Goal

Take away what sprint 019 made unnecessary.

The launcher can now fetch, verify and cache its own binary, and a cached
one still wins — so `dist/` has been dead weight since the last commit.
This sprint removes it, moves the build into the release job, and deletes
the two checks whose only subject was a committed binary.

The order matters and it is the reverse of how it reads. CI must build and
publish first: the moment `dist/` stops being tracked, every install
depends on a release having assets, and a release job that still copies
files nobody commits publishes nothing at all. So the release job changes,
then the tree.

What is being given up is written down rather than discovered. Offline
first-run goes: a fresh install now needs the network once. Third-party
verifiability goes with the reproducibility job: nobody outside CI will be
able to rebuild and confirm the published bytes. Both are real, both are
deliberate, and an ADR says so — because somebody will ask why procoder
reaches the network at session start, and "it seemed fine" is not an
answer.

The bar: after this sprint no file in the repository is binary, and a
release with no assets fails the job rather than publishing something that
looks finished and breaks every installer downstream.
