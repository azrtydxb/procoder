// OpenCode server plugin for procoder: injects the AGENTS.md contract
// into every turn and registers the command set. This shim is the one
// piece of non-Go glue in the repo — OpenCode plugins must be JS — and it
// stays thin: all content comes from AGENTS.md and .opencode/command/,
// which are pinned to the canonical sources by the portability checks.
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
);

export const ProcoderPlugin = async () => ({
  config: async (config) => {
    // OpenCode discovers .opencode/command/*.md itself; this hook only
    // adds the commands dir to the skills path for skill-aware hosts
    config.skills ??= {};
    config.skills.paths ??= [];
    const commands = path.join(ROOT, ".opencode", "command");
    if (!config.skills.paths.includes(commands)) {
      config.skills.paths.push(commands);
    }
  },
  "experimental.chat.system.transform": async (_input, output) => {
    try {
      const agents = await readFile(path.join(ROOT, "AGENTS.md"), "utf8");
      output.system.push(agents);
    } catch {
      // a missing AGENTS.md degrades to no injection, never to a crash
    }
  },
});

export default ProcoderPlugin;
