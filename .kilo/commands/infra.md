---
description: "DevOps hygiene where the files exist: Dockerfiles, Terraform, Kubernetes manifests, Helm charts."
---

The user invoked /procoder:infra.

The command below is the `procoder` binary on PATH.

1. Run `procoder infra`. Each instrument answers only where its files
   exist:
   - **Dockerfiles** (hadolint) — pin base-image tags, pin package
     versions; judge each warning honestly.
   - **Terraform** — `fmt` findings you apply like any formatter output;
     a FAILED `terraform validate` BLOCKS (objectively broken code);
     "NOT validated — not initialised" means run `terraform init` first
     when the user wants validation.
   - **Kubernetes manifests** (kubeconform) — schema violations with the
     exact field named; fix them.
   - **Helm charts** (`helm lint`) — errors and warnings to judge.
2. NOT-checked lines mean a tool is missing — `procoder init` installs
   exactly what this repository's inventory needs, nothing more.
3. Fix, re-run, and show the user the report with your judgments.
