#!/usr/bin/env node
// procoder — "a newer procoder is out" notice for the SessionStart hook.
//
// SessionStart has a 5 second timeout and runs before the user can type. A
// network call on that path would put a slow or captive network between the
// user and every session, for a cosmetic notice. So this follows what package
// managers do:
//
//   * the hook only ever READS a cached answer — it never opens a socket;
//   * when the cache is older than CACHE_TTL_MS, the hook spawns a detached
//     child (`--refresh`) and exits without waiting for it;
//   * that child writes the answer for the NEXT session to read.
//
// The visible consequence: the first session after a release is silent and the
// next one notifies. That is the trade — a session that always starts instantly
// against a notice that is at most one session late.
//
// Every failure mode here — no cache, corrupt cache, offline, DNS failure,
// GitHub down, garbage body — resolves to "say nothing". updateNotice() is the
// only entry point the hook calls, and it cannot throw.

const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');
const { getClaudeDir } = require('./procoder-config');

// One day. Releases are days to weeks apart, so a shorter interval buys
// nothing a user would notice while multiplying requests; a longer one leaves
// people on a stale version for a week after a fix. It also bounds this to one
// request per user per day.
const CACHE_TTL_MS = 24 * 60 * 60 * 1000;

// Only bounds the detached child, which nobody is waiting for. It exists so a
// captive portal that accepts the connection and never answers cannot leave a
// node process resident.
const FETCH_TIMEOUT_MS = 10000;

// The raw manifest, not the GitHub API: the API allows 60 unauthenticated
// requests/hour/IP shared by everything else on that IP, and rate limiting it
// would silence the check for reasons that have nothing to do with procoder.
// raw.githubusercontent.com is CDN-served, has no such budget, and the answer
// is the same field. PROCODER_UPDATE_URL retargets it at a fork or mirror.
const DEFAULT_URL =
  'https://raw.githubusercontent.com/azrtydxb/procoder/main/.claude-plugin/plugin.json';

const CACHE_FILE = '.procoder-update-check.json';
const MANIFEST = path.join(__dirname, '..', '.claude-plugin', 'plugin.json');

const cachePath = () => path.join(getClaudeDir(), CACHE_FILE);
const versionUrl = () => process.env.PROCODER_UPDATE_URL || DEFAULT_URL;

function readVersion(file) {
  return JSON.parse(fs.readFileSync(file, 'utf8')).version;
}

function parseVersion(value) {
  const m = /^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$/
    .exec(String(value == null ? '' : value).trim());
  return m ? { parts: [+m[1], +m[2], +m[3]], pre: m[4] || '' } : null;
}

const cmp = (a, b) => (a === b ? 0 : (a < b ? -1 : 1));

// semver §11: a pre-release ranks below the release it precedes, so
// 1.0.0-rc.1 < 1.0.0. Without this, shipping 1.0.0 would never notify anyone
// running its release candidate. '￿' sorts after any real identifier, so
// "no pre-release" wins. Build metadata is ignored, as semver requires.
const preRank = (pre) => pre || '￿';

// -1 / 0 / 1, or null when either side is not a version we understand — the
// caller treats null as "say nothing", so a malformed manifest on either end
// silences the notice instead of guessing. String comparison is wrong here:
// '0.10.0' < '0.9.0' lexically.
function compareVersions(a, b) {
  const x = parseVersion(a);
  const y = parseVersion(b);
  if (!x || !y) return null;
  return cmp(x.parts[0], y.parts[0])
    || cmp(x.parts[1], y.parts[1])
    || cmp(x.parts[2], y.parts[2])
    || cmp(preRank(x.pre), preRank(y.pre));
}

function readCache() {
  try {
    const cache = JSON.parse(fs.readFileSync(cachePath(), 'utf8'));
    return cache && typeof cache === 'object' ? cache : null;
  } catch (e) {
    // Missing, half-written, or hand-mangled all mean the same thing: no
    // usable answer. Reporting it as stale makes the next refresh rewrite it.
    return null;
  }
}

function writeCache(cache) {
  const file = cachePath();
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, JSON.stringify(cache));
}

// A redirect to a login page or a GitHub error page is not a manifest; cap
// what we are willing to buffer from a URL we do not control.
const MAX_BODY_BYTES = 65536;

function versionIn(body) {
  try {
    const version = JSON.parse(body).version;
    return parseVersion(version) ? version : null;
  } catch (e) {
    return null;
  }
}

// `finish` takes the answer, `abort` gives up on an oversized body.
function collectVersion(res, finish, abort) {
  if (res.statusCode !== 200) {
    res.resume();
    return finish(null);
  }
  let body = '';
  res.setEncoding('utf8');
  res.on('error', () => finish(null));
  res.on('data', (chunk) => {
    body += chunk;
    if (body.length > MAX_BODY_BYTES) abort();
  });
  res.on('end', () => finish(versionIn(body)));
}

// Resolves to a version string or null. Takes `get` so tests can drive every
// failure mode without opening a socket.
function fetchLatestVersion(get = require('https').get) {
  return new Promise((resolve) => {
    let settled = false;
    let request = null;
    const finish = (value) => {
      if (settled) return;
      settled = true;
      resolve(value);
    };
    const abort = () => {
      if (request) request.destroy();
      finish(null);
    };
    try {
      request = get(versionUrl(), { headers: { 'user-agent': 'procoder' } },
        (res) => collectVersion(res, finish, abort));
    } catch (e) {
      return finish(null);
    }
    request.on('error', () => finish(null));
    request.setTimeout(FETCH_TIMEOUT_MS, abort);
  });
}

// The detached child. Stamps the cache BEFORE fetching, so a host that hangs
// until the timeout cannot make every session spawn another child behind it.
async function refresh(get) {
  const previous = readCache() || {};
  writeCache({ checkedAt: Date.now(), latest: previous.latest || null });
  const latest = await fetchLatestVersion(get);
  // A failed fetch keeps the last known answer rather than blanking it: going
  // offline for a week should not un-notify someone about a release.
  if (latest) writeCache({ checkedAt: Date.now(), latest });
}

function spawnDetachedRefresh() {
  spawn(process.execPath, [__filename, '--refresh'], {
    detached: true,
    stdio: 'ignore',
    windowsHide: true,
  }).unref();
}

// The hook's whole interface. Returns the notice, or '' — and never throws,
// because the hook's actual job is injecting the doctrine and this is a
// nicety riding along with it.
function updateNotice({ spawnRefresh = spawnDetachedRefresh, now = Date.now() } = {}) {
  try {
    // Only meaningful for a plugin install: CLAUDE_PLUGIN_ROOT is set by the
    // host when it runs the hook. A source checkout, a test run, or the CLI
    // invoked by hand neither notifies nor phones home.
    if (!process.env.CLAUDE_PLUGIN_ROOT) return '';
    if (process.env.PROCODER_NO_UPDATE_CHECK === '1') return '';

    const cache = readCache();
    if (!(now - Number(cache && cache.checkedAt) < CACHE_TTL_MS)) spawnRefresh();
    if (!cache) return '';

    const installed = readVersion(MANIFEST);
    if (compareVersions(installed, cache.latest) !== -1) return '';
    return `procoder ${cache.latest} is available; this session is running ${installed}. `
      + 'Mention it once, and point the user at /procoder:update.';
  } catch (e) {
    return '';
  }
}

module.exports = {
  CACHE_TTL_MS,
  compareVersions,
  fetchLatestVersion,
  refresh,
  updateNotice,
};

if (require.main === module && process.argv[2] === '--refresh') {
  refresh().catch(() => {});
}
