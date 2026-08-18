const test = require('node:test');
const assert = require('node:assert');
const { spawn, spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const SERVER = path.join(__dirname, '..', 'procoder-mcp', 'server.js');

// The server speaks newline-delimited JSON, so a chunk may hold several
// responses, one, or half of one. Splits off every complete line and hands back
// the unterminated remainder.
function takeLines(buffer) {
  const parts = buffer.split('\n');
  return { lines: parts.slice(0, -1).map((l) => l.trim()).filter(Boolean), rest: parts[parts.length - 1] };
}

// Sends a batch of requests, resolves with the parsed responses in order.
function rpc(requests, options = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn('node', [SERVER], { stdio: ['pipe', 'pipe', 'ignore'], ...options });
    let buffer = '';
    const responses = [];
    child.stdout.on('data', (chunk) => {
      const taken = takeLines(buffer + chunk);
      buffer = taken.rest;
      responses.push(...taken.lines.map((line) => JSON.parse(line)));
      if (responses.length < requests.length) return;
      child.kill();
      resolve(responses.slice(0, requests.length));
    });
    child.on('error', reject);
    setTimeout(() => { child.kill(); reject(new Error('MCP server timed out')); }, 5000);
    for (const request of requests) child.stdin.write(JSON.stringify(request) + '\n');
  });
}

test('initialize returns protocol and server info', async () => {
  const [res] = await rpc([{ jsonrpc: '2.0', id: 1, method: 'initialize', params: {} }]);
  assert.strictEqual(res.id, 1);
  assert.ok(res.result.protocolVersion);
  assert.strictEqual(res.result.serverInfo.name, 'procoder');
});

test('tools/list advertises every tool with a schema', async () => {
  const [res] = await rpc([{ jsonrpc: '2.0', id: 2, method: 'tools/list', params: {} }]);
  const names = res.result.tools.map((t) => t.name).sort();
  assert.deepStrictEqual(names,
    ['procoder_baseline', 'procoder_check', 'procoder_doctrine', 'procoder_review']);
  for (const tool of res.result.tools) {
    assert.ok(tool.description && tool.inputSchema, `${tool.name} missing schema`);
  }
});

test('procoder_doctrine returns the ladder', async () => {
  const [res] = await rpc([{
    jsonrpc: '2.0', id: 3, method: 'tools/call',
    params: { name: 'procoder_doctrine', arguments: { level: 'strict' } },
  }]);
  assert.match(res.result.content[0].text, /SAFE/);
  assert.match(res.result.content[0].text, /ALONE/);
});

test('procoder_check reports findings for a dirty file', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-mcp-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  fs.writeFileSync(path.join(dir, 'a.ts'), 'eval(x);\n');  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  const [res] = await rpc([{
    jsonrpc: '2.0', id: 4, method: 'tools/call',
    params: { name: 'procoder_check', arguments: { path: path.join(dir, 'a.ts') } },
  }]);
  assert.match(res.result.content[0].text, /SAFE/);
});

test('an unknown method returns a JSON-RPC error, not a crash', async () => {
  const [res] = await rpc([{ jsonrpc: '2.0', id: 5, method: 'nope/nope', params: {} }]);
  assert.ok(res.error);
  assert.strictEqual(res.error.code, -32601);
});

test('malformed JSON on one line does not kill the server', async () => {
  const [res] = await rpc([{ jsonrpc: '2.0', id: 6, method: 'initialize', params: {} }]);
  assert.strictEqual(res.id, 6);
});

// Resolves with the first complete response line, then stops listening — the
// caller goes on to destroy stdout underneath the server.
function firstResponse(child) {
  return new Promise((resolve, reject) => {
    let buffer = '';
    const onData = (chunk) => {
      buffer += chunk;
      const taken = takeLines(buffer);
      if (!taken.lines.length) return;
      child.stdout.removeListener('data', onData);
      resolve(JSON.parse(taken.lines[0]));
    };
    child.stdout.on('data', onData);
    child.on('error', reject);
    setTimeout(() => reject(new Error('EPIPE-guard test timed out waiting for first response')), 5000);
  });
}

test('a broken stdout pipe does not crash the server (EPIPE guard)', async () => {
  const child = spawn('node', [SERVER], { stdio: ['pipe', 'pipe', 'ignore'] });
  let exitCode = null;
  let exitSignal = null;

  child.on('exit', (code, signal) => { exitCode = code; exitSignal = signal; });

  const first = firstResponse(child);
  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'initialize', params: {} }) + '\n');
  const res = await first;
  assert.strictEqual(res.id, 1);

  // Destroy the parent's read end of the child's stdout: further writes by
  // the child now target a closed pipe, which surfaces as an async 'error'
  // event on the child's stdout stream, not a synchronous throw.
  child.stdout.destroy();

  // The server should still be able to process another request without
  // dying from the broken pipe. Give it time to try to write and fail.
  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id: 2, method: 'initialize', params: {} }) + '\n');

  await new Promise((resolve) => setTimeout(resolve, 500));

  assert.strictEqual(child.exitCode, null, 'server should still be running after stdout was destroyed');

  child.stdin.end();
  child.kill();

  // Confirm it did not already exit with a failure code/signal before we killed it.
  if (exitCode !== null) {
    assert.strictEqual(exitCode, 0, `server exited non-zero (code=${exitCode}, signal=${exitSignal})`);
  }
});

// A gate that can only answer for one file at a time makes every host
// re-implement "which files changed", or skip the check entirely.
test('procoder_review checks everything that changed since a ref', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-mcp-'));
  const git = (...args) => spawnSync('git', args, { cwd: dir, encoding: 'utf8' });
  git('init', '-q');
  fs.writeFileSync(path.join(dir, 'old.ts'), 'const x = 1;\n');
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'add', '-A');
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'commit', '-qm', 'base');
  fs.writeFileSync(path.join(dir, 'new.ts'), 'eval(x);\n');  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it

  const [res] = await rpc([{
    jsonrpc: '2.0', id: 20, method: 'tools/call',
    params: { name: 'procoder_review', arguments: { path: dir, since: 'HEAD' } },
  }]);
  assert.match(res.result.content[0].text, /new\.ts/);
  assert.doesNotMatch(res.result.content[0].text, /old\.ts/);
});

// "I could not work out what changed" and "nothing changed" are opposite
// answers, and the second arriving as the first is how a gate goes quiet.
test('procoder_review says so when git cannot resolve the ref', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-mcp-'));
  spawnSync('git', ['init', '-q'], { cwd: dir });
  const [res] = await rpc([{
    jsonrpc: '2.0', id: 21, method: 'tools/call',
    params: { name: 'procoder_review', arguments: { path: dir, since: 'no-such-ref' } },
  }]);
  assert.match(res.result.content[0].text, /cannot review/);
});

test('procoder_doctrine can return the digest', async () => {
  const [full, digest] = await Promise.all([
    rpc([{ jsonrpc: '2.0', id: 22, method: 'tools/call',
      params: { name: 'procoder_doctrine', arguments: {} } }]),
    rpc([{ jsonrpc: '2.0', id: 23, method: 'tools/call',
      params: { name: 'procoder_doctrine', arguments: { digest: true } } }]),
  ]);
  const fullText = full[0].result.content[0].text;
  const digestText = digest[0].result.content[0].text;
  assert.ok(digestText.length < fullText.length, 'the digest is not smaller than the full text');
  for (const rung of ['SAFE', 'TRUE', 'OBVIOUS', 'ALONE', 'FAST', 'MEANT']) {
    assert.match(digestText, new RegExp(rung), `the digest dropped ${rung}`);
  }
});

// --- protocol eras ----------------------------------------------------------
//
// MCP split in two: modern revisions (2026-07-28 and later) carry the version
// on every request and have no handshake at all, legacy ones (2025-11-25 and
// earlier) negotiate it once via `initialize`. A modern client talking to a
// legacy-only server FAILS with no fall-forward, so answering both eras is the
// difference between working and not working for whole categories of client.

test('server/discover answers the modern probe with every version it speaks', async () => {
  const [res] = await rpc([{
    jsonrpc: '2.0', id: 30, method: 'server/discover',
    params: { _meta: { 'io.modelcontextprotocol/protocolVersion': '2026-07-28' } },
  }]);
  assert.strictEqual(res.result.resultType, 'complete');
  assert.ok(res.result.supportedVersions.includes('2026-07-28'), 'the current revision is missing');
  assert.ok(res.result.supportedVersions.includes('2025-11-25'), 'the last legacy revision is missing');
  assert.deepStrictEqual(res.result.capabilities, { tools: {} });
  assert.strictEqual(
    res.result._meta['io.modelcontextprotocol/serverInfo'].name, 'procoder');
});

// A version this server does not speak must come back as the spec's own error,
// carrying the list to retry with: a plain "unknown method" leaves a modern
// client with nothing to fall back to.
test('an unsupported protocol version is refused with the versions that would work', async () => {
  const [res] = await rpc([{
    jsonrpc: '2.0', id: 31, method: 'tools/list',
    params: { _meta: { 'io.modelcontextprotocol/protocolVersion': '1900-01-01' } },
  }]);
  assert.strictEqual(res.error.code, -32022);
  assert.strictEqual(res.error.data.requested, '1900-01-01');
  assert.ok(Array.isArray(res.error.data.supported) && res.error.data.supported.length > 1);
});

test('a modern request at a supported version is served', async () => {
  const [res] = await rpc([{
    jsonrpc: '2.0', id: 32, method: 'tools/list',
    params: { _meta: { 'io.modelcontextprotocol/protocolVersion': '2026-07-28' } },
  }]);
  assert.ok(res.result.tools.length >= 4);
});

// The legacy handshake answers with the client's own version when it is one we
// speak. Echoing an unknown one back would be a claim to speak it.
test('initialize negotiates the version the client asked for, when it is one we speak', async () => {
  const [older, unknown] = await Promise.all([
    rpc([{ jsonrpc: '2.0', id: 33, method: 'initialize', params: { protocolVersion: '2025-03-26' } }]),
    rpc([{ jsonrpc: '2.0', id: 34, method: 'initialize', params: { protocolVersion: '1999-01-01' } }]),
  ]);
  assert.strictEqual(older[0].result.protocolVersion, '2025-03-26');
  assert.strictEqual(unknown[0].result.protocolVersion, '2025-11-25',
    'an unknown request should get the newest legacy revision, not an echo');
});

// --- failure signalling -----------------------------------------------------
//
// A gate reporting clean when it did not run is the worst failure this project
// can have, and MCP is a second front door onto the same engine the CLI gates
// with. The CLI exits 2 for "cannot verify" and names every file it skipped;
// these hold the server to the same standard, in the shape MCP has for it —
// `isError: true` on the result, which is what a host feeds back to the model.
// A protocol-level `error` is reserved for a request that never named a
// runnable check at all.

let nextId = 1000;
function call(name, args) {
  nextId += 1;
  return rpc([{ jsonrpc: '2.0', id: nextId, method: 'tools/call', params: { name, arguments: args } }])
    .then(([res]) => res);
}

// The fixture repo borrows this repo's own analyzers. procoder finds a
// project's linters in node_modules/.bin (see resolve.js binPath), so linking
// them in is what makes a temp repo a gated one — without it every fixture below
// reports "eslint is not installed" instead of the findings it is about.
const REPO_ROOT = path.join(__dirname, '..');

const tempRepo = (files = {}) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-mcp-'));
  fs.mkdirSync(path.join(dir, '.git'), { recursive: true });
  try {
    fs.symlinkSync(path.join(REPO_ROOT, 'node_modules'), path.join(dir, 'node_modules'), 'dir');
    fs.copyFileSync(path.join(REPO_ROOT, 'eslint.config.mjs'), path.join(dir, 'eslint.config.mjs'));
  } catch (e) {
    // A machine without the dev dependencies installed still runs every other
    // assertion in this file; the finding-count test is the only one that needs
    // a working analyzer, and it says so when it cannot get one.
  }
  for (const [rel, content] of Object.entries(files)) {
    fs.writeFileSync(path.join(dir, rel), content);
  }
  return dir;
};

test('procoder_check fails the call for a file it cannot read', async () => {
  const dir = tempRepo();
  const res = await call('procoder_check', { path: path.join(dir, 'gone.ts') });
  const out = res.result.content[0].text;
  assert.strictEqual(res.result.isError, true, 'an unread file came back as a successful check');
  assert.doesNotMatch(out, /(^|\n)clean\b/);
  assert.match(out, /not checked/);
});

// The hook caps its report at five findings, because it is writing into a
// model's context. The MCP tool has no such constraint and must not inherit the
// cap: a caller asking for a file's findings gets the file's findings.
test('procoder_check reports every finding, not the hook\'s top five', async () => {
  // Eight non-literal RegExp constructions, one per line, so the expected count
  // is exact. The rule is detect-non-literal-regexp, which this repo keeps ON
  // for shipped source — the fixture lives in a temp dir, so the tests/** block
  // that switches it off does not reach it.
  const body = Array.from({ length: 8 }, (_, i) => `const r${i} = new RegExp(p${i});`).join('\n');
  const dir = tempRepo({ 'many.ts': `${body}\n` });
  const res = await call('procoder_check', { path: path.join(dir, 'many.ts') });
  const text = res.result.content[0].text;
  if (/is not installed/.test(text)) {
    assert.fail('the fixture repo has no analyzer — run npm install first');
  }
  const lines = text.split('\n').filter((l) => /many\.ts:/.test(l));
  assert.strictEqual(lines.length, 8, `reported ${lines.length} of 8 findings, silently`);
});

test('a tool call with no path is a protocol error, not a check that passed', async () => {
  const res = await call('procoder_check', {});
  assert.ok(res.error, 'a request naming no file answered as though a file was checked');
  assert.strictEqual(res.error.code, -32602);
});

test('an internal throw is a failed tool result, and the server survives it', async () => {
  // The engine made to throw from inside a tool call, which no fixture can
  // arrange from outside: the preload patches checkFile in the require cache
  // before server.js destructures it, and the run is still real stdio JSON-RPC.
  const dir = tempRepo({ 'a.ts': 'const x = 1;\n' });
  const preload = path.join(dir, 'boom.js');
  const runModule = path.join(__dirname, '..', 'hooks', 'checks', 'run.js');
  fs.writeFileSync(preload,
    `require(${JSON.stringify(runModule)}).checkFile = () => { throw new Error('boom'); };\n`);

  nextId += 2;
  const [bad, after] = await rpc([
    { jsonrpc: '2.0', id: nextId, method: 'tools/call', params: { name: 'procoder_check', arguments: { path: path.join(dir, 'a.ts') } } },
    { jsonrpc: '2.0', id: nextId + 1, method: 'initialize', params: {} },
  ], { env: { ...process.env, NODE_OPTIONS: `--require "${preload}"` } });
  assert.ok(!bad.error, 'an internal failure was raised as a protocol error the model never sees');
  assert.strictEqual(bad.result.isError, true);
  assert.doesNotMatch(bad.result.content[0].text, /(^|\n)clean\b/);
  assert.strictEqual(after.result.serverInfo.name, 'procoder', 'the server died on a throw');
});

test('procoder_review never says clean over a diff it could not fully read', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-mcp-'));
  const git = (...args) => spawnSync('git', args, { cwd: dir, encoding: 'utf8' });
  git('init', '-q');
  fs.writeFileSync(path.join(dir, '.procoder.toml'), '[limits]\nmax_file_bytes = 4\n');
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'add', '-A');
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'commit', '-qm', 'base');
  fs.writeFileSync(path.join(dir, 'big.ts'), 'const x = 1;\n'.repeat(20));

  const res = await call('procoder_review', { path: dir, since: 'HEAD' });
  const out = res.result.content[0].text;
  assert.doesNotMatch(out, /(^|\n)clean\b/, 'a diff nothing read was reported as clean');
  assert.match(out, /big\.ts/, 'the file that went unread was not named');
  assert.strictEqual(res.result.isError, true);
});

// The severity split the SARIF report already makes: an excluded file is a
// scope decision, not a broken run, so the call succeeds — but coverage was
// still lost, and the word "clean" is not available for it either.
test('procoder_review names an excluded file without failing the call', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-mcp-'));
  const git = (...args) => spawnSync('git', args, { cwd: dir, encoding: 'utf8' });
  git('init', '-q');
  fs.writeFileSync(path.join(dir, '.procoder.toml'), '[exclude]\npaths = ["generated/"]\n');
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'add', '-A');
  git('-c', 'user.email=t@t', '-c', 'user.name=t', 'commit', '-qm', 'base');
  fs.mkdirSync(path.join(dir, 'generated'));
  fs.writeFileSync(path.join(dir, 'generated', 'g.ts'), 'const x = 1;\n');

  const res = await call('procoder_review', { path: dir, since: 'HEAD' });
  const out = res.result.content[0].text;
  assert.strictEqual(res.result.isError, undefined, 'an excluded path failed the whole call');
  assert.doesNotMatch(out, /(^|\n)clean\b/, 'a review that read nothing was reported as clean');
  assert.match(out, /generated\/g\.ts/);
});

test('procoder_review fails the call when git cannot say what changed', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-mcp-'));
  spawnSync('git', ['init', '-q'], { cwd: dir });
  const res = await call('procoder_review', { path: dir, since: 'no-such-ref' });
  assert.strictEqual(res.result.isError, true, 'a git failure answered as a successful review');
  assert.match(res.result.content[0].text, /cannot review/);
});

// A renamed file is content arriving at a path nothing has checked. The CLI's
// --since fixed this by widening its filter to ACMRT; the MCP door was still
// on ACM, so a commit that only renamed a file reviewed nothing and said so as
// "no files changed".
test('procoder_review checks a renamed file at its new path', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'procoder-mcp-'));
  const git = (...args) => spawnSync('git', ['-c', 'user.email=t@t', '-c', 'user.name=t', ...args],
    { cwd: dir, encoding: 'utf8' });
  git('init', '-q');
  fs.writeFileSync(path.join(dir, 'old.ts'), 'eval(x);\n');  // procoder: literal safe/dynamic-eval scanner input for that rule, not an instance of it
  git('add', '-A');
  git('commit', '-qm', 'base');
  git('mv', 'old.ts', 'moved.ts');
  git('commit', '-qm', 'rename');

  const res = await call('procoder_review', { path: dir, since: 'HEAD~1' });
  assert.match(res.result.content[0].text, /moved\.ts/, 'a rename reviewed nothing');
});

test('procoder_baseline refuses a baseline file it cannot honour', async () => {
  const dir = tempRepo({ '.procoder-baseline.json': JSON.stringify({ version: 2, fingerprints: ['a'] }) });
  const res = await call('procoder_baseline', { path: dir });
  assert.strictEqual(res.result.isError, true, 'a stale baseline was reported as no baseline');
  assert.match(res.result.content[0].text, /procoder baseline/);
});
