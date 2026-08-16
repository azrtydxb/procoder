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
