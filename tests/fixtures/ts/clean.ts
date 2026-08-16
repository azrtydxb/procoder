// Clean fixture — real near-misses for every rule ts.js implements, all of
// which must stay silent.

function lookupUser(db: any, id: string) {
  return db.query("SELECT * FROM users WHERE id = ?", [id]);
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
