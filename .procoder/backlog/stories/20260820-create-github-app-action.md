# Create GitHub App + Action for GitHub Marketplace

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

Create a GitHub App manifest and a GitHub Action workflow that enables Procoder to run quality checks on PRs and commits. The App is submitted to GitHub Marketplace with categories `code-quality`, `agent-apps`, and `code-review`. The Action runs the procoder gate CI pipeline on PR events.

## Acceptance criteria

- [ ] `.github/workflows/procoder-gate.yml` created with PR trigger
- [ ] Workflow triggers on: `pull_request`, `push`, `check_run`
- [ ] Job runs: `procoder check`, `procoder lint`, `procoder security`
- [ ] Post findings as PR comments where possible
- [ ] Sets CI status on the PR
- [ ] `github-app/MANIFEST.json` created with App registration template
- [ ] Manifest has: name, description, url, webhook_url, public
- [ ] Manifest subscribes to events: pull_request, push, check_run
- [ ] Manifest has redirect_url and setup_url
- [ ] README documents the GitHub App workflow

## Evidence
