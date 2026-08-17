// procoder — renders the doctrine for a given intensity level.
//
// The doctrine is authored once in skills/procoder/SKILL.md. Blocks that only
// apply above a given level are wrapped in <!-- level:NAME --> ... <!-- /level -->
// and stripped here when NAME outranks the active level.
//
// A second, orthogonal cut exists for subagents. `<!-- digest:skip -->` ...
// `<!-- /digest -->` marks text a subagent does not need — see the digest note
// in hooks/procoder-subagent.js for what qualifies and why. The polarity is
// deliberate: a rule added tomorrow carries no marker and is therefore IN the
// digest. Only text somebody has explicitly judged unnecessary drops out, so the
// failure direction is a digest that is slightly too long, never one missing a
// rung.

const fs = require('fs');
const path = require('path');
const { normalizeLevel, DEFAULT_LEVEL, LEVEL_RANK } = require('./procoder-config');

const RANK = LEVEL_RANK;
const DOCTRINE_PATH = path.join(__dirname, '..', 'skills', 'procoder', 'SKILL.md');

const BLOCK = /<!-- level:([a-z]+) -->\n?([\s\S]*?)<!-- \/level -->\n?/g;
const DIGEST_BLOCK = /<!-- digest:skip -->\n?([\s\S]*?)<!-- \/digest -->\n?/g;
const FENCE = /```[\s\S]*?```/g;

// Collapses runs of 3+ newlines to a single blank line, but leaves fenced
// code blocks untouched — a blank line inside a fence is content, not
// formatting whitespace left over from stripped level blocks.
function collapseBlankRuns(text) {
  let out = '';
  let lastIndex = 0;
  let match;
  FENCE.lastIndex = 0;
  while ((match = FENCE.exec(text)) !== null) {
    out += text.slice(lastIndex, match.index).replace(/\n{3,}/g, '\n\n');
    out += match[0];
    lastIndex = FENCE.lastIndex;
  }
  out += text.slice(lastIndex).replace(/\n{3,}/g, '\n\n');
  return out;
}

function getProcoderInstructions(level, { digest = false } = {}) {
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
  const activeRank = RANK[active];

  const stripped = body.replace(BLOCK, (_match, blockLevel, content) =>
    (RANK[blockLevel] || 0) <= activeRank ? content : '');
  // The markers come out either way. Left in the full text they would be
  // instructions to the model about its own prompt.
  const cut = stripped.replace(DIGEST_BLOCK, (_match, content) => (digest ? '' : content));
  return collapseBlankRuns(cut).trim();
}

module.exports = { getProcoderInstructions, RANK, collapseBlankRuns };
