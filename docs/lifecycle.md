# When each check runs

**A reference.** Every check Procoder runs, placed at the lifecycle event
that fires it. [The ten domains](domains.md) lists the checks by subject;
this page is the other cut — by moment. For why the chain refuses at all,
see [the quality chain](quality-chain.md).

Nothing here waits to be asked. Each check fires on an event, whether or
not anyone remembered it existed — which is the whole point, because a
check you have to know about protects only the people who already know
about it.

Two tiers run the same rules at different scopes: **the gate answers
about the change, CI answers about the tree.** A commit is judged on what
it carries; the repository is judged on all of it.

<ul class="procoder-legend">
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span> stops the commit, or fails the job</li>
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span> printed; blocks only where the repository asked</li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span> context for a judgement you make</li>
</ul>

<div class="procoder-track">

<section class="procoder-stop">
<div class="procoder-stop__node">1</div>
<div class="procoder-stop__card">
<div class="procoder-stop__head">
<h3>Session starts</h3>
<code class="procoder-stop__trigger">SessionStart · 15s</code>
</div>
<p class="procoder-stop__why">The agent is handed the rules and the state of the repository before it writes anything — computed fresh, never carried over from a previous session.</p>
<ul class="procoder-checks">
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>engineering principles</b><i>The ladder, the delegation rules, the formatting contract.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>state of play</b><i>Branch against the default, dirty count, active sprint, open stories, unlearned lessons, index freshness — inside a hard three-second budget.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>deferred suites</b><i>Names the test suites the gate will not run here, so a green gate is never mistaken for a suite that passed.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>version check</b><i>Asks GitHub off the critical path — a slow network cannot hold a session open.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>receipt check</b><i>The payload opens with an instruction and closes with an end marker. A host that inlines only the first 2KB and writes the rest to a file leaves a reader holding a preview — the marker is how it can tell, and the instruction says to go and read the remainder.</i></span></li>
</ul>
</div>
</section>

<section class="procoder-stop">
<div class="procoder-stop__node">2</div>
<div class="procoder-stop__card">
<div class="procoder-stop__head">
<h3>Every file written</h3>
<code class="procoder-stop__trigger">PostToolUse · Write|Edit · 60s</code>
</div>
<p class="procoder-stop__why">The fastest feedback in the chain, on the one file that just changed. Everything here is milliseconds, because it runs on every edit.</p>
<ul class="procoder-checks">
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span><span><b>format verdict</b><i>Clean, unformatted, UNCHECKED, or out of scope — for that file.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span><span><b>secret scan</b><i>A flagged value is never echoed — not to the terminal, not into a question.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span><span><b>linter</b><i>The language's canonical linter, over just this file.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>code index</b><i>Kept current, but only where an index already exists.</i></span></li>
</ul>
</div>
</section>

<section class="procoder-stop procoder-stop--wide">
<div class="procoder-stop__node">3</div>
<div class="procoder-stop__card">
<div class="procoder-stop__head">
<h3>Before the commit — the gate</h3>
<code class="procoder-stop__trigger">PreToolUse · Bash · 120s</code>
</div>
<p class="procoder-stop__why">The big one. Eighteen legs over the changed set, intercepting <code>git commit</code> before it runs — and installable as a real pre-commit hook, so it holds outside the agent too. A file the gate could not look at is not a passing file: UNCHECKED fails it exactly like unformatted does.</p>
<ul class="procoder-checks">
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>secrets</b><i>In a changed file. Always blocking, never configurable.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>SAST</b><i>semgrep over the files carried, at the severity <code>[security] sast_blocks_at</code> sets.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>known vulnerabilities</b><i>Only when the commit touches a dependency manifest.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>agents drift</b><i>A host rule file out of step with AGENTS.md is another agent reading rules this repository dropped.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>documentation obligation</b><i>A new exported symbol changes a doc, or the message carries <code>docs: none — reason</code>.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>test suite</b><i>Narrowed to the changed packages. A failure blocks under <code>[test] policy</code>; a suite that could not run blocks either way.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>emptied documentation</b><i>A Markdown file reduced to nothing, with the command to restore it.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>config that could not apply</b><i>An unreadable config.toml never silently falls back to defaults.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>conflict markers, junk, oversized files</b><i>And commits landing straight on the default branch.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>AI attribution in the message</b><i>Commits are the author's alone.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span><span><b>format verdicts</b><i>Over every changed file. Unformatted fails the gate.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span><span><b>linters</b><i>Blocking where the repository sets <code>[lint] policy = "block"</code>.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span><span><b>complexity</b><i>Named as the function is written. Blocks under <code>[maintain] policy</code>.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span><span><b>debt with no revisit condition</b><i>Asked while the reason for the shortcut is still in your head.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span><span><b>workflow and infra hygiene</b><i>Pinned actions, timeouts, concurrency — across all workflows, not only changed ones.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--reports">Reports</span><span><b>loosened defaults</b><i>Every setting weakened from its default prints on every run. Strengthening one is silent.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>blast radius</b><i>What else the changed symbols reach, from the code index.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>open questions and captured leaks</b><i>Review findings recorded and never closed; questions waiting on a person.</i></span></li>
</ul>
</div>
</section>

<section class="procoder-stop">
<div class="procoder-stop__node">4</div>
<div class="procoder-stop__card">
<div class="procoder-stop__head">
<h3>Session ends or compacts</h3>
<code class="procoder-stop__trigger">Stop · PreCompact · 10s</code>
</div>
<p class="procoder-stop__why">The handoff. What the next session inherits is written down rather than reconstructed from scrollback.</p>
<ul class="procoder-checks">
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>handoff note</b><i>Written to state. A note that cannot be written is a lost note, never a broken session.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--block">Blocks</span><span><b>an unasked decision</b><i>A turn ending with a decision put to the user in prose, and none recorded in <code>.procoder/ask/decisions.md</code>, does not end. The hook reads <code>last_assistant_message</code> from the host and exits 2, which continues the conversation.</i></span></li>
</ul>
<p class="procoder-stop__why">The detector is deliberately conservative: an explicit ask, never a bare question mark, and an interrogative phrase only counts with a question mark in the same sentence — otherwise narration about a decision already taken reads as a new one. The same message never blocks twice, and the handoff note is written first, on every path including the blocking one.</p>
</div>
</section>

<section class="procoder-stop procoder-stop--wide">
<div class="procoder-stop__node">5</div>
<div class="procoder-stop__card">
<div class="procoder-stop__head">
<h3>Push and pull request</h3>
<code class="procoder-stop__trigger">CI · test + gate jobs</code>
</div>
<p class="procoder-stop__why">Where the tree gets answered for. The gate saw only what the commit carried; these run over everything, on three operating systems.</p>
<ul class="procoder-checks">
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>the suite — ubuntu, macOS, windows</b><i>The full run on all three, every push.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>committed binaries rebuild</b><i>Rebuilt reproducibly and compared by hash. A shipped binary nobody can reproduce is one nobody can check.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>gofmt, build, launcher</b><i>Including that the launcher resolves a binary in the shell each OS really uses.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>the gate over the whole tracked tree</b><i><code>git ls-files | xargs procoder check</code> — every file, not just the diff.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>security, the deep pass</b><i>Full SAST and dependency scan across the repository.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>documentation, external links included</b><i>The check that needs the network, kept off the commit path.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>maintain, debt, deps</b><i>The whole-tree pass: complexity everywhere, the full debt ledger, dependency freshness.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>doctor</b><i>Every tool present and answering, so no check above can pass by being absent.</i></span></li>
</ul>
</div>
</section>

<section class="procoder-stop">
<div class="procoder-stop__node">6</div>
<div class="procoder-stop__card">
<div class="procoder-stop__head">
<h3>Merged to main</h3>
<code class="procoder-stop__trigger">CI · docs job</code>
</div>
<p class="procoder-stop__why">Publication. Branch protection requires linear history and every check green on the tip, so nothing arrives here unverified.</p>
<ul class="procoder-checks">
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>mkdocs --strict</b><i>A broken reference fails the build rather than shipping a dead page.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>the changelog, served twice</b><i>One file copied into the site, so the repository and the docs cannot disagree.</i></span></li>
</ul>
</div>
</section>

<section class="procoder-stop">
<div class="procoder-stop__node">7</div>
<div class="procoder-stop__card">
<div class="procoder-stop__head">
<h3>Tagged <code>v*</code></h3>
<code class="procoder-stop__trigger">CI · release job</code>
</div>
<p class="procoder-stop__why">A tag is the release. The job runs only after the suite and the gate pass on the tagged tree — what people download is published on the same evidence as everything else.</p>
<ul class="procoder-checks">
<li><span class="procoder-verdict procoder-verdict--blocks">Blocks</span><span><b>suite and gate on the tagged tree</b><i>Re-verified at the tag, not inherited from the branch.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>the changelog entry becomes the notes</b><i>Extracted verbatim — what is written there is what a person downloading reads.</i></span></li>
<li><span class="procoder-verdict procoder-verdict--info">Info</span><span><b>five platform binaries and SHA256SUMS</b><i>The manifest <code>procoder self-upgrade</code> verifies a download against.</i></span></li>
</ul>
</div>
</section>

</div>

## Off the arrow: the controllers that refuse

These are not on the timeline, because they do not fire at a fixed
moment. They fire whenever you try to call something finished — and each
one refuses, naming what is missing.

| Controller                                       | What it refuses                                                                                                                                                                                              |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `procoder spec`, `procoder plan`                 | Hollow documents. A section left as its template comment is not a filled section.                                                                                                                            |
| `procoder backlog close story`                   | A story with no real description, unchecked criteria, or no recorded evidence — then it runs the gate and the suite before agreeing.                                                                         |
| `procoder backlog close epic`, `close milestone` | Closing while any child is still open.                                                                                                                                                                       |
| `procoder sprint close`                          | Closing unless every story is done or explicitly carried, with a reason.                                                                                                                                     |
| `procoder sprint start`                          | Starting until the last sprint's retro is written. The learning loop is enforced, not encouraged.                                                                                                            |
| `procoder adr check`                             | Hollow records, unknown statuses, and supersede references pointing at nothing.                                                                                                                              |
| `procoder release <version>`                     | Tagging until the version matches across every file, the changelog entry exists, the tree is clean, the gate is clean and the suite is green. It then prints the `git tag` command — it tags nothing itself. |
| `procoder audit`                                 | Nothing — it is the onboarding sweep, every domain's checks over a tree Procoder has not governed before.                                                                                                    |

## The three rules underneath

**No silent green.** A check that could not run never reads as one that
passed. A missing tool, an unparseable config, a suite that never
started — each says so, and blocks. The failure this exists to prevent is
the comfortable default: nothing found, therefore nothing wrong.

**No budget on the heavy checks.** A slow scan finishes and reports what
it found, rather than being cut off and reported anyway. A verdict that
depends on how fast your laptop is, is not a verdict about your code. The
ceiling that remains is a hung-process net: when it fires, it says the
check was NOT run.

**The binary prints, the agent writes.** Nothing here edits your files.
Every check computes and reports; acting on the result is the author's
move. That is what makes the whole chain safe to run on every event.
