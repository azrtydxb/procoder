// procoder — level resolution and shared path helpers.
// Pure: no file writes, no stdout. Safe to require from any hook.

const os = require('os');
const path = require('path');

const LEVELS = ['off', 'pragmatic', 'strict', 'paranoid'];
const DEFAULT_LEVEL = 'strict';

// How strict each level is, for the two places that compare two levels rather
// than just read one: stripping level-gated doctrine blocks, and resolving a
// per-path pin in .procoder.toml against the session level. `off` has no rank —
// it is not a stricter or looser gate, it is no gate — and both callers treat it
// separately.
const LEVEL_RANK = { pragmatic: 1, strict: 2, paranoid: 3 };

function normalizeLevel(value) {
  if (typeof value !== 'string') return null;
  const normalized = value.trim().toLowerCase();
  return LEVELS.includes(normalized) ? normalized : null;
}

function getDefaultLevel() {
  return normalizeLevel(process.env.PROCODER_DEFAULT_LEVEL) || DEFAULT_LEVEL;
}

function getClaudeDir() {
  return process.env.CLAUDE_CONFIG_DIR || path.join(os.homedir(), '.claude');
}

function getLevelFilePath() {
  return path.join(getClaudeDir(), '.procoder-active');
}

// Deactivation must be the WHOLE message. Matching the phrase anywhere turned
// procoder off mid-task on requests like "add a normal mode toggle".
function isDeactivationCommand(text) {
  const t = String(text || '').trim().toLowerCase().replace(/[.!?\s]+$/, '');
  return t === 'stop procoder' || t === 'normal mode';
}

// `/procoder:level <level>` is the form the host actually routes; the bare
// `/procoder <level>` is kept because the README and muscle memory still say it,
// and a level switch that reads as typed but does nothing is the worst failure
// shape there is. Only a message that STARTS with the command counts, so
// discussing either form in prose does not silently change the level.
function parseLevelCommand(text) {
  const m = /^\/procoder(?::level)?\s+(\S+)\s*$/i.exec(String(text || '').trim());
  return m ? normalizeLevel(m[1]) : null;
}

module.exports = {
  LEVELS,
  LEVEL_RANK,
  DEFAULT_LEVEL,
  normalizeLevel,
  getDefaultLevel,
  getClaudeDir,
  getLevelFilePath,
  isDeactivationCommand,
  parseLevelCommand,
};
