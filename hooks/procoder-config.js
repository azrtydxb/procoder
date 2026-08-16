#!/usr/bin/env node
// procoder — level resolution and shared path helpers.
// Pure: no file writes, no stdout. Safe to require from any hook.

const os = require('os');
const path = require('path');

const LEVELS = ['off', 'pragmatic', 'strict', 'paranoid'];
const DEFAULT_LEVEL = 'strict';

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

// Only a message that STARTS with the command counts, so discussing the command
// in prose does not silently change the level.
function parseLevelCommand(text) {
  const m = /^\/procoder\s+(\S+)\s*$/i.exec(String(text || '').trim());
  return m ? normalizeLevel(m[1]) : null;
}

module.exports = {
  LEVELS,
  DEFAULT_LEVEL,
  normalizeLevel,
  getDefaultLevel,
  getClaudeDir,
  getLevelFilePath,
  isDeactivationCommand,
  parseLevelCommand,
};
