# Updating in place stops leaving every previous version behind

Status: done
Created: 2026-08-26

## Goal

Reclaim the 1.1 GB, without ever deleting the version somebody is running.

`claude plugin update` writes each new version into its own directory and
removes none of the old ones. On the maintainer's machine that is 55
versions and 1.11 GB, one of them in use. `claude plugin prune` does not
cover it.

The whole risk here is in one direction. Reporting too little wastes disk;
removing too much leaves somebody with no working install and no rollback.
So: report by default and remove only under `--apply`, two independent
protections on the version in use, a retention window that keeps one
previous, and a refusal rather than a guess when the state cannot be read.

## Ordering

Seeded from the spec BEFORE implementation, unlike sprint 022. #186 is
about specs and stories being validated for truth rather than structure,
and the least this sprint can do is get the sequence right.
