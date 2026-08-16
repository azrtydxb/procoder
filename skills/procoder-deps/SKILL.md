---
name: procoder-deps
description: Audit dependencies for known vulnerabilities, abandonment, floating versions, and unused packages. Use when the user says "procoder deps", "dependency audit", "supply chain", "are my packages safe", "vulnerable dependencies", "check my dependencies", or invokes /procoder-deps. Reports what each ecosystem's own auditor says; it never guesses at CVEs.
---

# procoder-deps

A new dependency is a new trust boundary, and most real CVEs arrive through one.

## Procedure

1. **Detect ecosystems.** Read the manifests present at the repo root (and at
   each workspace root in a monorepo), or run:
   `node -e "console.log(JSON.stringify(require('<plugin>/hooks/checks/deps').detectEcosystems(process.cwd())))"`

2. **Run each ecosystem's own auditor.** Never substitute your own judgment for
   a tool's output.

   | Ecosystem | Manifest | Command | Install if missing |
   |---|---|---|---|
   | npm | `package.json` | `npm audit --json` | ships with Node |
   | python | `pyproject.toml`, `requirements.txt` | `pip-audit --format json` | `pipx install pip-audit` |
   | go | `go.mod` | `govulncheck ./...` | `go install golang.org/x/vuln/cmd/govulncheck@latest` |
   | rust | `Cargo.toml` | `cargo audit --json` | `cargo install cargo-audit` |
   | dotnet | `*.csproj`, `Directory.Packages.props` | `dotnet list package --vulnerable --include-transitive` | ships with the SDK |

   If a tool is absent, say so in one line and name its install command. Do not
   report vulnerabilities for that ecosystem.

3. **Check pinning and lockfiles.** Run
   `node <plugin>/bin/procoder.js check <manifest>` — the engine reports
   `safe/missing-lockfile` and `safe/floating-version` deterministically. Report
   those as-is.

4. **Check abandonment.** For each top-level dependency, get the last release
   date (`npm view <pkg> time.modified`, `pip index versions <pkg>`,
   crates.io / pkg.go.dev). Over two years with no release is a finding.

5. **Check unused.** For each declared dependency, `git grep -n "<pkg>"` across
   source. Declared and imported nowhere is a `[4 ALONE]` finding. Build-only
   and plugin-loaded packages (linters, type packages, framework plugins) are
   not unused — check the config files before flagging.

## Output

Standard one-line findings, grouped by ecosystem, most severe first:

```
[1 SAFE]    package.json:14   lodash 4.17.15 — GHSA-35jh-r3h4-6jhm (npm audit, high) → upgrade to 4.17.21
[1 SAFE]    package.json:1    npm manifest with no lockfile committed → commit package-lock.json
[1 SAFE]    package.json:22   left-pad declared as latest → pin to an exact version
[4 ALONE]   package.json:19   moment declared, imported nowhere → remove
```

One line per finding. Close with: `N dependencies, M vulnerable, K unused.`
Name the auditor and severity for every vulnerability, in the fix clause.

## Do not

- Do not report a CVE you did not get from a tool. No advisory, no finding.
- Do not report anything for an ecosystem whose auditor is absent — say the tool
  is missing instead.
- Do not recommend crossing a major version without saying it is breaking.
- Do not suggest adding a dependency to solve a dependency problem.
- Do not re-derive pinning or lockfile findings by eye; the engine computes them.
