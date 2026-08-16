// tests/hook-stdin.js
//
// Runs a hook with a payload on stdin without the PARENT writing to a pipe.
//
// execFileSync's `input:` option does not hand the child a buffer — it makes
// the parent write those bytes into the child's stdin pipe while the child
// runs. procoder-activate.js and procoder-subagent.js never read stdin; they
// are pure SessionStart/SubagentStart emitters. So the parent's write races
// the child's exit, and losing that race means the read end is already gone:
// the write fails with EPIPE and spawnSync reports it as `spawnSync node
// EPIPE` — a failure of the test process, not of the hook under test.
//
// The window is only as wide as a scheduler stall, so the race is invisible
// when a test file runs alone and opens up under the CPU contention of
// `node --test`'s concurrent file runners. That is what made it a flake.
//
// A regular file as stdin delivers the same bytes with no write to lose: the
// child reads the payload, or ignores it and exits, and either way nothing on
// the parent side can fail. Hooks that DO drain stdin — procoder-check.js and
// procoder-mode-tracker.js, both via readHookInput() — are safe with `input:`
// and are left alone.

const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

// Runs `node <script>` with `payload` readable on stdin, and returns stdout.
// Throws exactly as execFileSync does when the hook exits non-zero.
function execHook(script, options = {}, payload = '{}') {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-stdin-'));
  const file = path.join(dir, 'payload.json');
  fs.writeFileSync(file, payload);
  const fd = fs.openSync(file, 'r');
  try {
    // stdio is set last: it is the whole point of this helper, and a caller
    // passing `input:` or its own stdio would reopen the race.
    return execFileSync('node', [script], {
      encoding: 'utf8', ...options, input: undefined, stdio: [fd, 'pipe', 'pipe'],
    });
  } finally {
    fs.closeSync(fd);
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

module.exports = { execHook };
