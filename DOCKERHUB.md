# RavenDB Kubernetes Operator

`ravendb/ravendb-operator` is the controller image for the **RavenDB Kubernetes Operator**. It runs inside your Kubernetes cluster and reconciles `RavenDBCluster` custom resources into fully managed, secure RavenDB clusters: certificate management, cluster bootstrap, rolling upgrades with safety gates, external access, persistent storage, and continuous health evaluation.

> **This is the operator, not the database.** The RavenDB server image is [`ravendb/ravendb`](https://hub.docker.com/r/ravendb/ravendb). You do not run this image with `docker run`; it is deployed into Kubernetes via Helm or OLM (see below), and the operator then runs RavenDB server pods for you.

## Supported tags

- `latest`, and semantic-version tags such as `2.0.0`
- Multi-arch: `linux/amd64`, `linux/arm64`

## Install

The operator is distributed as two Helm charts (the operator once per cluster, then one cluster chart per RavenDB cluster) or as an OLM bundle. You point Kubernetes at it, not Docker.

```bash
helm repo add ravendb-operator https://ravendb.github.io/ravendb-operator/helm
helm repo update

# cert-manager is a prerequisite (admission-webhook TLS)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.0/cert-manager.yaml

helm install ravendb-operator ravendb-operator/ravendb-operator \
  -n ravendb-operator-system --create-namespace
```

Then deploy a cluster with the `ravendb-cluster` chart, or apply a `RavenDBCluster` CR directly. The full walkthrough is in the guide linked below.

## What it does

- Provisions and rotates TLS certificates (Let's Encrypt or self-signed)
- Bootstraps the cluster and forms the node topology
- Rolling upgrades with health gates and version-safety checks
- External access via Ingress (Traefik, NGINX, HAProxy) or cloud load balancers (AWS NLB, Azure LB)
- Persistent storage orchestration (data, logs, audit, extra volumes)
- One `RavenDBCluster` per namespace, with operator-managed RBAC

## Links

- Source and docs: https://github.com/ravendb/ravendb-operator
- Guide: https://docs.ravendb.net/guides/the-ravendb-kubernetes-operator-way
- Helm charts on ArtifactHub: https://artifacthub.io/packages/helm/ravendb-operator/ravendb-operator
- OperatorHub: https://operatorhub.io/operator/ravendb-operator
- RavenDB server image: https://hub.docker.com/r/ravendb/ravendb

The operator is open source (Apache-2.0). Running RavenDB requires a RavenDB license (free developer and community options at https://ravendb.net/buy).
