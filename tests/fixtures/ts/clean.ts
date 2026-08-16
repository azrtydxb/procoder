// Clean fixture — real near-misses for every rule ts.js implements, all of
// which must stay silent.

function lookupUser(db: any, id: string) {
  return db.query("SELECT * FROM users WHERE id = ?", [id]);
}

function lookupUserNumbered(db: any, id: string) {
  // build the query with "SELECT " + cols
  return db.query("SELECT * FROM users WHERE id = $1", [id]);
}

function greet(name: string) {
  const msg = "hello " + name;
  return msg;
}

function renderBio(el: any, userInput: string) {
  el.textContent = userInput;
}

function computeJitter() {
  const jitter = Math.random() * 100;
  return jitter;
}

function makeToken() {
  const token = crypto.randomUUID();
  return token;
}

function riskyCall() {
  try {
    go();
  } catch (e) {
    logger.error(e);
    throw e;
  }
}

function startup() {
  logger.info("started");
}

function pickLabel(a: boolean) {
  const x = a ? 1 : 2;
  return x;
}

function fourParams(a: number, b: number, c: number, d: number) {
  return a + b + c + d;
}

function runGitLog(branch: string) {
  execFile('git', ['log', branch]);
}

function runShell(dir: string) {
  spawn('ls', [dir]);
}

// Documentation that warns against a practice must not be flagged for the
// practice: every rule ts.js has, named in prose, still silent.
//
//   never db.query(`SELECT * FROM users WHERE id = ${id}`)
//   never el.innerHTML = userInput, and never document.write(payload)
//   never eval(payload) or new Function(src)
//   never exec(`git log ${branch}`) or execSync('rm -rf ' + dir)
//   never rejectUnauthorized: false
//   never const token = Math.random().toString(36)
//   never catch (e) {} — log it and rethrow
//   no leftover console.log("here") or debugger;
//   never const x = a ? b ? 1 : 2 : 3;
export function documented(value: string) {
  return value;
}

// A pattern that spells a sink is a matcher for it, not a call to it.
export const SINK_PATTERN = /dangerouslySetInnerHTML|\.innerHTML\s*=|document\.write\(/;

// Assign-then-use, done right: the shape local taint tracking reads, with the
// value bound to a name before it reaches the sink. All four stay silent —
// a bound placeholder, a value built only from literals, a binding cleared by
// a literal reassignment, and a tainted value that never reaches a sink.
function lookupUserBound(db: any, id: string) {
  const q = "SELECT * FROM users WHERE id = ?";
  return db.query(q, [id]);
}

function listColumns(db: any) {
  const q = "SELECT " + "id, name" + " FROM users";
  return db.query(q);
}

function rebuildQuery(db: any, id: string) {
  let q = "SELECT * FROM users WHERE id = " + id;
  q = "SELECT * FROM users";
  return db.query(q);
}

function describeDir(dir: string) {
  const cmd = "ls " + dir;
  logger.info(cmd);
}
