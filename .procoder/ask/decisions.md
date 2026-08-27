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

## Remove the cached 3.1.0 plugin too, or keep it as the rollback?

**Decided: keep 3.1.0.** The rollback is worth 45 MB; prune's
active-plus-one-previous policy stands as written.

`procoder prune --apply` removed 3.0.0 and reclaimed 45 MB. It kept 3.1.0
deliberately: the policy is the active version plus one previous, so a
release that misbehaves has somewhere to fall back to. Removing 3.1.0 is
outside what prune will do — it would be a manual `rm -rf` of
`~/.claude/plugins/cache/procoder/procoder/3.1.0`.

The trade is a fixed ~45 MB against the one-step rollback. Note that 3.1.0
is now three releases behind and predates `release.maintainers`, so as a
rollback target it would reintroduce the false positive that blocked the
3.3.0 release commit.

- keep 3.1.0: prune's own policy, and a rollback that costs 45 MB.
- remove 3.1.0: reclaims the space; rollback then means reinstalling from
  the marketplace rather than a local directory.

## `procoder wizard` (#192): what shape may it take?

CORRECTED after reading the code. An earlier pass here claimed #192
contradicts AGENTS.md and that #200 shipped a fingerprinted approval to
gate it on. Both were wrong, and both came from grepping a summary line
instead of reading what shipped.

The rule reads "never executed AUTOMATICALLY", and it names `procoder run`
as the shape to copy: prints the candidates, executes only under `--exec`,
refuses when more than one candidate exists. `procoder run --exec` already
executes a command the repository declared. So a human typing
`procoder wizard run <name>` is the sanctioned shape, not a violation.

What #200 actually shipped is ten lines in internal/runcmd/runcmd.go that
print which binary `--exec` resolved to on PATH. There is no fingerprint
store and no approval record. Anything needing one builds it first.

**Decided: the `procoder run` shape.** Print by default, execute under an
explicit flag, refuse rather than guess. No new boundary, no new mechanism.

- build it in the `procoder run` shape: print by default, execute under an
  explicit flag, refuse rather than guess. No new boundary needed.
- build the fingerprinted approval first, then the wizard on top of it.
- close #192: the manual procedures it targets (#66-#73) are one-offs.

## What is next after v3.3.0?

**Decided: all of them.** Working the six roadmap issues in dependency
order: #194, then #211, then #209 (the docs cluster, which cross-reference
each other), then #192, then #189, then #190 last as the largest.

The ordinary queue is empty; all six open issues are roadmap-labelled.

- #189 SKILL.md hardening: smallest, and its own research note calls it the
  single highest-leverage change. Three of its four criteria still apply.
- #194 + #211 docs cluster: pitfalls tables, honest-limits, positioning,
  comparable projects. Medium, low risk, aimed at a skeptical adopter.
- #209 research bibliography: needs real external citations, verified. The
  one item where the failure mode is fabricating a source in a document
  whose entire purpose is evidentiary honesty.
- #190 `procoder learn`: largest by far, and the issue itself says it needs
  a spec before any implementation.

## #211 asks for a claim the issue record contradicts

**Decided: write it accurately.** Sources are named as sources; the
convergence argument is made only where it holds.

#211's acceptance criteria require a "framing paragraph [that] makes the
'independent convergence as evidence' argument explicit" about unlazy,
addyosmani/agent-skills and mattpocock/skills.

Checked against this repo's own issue record, that framing is false for
those three. #199 (leases), #200 (approval binding), #201 (never-execute),
#202 (dispatch waves) and #207 (depth scaling) each open with
"## Source — Leonxlnx/unlazy's ..." and were all filed on 2026-08-26 in one
research sweep. #189's anti-rationalization table cites addyosmani's repo
the same way. Procoder did not converge on these independently; it read
them and took them, deliberately and recently.

Verified against each project's own repo today: unlazy does ship the Depth
Tree, lease coordination, a Stop hook, and pre-execution PATH disclosure;
addyosmani/agent-skills does ship per-skill rationalization tables, a
docs/comparison.md and a docs/skill-anatomy.md.

A convergence claim is still available, but only for what procoder had
BEFORE the sweep — the gate, the quality controllers, evidence-gated
closes — which unlazy arrived at separately. That is a narrower and
checkable claim.

- write it accurately: name unlazy and agent-skills as SOURCES in
  influences.md (a fifth relationship: borrowed from, recently), and make
  the convergence argument only for the pre-sweep features where it holds.
- follow the criteria as written: make the broad convergence argument.
  Publishing a claim this repo's own issues contradict, in a document whose
  purpose is credibility.
- split: comparable-projects.md lists the projects factually with no
  convergence framing; the argument moves to #209's research page or is
  dropped.

## The tracked-tree gate is 787 gitleaks process startups — batch it?

`Secrets` calls `scanOne` once per path, and `scanOne` starts a gitleaks
process. Over 787 tracked files that is 787 startups. A probe scanned the
whole tree with ONE invocation in about a second, which accounts for the
~290s of the 341s CI step that four other explanations did not.

`SastChanged` in the same file already solves this shape: scan the tree
once, then filter the FINDINGS to the paths asked about, rather than naming
targets. Its comment says why — naming targets made semgrep scan files its
own default selection skips.

- batch above a threshold: one scan when the path set is large, per-file
  when it is small. Keeps a three-file commit from scanning a monorepo.
- always scan the tree and filter, like SastChanged: one rule, no
  threshold, no heuristic to tune — and a large repository pays it on
  every commit.
- leave it: the local cost is 41s and only CI sees the full tree.

## PR #241 is a probe that must not merge

It adds timing steps to the gate job and exists only to answer where the
441s went. It has answered.

- revert the probe steps and close #241 unmerged.
- keep the runner-cores line, drop the rest: the cores line is a permanent
  diagnostic; the PROBE steps are not.
