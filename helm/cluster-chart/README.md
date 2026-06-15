# RavenDB Cluster

## Overview

This chart deploys a single `RavenDBCluster` custom resource into a dedicated namespace, optionally provisions the cert and license Secrets the cluster depends on, and renders any cluster-specific extras the operator does not create itself (currently: Traefik `IngressRouteTCP`s).

It is the workload-facing companion to the [RavenDB Operator chart](../chart/README.md). The operator chart installs the controller and CRDs once per Kubernetes cluster; this chart is installed once per RavenDB cluster you want to run - always in its own namespace.

## What this chart does

- Deploys resources into the release namespace (the namespace must already exist, or be created at install time with `helm install --create-namespace`).
- Provisions cert and license Secrets from `--set-file` inputs (or references pre-existing Secrets when no file is provided).
- Renders a `RavenDBCluster` CR from `values.yaml` (values mirror `RavenDBClusterSpec` 1:1).
- When `externalAccessConfiguration.type == ingress-controller` and `ingressClassName == traefik`, renders one `IngressRouteTCP` per node for HTTPS (port 443) and TCP (port 38888), with TLS passthrough. Hostnames are extracted from `publicServerUrl` and `publicServerUrlTcp`. Both routes target the per-node Service `ravendb-<tag>` (created by the operator). Traefik entrypoint names are configurable via `traefik.entryPoints.https` and `traefik.entryPoints.tcp`.

## What this chart does NOT do

- **Does not install the RavenDB Operator.** Install it first via the operator chart.
- **Does not install cert-manager.** Required by the operator for webhook TLS; install before the operator.
- **Does not install ingress controllers** (Traefik, NGINX, HAProxy) or load balancer add-ons (MetalLB, AWS Load Balancer Controller, etc.).
- **Does not generate certificates.** Use the RavenDB setup package (`rvn create-setup-package`) for Let's Encrypt or supply your own self-signed material.
- **Does not install Traefik CRDs.** Required when using Traefik external access.

## Prerequisites

- A running Kubernetes cluster.
- [cert-manager](https://cert-manager.io/) installed (operator dependency).
- The [RavenDB Operator](../chart/README.md) installed.
- For Traefik external access: Traefik installed in the cluster and Traefik CRDs (`traefik.io/v1alpha1`) registered.

## Secrets - hybrid provisioning model

For each Secret slot the cluster needs, `values.yaml` exposes:

- `name`: the Kubernetes Secret name (used both as the name to create and the name the CR references).
- `file`: optional file content set via `--set-file`.

**Behavior:**
- If `file` is set → the chart creates the Secret with `name`.
- If `file` is empty → the chart skips creation and assumes a Secret with `name` already exists in the release namespace.

The chart auto-wires `licenseSecretRef`, `clientCertSecretRef`, `clusterCertSecretRef`, `caCertSecretRef`, and per-node `certSecretRef` on the CR from these names. Users do not set the `*SecretRef` fields directly.

| Secret slot | values key | Default name | Required when |
|---|---|---|---|
| License | `secrets.license` | `ravendb-license` | always |
| Admin client cert | `secrets.clientCert` | `ravendb-client-cert` | always |
| Per-node server cert | `secrets.nodeCerts.files.<tag>` | `ravendb-certs-<tag>` | always in `mode: LetsEncrypt` (the Secret must already exist or be created by this chart for admission to succeed) |
| Cluster server cert | `secrets.clusterCert` | `ravendb-cert` | always in `mode: None` |
| CA cert | `secrets.caCert` | `ravendb-ca-cert` | always in `mode: None` |

## Installation

### Provisioning flow (chart creates Secrets)

```bash
helm install my-cluster ravendb-cluster/ravendb-cluster \
  -n ravendb --create-namespace \
  -f my-values.yaml \
  --set-file secrets.license.file=/path/to/license.json \
  --set-file secrets.clientCert.file=/path/to/admin-client-cert.pfx \
  --set-file secrets.nodeCerts.files.a=/path/to/node-a/server.pfx \
  --set-file secrets.nodeCerts.files.b=/path/to/node-b/server.pfx \
  --set-file secrets.nodeCerts.files.c=/path/to/node-c/server.pfx
```

### BYO Secrets flow (Secrets pre-created)

```bash
kubectl create namespace ravendb
kubectl create secret generic ravendb-license \
  --from-file=license.json=/path/to/license.json -n ravendb
kubectl create secret generic ravendb-client-cert \
  --from-file=client.pfx=/path/to/admin-client-cert.pfx -n ravendb
# ... per-node cert Secrets ...

helm install my-cluster ravendb-cluster/ravendb-cluster \
  -n ravendb \
  -f my-values.yaml
```

Pass `--create-namespace` if the namespace doesn't already exist. Installing into the `default` namespace is rejected by the chart.

> **One cluster per namespace.** The operator's validation webhook enforces a single `RavenDBCluster` per namespace. To run multiple clusters, install this chart multiple times with different `-n <namespace>` values.

## Values

`values.yaml` is organized in a few top-level blocks:

| Key | Description |
|---|---|
| `nameOverride` | Override the `RavenDBCluster` resource name. Defaults to the Helm release name. |
| `secrets.*` | Secret slot definitions (name + optional file). See [Secrets — hybrid provisioning model](#secrets--hybrid-provisioning-model). |
| `spec.*` | Mirrors `RavenDBClusterSpec` 1:1. Required fields fail at template time if missing. |
| `traefik.entryPoints.https` | Traefik entrypoint name for HTTPS routes. Default: `websecure`. |
| `traefik.entryPoints.tcp` | Traefik entrypoint name for TCP routes. Default: `tcp`. |

See [`values.yaml`](values.yaml) for the full shape and inline documentation, including all `spec.*` fields (nodes, mode, domain, external access types, storage, env, additional volumes, …).

## Verifying the cluster

After install, watch the `RavenDBCluster` come up:

```bash
kubectl get ravendbcluster -n <release-namespace> -w
kubectl describe ravendbcluster <release-name> -n <release-namespace>
kubectl get pods -n <release-namespace>
```

The operator surfaces progress via `.status.phase` (`Deploying` / `Running` / `Error`), `.status.message`, and detailed `.status.conditions[]` (`Ready`, `Progressing`, `Degraded`, `CertificatesReady`, `LicensesValid`, `StorageReady`, `NodesHealthy`, `BootstrapCompleted`, `ExternalAccessReady`). Every condition transition also produces a Kubernetes Event, so `kubectl describe` shows exactly what is blocking readiness and why.

## Upgrade

Re-render the chart with whatever values you want to change. The most common pattern is `--reuse-values` plus targeted `--set` overrides:

```bash
helm repo update
helm upgrade <release-name> ravendb-operator/ravendb-cluster \
  -n <release-namespace> \
  --reuse-values \
  --set spec.env.RAVEN_Logs_Mode=Information \
  --wait
```

The chart re-renders the `RavenDBCluster` CR; the operator's reconciliation, validation webhook, and rolling-upgrade logic apply the change cluster-side. Some spec fields are immutable -- the operator's webhook will reject the upgrade at admission time if you try to change them; `helm upgrade --atomic` cleanly reverts Helm state in that case.

For Secret rotation, re-pass the relevant `--set-file secrets.<slot>.file=…` argument; the chart updates the Secret in place. The operator picks up the new material via its existing watch.

## Uninstall

```bash
helm uninstall <release-name> -n <release-namespace>
```

This removes the `RavenDBCluster` CR, the Secrets the chart created (provisioning mode), and any Traefik `IngressRouteTCP`s rendered by the chart. The operator then deletes the underlying StatefulSets, Services, and Jobs as the CR is finalized. Persistent volumes follow your `PersistentVolumeClaim` reclaim policy (the chart does not force-delete them).

If you used **BYO mode** (Secrets created with `kubectl`, referenced by name), those Secrets are not owned by the chart and will not be removed -- delete them manually if you no longer need them.

## Limitations and known constraints

- **No `default` namespace.** The chart fails template rendering if the release namespace is `default`.
- **Helm cannot detect missing CRDs at template time.** If Traefik CRDs are not installed but the chart renders `IngressRouteTCP`s, the install fails at apply time with `no matches for kind "IngressRouteTCP"`. Expected behavior — install Traefik first.
- **Mode-conditional Secrets.** `secrets.nodeCerts.files.*` only valid in `mode: LetsEncrypt`. `secrets.clusterCert.file` and `secrets.caCert.file` only valid in `mode: None`. Misuse fails at template time with a clear error.
- **Helm `--atomic` covers Helm state only.** It rolls back the CR spec if apply fails. It does not roll back operator-driven reconciliation that has already begun.
- **Immutable spec fields.** Some fields (e.g., image downgrades) are rejected by the operator's webhook. `helm upgrade` will fail at admission; `--atomic` will revert Helm state cleanly.
- **One `RavenDBCluster` per namespace.** Enforced by the operator's validation webhook. To run multiple RavenDB clusters, install this chart multiple times with different `-n <namespace>` values.

## Further reading

- [Top-level repo README](../../README.md) -- full feature list, supported external access modes, dev/OLM install paths.
- [`ravendb-operator` chart](../chart/README.md) -- installs the controller, CRDs, and webhooks (prerequisite for this chart).
- [Examples](../../examples) -- TLS modes, external access (AWS NLB / Azure LB / NGINX / HAProxy / Traefik), storage, bootstrapping.
