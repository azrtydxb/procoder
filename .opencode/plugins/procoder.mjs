// Server plugin for procoder, shared by OpenCode and Kilo: injects the
// AGENTS.md contract into every turn, registers the command set, and holds
// the commit gate at the tool boundary. Kilo's CLI is an OpenCode fork and
// its plugin API is the same one, so this file ships byte-identical to both
// `.opencode/plugins/procoder.mjs` and `.kilo/plugin/procoder.js` (the two
// hosts scan different extensions); the parity test pins them together.
//
// This shim is the one piece of non-Go glue in the repo — plugins must be JS
// — and it stays thin: all content comes from AGENTS.md and the command
// directory, and every gate decision comes from the binary.
import { readFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { execFile } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const SELF = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(SELF, "..", "..");

// Kilo reads AGENTS.md natively; OpenCode does not, so the contract is
// injected there and only there. Kilo says so itself through KILO in the
// plugin's environment (it sets OPENCODE too — it is a fork — so that one
// answers nothing). The host is asked rather than guessed from this file's
// own path, because Kilo loads BOTH copies of this plugin and the path
// therefore lies for one of them.
const isKilo = Boolean(process.env.KILO || process.env.KILOCODE_VERSION);

// Kilo reads opencode.json as well as scanning its own plugin directory, so
// in a repository carrying both adapters this file registers twice in one
// session. The first registration serves; a second would spawn the gate
// twice per commit and say everything twice.
const registered = Symbol.for("procoder.plugin.registered");

// The directories a skill-aware host should scan in a governed repository:
// the command set — each host names it differently, and the plugin sits one
// level below whichever it is — and `skills/`, where the canonical procoder
// SKILL.md lives, so a repository that has procoder needs no per-host copy
// of it.
function skillPaths() {
  const dirs = [];
  for (const name of ["command", "commands"]) {
    const dir = path.join(SELF, "..", name);
    if (existsSync(dir)) dirs.push(dir);
  }
  const skills = path.join(ROOT, "skills");
  if (existsSync(skills)) dirs.push(skills);
  return dirs;
}

// The gate runs in the binary, not here: `hook pre-tool-use` is the same
// entry point Claude Code and Codex call, so every host gets one verdict
// from one implementation, config (`[git] commit_gate`) included.
function gateVerdict(command, cwd) {
  const payload = JSON.stringify({
    tool_name: "Bash",
    cwd,
    tool_input: { command },
  });
  return new Promise((resolve) => {
    const child = execFile(
      "procoder",
      ["hook", "pre-tool-use"],
      { timeout: 180_000 },
      (err, stdout) => {
        const text = (stdout || "").trim();
        if (!text) {
          // A clean gate prints nothing and says nothing. A gate that could
          // not run has NOT judged this commit, and says so rather than
          // blocking every commit on a machine without procoder — a broken
          // session teaches people to uninstall the plugin.
          return resolve(
            err
              ? {
                  decision: "unavailable",
                  reason:
                    "procoder gate did NOT run, so this commit is unchecked: " +
                    err.message,
                }
              : { decision: "allow" },
          );
        }
        try {
          const parsed = JSON.parse(text);
          const out = parsed.hookSpecificOutput ?? parsed;
          resolve({
            decision: out.permissionDecision,
            reason: out.permissionDecisionReason,
          });
        } catch {
          resolve({
            decision: "unavailable",
            reason:
              "procoder gate printed no verdict — run `procoder check` directly",
          });
        }
      },
    );
    child.stdin.end(payload);
  });
}

export const ProcoderPlugin = async ({ client, directory } = {}) => {
  if (globalThis[registered]) return {};
  globalThis[registered] = true;
  const log = async (level, message) => {
    try {
      await client?.app?.log({ body: { service: "procoder", level, message } });
    } catch {
      // logging is never worth failing a turn over
    }
  };
  return {
    config: async (config) => {
      // Both hosts discover their own command directory; this hook adds it,
      // and the repository's skills/, to what a skill-aware host scans
      const dirs = skillPaths();
      if (dirs.length === 0) return;
      config.skills ??= {};
      config.skills.paths ??= [];
      for (const dir of dirs) {
        if (!config.skills.paths.includes(dir)) config.skills.paths.push(dir);
      }
    },
    "experimental.chat.system.transform": async (_input, output) => {
      if (isKilo) return; // read natively there — injecting it twice is waste
      try {
        const agents = await readFile(path.join(ROOT, "AGENTS.md"), "utf8");
        output.system.push(agents);
      } catch {
        // a missing AGENTS.md degrades to no injection, never to a crash
      }
    },
    "tool.execute.before": async (input, output) => {
      if (input.tool !== "bash") return;
      const command = output?.args?.command;
      // every `git commit` carries the word; anything else never reaches the
      // binary, so the gate costs nothing on ordinary shell calls
      if (typeof command !== "string" || !command.includes("commit")) return;
      const { decision, reason } = await gateVerdict(
        command,
        directory ?? ROOT,
      );
      if (decision === "deny") {
        throw new Error(
          reason || "procoder gate: blocking findings — run `procoder check`",
        );
      }
      if (reason)
        await log(decision === "unavailable" ? "warn" : "info", reason);
    },
  };
};

export default ProcoderPlugin;
