// pi extension for procoder: the four enforcement points every other host
// gets, plus the command set and the gate as a callable tool.
//
// Thin in the way that matters and thick in the way that does not. Every
// verdict — whether a file is formatted, whether a commit may proceed, whether
// a decision went unasked — comes from the same `procoder hook` entry points
// Claude Code calls, so one implementation judges every host. What lives here
// is wiring: which pi event calls which process, and how the answer is shaped
// for pi rather than for Claude.
//
// Two rules held throughout, both learned the expensive way elsewhere in this
// repository:
//
//   - Nothing is decided twice. `format.Check` decides formatting and `hook
//     stop` decides the unasked-decision block, remembering which message it
//     last refused. Re-implementing either here would put two answers in play.
//
//   - A hook that cannot run does not break the session. Every spawn below
//     degrades to a warning, because a coder who cannot start a session
//     uninstalls the thing that could not start it, and procoder would then be
//     the tool that broke editing in order to check it.
//
// ESM, and named .mjs, because pi validates the export shape at install time
// and rejects a CommonJS `module.exports` outright (#105). Node's interop would
// have handed a CJS export back as `default` on import, so nothing here failed
// locally — the host's own validator was the only thing that ever saw the
// difference. The portability tests read this file as text for that reason.
import { spawn } from "node:child_process";
import { existsSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { hostCommandText } from "../hooks/host-command-text.mjs";

const SELF = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(SELF, "..");
const AGENTS = join(ROOT, "AGENTS.md");
const COMMANDS = join(ROOT, "commands");

// update.md drives the Claude plugin's self-update flow. A pi package is pinned
// by its install ref and moves with `pi install git:…@vX`, so a command that
// self-updated here would move something else entirely. The same skip the
// OpenCode and Kilo command sets already carry.
const UNSHIPPED = new Set(["update.md"]);

// The reporting half of procoder, callable by the model. Everything in this
// list reads: none of it closes work, tags a release, or rewrites a file. The
// mutators stay slash commands a human types, which is the line `procoder run`
// already draws for launch commands — an agent session could have written the
// sentence that asks for them.
const REPORTING_COMMANDS = [
  "check",
  "test",
  "lint",
  "security",
  "debt",
  "status",
  "review",
  "doctor",
  "index",
];

// pi's own guidance for tool output: 2,000 lines and 50KB before a result
// starts costing more context than it is worth.
const MAX_LINES = 2000;
const MAX_BYTES = 50 * 1024;

// The cheap half of the commit question: does this command say `commit` as a
// word of its own? The binary decides whether it is really a commit; this only
// keeps an ordinary shell call from paying for a process spawn. `git log
// --abbrev-commit` is in every session and is not a commit by any reading.
const isCommitish = /(^|\s)commit(\s|$)/;

/**
 * Where this installation's launcher lives, as a pure function of the platform
 * and the directory the extension was loaded from. Exported so a test can ask
 * the question directly instead of inferring it from what got run.
 */
export function launcherPath(platform, self) {
  const name = platform === "win32" ? "launcher.cmd" : "launcher.sh";
  return join(self, "..", "hooks", name);
}

// Never PATH, and never a bare `procoder`. The launcher resolves the binary
// this package shipped with — and fetches it, verified, on a first run that has
// not got it. A procoder found on PATH is whatever version somebody left
// there: the machine this adapter was written on keeps a 1.0.2 while the
// package is 3.4.0, and a gate running the wrong binary reports a clean tree it
// has never seen.
const LAUNCHER = launcherPath(process.platform, SELF);

/**
 * The command text a host runs, given the Claude Code source and the string
 * that stands for procoder there. Exported for the same reason as launcherPath.
 */
export function commandText(body, launcher) {
  return hostCommandText(body, launcher);
}

// The subcommand words a command file names, offered as its argument
// completion: the plugin-root form, the PATH form, and prose mentions alike.
function argumentWords(body) {
  const words = new Set();
  for (const m of body.matchAll(/launcher\.sh"?\s+([a-z][a-z-]+)/g)) {
    words.add(m[1]);
  }
  for (const m of body.matchAll(/`procoder ([a-z][a-z-]+)/g)) {
    words.add(m[1]);
  }
  return [...words].sort();
}

function readIf(path) {
  try {
    return readFileSync(path, "utf8");
  } catch {
    return ""; // no AGENTS.md is a repository without one, not a crash
  }
}

// The frontmatter description, which is what pi lists in its command menu.
function descriptionOf(body) {
  const m = /^description:[ \t]*(.+)$/m.exec(body);
  return m ? m[1].trim() : "";
}

/**
 * Run the launcher with these arguments, optionally feeding it a hook payload.
 *
 * Resolves never: a missing binary, a refused fetch, and a child that dies on
 * the way out all resolve with an exit code and whatever it managed to print.
 * The callers below turn that into a warning, and each of them prefers "did not
 * judge" over "blocked the session".
 */
function run(args, stdin, timeout) {
  return new Promise((resolveRun) => {
    const done = (value) =>
      resolveRun({ stdout: "", stderr: "", code: 1, failed: false, ...value });
    if (!existsSync(LAUNCHER)) {
      return done({ stderr: `no launcher at ${LAUNCHER}`, failed: true });
    }
    // Windows runs a .cmd through cmd.exe rather than through one shell
    // string, because the arguments below can carry text the model typed. A
    // concatenated command string hands that text to a shell to interpret with
    // the current environment attached; cmd.exe with the launcher as the
    // command and each argument as its own argv entry does not.
    // debt: cmd.exe still applies its own metacharacter rules to the trailing
    // arguments, and nothing in this file is executed on Windows because no CI
    // runs one. Ceiling: an argument containing cmd metacharacters on a Windows
    // install. Revisit when one reports in, or when the tool takes free text
    // rather than a subcommand and a path.
    const child =
      process.platform === "win32"
        ? spawn("cmd.exe", ["/d", "/s", "/c", LAUNCHER, ...args], {
            windowsHide: true,
          })
        : spawn(LAUNCHER, args, { windowsHide: true });

    // A chatty child is bounded in bytes, counted where they are actually
    // spent. The ceiling is the one the OpenCode adapter already uses, and for
    // the same reason: a deny verdict that arrives truncated reads back as no
    // verdict at all, which lets the commit through.
    const CAP = 32 * 1024 * 1024;
    const out = [];
    const err = [];
    let outBytes = 0;
    let errBytes = 0;
    child.stdout.on("data", (chunk) => {
      if (outBytes >= CAP) return;
      outBytes += chunk.length;
      out.push(chunk);
    });
    child.stderr.on("data", (chunk) => {
      if (errBytes >= CAP) return;
      errBytes += chunk.length;
      err.push(chunk);
    });

    const timer = setTimeout(() => {
      child.kill("SIGKILL");
      done({
        stdout: buf(out),
        stderr: `timed out after ${timeout}ms`,
        killed: true,
      });
    }, timeout);

    child.on("error", (e) => {
      clearTimeout(timer);
      done({ stderr: e.message, failed: true });
    });
    child.on("close", (code) => {
      clearTimeout(timer);
      // 126 and 127 are "could not run": the launcher exec'd a binary that is
      // not there or cannot be exec'd. Anything else that exited non-zero did
      // run and may be saying no — a gate exiting 1 over an unformatted tree is
      // the report working, not the tool broken.
      const exit = code ?? 1;
      done({
        stdout: buf(out),
        stderr: buf(err),
        code: exit,
        failed: exit === 126 || exit === 127,
      });
    });
    if (stdin !== undefined) {
      // The child can exit before the payload is flushed — a procoder that is
      // not there, or one that answers and leaves. An unhandled 'error' on this
      // stream takes the whole process down with an EPIPE, which is a very
      // expensive way to learn the gate could not run.
      child.stdin.on("error", () => {});
      child.stdin.end(stdin);
    }
  });
}

// Joining captured chunks, which arrive as Buffer or string depending on how
// the child was spawned and what it printed.
function buf(chunks) {
  return chunks
    .map((c) => (typeof c === "string" ? c : c.toString("utf8")))
    .join("");
}

// The hosts share one envelope; Copilot takes the flat object. Reading both
// keeps this adapter working whichever shape the running binary emits.
function envelope(text) {
  const trimmed = String(text || "").trim();
  if (!trimmed) return null;
  try {
    const parsed = JSON.parse(trimmed);
    return parsed.hookSpecificOutput ?? parsed;
  } catch {
    return null;
  }
}

// pi reports the path as the model wrote it; the binary wants the file. A
// leading @ is Copilot's habit leaking into a pi session — pi strips it from
// its own tools, and a relative path here would check the wrong tree.
function absolute(path, cwd) {
  if (typeof path !== "string" || path === "") return "";
  return resolve(cwd || ROOT, path.startsWith("@") ? path.slice(1) : path);
}

// pi's truncation contract: keep the head, spare the context window, and never
// leave the model holding a partial list it believes is complete.
function truncate(text) {
  const lines = text.split("\n");
  const bytes = Buffer.byteLength(text, "utf8");
  if (lines.length <= MAX_LINES && bytes <= MAX_BYTES) {
    return { text, truncated: false, fullPath: "" };
  }
  const kept = [];
  let used = 0;
  for (const line of lines) {
    const size = Buffer.byteLength(line, "utf8") + (kept.length ? 1 : 0);
    if (kept.length >= MAX_LINES || used + size > MAX_BYTES) break;
    kept.push(line);
    used += size;
  }
  let fullPath = "";
  try {
    fullPath = join(tmpdir(), `procoder-${Date.now()}.txt`);
    writeFileSync(fullPath, text, "utf8");
  } catch {
    // The note below still says what was dropped, which is the part the model
    // cannot recover on its own.
  }
  const note = `\n\n[procoder: output truncated, ${kept.length} of ${lines.length} lines (${bytes} bytes).${
    fullPath
      ? ` Full output: ${fullPath}`
      : " Full output could not be written."
  }]`;
  return { text: kept.join("\n") + note, truncated: true, fullPath };
}

export default function register(pi) {
  // ---- the contract, once per session --------------------------------------
  //
  // pi loads AGENTS.md as a context file, so the contract is already in the
  // system prompt and injecting it again is a 12KB tax on every turn — the
  // defect this adapter shipped with. What pi does NOT load is the engineering
  // principles: `.procoder/PRINCIPLES.md` is procoder's own file, and every
  // other host is sent it.
  //
  // It goes out as one persistent message rather than a system-prompt edit, for
  // the reason the Claude hook sends it once per session instead of once per
  // request: a resumed session already has it above the fold, and #175 measured
  // a working day of those at ~187k tokens of repeated text. A message already
  // in the transcript is also a stable prefix, which a mutated prompt is not.
  let contractSent = false;
  let warnedInjection = false;

  pi.on("session_start", async (event) => {
    // resume and reload keep the same transcript, and /fork duplicates the
    // parent's. All three already contain the message.
    contractSent = ["resume", "reload", "fork"].includes(event?.reason);
    warnedInjection = false;
  });

  pi.on("before_agent_start", async (event, ctx) => {
    if (contractSent) return undefined;
    contractSent = true;

    const options = event?.systemPromptOptions ?? {};
    const loaded = (options.contextFiles ?? [])
      .map((f) => absolute(f.path, options.cwd))
      .filter(Boolean);
    const parts = [];

    const agents = readIf(AGENTS);
    // Judged by what pi actually loaded, not by which host this is: a session
    // started with context files disabled, and a repository whose AGENTS.md is
    // not at this package's root, both need the text, and neither is a host
    // name this could switch on.
    if (agents && !loaded.includes(resolve(AGENTS))) parts.push(agents);

    const principles = await run(["principles"], undefined, 15_000);
    if (principles.code === 0 && principles.stdout.trim() !== "") {
      parts.push(principles.stdout.trimEnd());
    } else if (!warnedInjection) {
      warnedInjection = true;
      notify(
        ctx,
        "procoder principles did NOT reach this session — the binary did not answer",
        "warning",
      );
    }

    if (parts.length === 0) return undefined;
    return {
      message: {
        customType: "procoder",
        content: parts.join("\n\n"),
        display: false,
      },
    };
  });

  // ---- the commit gate -----------------------------------------------------
  pi.on("tool_call", async (event, ctx) => {
    if (event?.toolName !== "bash") return undefined;
    const command = event?.input?.command;
    if (typeof command !== "string" || !isCommitish.test(command)) {
      return undefined;
    }

    const res = await run(
      ["hook", "pre-tool-use"],
      JSON.stringify({
        tool_name: "Bash",
        cwd: ctx?.cwd || ROOT,
        tool_input: { command },
      }),
      180_000,
    );
    const parsed = envelope(res.stdout);
    if (!parsed) {
      // No verdict is not a pass, and it is not a block either. The commit
      // belongs to the coder; a gate that could not judge says so and steps
      // aside rather than wedging a session on a machine that cannot fetch.
      notify(
        ctx,
        "procoder gate did NOT judge this commit — run `procoder check` before you commit",
        "warning",
      );
      return undefined;
    }
    if (parsed.permissionDecision === "deny") {
      return {
        block: true,
        reason:
          parsed.permissionDecisionReason || "procoder gate: blocking findings",
      };
    }
    if (parsed.permissionDecisionReason) {
      notify(ctx, parsed.permissionDecisionReason, "info");
    }
    return undefined;
  });

  // ---- the write hook ------------------------------------------------------
  //
  // pi lets a tool result be patched, which Claude Code does not: the findings
  // land inside the write that caused them rather than in a side-channel
  // `additionalContext` the model has to connect back to a file itself.
  pi.on("tool_result", async (event, ctx) => {
    if (!["write", "edit"].includes(event?.toolName)) return undefined;
    const file = absolute(event?.input?.path, ctx?.cwd);
    if (!file) return undefined;

    const res = await run(
      ["hook", "post-tool-use"],
      JSON.stringify({
        tool_name: toolLabel(event.toolName),
        tool_input: { file_path: file },
      }),
      60_000,
    );
    const context = envelope(res.stdout)?.additionalContext;
    if (!context) return undefined;
    const original = Array.isArray(event.content) ? event.content : [];
    return {
      content: [...original, { type: "text", text: String(context).trimEnd() }],
    };
  });

  // ---- the handoff, and the decision that was not asked --------------------
  //
  // Claude Code gets a Stop hook per turn and a PreCompact one. pi gets both
  // boundaries and one it does not: agent_settled fires when pi will not
  // continue on its own, which is exactly when a turn is over.
  pi.on("agent_settled", async (_event, ctx) => {
    const res = await run(
      ["hook", "stop"],
      JSON.stringify({
        cwd: ctx?.cwd || ROOT,
        last_assistant_message: lastAssistantMessage(ctx),
      }),
      30_000,
    );
    if (res.code !== 2) return undefined;
    // Exit 2 is Claude's "send the reason back and continue". The dedupe that
    // keeps this from looping lives in the binary, on the last-unasked-decision
    // record — not in a variable here, because a per-process guard is forgotten
    // by the next reload, and #242 was exactly that bug.
    const reason = String(res.stderr || "").trim();
    if (reason === "") return undefined;
    if (ctx?.hasUI) {
      // A follow-up is what makes the turn continue: the model reads the refusal
      // and asks the question it was told to ask.
      try {
        pi.sendUserMessage(reason, { deliverAs: "followUp" });
      } catch {
        // A session mid-shutdown can refuse the send. The reason is still worth
        // printing, which is the fallback below.
        process.stderr.write(`procoder: ${reason}\n`);
      }
      return undefined;
    }
    // Print and JSON modes have no UI to notify and, past that, nothing to
    // deliver a follow-up to: the session exits once the agent settles. A live
    // run showed the binary refusing a turn, writing its dedupe record, and the
    // reason vanishing with the process — the question both unasked and, to
    // anyone reading the log afterwards, never asked at all. stderr is the
    // channel that survives, since stdout carries the event stream.
    process.stderr.write(`procoder: ${reason}\n`);
    return undefined;
  });

  pi.on("session_before_compact", async (_event, ctx) => {
    // The note is written before a compaction because a compaction is how a
    // session loses the turns a note remembers. Whatever happens to the
    // summary, the handoff survives. Nothing is cancelled here.
    await run(
      ["hook", "stop"],
      JSON.stringify({ cwd: ctx?.cwd || ROOT }),
      30_000,
    );
    return undefined;
  });

  // ---- the command set -----------------------------------------------------
  // Quoted because it lands inside a sentence the model will paste into a
  // shell, and an installed path carries spaces on the platforms that do that.
  const launcherToken = `"${LAUNCHER}"`;
  if (existsSync(COMMANDS)) {
    for (const entry of readdirSync(COMMANDS).sort()) {
      if (!entry.endsWith(".md") || UNSHIPPED.has(entry)) continue;
      const body = readIf(join(COMMANDS, entry));
      const words = argumentWords(body);
      pi.registerCommand(`procoder:${entry.replace(/\.md$/, "")}`, {
        description: descriptionOf(body),
        getArgumentCompletions: (prefix) => {
          const items = words
            .filter((w) => w.startsWith(prefix || ""))
            .map((w) => ({ value: w, label: w }));
          return items.length > 0 ? items : null;
        },
        handler: async (args) => {
          const text = commandText(body, launcherToken).replaceAll(
            "$ARGUMENTS",
            args || "",
          );
          // expandPromptTemplates defaults to false, so this can never come
          // back around and re-dispatch itself as a command.
          pi.sendUserMessage(text);
        },
      });
    }
  }

  // ---- the gate as a tool --------------------------------------------------
  pi.registerTool({
    name: "procoder",
    label: "procoder",
    description:
      "Run one read-only procoder report over this repository: the gate (check), " +
      "the suite (test), lint, security, the debt ledger, repository status, a " +
      "review, the doctor, or the code index. Anything that changes state — " +
      "closing tasks, seeding the backlog, releasing — is a /procoder: slash " +
      "command a human invokes, not this.",
    promptSnippet:
      "Run a read-only procoder report (check, test, lint, security, debt, status, review, doctor, index)",
    promptGuidelines: [
      "Use the procoder tool to run a read-only procoder report rather than shelling out to a procoder that may be a different version.",
      "The procoder tool refuses anything that mutates state; use the /procoder: slash command to close, seed, or release.",
    ],
    parameters: {
      type: "object",
      properties: {
        command: {
          type: "string",
          enum: REPORTING_COMMANDS,
          description: "Which procoder report to run.",
        },
        args: {
          type: "string",
          description:
            "Arguments for it, as one string. Empty runs that command's default.",
        },
      },
      required: ["command"],
    },
    async execute(_toolCallId, params, _signal, _onUpdate, ctx) {
      const command = String(params?.command || "");
      if (!REPORTING_COMMANDS.includes(command)) {
        // Thrown, not returned: a returned value is never an error to pi, and a
        // refusal the model can read as success is worse than the mutation.
        throw new Error(
          `procoder refuses "${command}" — anything that changes state is a /procoder: slash command, so a human asked for it`,
        );
      }
      const args = [command];
      const extra = String(params?.args || "").trim();
      if (extra) args.push(...extra.split(/\s+/));

      const res = await run(args, undefined, 300_000);
      if (res.killed) {
        return {
          content: [
            {
              type: "text",
              text: `procoder ${command} was killed after 300s and did not finish.`,
            },
          ],
          details: { code: -1 },
          isError: true,
        };
      }
      const body = `${res.stdout ?? ""}${
        res.stderr && res.code !== 0 ? `\n${res.stderr}` : ""
      }`;
      const cut = truncate(
        body.trimEnd() || `(procoder ${command} printed nothing)`,
      );
      return {
        content: [{ type: "text", text: cut.text }],
        details: {
          command,
          code: res.code,
          truncated: cut.truncated,
          fullOutput: cut.fullPath,
        },
        // A non-zero procoder exit is a verdict; only a process that never ran
        // is an error the model should treat as one.
        isError: Boolean(res.failed),
      };
    },
  });
}

function toolLabel(name) {
  return name === "edit" ? "Edit" : "Write";
}

// The last assistant text of the session, read from the session itself. Claude
// Code hands it on the Stop payload precisely because the transcript is
// documented as lagging the current turn; pi's sessionManager IS the transcript
// and is specified as current through the live message at these events.
function lastAssistantMessage(ctx) {
  try {
    const entries = ctx?.sessionManager?.getBranch?.() ?? [];
    for (let i = entries.length - 1; i >= 0; i--) {
      const entry = entries[i];
      if (entry?.type !== "message" || entry.message?.role !== "assistant") {
        continue;
      }
      const content = entry.message.content;
      if (typeof content === "string") return content;
      if (Array.isArray(content)) {
        const text = content
          .filter((c) => c?.type === "text")
          .map((c) => c.text)
          .join("");
        if (text.trim() !== "") return text;
      }
    }
  } catch {
    // A session that cannot be read is a session with no last message: the note
    // is still written, and the unasked-decision check simply has less to work
    // with. It is not a reason to lose the handoff.
  }
  return "";
}

// Notifications are fire-and-forget, and pi has no UI at all in print and JSON
// modes. Both cases are swallowed: failing a turn over a message about a message
// is how a helper becomes the outage.
function notify(ctx, message, level) {
  try {
    if (!ctx?.hasUI || !message) return;
    ctx.ui?.notify?.(message, level);
  } catch {
    // nothing worth doing
  }
}
