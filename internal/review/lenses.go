package review

// The shipped lenses. Each is a stance, not a checklist: a distinct way of
// being wrong about a change, written so that two lenses reading the same
// diff disagree about what matters. Overlap between them is signal — the
// same line reached by two routes is a line worth looking at — so nothing
// here tries to partition the space.
//
// These are procoder's own words. The idea of reviewing through named
// lenses is not procoder's and is not claimed to be; the expression is,
// which is what keeps this file free of anyone else's licence.
//
// A repository replaces any of them from .procoder/review/lenses/.

const adversarialLens = `# Adversarial

Assume this change is wrong and find where. You are not confirming it
works; you are looking for the case its author did not.

Read for the gap between what the change intends and what it does. A
condition that is nearly right. An assumption that holds today because
of something unrelated. A name that says one thing while the code does
another. State the input or the sequence that breaks it — a concern
without a trigger is not a finding.

Zero findings is not a result here. Every other lens may legitimately
find nothing; this one finding nothing almost always means it was not
run properly, so treat an empty list as a signal to read again rather
than an answer to report.`

const edgeCaseLens = `# Edge cases

Enumerate paths. Do not judge whether the change is good.

Walk every branch mechanically rather than hunting by intuition: each
condition, each loop bound, each early return, each error the code can
receive. For every path, ask only whether it is handled.

Report the unhandled ones and discard the rest silently — a list that
includes what already works buries what does not. Empty, zero, one,
many, absent, malformed, and the boundary either side of every limit
the code names.`

const verificationGapLens = `# Verification gap

One question: if the behaviour this change is supposed to produce broke
where it is actually used, would verification fail?

Trace from the changed behaviour to whatever is supposed to catch it
breaking — a test, an assertion, a type, a check downstream. Then ask
whether that thing would actually go red, not whether it exists. A test
that exercises the function but asserts nothing about the changed
behaviour is a gap. A test whose fixture cannot reach the new branch is
a gap. A test that would pass against the old code is a gap.

Do not hunt for correctness bugs, but report the ones you notice while
tracing.`

const structureLens = `# Structure

Judge how this is organised, never whether its claims are true. The
content is the author's; the shape is what you are reading.

Ask what a reader needs first and whether they get it there. Whether
each section earns its place or restates its neighbour. Whether the
order follows the reader's questions or the writer's discovery. Whether
something buried three levels down is the thing most people came for.

Name what to move, merge, split, or cut, and say what the reader gains.`

const proseLens = `# Prose

Judge how this is expressed, never whether its claims are true.

Read for the sentence that has to be read twice. The clause that could
go without losing anything. The word doing less work than its length
suggests. Hedging that costs the reader confidence the writer meant to
give them. A term used two ways in one document.

Quote the passage and give the replacement. "This is unclear" is not a
finding; the rewritten sentence is.`

// Lenses are the shipped set, in the order a full review runs them.
// Reading order is deliberate: the behavioural lenses go first, because a
// document whose logic is wrong does not benefit from being told its
// paragraphs are in the wrong order.
var Lenses = []Lens{
	{Name: "adversarial", Body: adversarialLens},
	{Name: "edge-case", Body: edgeCaseLens},
	{Name: "verification-gap", Body: verificationGapLens},
	{Name: "structure", Body: structureLens},
	{Name: "prose", Body: proseLens},
}
