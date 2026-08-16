const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { check, EXTENSIONS } = require('../hooks/checks/lang/dotnet');
const { DEFAULTS } = require('../hooks/checks/config');

const config = { ...DEFAULTS, root: '/tmp' };
const ids = (src) => check(src, { relPath: 'X.cs', config }).map((f) => f.id);

test('owns the .cs extension', () => {
  assert.deepStrictEqual(EXTENSIONS, ['.cs']);
});

test('flags SQL string building', () => {
  assert.ok(ids('new SqlCommand($"SELECT * FROM t WHERE id = {id}", conn);').includes('safe/sql-injection'));
  assert.ok(ids('cmd.CommandText = "SELECT * FROM t WHERE id = " + id;').includes('safe/sql-injection'));
  assert.ok(!ids('cmd.Parameters.AddWithValue("@id", id);').includes('safe/sql-injection'));
});

test('does not flag FromSqlInterpolated, the safe EF Core API', () => {
  // FromSqlInterpolated takes a C# interpolated string by design and
  // parameterizes it internally — it must not be confused with the unsafe
  // FromSqlRaw/string-concatenation path.
  assert.ok(!ids('context.Users.FromSqlInterpolated($"SELECT * FROM Users WHERE Id = {id}");').includes('safe/sql-injection'));
});

test('flags unsafe deserialization and weak crypto', () => {
  assert.ok(ids('var f = new BinaryFormatter();').includes('safe/unsafe-deserialize'));
  assert.ok(ids('MD5.Create()').includes('safe/weak-hash'));
  assert.ok(ids('var token = new Random().Next().ToString();').includes('safe/weak-random'));
  assert.ok(!ids('SHA256.Create()').includes('safe/weak-hash'));
});

test('flags disabled certificate validation', () => {
  assert.ok(ids('ServerCertificateValidationCallback = (a, b, c, d) => true;').includes('safe/tls-disabled'));
});

test('flags swallowed exceptions and leftover debugging', () => {
  assert.ok(ids('try { Go(); } catch (Exception) { }').includes('true/swallowed-error'));
  assert.ok(ids('Console.WriteLine("here");').includes('alone/debug-leftover'));
  assert.ok(!ids('_logger.LogInformation("started");').includes('alone/debug-leftover'));
});

test('the signature regex does not treat if/catch as methods', () => {
  const src = [
    'class X {',
    '    void Run() {',
    '        if (Ready()) {',
    '            DoThing();',
    '        }',
    '        try {',
    '            DoThing();',
    '        } catch (Exception e) {',
    '            _logger.LogError(e, "failed");',
    '            throw;',
    '        }',
    '    }',
    '}',
  ].join('\n');
  assert.deepStrictEqual(check(src, { relPath: 'X.cs', config }), []);
});

test('the clean fixture is silent and the dirty one is not', () => {
  const dir = path.join(__dirname, 'fixtures', 'dotnet');
  const clean = check(fs.readFileSync(path.join(dir, 'clean.cs'), 'utf8'),
    { relPath: 'clean.cs', config });
  assert.deepStrictEqual(clean, [], `clean fixture fired: ${clean.map((f) => f.id).join(', ')}`);
  assert.ok(check(fs.readFileSync(path.join(dir, 'dirty.cs'), 'utf8'),
    { relPath: 'dirty.cs', config }).length >= 4);
});
