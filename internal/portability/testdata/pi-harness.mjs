// Test harness for the pi adapter, driven from Go (see pi_test.go).
//
// pi is not installed on the machine that runs `go test`, and an adapter that
// can only be exercised inside a live session is an adapter nobody tests. So
// the harness imports the real adapter and hands it the object pi would hand
// it: the same registration calls, the same event handlers, a ctx whose UI
// records instead of painting. What is fake is the host, never the code under
// test — the adapter still reads the real commands/, still resolves its own
// launcher path, still spawns the real launcher and whatever PROCODER_BIN names.
//
// Usage: node pi-harness.mjs <mode> <json-config>
// The repo root arrives in PROCODER_REPO so the harness does not guess where it
// lives relative to a testdata directory that moves.
import { pathToFileURL } from "node:url";
import { join } from "node:path";

const mode = process.argv[2] ?? "";
const config = JSON.parse(process.argv[3] || "{}") || {};
const root = process.env.PROCODER_REPO;
if (!root) {
  console.error("PROCODER_REPO is not set");
  process.exit(2);
}
const adapter = await import(pathToFileURL(join(root, "pi-extension", "index.mjs")).href);

const state = {
  handlers: {},
  commands: {},
  tools: {},
  sent: [],
  notices: [],
  branch: config.branch ?? [],
};

const pi = {
  on(event, handler) {
    (state.handlers[event] ||= []).push(handler);
  },
  registerCommand(name, options) {
    state.commands[name] = {
      name,
      description: options.description ?? "",
      completions: options.getArgumentCompletions ? options.getArgumentCompletions("") : null,
    };
  },
  registerTool(definition) {
    state.tools[definition.name] = definition;
  },
  sendUserMessage(text, options) {
    state.sent.push({ text, options: options ?? null });
  },
};

const ctx = {
  cwd: config.cwd ?? root,
  // A UI-shaped host by default, because that is the only shape in which a
  // notice is observable at all. Print mode's no-op notifications are pi's own
  // behaviour and are not this adapter's to assert.
  hasUI: config.hasUI ?? true,
  ui: {
    notify(message, level) {
      state.notices.push({ message, level });
    },
  },
  sessionManager: {
    getBranch: () => state.branch,
  },
};

function emit(value) {
  console.log(JSON.stringify(value ?? null));
}

async function fire(event, ...args) {
  const handlers = state.handlers[event] ?? [];
  const results = [];
  for (const handler of handlers) results.push(await handler(...args));
  return results.filter((r) => r !== undefined);
}

adapter.default(pi);

const AGENT_MARKER = "The trailer your host adds";

switch (mode) {
  case "registry": {
    const tool = state.tools.procoder;
    emit({
      commands: Object.values(state.commands).sort((a, b) => a.name.localeCompare(b.name)),
      tool: tool
        ? {
            name: tool.name,
            subcommands: tool.parameters?.properties?.command?.enum ?? [],
            required: tool.parameters?.required ?? [],
          }
        : null,
      events: Object.keys(state.handlers).sort(),
    });
    break;
  }

  case "launcher": {
    const { launcherPath, commandText } = adapter;
    emit({
      posix: launcherPath("darwin", "/opt/pkg/pi-extension"),
      win32: launcherPath("win32", "C:\\Users\\me\\Documents\\pkg\\pi-extension"),
      text: commandText(
        'The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"\n\nRun:\n\n    "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh" check $ARGUMENTS\n\nfile\'s formatted result with `launcher.sh format <file>`',
        '"/opt/pkg/hooks/launcher.sh"',
      ),
    });
    break;
  }

  case "inject": {
    await fire("session_start", { reason: config.reason ?? "startup" });
    const event = {
      systemPrompt: "base prompt",
      systemPromptOptions: {
        cwd: config.cwd ?? root,
        contextFiles: config.contextFiles ?? [],
      },
    };
    const first = await fire("before_agent_start", event, ctx);
    const second = await fire("before_agent_start", event, ctx);
    const text = (first[0]?.message?.content ?? "") + (first[0]?.systemPrompt ?? "");
    emit({
      first: first.length,
      second: second.length,
      markerInMessage: (text.match(new RegExp(AGENT_MARKER, "g")) || []).length,
      principles: text.includes("Build like a senior developer"),
      notices: state.notices,
    });
    break;
  }

  case "gate": {
    const results = await fire(
      "tool_call",
      { toolName: config.toolName ?? "bash", input: { command: config.command } },
      ctx,
    );
    emit({ results, notices: state.notices });
    break;
  }

  case "write": {
    const results = await fire(
      "tool_result",
      { toolName: config.toolName ?? "write", input: { path: config.path }, content: config.content ?? [] },
      ctx,
    );
    emit({ results, notices: state.notices });
    break;
  }

  case "tool": {
    const tool = state.tools.procoder;
    if (!tool) {
      emit({ error: "no procoder tool registered" });
      break;
    }
    let result;
    let threw = "";
    try {
      result = await tool.execute("tc-1", config.params ?? {}, undefined, undefined, ctx);
    } catch (e) {
      threw = String(e?.message ?? e);
    }
    emit({
      threw,
      isError: result?.isError,
      text: result?.content?.map((c) => c.text).join("\n") ?? "",
      details: result?.details,
    });
    break;
  }

  case "stop": {
    await fire("agent_settled", {}, ctx);
    emit({ sent: state.sent, notices: state.notices });
    break;
  }

  default:
    console.error(`unknown mode ${mode}`);
    process.exit(2);
}
