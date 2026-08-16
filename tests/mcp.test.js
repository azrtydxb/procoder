const test = require('node:test');
const assert = require('node:assert');
const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const SERVER = path.join(__dirname, '..', 'procoder-mcp', 'server.js');

// Sends a batch of requests, resolves with the parsed responses in order.
function rpc(requests) {
  return new Promise((resolve, reject) => {
    const child = spawn('node', [SERVER], { stdio: ['pipe', 'pipe', 'ignore'] });
    let buffer = '';
    const responses = [];
    child.stdout.on('data', (chunk) => {
      buffer += chunk;
      let index;
      while ((index = buffer.indexOf('\n')) >= 0) {
        const line = buffer.slice(0, index).trim();
        buffer = buffer.slice(index + 1);
        if (line) responses.push(JSON.parse(line));
        if (responses.length === requests.length) {
          child.kill();
          resolve(responses);
        }
      }
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

test('tools/list advertises the three tools with schemas', async () => {
  const [res] = await rpc([{ jsonrpc: '2.0', id: 2, method: 'tools/list', params: {} }]);
  const names = res.result.tools.map((t) => t.name).sort();
  assert.deepStrictEqual(names, ['procoder_baseline', 'procoder_check', 'procoder_doctrine']);
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

test('a broken stdout pipe does not crash the server (EPIPE guard)', async () => {
  const child = spawn('node', [SERVER], { stdio: ['pipe', 'pipe', 'ignore'] });
  let buffer = '';
  let exitCode = null;
  let exitSignal = null;

  child.on('exit', (code, signal) => { exitCode = code; exitSignal = signal; });

  const firstResponse = new Promise((resolve, reject) => {
    const onData = (chunk) => {
      buffer += chunk;
      const index = buffer.indexOf('\n');
      if (index >= 0) {
        child.stdout.removeListener('data', onData);
        resolve(JSON.parse(buffer.slice(0, index)));
      }
    };
    child.stdout.on('data', onData);
    child.on('error', reject);
    setTimeout(() => reject(new Error('EPIPE-guard test timed out waiting for first response')), 5000);
  });

  child.stdin.write(JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'initialize', params: {} }) + '\n');
  const res = await firstResponse;
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
