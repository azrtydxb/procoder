#!/usr/bin/env node
// procoder — renders the doctrine for a given intensity level.
//
// The doctrine is authored once in skills/procoder/SKILL.md. Blocks that only
// apply above a given level are wrapped in <!-- level:NAME --> ... <!-- /level -->
// and stripped here when NAME outranks the active level.

const fs = require('fs');
const path = require('path');
const { normalizeLevel, DEFAULT_LEVEL } = require('./procoder-config');

const RANK = { pragmatic: 1, strict: 2, paranoid: 3 };
const DOCTRINE_PATH = path.join(__dirname, '..', 'skills', 'procoder', 'SKILL.md');

const BLOCK = /<!-- level:([a-z]+) -->\n?([\s\S]*?)<!-- \/level -->\n?/g;

function getProcoderInstructions(level) {
  const active = normalizeLevel(level) || DEFAULT_LEVEL;
  if (active === 'off') return '';

  let doctrine;
  try {
    doctrine = fs.readFileSync(DOCTRINE_PATH, 'utf8');
  } catch (e) {
    // A missing doctrine file must not break the session: no context is better
    // than a crashed hook.
    return '';
  }

  const body = doctrine.replace(/^---\n[\s\S]*?\n---\n/, '');
  const activeRank = RANK[active] || RANK[DEFAULT_LEVEL];

  return body
    .replace(BLOCK, (_match, blockLevel, content) =>
      (RANK[blockLevel] || 0) <= activeRank ? content : '')
    .replace(/\n{3,}/g, '\n\n')
    .trim();
}

module.exports = { getProcoderInstructions, RANK };
