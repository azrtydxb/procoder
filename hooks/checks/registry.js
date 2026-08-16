#!/usr/bin/env node
// procoder — extension → pack, and pack → preferred external tool.
//
// The tool entries describe how to INVOKE and PARSE each linter. Whether one is
// actually configured in the project is resolve.js's job.

const path = require('path');
const { finding } = require('./finding');

const ts = require('./lang/ts');
const py = require('./lang/py');
const go = require('./lang/go');
const rust = require('./lang/rust');
const jvm = require('./lang/jvm');
const dotnet = require('./lang/dotnet');

const PACKS = [ts, py, go, rust, jvm, dotnet];

// External findings land on rung TRUE: a configured linter's rules are the
// project's own definition of correct, and procoder defers to them.
function externalFinding(line, message, tool) {
  return finding({
    rung: 'TRUE', id: `true/${tool}`, line,
    message: String(message).slice(0, 120),
    fix: `resolve the ${tool} finding`,
  });
}

const TOOLS = {
  py: {
    name: 'ruff',
    configFiles: ['ruff.toml', '.ruff.toml', 'pyproject.toml', 'setup.cfg'],
    argv: (file) => ['check', '--output-format', 'json', '--force-exclude', file],
    parse: (stdout) => {
      try {
        return JSON.parse(stdout).map((item) =>
          externalFinding(item.location && item.location.row, `${item.code}: ${item.message}`, 'ruff'));
      } catch (e) {
        return [];
      }
    },
  },
  ts: {
    name: 'eslint',
    configFiles: ['eslint.config.js', 'eslint.config.mjs', '.eslintrc', '.eslintrc.json', '.eslintrc.cjs', '.eslintrc.js'],
    argv: (file) => ['--format', 'json', file],
    parse: (stdout) => {
      try {
        const results = JSON.parse(stdout);
        return results.flatMap((result) => (result.messages || []).map((m) =>
          externalFinding(m.line, `${m.ruleId || 'eslint'}: ${m.message}`, 'eslint')));
      } catch (e) {
        return [];
      }
    },
  },
  go: {
    name: 'golangci-lint',
    configFiles: ['.golangci.yml', '.golangci.yaml', '.golangci.toml'],
    argv: (file) => ['run', '--out-format', 'json', file],
    parse: (stdout) => {
      try {
        return (JSON.parse(stdout).Issues || []).map((issue) =>
          externalFinding(issue.Pos && issue.Pos.Line, `${issue.FromLinter}: ${issue.Text}`, 'golangci-lint'));
      } catch (e) {
        return [];
      }
    },
  },
  rust: {
    name: 'clippy',
    configFiles: ['clippy.toml', '.clippy.toml', 'Cargo.toml'],
    argv: () => ['clippy', '--message-format', 'short', '--quiet'],
    parse: (stdout) => String(stdout).split('\n')
      .map((line) => /^[^:]+:(\d+):\d+:\s*(?:warning|error):\s*(.+)$/.exec(line))
      .filter(Boolean)
      .map((m) => externalFinding(Number(m[1]), m[2], 'clippy')),
  },
};

const EXT_TO_TOOL = new Map();
for (const ext of ts.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.ts);
for (const ext of py.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.py);
for (const ext of go.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.go);
for (const ext of rust.EXTENSIONS) EXT_TO_TOOL.set(ext, TOOLS.rust);
// jvm and dotnet have no fast single-file linter worth a 1.5s budget; their
// built-in packs always run instead.

const EXT_TO_PACK = new Map();
for (const pack of PACKS) {
  for (const ext of pack.EXTENSIONS) EXT_TO_PACK.set(ext, pack);
}

function packFor(relPath) {
  return EXT_TO_PACK.get(path.extname(String(relPath || '')).toLowerCase()) || null;
}

function toolFor(relPath) {
  return EXT_TO_TOOL.get(path.extname(String(relPath || '')).toLowerCase()) || null;
}

module.exports = { PACKS, TOOLS, packFor, toolFor };
