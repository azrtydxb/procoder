package review

// Perspectives are who is reading, where lenses are how.
//
// A lens is a method — enumerate paths, trace verification, judge
// structure. A perspective is a set of concerns somebody brings before
// any method is applied: the architect asks what this commits the system
// to, the implementer asks what it will be like to work in, and they
// reach different conclusions about the same correct code.
//
// Deliberately not personalities. BMad's equivalent gives each a name and
// a voice, which works for a chat-first tool; procoder has no voice by
// design — the binary reports facts and the agent speaks — so it takes
// the multi-angle read and leaves the cast. A stance without a name is
// still a stance.
//
// Applied at spec and plan time rather than to a diff. By the time there
// is a diff the architectural question has been answered, and the cost
// of answering it differently is the whole change.

const analystPerspective = `# Analyst

You are asking whether this is the right problem, before anyone asks
whether it is the right solution.

Who has this problem, how often, and what do they do today instead? What
happens if nothing is built — not "it stays broken", but what the person
actually does that morning. Which part of the proposal serves the problem
as stated and which part serves a problem somebody assumed.

The finding worth most here is the one that says the problem is narrower,
wider, or different than written.`

const architectPerspective = `# Architect

You are asking what this commits the system to.

Which decisions here are cheap to reverse and which are load-bearing for
years. What this makes easy that was hard, and what it makes hard that
was easy. Where the seams are, and whether the ones being added are in
places that will still make sense when the requirement changes.

Name the commitment, not the preference. "I would have used a different
structure" is taste; "this couples X to Y so changing Y means changing X"
is a consequence somebody will pay.`

const implementerPerspective = `# Implementer

You are asking what it will be like to build and live in this.

Where is the part that reads clearly and is wrong. What has to be held in
your head at once to change it safely. Which failure will be reported as
something else entirely, three layers away from its cause. What a person
does at 3am when it breaks, and whether the code tells them anything.

The concern here is the second change, not the first: a shape that is
fine to write once and painful to touch again is a finding.`

const reviewerPerspective = `# Reviewer

You are asking what a reader is owed.

What a person needs to know before this makes sense, and whether they are
given it or expected to have it. Which claim is asserted where it should
be shown. What the document says will happen versus what it says will be
true — a promise and a property are different, and confusing them is how
scope goes missing.

Ask what is NOT here that its absence would be easy to miss: the
unmentioned case, the unstated assumption, the section that ends before
the hard part.`

// PerspectiveSet is the shipped set, in the order a full read applies
// them: problem before structure, structure before construction,
// construction before the account given of it.
var PerspectiveSet = []Lens{
	{Name: "analyst", Body: analystPerspective},
	{Name: "architect", Body: architectPerspective},
	{Name: "implementer", Body: implementerPerspective},
	{Name: "reviewer", Body: reviewerPerspective},
}

// PerspectiveDir is where a repository puts its own.
const PerspectiveDir = ".procoder/review/perspectives"
