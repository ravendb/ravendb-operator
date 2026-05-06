---
name: pre-release-report
description: Run if user wants to prepare for release. Six-pass audit of the repo before cutting a release. Passes 1-2 (CSV markers, samples) propose changes for user approval and commit them. Passes 3-6 (breaking changes, Helm parity, reviewers, dependencies) report findings; the user directs follow-up.
---

Six-pass pre-release audit. Run them in order. End with a structured summary.

Use the current branch's ticket prefix (e.g., `RavenDB-26535`) for any commit subjects. Determine it from `git branch --show-current`.

## Pass 1 — CSV markers (action: propose → confirm → apply → commit)

Walk `api/v1/*.go`. For every exported field with a JSON tag, decide whether it needs a `+operator-sdk:csv:customresourcedefinitions:` marker.

**Skip:**
- `Spec`, `Status`, `Items`, `TypeMeta`, `ObjectMeta`, `ListMeta` — structural plumbing.
- Embedded k8s core types (`corev1.VolumeSource`, `corev1.EnvVar`, `corev1.SecretReference`).
- Pure container fields whose descriptors come from nested children (e.g., `Data`, `Logs`, `RavenDB`, `Audit` in `StorageSpec` — descriptors live on `Size`, `StorageClassName`, etc., reached via `storage.data.size` etc.).

**Mark:** spec fields → `type=spec`, status fields → `type=status`. Format:
`// +operator-sdk:csv:customresourcedefinitions:type=<spec|status>,displayName="<DisplayName>"`

DisplayName: CamelCase → "Space-Separated Words"; keep acronyms (URL, ID, IP, EIP, CA, TCP, DNS); prefer domain-clear (`CertSecretRef` → "Certificate Secret"). Match the style of existing markers in the same file. Do NOT add `description="..."` — operator-sdk's parser rejects it at this level.

Show proposals as `file:line  field  →  marker`, ask for confirmation/overrides, apply, run `make verify-samples` and (if available) `make bundle` in WSL to verify operator-sdk parses everything. Commit: `<ticket>: refresh CSV markers in api/v1`.

## Pass 2 — Samples (action: validate + propose updates → confirm → apply → commit)

First, run `make verify-samples`. If it fails, the strict-unmarshal error names the file/field — fix and re-run.

Then, judgment pass: diff `api/v1/` spec types against the most recent release tag (`git describe --tags --abbrev=0`).

For incremental additions (new optional fields, new array entries on existing types), ask: "Should this field appear in `ravendb_v1_ravendbcluster_letsencrypt.yaml`, `..._selfsigned.yaml`, both, or neither?"

For changes that introduce a fundamentally distinct use case — a new `mode:` value, a new external-access provider type, a new storage backend, a new auth scheme — propose adding a NEW sample file: `ravendb_v1_ravendbcluster_<scenario>.yaml`. The user may also request a new sample even for incremental changes if they want a separate example. Register any new sample in `config/samples/kustomization.yaml` so operator-sdk folds it into alm-examples.

Apply approved additions/files. Re-run `make verify-samples`. Commit: `<ticket>: update config/samples for new spec fields`.

## Pass 3 — CRD breaking-change review (report only)

Render the CRD at the most recent release tag and at HEAD, then diff. Render with `kustomize build config/crd | yq` (or operator-sdk equivalent). Surface:

| Change | Severity | Why it matters |
|---|---|---|
| Removed field | High | Breaks users who set it |
| Type change | High | Wire-incompatible |
| New required field, no default | High | Breaks existing CRs on upgrade |
| Narrowed enum or smaller min/max | Medium | Breaks users at the boundary |
| Changed default | Medium | Silent behavior change |

Output severity, field path, before → after. Do NOT auto-fix — these are deliberate decisions (keep with a major version bump, document migration, or revert).

## Pass 4 — Helm chart parity (report only)

`helm/chart/templates/` and `config/` are maintained in parallel. Diff them across:
- **RBAC**: verbs/resources/apiGroups in `config/rbac/role.yaml` vs `helm/chart/templates/rbac.yaml` (or equivalent).
- **Manager Deployment**: env vars, volume mounts, args, resource limits in `config/manager/manager.yaml` vs `helm/chart/templates/deployment.yaml`.
- **Webhook**: cert-manager Issuer/Certificate refs, service ports.

Report drift as `area | config has | helm has | risk`. Do NOT auto-fix — which side is canonical depends on what the change was for.

## Pass 5 — Reviewer list (report + interactive)

Read `bundle/ravendb-operator/ci.yaml`. Surface the current `reviewers:` list (or note absence — without it OperatorHub auto-merge doesn't fire). Ask: "Is this list current? Anyone to add or remove?" If the user wants edits, propose, confirm, apply, commit `<ticket>: refresh OperatorHub reviewers list`.

## Pass 6 — Dependencies audit (report + interactive, top-level only)

Surface pin vs latest for these only — don't go deep into transitive Go modules:

- `OPERATOR_SDK_VERSION` in `Makefile` vs latest `operator-framework/operator-sdk` GitHub release.
- `CONTROLLER_TOOLS_VERSION`, `KUSTOMIZE_VERSION`, `ENVTEST_VERSION`, `GOLANGCI_LINT_VERSION` in `Makefile`.
- `go` directive in `go.mod` vs current stable Go.
- cert-manager version in `.github/workflows/release.yml` smoke-test URL.
- GitHub Action versions in `.github/workflows/*.yml` — surface floating tags (`@v3`, `@v4`); recommend SHA pinning where missing.

Report as `dep | pinned | latest | gap | recommendation`. Recommendation tone: "consider bumping" or "looks current". Do NOT auto-bump — bumps need a deliberate test cycle, not a release-prep slipstream. Ask the user if they want to schedule any bumps as separate work.

## Final summary

```
Pre-release report — <ticket>
=============================
Pass 1 (CSV markers):     <committed sha | "no gaps">
Pass 2 (Samples):         <committed sha | "no updates needed">
Pass 3 (CRD breaking):    <N items by severity>
Pass 4 (Helm parity):     <N drift items>
Pass 5 (Reviewers):       <committed sha | "current">
Pass 6 (Dependencies):    <N items behind>
```

After this report, the user has a clear picture of what's actionable before invoking `prepare-release.yml`.
