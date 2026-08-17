// Deliberately unsafe/broken fixture — exercises every ts.js finding id.

function lookupUser(db: any, id: string) {
  return db.query(`SELECT * FROM users WHERE id = ${id}`);
}

function lookupUserQuoted(db: any, id: string) {
  return db.query("SELECT * FROM u WHERE id = '" + id + "'");
}

function lookupUserQuotedSingle(db: any, name: string) {
  return db.query('SELECT * FROM u WHERE name = "' + name + '"');
}

function removeQuoted(x: string) {
  exec("rm '" + x + "'");
}

function removeQuotedSingle(x: string) {
  exec('rm "' + x + '"');
}

function removeTemplate(x: string) {
  exec(`rm '${x}'`);
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

// Assign-then-use: the same two violations the inline forms above commit,
// written the way real code writes them.
export function lookupUserTainted(db: any, id: string) {
  const q = "SELECT * FROM users WHERE id = " + id;
  return db.query(q);
}

export function runTainted(dir: string) {
  const cmd = `ls ${dir}`;
  return exec(cmd);
}

// The structural shapes: a field, a helper's return value, a branch, a loop, a
// wrapped right-hand side, a transformation at the sink, and a container. Each
// one reported nothing before taint.js grew a statement model, a block-aware
// merge, path bindings and a return pass.
export function fieldTainted(db: any, id: string) {
  const o: any = {};
  o.q = "SELECT * FROM users WHERE id = " + id;
  return db.query(o.q);
}

function buildQuery(id: string) {
  return "SELECT * FROM users WHERE id = " + id;
}

export function returnedTainted(db: any, id: string) {
  const q = buildQuery(id);
  return db.query(q);
}

export function returnedInline(db: any, id: string) {
  return db.query(buildQuery(id));
}

export function branchTainted(db: any, id: string, all: boolean) {
  let q = "SELECT * FROM users";
  if (!all) {
    q = "SELECT * FROM users WHERE id = " + id;
  }
  return db.query(q);
}

export function loopTainted(db: any, clauses: string[]) {
  let q = "SELECT * FROM users WHERE 1=1";
  for (const clause of clauses) {
    q = q + clause;
  }
  return db.query(q);
}

export function wrappedTainted(db: any, id: string) {
  const q =
    "SELECT * FROM users WHERE id = " + id;
  return db.query(q);
}

export function trimmedTainted(db: any, id: string) {
  const q = "SELECT * FROM users WHERE id = " + id;
  return db.query(q.trim());
}

export function containerTainted(db: any, id: string) {
  const parts = ["SELECT * FROM users WHERE id = ", id];
  const q = parts.join("");
  return db.query(q);
}
