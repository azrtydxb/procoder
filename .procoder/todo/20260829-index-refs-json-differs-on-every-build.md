# index: refs.json differs on every build

Status: open
Created: 2026-08-29

## Description

`procoder index build` writes a byte-different `.procoder/index/refs.json`
every time it runs, on an unchanged tree. Two consecutive builds of the
same fixture produce files that are not equal; the CONTENT is equivalent —
canonicalising the JSON (sorting each document's `symbols` and
`occurrences`) makes them compare equal — so what varies is the ORDER of
elements within each document, which comes from map iteration somewhere
between the SCIP output and the merged file.

Found while proving the state seam (#117 phase 0) changed no behaviour: it
was the one file out of thirty-seven whose bytes differed between the old
and new binaries. Checking whether the OLD binary agreed with itself is
what showed it was not a regression — it never has agreed with itself.

This matters for the same reason #236 and #245 did. A file that changes
without the tree changing is one that shows up in `git status` for no
reason, produces noise in a diff, and defeats any cache or comparison
keyed on its contents. It also means the file cannot be used as evidence
that an index is current.

The index is gitignored, so nobody has been bitten by it yet. That is why
it is a task and not a fix in flight.

Done means two consecutive `index build` runs over an unchanged tree
produce byte-identical `refs.json`.

## Acceptance criteria

- [ ] A test builds the index twice over one fixture and asserts the two
      `refs.json` files are byte-identical. Fails today.
- [ ] The ordering is fixed at the point the file is assembled — documents
      by path, occurrences and symbols by a stable key — rather than by
      sorting on read, so every reader gets the same file.
- [ ] `procoder index stats`, `find`, `search`, `refs`, `entrypoints`,
      `unused`, `outline` and `impact` report what they report today; the
      ordering change is in the file, not in the answers.
- [ ] `procoder check` is clean.

## Evidence

