# RavenDB Operator

## Overview

This chart installs the RavenDB Kubernetes Operator -- the controller that watches `RavenDBCluster` custom resources and reconciles all underlying Kubernetes objects (StatefulSets, Services, Ingresses, Jobs, PVCs) needed to run secure, multi-node RavenDB clusters. It handles certificate management, bootstrapping, rolling upgrades with safety gates, external access, persistent storage orchestration, node lifecycle management, and continuous health/status evaluation -- all driven from a single `RavenDBCluster` spec.

It is the **cluster-wide infrastructure half** of the RavenDB Operator distribution. Per-cluster workload resources (the `RavenDBCluster` CR and its license/cert Secrets) are owned by the separate [`ravendb-cluster`](../cluster-chart/README.md) chart.

## What this chart installs

- The operator controller `Deployment` (cluster-scoped, runs in its own namespace, e.g. `ravendb-operator-system`).
- The `RavenDBCluster` CRD (`ravendb.ravendb.io/v1`).
- `ClusterRole` / `ClusterRoleBinding` and a `ServiceAccount` for the operator.
- `MutatingWebhookConfiguration` and `ValidatingWebhookConfiguration` for `RavenDBCluster` validation/defaulting.
- A cert-manager `Certificate` and `Issuer` for webhook TLS.

## What this chart does NOT do

- **Does not install cert-manager.** The chart depends on it for webhook TLS; install before installing this chart.
- **Does not deploy any `RavenDBCluster` resource.** Use the [`ravendb-cluster`](../cluster-chart/README.md) chart, or apply your own CR YAML.
- **Does not create license or certificate Secrets.** As of `2.0.0` this responsibility belongs to the cluster chart. See [Migrating from 1.x to 2.0.0](#migrating-from-1x-to-200) below.
- **Does not install ingress controllers, load balancers, or Traefik CRDs.** Those are platform-level concerns.

## Prerequisites

- A Kubernetes cluster, version **1.19 or higher** ([EKS](https://aws.amazon.com/eks/), [AKS](http://azure.microsoft.com/en-us/products/kubernetes-service), [kubeadm](https://kubernetes.io/docs/reference/setup-tools/kubeadm/), [kind](https://kind.sigs.k8s.io/), [minikube](https://minikube.sigs.k8s.io/docs/), …).
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/).
- [Helm](https://helm.sh/docs/intro/install/) v3.
- [cert-manager](https://cert-manager.io/) installed in the cluster.

## Installation

```bash
helm repo add ravendb-operator https://ravendb.github.io/ravendb-operator/helm
helm repo update

helm install ravendb-operator ravendb-operator/ravendb-operator \
  -n ravendb-operator-system --create-namespace
```

Verify the operator is running:

```bash
kubectl get pods -n ravendb-operator-system
kubectl get crd ravendbclusters.ravendb.ravendb.io
```

To run an actual RavenDB cluster, install the [`ravendb-cluster`](../cluster-chart/README.md) chart in a separate workload namespace.

## Configuration

The operator chart exposes a small surface focused on the controller deployment itself. Workload-side configuration lives in the cluster chart.

| Key | Default | Description |
|---|---|---|
| `controllerManager.image.repository` | `ravendb/ravendb-operator` | Controller image. |
| `controllerManager.image.tag` | `latest` | Controller image tag. Falls back to `appVersion` if empty. |
| `controllerManager.replicaCount` | `1` | Controller replica count. |
| `controllerManager.resources` | (unset) | Optional pod resource requests/limits. See `values.yaml` for the shape. |

See [`values.yaml`](values.yaml) for inline documentation.

## Pod security

The RavenDB pods the operator generates (StatefulSets and the bootstrapper Job)
ship a hardened security context, compatible with the PodSecurity `restricted`
profile out of the box:

- **Non-root, fixed identity.** Containers run as uid/gid `999`, matching the
  `USER ravendb` baked into the RavenDB image. The value is a single source of
  truth in the operator (`pkg/common.RavenDBUID`/`RavenDBGID`); a CI guard
  asserts the upstream image still uses it.
- **`fsGroup: 999`** on the pod, with `fsGroupChangePolicy: OnRootMismatch`. This
  lets the non-root process write to freshly provisioned PVCs, which some storage
  providers (e.g. Longhorn) otherwise mount root-owned, causing RavenDB to fail
  at startup with a `DataDir ... denied` error.
- **`runAsNonRoot: true`, `allowPrivilegeEscalation: false`, all capabilities
  dropped, `seccompProfile: RuntimeDefault`.** Binding port 443 does not need
  `NET_BIND_SERVICE`: the pod sets the safe sysctl
  `net.ipv4.ip_unprivileged_port_start=0`.

These values are not configurable: they are dictated by the RavenDB image, not by
the user.

## Migrating from 1.x to 2.0.0

**Breaking change.** The optional provisioning flow that allowed the operator chart to create license / client cert / per-node cert / self-signed Secrets via `--set-file provisioning.*` has been removed. Cluster-level Secrets are now owned by the dedicated [`ravendb-cluster`](../cluster-chart/README.md) chart.

**If you never used `--set-file provisioning.*`** (the default), no action is required.

**If you did use the provisioning flow**, the Secrets it created (`ravendb-license`, `ravendb-client-cert`, `ravendb-certs-<tag>`, `ravendb-cert`, `ravendb-ca-cert` -- whichever match your mode) are owned by the operator's Helm release. Because the templates that produced them no longer exist in `2.0.0`, `helm upgrade` to this version would otherwise delete them on transition, degrading any `RavenDBCluster` that references them.

**Before upgrading to `2.0.0`**, detach those Secrets from Helm's lifecycle by annotating them with `helm.sh/resource-policy=keep`. Substitute `<ns>` with the namespace where they live and adjust the list to match your mode/tags:

```bash
kubectl annotate secret ravendb-license       helm.sh/resource-policy=keep -n <ns>
kubectl annotate secret ravendb-client-cert   helm.sh/resource-policy=keep -n <ns>
# Let's Encrypt mode -- one per node tag:
kubectl annotate secret ravendb-certs-a       helm.sh/resource-policy=keep -n <ns>
kubectl annotate secret ravendb-certs-b       helm.sh/resource-policy=keep -n <ns>
kubectl annotate secret ravendb-certs-c       helm.sh/resource-policy=keep -n <ns>
# Self-signed mode -- replace the per-node block with:
# kubectl annotate secret ravendb-cert        helm.sh/resource-policy=keep -n <ns>
# kubectl annotate secret ravendb-ca-cert     helm.sh/resource-policy=keep -n <ns>
```

After upgrading, those Secrets remain in the namespace as orphans (no longer tracked by any Helm release). Either adopt them via the `ravendb-cluster` chart in BYO mode (point its `secrets.<slot>.name` values at the existing Secret names) or delete them manually with `kubectl delete secret ...` once you are sure they are no longer referenced.

## Upgrade

```bash
helm repo update
helm upgrade ravendb-operator ravendb-operator/ravendb-operator \
  -n ravendb-operator-system
```

The operator's reconciliation logic is backwards-compatible across minor versions of the same major. For major version transitions (e.g. `1.x` → `2.0.0`), read the migration section above first.

## Uninstall

```bash
helm uninstall ravendb-operator -n ravendb-operator-system
```

This removes the controller, CRD, RBAC, and webhooks managed by this chart. It does **not** automatically delete the namespace. **Existing `RavenDBCluster` resources and their workload Secrets are not deleted by this action** -- you should uninstall any `ravendb-cluster` releases first.

## Further reading

- [`RavenDBCluster` spec reference and feature list](../../README.md) (top-level repo README).
- [`ravendb-cluster` chart](../cluster-chart/README.md) -- workload-side companion.
- [Examples](../../examples) -- TLS modes, external access, storage, bootstrapping.
