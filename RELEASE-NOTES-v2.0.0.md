v2.0.0 - Decoupled Two-Chart Deployment, Cluster-Owned Secrets & Native Traefik Ingress 🧩

2.0 splits the operator and your RavenDB clusters into two separate Helm charts. You install the operator once per Kubernetes cluster. It provides the controller, CRDs, RBAC, and webhooks that every cluster relies on. From then on, each RavenDB cluster ships as its own chart, bundling its CR, Secrets, and Traefik routes. Because every cluster is now its own Helm release, you can deploy, upgrade, or remove one without touching the others or the operator. And spinning up a new cluster is still a single `helm install`.

Heads-up: if you provisioned Secrets through the operator chart's `--set-file provisioning.*` flags, this is a breaking change. See the migration note at the end.

- Operator docs: https://docs.ravendb.net/guides/the-ravendb-kubernetes-operator-way/
- Cluster chart: https://artifacthub.io/packages/helm/ravendb-operator/ravendb-cluster
- GitHub Release asset: `install.yaml` (attached below)

## Highlights

### Two charts, one job each

Install the operator once. Deploy a cluster whenever you need one. The **`ravendb-operator`** chart is the shared controller, CRD, RBAC, and webhooks that every cluster uses. The new **`ravendb-cluster`** chart renders everything one cluster needs: its RavenDBCluster CR, its Secrets, and its Traefik routes. Because each cluster is its own Helm release, you deploy, upgrade, and remove it on its own, without touching the shared operator.

```bash
helm repo add ravendb-operator https://ravendb.github.io/ravendb-operator/helm
helm repo update

# cert-manager is a prerequisite
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.0/cert-manager.yaml

helm install ravendb-operator ravendb-operator/ravendb-operator \
  -n ravendb-operator-system --create-namespace

helm install my-cluster ravendb-operator/ravendb-cluster \
  -n ravendb --create-namespace \
  -f my-values.yaml
```

### The cluster chart manages each cluster's Secrets

The cluster chart owns each cluster's Secrets (license, client cert, node certs), right next to the CR that uses them. You have two easy ways to fill each slot: let the chart create the Secret from a file with `--set-file`, or point it at a Secret you already manage. Either way the chart wires it onto the CR by name, and you can mix the two slot by slot. In 1.x this provisioning was an optional convenience offered by the operator chart; 2.0 moves it to the cluster chart, where it belongs.

```bash
kubectl create namespace ravendb

# your own node-cert Secrets (key: server.pfx)
kubectl create secret generic ravendb-certs-a \
  --from-file=server.pfx=/path/to/A/server.pfx -n ravendb   # and -b, -c

# chart creates the license + client cert, references the node certs above
helm install my-cluster ravendb-operator/ravendb-cluster \
  -n ravendb \
  -f my-values.yaml \
  --set-file secrets.license.file=/path/to/license.json \
  --set-file secrets.clientCert.file=/path/to/admin-client-cert.pfx
```

### Native Traefik ingress

The cluster chart renders Traefik `IngressRouteTCP` resources with TLS passthrough, so there's no route YAML to hand-write. With Traefik already installed (its CRDs and TLS-passthrough entrypoints named `websecure` for HTTPS and `tcp`, or override `traefik.entryPoints.https` / `traefik.entryPoints.tcp` to match your install) and DNS pointed at it, set `ingressClassName: traefik` and the chart renders the routes.

```yaml
# my-values.yaml (Traefik path)
spec:
  image: ravendb/ravendb:6.2-latest
  mode: LetsEncrypt
  email: ops@example.com
  domain: db.example.com
  nodes:
    - tag: a
      publicServerUrl: https://a.db.example.com:443
      publicServerUrlTcp: tcp://a-tcp.db.example.com:443
    # repeats for b, c
  externalAccessConfiguration:
    type: ingress-controller
    ingressControllerContext:
      ingressClassName: traefik
  storage:
    data:
      size: 10Gi
```

## Migrating from 1.x to 2.0.0

This is the only breaking change in 2.0.0, and it's a one-time step.

**If you never used `--set-file provisioning.*`** (the default), no action is needed, just upgrade as usual.

**If you did**, the operator chart no longer creates those cluster Secrets. Upgrading would otherwise delete them and degrade your cluster. Before upgrading, detach them from Helm so they survive:

```bash
kubectl annotate secret ravendb-license     helm.sh/resource-policy=keep -n <ns>
kubectl annotate secret ravendb-client-cert helm.sh/resource-policy=keep -n <ns>
# Plus the cert Secrets for your mode (e.g. ravendb-certs-a/b/c for Let's Encrypt).
```

After upgrading, adopt those Secrets in the `ravendb-cluster` chart's BYO mode (set each slot's `name`, leave `file` unset) or delete them once nothing references them.
