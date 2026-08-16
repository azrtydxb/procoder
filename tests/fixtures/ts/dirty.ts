// Deliberately unsafe/broken fixture — exercises every ts.js finding id.

function lookupUser(db: any, id: string) {
  return db.query(`SELECT * FROM users WHERE id = ${id}`);
}

function renderBio(el: any, userInput: string) {
  el.innerHTML = userInput;
}

function runPayload(payload: string) {
  eval(payload);
}

const agentOptions = {
  rejectUnauthorized: false,
};

function makeToken() {
  const token = Math.random().toString(36);
  return token;
}

function riskyCall() {
  try {
    go();
  } catch (e) {}
}

function debugPrint() {
  console.log("here");
  debugger;
}

function pickLabel(a: boolean, b: boolean) {
  const x = a ? b ? 1 : 2 : 3;
  return x;
}

function runGitLog(branch: string) {
  exec(`git log ${branch}`);
}

function runShell(cmd: string) {
  spawn('sh', [cmd], { shell: true });
}

function big(a, b, c, d, e) {
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
  work();
}
