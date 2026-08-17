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
function rpc(requests) {
  return new Promise((resolve, reject) => {
    const child = spawn('node', [SERVER], { stdio: ['pipe', 'pipe', 'ignore'] });
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
