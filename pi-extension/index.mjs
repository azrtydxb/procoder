// pi extension for procoder: injects the AGENTS.md contract at agent
// start. Thin by design — content comes from the canonical AGENTS.md,
// which the portability checks pin.
//
// ESM, and named .mjs, because pi validates the export shape at install
// time and rejects a CommonJS `module.exports` outright (#105). Node's
// interop would have handed a CJS export back as `default` on import, so
// nothing here failed locally — the host's own validator was the only
// thing that ever saw the difference. The extension contract is an ESM
// default factory; this file is one, and the portability test reads the
// source rather than the loaded module so the two can never diverge again.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const AGENTS = join(dirname(fileURLToPath(import.meta.url)), "..", "AGENTS.md");

export default function register(pi) {
  pi.on?.("before_agent_start", (event) => {
    try {
      const body = readFileSync(AGENTS, "utf8");
      return { systemPrompt: `${event.systemPrompt}\n\n${body}` };
    } catch {
      return undefined; // no AGENTS.md → inject nothing, never crash
    }
  });
}
