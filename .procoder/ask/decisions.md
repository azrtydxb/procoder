# Decisions waiting on a human

## Close #206, whose premise does not hold for procoder?

#206 asks for ReDoS defence — bounded, sandboxed regex evaluation — because
procoder "evaluates regex in several places that ultimately read
repo-controlled or config-controlled text".

Checked rather than taken on trust, the way #201's premise was:

- there is no `regexp.Compile` anywhere in the tree;
- the only two `MustCompile` calls with a non-literal argument build their
  pattern from constants in source (`gitx.aiIdentities`,
  `security.pyDeps`);
- `.procoder/lint/RULES.md`'s `checks` list is passed to clang-tidy as its
  `--checks` value. It never reaches a Go regex engine.

So no repository- or config-controlled text becomes a pattern procoder
compiles. The exposure the issue describes is not there.

- close it, with the evidence recorded, and reopen if a runtime-compiled
  pattern is ever introduced.
- keep it open as a standing constraint on future work — a reminder not to
  add one.

## Do #210 now, before #193?

`docs/influences.md` credits superpowers, ponytail and serena. BMad appears
in it zero times, while `internal/planning/bmad.go` has shipped since 2.0.0
and `[planning] method = "bmad"` reads BMad's own artifacts. Verified: the
integration is real and the doc gap is real.

Small, factual, and the trademark constraint is already understood — BMad
is named to describe interoperation, never as a procoder feature name.

- do it now: ten minutes, and it closes a verified gap in a provenance
  record that is currently wrong by omission.
- after #193: #193 was already chosen as the next piece of work.

## Label #209 and #211 roadmap alongside #194?

All three are documentation-positioning work: an evidence bibliography, a
comparable-projects doc, and the docs-hardening issue already labelled
roadmap. They are one cluster and none is small.

- label both roadmap: the cluster reads as direction rather than queued
  work, which is what the label was created for.
- leave them unlabelled: they stay in the ordinary queue.
