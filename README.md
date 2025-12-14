# RavenDB Kubernetes Operator

<table>
<tr>
<td>

The RavenDB Kubernetes Operator provides a fully automated way to deploy and manage secure RavenDB clusters on Kubernetes. 
It handles certificate management, bootstrapping, rolling upgrades with safety gates, external access, persistent storage orchestration, node lifecycle management, and continuous health and status evaluation - all driven from a single `RavenDBCluster` custom resource.
The operator ensures that every component stays aligned with the declared spec, enabling fully reproducible, declarative RavenDB deployments.

</td>
<td align="right" width="250">

<img src="assets/logo.png" width="250"/>

</td>
</tr>
</table>

---
## Table of Contents

- [Prerequisites](#Prerequisites)  
- [Installation](#installation)  
- [Custom Resource (RavenDBCluster)](#custom-resource-ravendbcluster)  
- [Features](#features)


---
## Prerequisites
Before installing the RavenDB Kubernetes Operator, ensure your environment meets the following requirements:
- A running 1.19 or higher **Kubernetes cluster** (such as: [EKS](https://aws.amazon.com/eks/), [AKS](http://azure.microsoft.com/en-us/products/kubernetes-service), [Kubeadm-based-clusters](https://kubernetes.io/docs/reference/setup-tools/kubeadm/), [Kind](https://kind.sigs.k8s.io/), [Minikube (1.25 or higher)](https://minikube.sigs.k8s.io/docs/))
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [Helm](https://helm.sh/docs/intro/install/)
- [cert-manager](https://cert-manager.io/) - The RavenDB Operator uses admission webhooks, which require TLS certificates.
- [RavenDB License](https://ravendb.net/buy)
- [RavenDB Let's Encrypt Setup Package](https://docs.ravendb.net/7.1/server/security/authentication/certificate-configuration)/Self Signed Certificates Materials

## Installation

There are three supported installation methods: 

1. **Helm (recommended)**  
   - Install the operator using the official chart published on ArtifactHub.
2. **make deploy (development / local testing)**  
   - Deploys the operator using the raw manifests under `config/` 
3. **OLM bundle installation**
   - For clusters running Operator Lifecycle Manager (OLM); installs the operator via its bundle.


### 1. Install via Helm (recommended)
The Helm chart is the recommended and simplest way to deploy the RavenDB Operator.  
> **Note:**  The Helm chart deploys **only the RavenDB Operator**. A RavenDB cluster will start **only after** you:
> - create the required Secrets via the chart/manually 
> - apply a `RavenDBCluster` custom resource that uses them. 


**Add the Helm repository**
```bash
helm repo add ravendb-operator https://<TODO>
helm repo update
```

Install the Operator by choosing one of the following flows:
<details>
<summary><strong>1. Install the operator only and create all Secrets yourself with <code>kubectl</code></strong>. </summary>

In this flow, you are responsible for creating all required Secrets in the `ravendb` namespace:


```bash
kubectl create secret generic ravendb-license --from-file=license.json=/path/to/license.json -n ravendb
kubectl create secret generic ravendb-client-cert --from-file=client.pfx=/path/to/admin-client-cert.pfx -n ravendb
kubectl create secret generic ravendb-certs-a --from-file=server.pfx=/path/to/node-a/server-cert.pfx -n ravendb
kubectl create secret generic ravendb-certs-b --from-file=server.pfx=/path/to/node-b/server-cert.pfx -n ravendb
kubectl create secret generic ravendb-certs-c --from-file=server.pfx=/path/to/node-b/server-cert.pfx -n ravendb
```

Once all required Secrets are created and available in the `ravendb` namespace, you may proceed to install the Helm chart that deploys the RavenDB Operator.

```bash
helm install ravendb-operator -n ravendb-operator-system --create-namespace
```

> **Notes:**  
> - You may choose any Secret names you want - just make sure to reference them correctly later inside your RavenDBCluster spec.
> - You can deploy as many nodes as you wish; create one server-certificate Secret per node and map them to node tags later inside your RavenDBCluster spec
> - The example above demonstrates manual Secret creation for a Let's Encrypt–based setup. For instructions on obtaining the setup package, see the prerequisites section above.
> 
</details>


<details>
<summary><strong>2. Install the operator and let the chart create Secrets - <code>Let's Encrypt Mode</code></strong>. </summary>

In this flow, the Helm chart will install the operator and create all required Secrets in the `ravendb` namespace for you,
using paths you provide to the setup package artifacts.

```bash
helm install ravendb-operator ./helm/chart -n ravendb-operator-system --create-namespace \
    --set "provisioning.nodeTags={a,b,c}" \
    --set-file provisioning.licenseJson=/path/to/license.json \
    --set-file provisioning.clientPfx=/path/to/admin-client-cert.pfx \
    --set-file provisioning.nodePfx.a=/path/to/node-a/server-cert.pfx \
    --set-file provisioning.nodePfx.b=/path/to/node-b/server-cert.pfx \
    --set-file provisioning.nodePfx.c=/path/to/node-c/server-cert.pfx
```

This command will:
- Deploys the operator into `ravendb-operator-system` 
- Creates a `ravendb` namespace 
- Automatically generate all required Secrets for the license, the client certificate, and per-node server certificates.

> **Notes:**  
> - Secret names produced by the chart follow a predictable pattern, but you may override them if needed (see more in `values.yaml') just make sure your RavenDBCluster spec references the correct names.
> -  You can deploy as many nodes as you wish. Simply provide one server certificate file per node to Helm, and the chart will create the corresponding Secrets and map them to node tags considering the provisioning.nodeTags you provided -  the tags must match the node definitions you selected when generating the Setup Package.
> - This example demonstrates using certificates from a RavenDB Setup Package. For instructions on obtaining this package, refer to the prerequisites section above.
>
</details>


<details>
<summary><strong>3. Install the operator and let the chart create Secrets - <code>Self Signed Mode</code></strong>. </summary>

In this flow, the Helm chart will install the operator and create all required Secrets in the `ravendb` namespace for you,
using paths you provide to single server PFX, a client PFX, and the CA certificate.

```bash
 helm install ravendb-operator -n ravendb-operator-system --create-namespace \
  --set provisioning.mode=None \
  --set "provisioning.nodeTags={a,b,c}" \
  --set-file provisioning.licenseJson=/path/to/license.json \
  --set-file provisioning.clientPfx=/path/to/admin-client-cert.pfx \
  --set-file provisioning.serverPfx=/path/to/server-cert.pfx  \
  --set-file provisioning.caCrt=/path/to/ca.crt
```

This command will:
- Deploys the operator into `ravendb-operator-system` 
- Creates a `ravendb` namespace 
- Automatically generate all required Secrets for the license, the client certificate, and per-node server certificates.

> **Notes:**  
> - Secret names produced by the chart follow a predictable pattern, but you may override them if needed (see more in `values.yaml') just make sure your RavenDBCluster spec references the correct names.
> -  You can deploy as many nodes as you wish. Simply provide one server certificate file to Helm, and the chart will create the corresponding Secrets and map them to node tags considering the provisioning.nodeTags you provided.
>
</details>


### 2. make deploy (development / local testing)

This installation method is intended **only for local development**, contributing to the operator, or testing changes before building a production image.  
It uses the manifests generated by Kubebuilder under the `config/` directory and deploys the operator **directly from your local source tree**.

```bash
git clone https://github.com/ravendb/ravendb-operator.git
cd ravendb-operator
# Option A: If you're modifying code or testing custom logic, build the operator binary.
# This compiles the controller and ensures your local environment is correct before deploying.
make build

#Option B: If you want to test the operator inside your own cluster (kind, minikube, or remote):
make docker-build IMG=<your-registry>/<repo>:<tag>
make docker-push IMG=<your-registry>/<repo>:<tag> # or load it inside your cluster

# Uninstall (clean up)
make undeploy
```

### 3. **OLM bundle installation**  
```bash
# Install Operator Lifecycle Manager (OLM), a tool to help manage the Operators running on your cluster.
$ curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.38.0/install.sh | bash -s v0.38.0

# Install the operator by running the following command
$ kubectl create -f https://operatorhub.io/install/ravendb-operator.yaml

# After install, watch the operator come up using next command
$ kubectl get csv -n operators
```
---

## Custom Resource (`RavenDBCluster`)

The `RavenDBCluster` custom resource is the **single source of truth** for your RavenDB deployment.  
It defines node topology, TLS mode (Let's Encrypt or self-signed), external access strategy (Ingress / LoadBalancer), storage layout, images, and bootstrap behavior.  
Once the operator reconciles this resource, it will create and manage all underlying Kubernetes objects (StatefulSets, Services, Ingresses, Jobs, PVCs, etc.) needed to run the cluster.
>Note: Feel free to shape the CRD however you need - the validation webhooks have your back, watching for misconfigurations and letting you know right away if something doesn’t look right.

For a deeper dive into each aspect of the spec, see the dedicated examples and documentation:
- **TLS modes (Let's Encrypt / Self-Signed)** [examples/tls/README.md](examples/tls/readme.md)
- **External access options**  
  - AWS NLB: [examples/networking/external_access/aws-nlb/README.md](examples/networking/external_access/aws-nlb/readme.md)
  - Azure LB: [examples/networking/external_access/azure-lb/README.md](examples/networking/external_access/azure-lb/readme.md)
  - HAProxy: [examples/networking/external_access/haproxy/README.md](examples/networking/external_access/haproxy/readme.md)
  - Traefik: [examples/networking/external_access/traefik/README.md](examples/networking/external_access/traefik/readme.md)
  - NGINX: [examples/networking/external_access/nginx/README.md](examples/networking/external_access/nginx/readme.md)
- **Storage options** [examples/storage/README.md](examples/storage/readme.md)
- **Cluster bootstrapping** [examples/cluster/README.md](examples/cluster/readme.md)

Below is an **example** of a **Let's Encrypt–based** cluster using the **AWS-NLB** external access mode and **Perssist/Restoring Data**. 

```yaml
apiVersion: ravendb.ravendb.io/v1
kind: RavenDBCluster
metadata:
  labels:
    app.kubernetes.io/name: ravendb-operator
  name: ravendbcluster-sample
  namespace: ravendb
spec:
  nodes:
    - tag: a
      publicServerUrl: https://a.domain.development.run:443
      publicServerUrlTcp: tcp://a-tcp.domain.development.run:443
      certSecretRef: ravendb-certs-a
    - tag: b
      publicServerUrl: https://b.domain.development.run:443
      publicServerUrlTcp: tcp://b-tcp.domain.development.run:443
      certSecretRef: ravendb-certs-b
    - tag: c
      publicServerUrl: https://c.domain.development.run:443
      publicServerUrlTcp: tcp://c-tcp.domain.development.run:443
      certSecretRef: ravendb-certs-c
  image: ravendb/ravendb:latest
  imagePullPolicy: IfNotPresent
  mode: LetsEncrypt
  email: user@ravendb.net
  licenseSecretRef: ravendb-license
  clientCertSecretRef: ravendb-client-cert
  domain: domain.development.run
  env:
    RAVEN_Features_Availability: "Experimental"
  externalAccessConfiguration:
    type: aws-nlb
    awsExternalAccessContext:
      nodeMappings:
        - tag: a
          eipAllocationId: eipalloc-0f12ab34cd56ef789
          subnetId: subnet-0aa1bb22cc33dd44e
          availabilityZone: us-east-1a
        - tag: b
          eipAllocationId: eipalloc-0123abcd4567ef890
          subnetId: subnet-0555aa66bb77cc88d
          availabilityZone: us-east-1b
        - tag: c
          eipAllocationId: eipalloc-0abc1234def567890
          subnetId: subnet-0999ee11ff22gg33h
          availabilityZone: us-east-1c
  storage:
    data:
      size: 10Gi
      storageClassName: gp3
      accessModes:
        - ReadWriteOnce
    logs:
      ravendb:
        size: 1Gi
        storageClassName: gp2
        accessModes:
          - ReadWriteOnce
      audit:
        size: 1Gi
        storageClassName: gp2
    additionalVolumes:
        - name: scripts
            mountPath: /tmp/scripts
            volumeSource:
            configMap:
                name: myscripts
        - name: restore-volume
            mountPath: /tmp/restore
            volumeSource:
            persistentVolumeClaim:
                claimName: restore-backup-pvc      
```

---

## Features
#### Certificate and TLS Management
- Let's Encrypt mode using RavenDB’s built-in LE support
- Self-signed / bring-your-own certificate mode
- Works with:
  - Per-node certificates from a RavenDB Setup Package.
  - A single shared server certificate in self-signed scenarios.
- Manages webhook serving certificates through cert-manager.
- Handles certificate provisioning for nodes and clients.
- Automatically rotated secrets upon server-certificate renewal

#### Cluster Bootstrapper
- Runs a short-lived Kubernetes `Job` to:
  - Wait for all RavenDB pods to be running.
  - Validate HTTPS reachability on all nodes.
  - Register the ClusterAdmin client certificate on the leader node.
  - Form the cluster (leader + members/watchers) based on `spec.nodes`.
- Declarative definition of node topology, URLs, and certificate references.

#### External Access Management
- Supports multiple exposure mechanisms:
  - AWS Network Load Balancer (one NLB per node, with explicit `tag` → EIP/subnet/AZ mapping).
  - Azure Load Balancer (per-node public IPs via AKS-managed SLB frontends).
  - NGINX Ingress.
  - HAProxy Ingress.
  - Traefik Ingress.

#### Storage Configuration
- Declarative configuration of data, log, and audit volumes.
- Supports Kubernetes storage backends including:
  - local-path
  - AWS EBS / EBS-CSI
  - Azure Disk / Azure Disk CSI
  - Additional volumes (ConfigMap, Secret, PVC, emptyDir)

#### Environment Options
  - Ability to set environment variables on RavenDB pods via `spec.env` (for feature flags and advanced configuration).

#### Rolling Upgrades
- Orchestrates rolling upgrades of RavenDB nodes.
- Ensures availability and ordering requirements during updates.
- Performs node-by-node upgrades in the order defined in the `RavenDBCluster` spec.
- Stops the upgrade on failed gates, keeps the state visible in status and Events, and automatically resumes from the same node once the underlying issue is fixed.
- Prevents accidental version downgrades.

#### Health and Status Reporting
- Exposes a single lifecycle field, `.status.phase` (`Deploying`, `Running`, `Error`), plus a short `.status.message` explaining the current state.
- Maintains detailed `.status.conditions[]` for `Ready`, `Progressing`, `Degraded`, and required health gates such as `CertificatesReady`, `LicensesValid`, `StorageReady`, `NodesHealthy`, `BootstrapCompleted`, and `ExternalAccessReady`.
- Derives `phase` deterministically from these conditions and **emits Kubernetes Events** on every condition transition, so `kubectl describe ravendbclusters <name>` shows exactly what is blocking readiness and why.

#### Development and Testing Support
- Local deployment via `make deploy` without requiring Helm or OLM.
- Validating and mutating admission webhooks for CRD correctness.
- Server-side apply for consistent updates and ownership.
- Supports incremental and partial reconciliations based on resource changes.


