---
name: procoder-threat
description: Map every trust boundary and what validates it — rung 1, SAFE. Use when the user says "procoder threat", "threat model", "trust boundaries", "attack surface", "security review", "where does untrusted input enter", "what's our exposure", or invokes /procoder-threat. Produces a boundary table with gaps, not a STRIDE essay.
---

# procoder-threat

You cannot validate a boundary you have not listed. This skill produces the list,
then checks each entry against the code.

## Procedure

**Step 0 — run the engine.** `node <plugin>/bin/procoder.js check <scope>`.
Its `safe/*` rules (`sql-injection`, `shell-injection`, `xss-sink`,
`unsafe-deserialize`, `hardcoded-secret`, `secret-in-log`, `pii-in-log`,
`dynamic-eval`, `tls-disabled`, `weak-hash`, `weak-random`, `xxe-risk`,
`unsafe-block`) are deterministic. Report them as-is and use them to seed the
sink column. Never re-derive them by reading files; never drop one.

**1 — Enumerate entry points.** Search by framework idiom for the languages
present. Every hit is a row.

| Entry kind | Search for |
|---|---|
| HTTP route handler | `app.get\|router.post\|@Get\|@RequestMapping\|@app.route\|http.HandleFunc\|[HttpPost]` |
| GraphQL resolver | `Resolver\|resolvers\s*[:=]\|@Query\|@Mutation` |
| gRPC method | generated service base classes, `RegisterXServer`, `ServiceImplBase` |
| Queue / topic consumer | `@KafkaListener\|consume(\|subscribe(\|SQS\|@RabbitListener\|on('message'` |
| Webhook receiver | route paths containing `webhook\|callback\|hooks/`, signature-verification helpers |
| CLI arguments | `process.argv\|argparse\|clap\|flag.Parse\|os.Args` |
| File / upload reader | `multer\|readFile\|open(\|FormFile\|IFormFile` |
| Environment & config | `process.env\|os.getenv\|Environment.Get\|viper\|std::env::var` |
| IPC / socket handler | `ipcMain.on\|net.createServer\|WebSocket\|unix socket` |
| Deserialization site | `JSON.parse\|pickle.loads\|yaml.load\|ObjectInputStream\|BinaryFormatter\|serde_json::from_str` |
| Third-party callback | OAuth redirect handlers, payment-provider callbacks, SSO assertion consumers |

**2 — Enumerate sinks.**

| Sink kind | Search for |
|---|---|
| SQL / ORM raw | `query(\|execute(\|raw(\|createQueryBuilder\|Statement\|.Raw(` with string concat or f-string |
| Shell / process | `exec(\|execSync\|spawn(\|subprocess\|os.system\|Runtime.exec\|Command::new` |
| Filesystem path | `path.join\|open(\|readFile\|File(` built from request data |
| HTTP client (SSRF) | `fetch(\|axios\|requests.get\|http.Get\|HttpClient` with a URL from input |
| Template / HTML render | `innerHTML\|dangerouslySetInnerHTML\|render_template_string\|\|safe\|v-html` |
| Deserializer | same list as entry deserialization |
| Redirect target | `redirect(\|Location:\|res.redirect` with input |
| Authorization decision | role/permission checks, `can(\|isAdmin\|@PreAuthorize\|policy` |

**3 — Trace.** For each entry point, follow the data to the sinks it can actually
reach. A boundary with no reachable sink is still a row; its gap is empty.

**4 — Answer four questions per boundary.** What validates the input (and is it
allowlist, at the boundary)? Where is authorization enforced, on which object,
server-side per request? What of this input is logged? What happens on malformed
input — reject loudly, or continue with a default?

## Output

One table. This is the deliverable.

| # | Boundary | Entry | Reaches sink | Validated by | Authz | Gap |
|---|---|---|---|---|---|---|
| 1 | POST /users | `api/users.ts:31` | SQL `db.ts:88` | zod `UserCreate` | route middleware | — |
| 2 | Stripe webhook | `api/hooks.ts:12` | SQL `billing.ts:40` | none | none | `[1 SAFE]` unverified signature |

`Gap` is `—` when the boundary is sound, or a one-line `[1 SAFE]` finding when it
is not. Follow the table with the findings in standard format, most severe first:

```
[1 SAFE]    api/hooks.ts:12   Stripe webhook body trusted without signature check → verify Stripe-Signature before parsing
```

Close with one line: `N boundaries, M with gaps.`

**At `paranoid` level**, add one clause per gap: what an attacker gets if it is
exploited.

## Do not

- Do not speculate about vulnerabilities you cannot trace to a file and line.
- Do not report framework-handled protections as gaps: an ORM's parameterized
  query, a template engine's auto-escaping, a framework's CSRF middleware.
- Do not produce a STRIDE essay or a threat-actor narrative. The table is the
  deliverable.
- Do not re-derive engine findings by eye, and do not omit one because the
  surrounding code looks safe.
- Do not fix anything. This skill reports.
