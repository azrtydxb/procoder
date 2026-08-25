# the binaries leave the tree and CI builds them

Status: closed 2026-08-25
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

## Result

committed: 4
done: 4 (20260825-ci-runs-no-reproducibility-job-and-procoder-release-no, 20260825-no-file-under-dist-is-tracked-and-git-ls-files-reports-no, 20260825-the-launcher-s-comment-docs-and-a-new-adr-each-state-that, 20260825-the-release-job-builds-all-five-targets-from-the-tagged)
carried: 0

## Retro

**Removing a check is a design act, not cleanup.** Two went: CI's
reproducibility job and the release controller's shipped-binary check. The
second was written the day before, against exactly the failure this sprint
makes impossible — which is the ordinary lifecycle of a guard, not a
mistake. What mattered was deleting their tests with them. A test left
asserting behaviour that no longer exists is worse than no test: it passes,
it looks like coverage, and it describes a world that is gone.

**The order was the reverse of how it reads.** CI had to build and publish
before dist/ could stop being tracked, because a release job still copying
files nobody commits publishes nothing at all. Written into the sprint goal
before any of it was done, which is the only reason it was done that way.

**A criterion was too broad and the test corrected it, not the other way
round.** "git ls-files reports no binary anywhere in the tree" is false on
its face — the repository tracks PNG logos. The test checks for ELF, Mach-O
and PE magic instead, and says why: a test that failed on the brand assets
would have been deleted rather than obeyed, and would have taught nobody
anything.

**The docs were wrong, not stale, and that distinction is worth keeping.**
`how-to-install-manually.md` told a reader to clone and put an empty
directory on PATH. Amending it would have left the instruction broken;
rewriting it around downloading from the release and verifying the
checksum makes it the thing the launcher does anyway. A doc that describes
a procedure nobody can follow is a defect, and it belongs in the same
sprint as the change that broke it.

**The adaptation worth keeping: name what a decision costs, in the
decision.** ADR 0004 has a Consequences section that spends as many words
on the offline first run and the lost third-party verifiability as on the
39MB a release. Six months from now somebody will ask why procoder reaches
the network at session start, and the answer needs to be in the record
rather than in whoever still remembers.
