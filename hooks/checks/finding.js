#!/usr/bin/env node
// procoder — the finding shape and its one-line output format.

const RUNGS = ['SAFE', 'TRUE', 'OBVIOUS', 'ALONE'];

function rungIndex(rung) {
  return RUNGS.indexOf(rung);
}

function finding({ rung, id, line, message, fix }) {
  if (!RUNGS.includes(rung)) throw new Error(`unknown rung: ${rung}`);
  return { rung, id, line: Number(line) || 0, message: String(message), fix: String(fix) };
}

function sortFindings(findings) {
  return findings.slice().sort((a, b) =>
    rungIndex(a.rung) - rungIndex(b.rung) || a.line - b.line);
}

function capFindings(findings, max) {
  return findings.slice(0, max);
}

// [1 SAFE]    api/users.ts:42   what is wrong → what to do
// Rationale belongs in the fix clause, never on its own line.
function formatFindings(findings, relPath) {
  return findings.map((f) => {
    const label = `[${rungIndex(f.rung) + 1} ${f.rung}]`.padEnd(11);
    const location = `${relPath}:${f.line}`.padEnd(17);
    return `${label} ${location} ${f.message} → ${f.fix}`;
  }).join('\n');
}

module.exports = { RUNGS, rungIndex, finding, sortFindings, capFindings, formatFindings };
