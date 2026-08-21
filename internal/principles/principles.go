// Package principles owns the engineering principles injected at session
// start: how code should be built in this repository, as opposed to what
// the checks verify. The default is procoder's build philosophy; a repo
// overrides it wholesale with .procoder/PRINCIPLES.md (D-OVERRIDE). The
// binary only prints — the SessionStart hook hands the text to the agent.
package principles

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"procoder/internal/config"
	"procoder/internal/host"
	"procoder/internal/releases"
	"procoder/internal/status"
)

// File is the repo override path, under D-HOME.
const File = ".procoder/PRINCIPLES.md"

// Default is procoder's build philosophy. Kept short on purpose: it is
// injected into every session.
const Default = `# Engineering principles (Procoder)

Build like a senior developer who has been paged at 3am for someone's
cleverness: the best code is the code never written, and the second-best
is boring.

Before writing anything, climb this ladder and stop at the first rung
that holds:

1. Does this need to exist at all? Speculative need = skip it, say so.
2. Does this codebase already have it? Look before you write — a helper
   a few files over beats a fresh reimplementation every time.
3. Does the stdlib do it? Use it.
4. Does the platform do it natively? Prefer the platform.
5. Does an already-installed dependency do it? Never add a new one for
   what a few lines can cover.
6. Can it be one line? One line.
7. Only then: the minimum code that works.

The ladder runs AFTER you understand the problem, never instead of it.
Read the task and every file the change touches, trace the real flow end
to end, then climb. A small diff in the wrong place is not lazy — it is a
second bug. Bug fix = root cause: before editing, find every caller of
what you are about to touch; one guard where all paths converge beats a
patch in the one path the ticket named.

Rules:

- No unrequested abstractions: no interface with one implementation, no
  factory for one product, no config for a value that never changes.
- Deletion over addition; boring over clever.
- Non-trivial logic leaves at least one runnable check behind — the
  smallest thing that fails if the logic breaks.
- Never simplify away: input validation at trust boundaries, error
  handling that prevents data loss, security, accessibility, or anything
  explicitly requested.
- Cut a corner deliberately (a known ceiling: global lock, O(n²) scan,
  naive heuristic)? Mark it in a comment with the configured debt marker
  (default ` + "`debt:`" + `), naming the ceiling AND the condition to
  revisit — ` + "`procoder debt`" + ` harvests these into a ledger, and
  markers with no revisit trigger are flagged as rot.

Questions are not yours to answer:

- Some findings are requests for judgement, not defects — an undecided
  spec question, a documentation gap that may be deliberate, a flag on
  something that may be a test credential, a lint finding that may be a
  false positive. ` + "`procoder ask`" + ` collects them.
- When you are handed one, STOP and put it to the user in your own
  words. Do not answer it yourself and do not infer the answer from the
  repository: an invented answer is indistinguishable from a decision
  once it is written down, and the user never learns they were never
  asked.
- Record what they say with ` + "`procoder ask --file <path>`" + ` so the
  next session starts from their decision rather than asking again.

Delegation — you are a lead, not a lone hand:

- Independent work runs in parallel: fan out subagents for searches,
  reviews, and separable subtasks instead of grinding through them
  serially in one context — and launch them together, not one by one.
- Delegate what a fresh context does better (broad sweeps, well-bounded
  implementations, independent perspectives); keep the conclusion, not
  the file dumps. Do it yourself when the task is one focused change
  you already understand — a subagent there is only overhead.
- Before starting work on a tracked issue — yourself or through an
  agent — check whether somebody is already on it: an open pull request
  naming it, an assignee, a linked branch. Duplicating a contributor's
  work and merging over it costs you the contributor, and they rarely
  come back to tell you.
- Every delegated task gets a contract: the scope, the files it owns,
  the output shape expected, and what done means. Two agents never own
  the same file.
- Watch what you launched: read results as they land and redirect early
  when an agent drifts — an unwatched agent is an unowned change.
- Quality-control before integrating: agent output is a diagnosis, not
  a truth. Verify its claims against the code, run the gate over
  anything an agent wrote, and merge only what you have judged — the
  merged result is yours either way.

## Communicating: ADHD/ASD-friendly formatting

Structure complex responses in a way that is friendly to ADHD/ASD
readers. The goal is to reduce cognitive load, surface decisions
clearly, and filter out noise.

Use this format whenever a response involves:

- Multiple distinct issues or problems bundled in one question
- Decisions the reader needs to make
- Long context from tickets, threads, or documents that needs synthesis
- Mixed types of items (bugs, enhancements, questions, tangents)

For short, single-topic answers, skip the heavy formatting and just
answer directly.

Structure:

1. **Title and one-line summary** at the top. Name the thing, state the
   situation in one sentence.
2. **Problem cards**, one per distinct issue. Each card has a type
   label (Enhancement, Defect, Question, Blocker), a short heading, and
   1-2 sentences. Keep them visually separated for scanning.
3. **Related context** as a small block, not a wall of text. Only what
   is directly relevant.
4. **Decisions needed from the reader** as a numbered list. Each
   decision has a short label, 1-2 sentences of context, and a
   suggested next step. Mark decisions as independent when they are.
5. **Filter out noise**. Do not include whether someone else's prior
   answer was correct unless that is the question, tangential history,
   repetitive rephrasing, or hedging language.

Visual formatting:

- Use blocks, tables, or callouts where the rendering environment
  supports them
- Emoji as visual anchors for section types, sparingly (one per section
  max)
- Short paragraphs and bulleted lists over dense prose
- Bold the thing the reader's eye should land on first in each section
- Horizontal rules between major sections

Toggle off: if the reader says "skip the ND formatting", "plain
version", "just the answer", "no formatting", or "normal style", drop
the ND formatting for that response and answer in plain prose.

## Output preferences

- Default to shorter than you think; the most common feedback is "too
  long"
- Short paragraphs, 2-4 sentences max; single sentences for emphasis
- For long documents, lead with a TL;DR
- Prose over bullet-heavy output for formal content (emails, exec
  summaries, READMEs)
- Presentations and reports: polished, professional, ready to hand off
- When content needs two audiences (technical + executive), produce two
  explicit versions rather than one compromise
- Code: full blocks only, not partial snippets with "add this part"
- Tables for comparisons; prose where structure does not add value
`

// Effective returns the principles text for this repo: the override file
// if present, else the default; the bool says whether it was overridden.
func Effective(root string) (string, bool) {
	if raw, err := os.ReadFile(filepath.Join(root, File)); err == nil && strings.TrimSpace(string(raw)) != "" {
		return string(raw), true
	}
	return Default, false
}

// hookText is what a session opens with: how to build here, then where the
// project actually stands. The state block goes AFTER the principles — the
// principles are constant, the state is today's, and the last thing read is
// the thing acted on. status.Report carries its own 3-second budget, so a
// slow repository costs the session a note, never a wait.
func hookText(root string) string {
	text, _ := Effective(root)
	var b strings.Builder
	b.WriteString(strings.TrimRight(text, "\n"))
	b.WriteString("\n\n" + status.Header + "\n")
	for _, line := range status.Report(root) {
		b.WriteString(line + "\n")
	}
	return b.String()
}

// RunHook prints the principles in the shape the running host's
// SessionStart hook expects: Claude Code reads raw stdout; Codex, Copilot,
// and Qoder each want a JSON envelope. One hooks file serves them all.
func RunHook(root string, out func(string)) int {
	// The version check runs alongside the payload, never in front of it:
	// the hook's stdout is parsed as JSON by three of the four hosts, so the
	// warning goes to stderr (R-07), and a slow or absent GitHub cannot hold
	// a session start open (N-02, N-03).
	done := versionWarning(root)
	text := hookText(root)
	switch h := host.Detect(); h {
	case host.Claude:
		out(strings.TrimRight(text, "\n"))
	case host.Copilot:
		enc, _ := json.Marshal(map[string]string{"additionalContext": text})
		out(string(enc))
	default: // codex, qoder: hookSpecificOutput envelope; codex adds systemMessage
		payload := map[string]any{"hookSpecificOutput": map[string]string{
			"hookEventName": "SessionStart", "additionalContext": text}}
		if h == host.Codex {
			payload["systemMessage"] = "procoder principles active"
		}
		enc, _ := json.Marshal(payload)
		out(string(enc))
	}
	done()
	return 0
}

// versionWarning starts the check and returns the function that reports it.
// Splitting the two is what keeps the check off the session's critical path:
// GitHub is asked while the principles text is being assembled, and the
// answer — if it arrived — is printed after the payload is out.
//
// A check that did not answer says nothing at all here. This is the one
// place where silence is right: a session start is not the moment to explain
// a network failure, and `procoder version --check` says it plainly when
// somebody asks on purpose.
func versionWarning(root string) func() {
	if config.Load(root).VersionCheckOff {
		return func() {}
	}
	ch := make(chan string, 1)
	go func() {
		latest, warn, err := releases.Check(Version, releases.Timeout)
		if err != nil || !warn {
			ch <- ""
			return
		}
		ch <- releases.WarningLine(Version, latest)
	}()
	return func() {
		select {
		case line := <-ch:
			if line != "" {
				fmt.Fprintln(Stderr, line)
			}
		case <-time.After(releases.Timeout):
			// The goroutine outlives this call and writes to a buffered
			// channel, so nothing leaks and nothing blocks.
		}
	}
}

// Stderr is where a version warning goes: this package writes it, so this
// package owns the sink. The hook's stdout is a JSON payload to three of
// the four hosts, and a friendly line in the middle of it is a corrupted
// payload.
var Stderr io.Writer = os.Stderr

// Version is the running binary's version, set by main at startup. The
// principles hook needs it to say what is newer, and importing main is not
// a thing a package can do.
var Version = releases.Dev

// Run prints the effective principles and where they came from.
func Run(root string, out func(string)) int {
	text, overridden := Effective(root)
	if overridden {
		out("== engineering principles (repo override: " + File + ")")
	} else {
		out("== engineering principles (procoder default — override with " + File + ")")
	}
	out(strings.TrimRight(text, "\n"))
	return 0
}
