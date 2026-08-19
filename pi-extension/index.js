// pi extension for procoder: injects the AGENTS.md contract at agent
// start. Thin by design — content comes from the canonical AGENTS.md,
// which the portability checks pin.
const fs = require("node:fs");
const path = require("node:path");

const AGENTS = path.join(__dirname, "..", "AGENTS.md");

module.exports = function register(pi) {
  pi.on?.("before_agent_start", (event) => {
    try {
      const body = fs.readFileSync(AGENTS, "utf8");
      return { systemPrompt: `${event.systemPrompt}\n\n${body}` };
    } catch {
      return undefined; // no AGENTS.md → inject nothing, never crash
    }
  });
};
