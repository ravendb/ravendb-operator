# Lab 01: safely migrate RavenDB PodTemplates during an upgrade

## For

- RavenDB Operator maintainers reviewing a PodTemplate security change
- contributors changing fields under `StatefulSet.spec.template`
- release engineers deciding whether a template change needs a major release
- anyone who wants to reproduce the migration on a real cluster

## The case

The operator starts managing security-sensitive PodTemplate fields that older
operator versions did not set:

- RavenDB runs as numeric user and group `999:999`;
- containers cannot gain privileges;
- every Linux capability is dropped;
- the runtime-default seccomp profile is enabled;
- PVCs are prepared through `fsGroup: 999`;
- `fsGroupChangePolicy: OnRootMismatch` avoids recursively changing ownership
  on every mount;
- an internal PodTemplate revision records which template contract produced a
  workload.

Changing a StatefulSet PodTemplate restarts its Pod. Applying the new template
to every RavenDB node immediately after an operator upgrade would therefore
turn a control-plane deployment into an implicit database-cluster rollout.

This lab proves the intended alternative:

1. a healthy, already-bootstrapped cluster is not restarted merely because its
   stored PodTemplate is legacy;
2. the current template is carried by the next intentional RavenDB image
   upgrade;
3. nodes move one at a time in deterministic `A -> B -> C` order.

This is not a mock. The runnable test creates a real Kind cluster and boots
three RavenDB nodes on the official images:

```text
ravendb/ravendb:6.2.11-ubuntu.22.04-x64
                    |
                    | intentional spec.image change
                    v
ravendb/ravendb:7.1.3-ubuntu.22.04-x64
```

## What this lab changes on your machine

The test:

- creates a temporary Kind cluster named `ravendb`;
- builds the operator from the current checkout under a caller-supplied,
  non-floating image tag;
- pulls and loads the two concrete RavenDB version tags listed above;
- builds a tiny local image that generates two-day, self-signed lab
  certificates;
- reads a RavenDB license from a local file;
- creates temporary Kubernetes Secrets inside the Kind cluster;
- provisions three local-path PVCs;
- destroys the Kind cluster when the Go test process finishes.

The license is never copied into this repository or printed by the test. The
generated PFX files are copied into a Go test temporary directory and the
ephemeral Kubernetes cluster. The temporary certificate image is removed by
test cleanup; the certificates are self-signed and have no trust outside this
lab.

Like any Kubernetes Secret, the temporary license Secret can be read by a
principal with sufficient access to that Kind cluster while the test is
running. Run the lab on a trusted development machine and do not share the
generated kubeconfig.

## Source map

The runnable behavior lives in:

- [`test/e2e/pod_template_migration_lab_test.go`](../test/e2e/pod_template_migration_lab_test.go)
  — the scenario and its assertions;
- [`labs/upgrade-certificates/Dockerfile`](upgrade-certificates/Dockerfile)
  — the ephemeral CA, server certificate, and client certificate generator;
- [`labs/kind-runner.Dockerfile`](kind-runner.Dockerfile)
  — a Linux Go/Kind/kubectl runner, useful on Docker Desktop for Windows;
- [`pkg/resource/security.go`](../pkg/resource/security.go)
  — the desired hardened Pod security contexts and template revision;
- [`pkg/upgrade/upgrader.go`](../pkg/upgrade/upgrader.go)
  — node selection and lazy migration.

The test is opt-in. Normal `go test ./...` runs do not pull RavenDB images or
start this lab because the test requires:

```text
RAVEN_RUN_MIGRATION_LAB=1
```

## Prerequisites

You need:

- Docker with enough resources for one Kind control-plane and three RavenDB
  nodes;
- an x86-64 host (the supplied runner verifies amd64 Kind and kubectl
  binaries);
- a RavenDB development license saved as a JSON file outside the repository;
- approximately 20 minutes for a cold run, mostly for image downloads and
  cluster bootstrap;
- internet access for versioned Kubernetes manifests and container images.

For a direct Linux run, also install:

- Go 1.25;
- Kind 0.32 or compatible;
- kubectl 1.34 or compatible.

The supplied runner image installs the last three tools for the Docker Desktop
path.

## Keep the license out of the repository

Store the license at a path that is not under this checkout. For example:

```bash
install -m 0600 /path/from/download/license.json \
  "$HOME/.config/ravendb/lab-license.json"
```

Do not put the license in an environment variable. Point
`RAVEN_LAB_LICENSE_PATH` at the file instead. The test reads the file without
logging its contents.

Before running:

```bash
git status --short
```

The license must not appear in that output.

## Run it directly on Linux

From the repository root:

```bash
export PROJECT_ROOT="$PWD"
export RAVEN_RUN_MIGRATION_LAB=1
export RAVEN_E2E_MINIMAL=1
export RAVEN_LAB_LICENSE_PATH="$HOME/.config/ravendb/lab-license.json"

# A unique tag prevents Kind from reusing an older locally cached `latest`.
export RAVEN_OPERATOR_IMAGE="ravendb/ravendb-operator:pod-template-lab-$(git rev-parse --short HEAD)-local"

go test -v -count=1 -timeout 30m ./test/e2e \
  -run '^TestLab_PodTemplateMigrationOnRealKind$'
```

`RAVEN_E2E_MINIMAL=1` skips MetalLB and ingress because this lab communicates
only over cluster-internal Services. Cert-manager and the local-path
provisioner remain enabled because they are part of the real operator setup.

## Run it on Docker Desktop for Windows

First build the reusable Linux runner:

```powershell
docker build `
  -f labs/kind-runner.Dockerfile `
  -t ravendb/operator-upgrade-lab-runner:local `
  .
```

Then run the repository and license as read-only inputs to the Linux toolchain.
This command is written for Docker Desktop's internal Docker socket:

```powershell
$repo = (Get-Location).Path
$license = "C:\secure\ravendb\license.json"

docker run --rm `
  --network host `
  -v /run/host-services/docker.proxy.sock:/var/run/docker.sock `
  -v "${repo}:/src:ro" `
  -v "${license}:/run/secrets/license.json:ro" `
  -v ravendb-lab-gocache:/root/.cache/go-build `
  -v ravendb-lab-gomod:/go/pkg/mod `
  -e PROJECT_ROOT=/src `
  -e RAVEN_RUN_MIGRATION_LAB=1 `
  -e RAVEN_E2E_MINIMAL=1 `
  -e RAVEN_LAB_LICENSE_PATH=/run/secrets/license.json `
  -e RAVEN_OPERATOR_IMAGE=ravendb/ravendb-operator:pod-template-lab-local `
  ravendb/operator-upgrade-lab-runner:local `
  go test -v -count=1 -timeout 30m ./test/e2e `
    -run '^TestLab_PodTemplateMigrationOnRealKind$'
```

If your Docker Desktop installation exposes a different socket path, replace
only the left side of the socket mount. Do not mount a remote or shared Docker
daemon: the Kind cluster contains the temporary license Secret.

The runner receives control of the local Docker daemon, which is effectively
host-level authority. Its base images are therefore pinned by digest, and the
downloaded Kind and kubectl binaries are checked against their release
SHA-256 values. The operator's builder and final runtime image are also pinned
by digest in the root `Dockerfile`. Review changes to both Dockerfiles before
running an untrusted branch.

The RavenDB CR webhook intentionally accepts concrete version tags and rejects
digest references. The RavenDB and Kubernetes test dependencies are therefore
versioned but not fully hermetic; their upstream tags/assets can still be
republished. The weekly UID/GID guard catches RavenDB identity drift, but run
this developer lab only against upstreams and a checkout you trust.

## Architecture created by the test

```text
Docker
└── Kind cluster: ravendb
    ├── namespace: ravendb-operator-system
    │   └── RavenDB Operator built from this checkout
    ├── namespace: cert-manager
    ├── namespace: local-path-storage
    └── namespace: lab
        ├── RavenDBCluster: pod-template-migration
        ├── Secret: lab-license
        ├── Secret: lab-client-cert
        ├── Secret: lab-server-cert
        ├── Secret: lab-ca-cert
        ├── Job: ravendb-bootstrapper
        ├── StatefulSet/Pod: ravendb-a / ravendb-a-0
        ├── StatefulSet/Pod: ravendb-b / ravendb-b-0
        ├── StatefulSet/Pod: ravendb-c / ravendb-c-0
        ├── Services: a, b, c
        ├── TCP Services: a-tcp, b-tcp, c-tcp
        └── one local-path PVC per node
```

The short `a`, `b`, and `c` Service names let the server certificate cover
stable internal names:

```text
a.lab.svc.cluster.local
b.lab.svc.cluster.local
c.lab.svc.cluster.local
```

The `*-tcp` Services make the RavenDB public TCP URLs routable inside the
cluster. No ingress, public DNS, or Let's Encrypt certificate is required.

## PodTemplate revision is not a product version

The revision annotation is:

```text
ravendb.ravendb.io/pod-template-revision
```

It is an internal schema stamp for the operator-owned PodTemplate, not:

- the RavenDB version;
- the operator version;
- the CustomResource API version;
- a SemVer promise exposed to application clients.

The mapping in this change is:

| Stored value | Meaning |
| --- | --- |
| annotation missing | legacy revision `0`, produced before explicit revisioning |
| `"1"` | first explicit contract: non-root `999:999`, PVC group ownership, restricted-compatible container security |

Starting with `"1"` avoids inventing a nonexistent public revision `"2"`.
Future code may introduce `"2"` when it intentionally changes the template
contract again.

## Why the migration is lazy

There are two separate questions:

1. What PodTemplate would the current operator build?
2. Should a running RavenDB Pod be replaced right now?

The resource builder answers the first question. The upgrader answers the
second.

If the operator used template inequality alone as permission to act, deploying
a new operator could roll every database node even though the
`RavenDBCluster.spec.image` did not change. That couples the operator release
to an unrequested data-plane maintenance event.

The selection policy is therefore:

```text
existing in-flight marker?
├── yes -> resume exactly that node
└── no
    ├── StatefulSet missing? -> recover/bootstrap it
    ├── desired image differs? -> intentionally upgrade one node
    ├── cluster not bootstrapped and revision is legacy?
    │   └── migrate one bootstrap/recovery node
    └── healthy bootstrapped cluster with only a legacy revision?
        └── leave it running until the next intentional image upgrade
```

This is a rollout-policy choice, not a claim that the legacy security posture
is equivalent. Administrators who need the hardening immediately can perform
an intentional RavenDB image rollout during their maintenance window.

## Phase 1: bootstrap real RavenDB 6.2.11

The test creates:

```go
RavenDBClusterSpec{
    Image:               "ravendb/ravendb:6.2.11-ubuntu.22.04-x64",
    ImagePullPolicy:     "IfNotPresent",
    Mode:                ModeNone,
    LicenseSecretRef:    "lab-license",
    ClientCertSecretRef: "lab-client-cert",
    Nodes:               []RavenDBNode{/* A, B, C */},
}
```

It waits for `ConditionReady=True`. This proves that the test did not merely
create Kubernetes objects: RavenDB bootstrapped and the operator's normal
health logic accepted the cluster.

The real run also exercises the bounded membership retry used when RavenDB
briefly answers HTTP 307 while the newly bootstrapped leader settles. Only 307
is retried; other failures remain fatal, and redirects are never followed.

The test also reads the completed bootstrap Job and attempts to mutate
`job.spec.template`. Kubernetes must reject the update with `immutable`.
Before that mutation, it verifies the real Job template carries revision `"1"`
and the same restricted security baseline as the node Pods, without the
node-only low-port sysctl. The immutability assertion protects the separate Job
lifecycle rule: the operator must not try to update a completed Job's
PodTemplate in place.

Expected log:

```text
phase 1: bootstrap a real RavenDB 6.2.11 cluster through the operator
```

## Phase 2: model a cluster created before revisioning

For each node, the test first proves that the current operator created revision
`"1"`. It then uses server-side apply to remove:

- the revision annotation;
- the pod security context;
- the RavenDB container security context.

This models persisted StatefulSets from an older operator without needing to
publish and install an artificial historical operator image.

The deliberate test mutation causes one replacement Pod per node. After those
legacy Pods are Ready, the test records their Kubernetes UIDs and watches for
eight seconds.

During that window it continuously asserts:

- all three UIDs stay unchanged;
- all three templates remain legacy;
- the current operator does not “helpfully” reconcile the new security fields
  into the healthy cluster.

The eight-second interval spans multiple controller reconciliations. It is a
negative assertion: no implicit rollout is the expected behavior.

Expected log:

```text
phase 2: model a healthy cluster created by the pre-revision operator
```

## Phase 3: intentional upgrade carries the current template

The test changes only:

```text
RavenDBCluster.spec.image
```

from RavenDB `6.2.11` to `7.1.3`.

The test observes the StatefulSets at the boundaries:

```text
after selecting A: A=7.1.3, B=6.2.11, C=6.2.11
after selecting B: A=7.1.3, B=7.1.3, C=6.2.11
after selecting C: A=7.1.3, B=7.1.3, C=7.1.3
```

For every resulting Pod it asserts:

```text
runAsNonRoot              = true
runAsUser                 = 999
runAsGroup                = 999
fsGroup                   = 999
fsGroupChangePolicy       = OnRootMismatch
seccompProfile.type       = RuntimeDefault
allowPrivilegeEscalation  = false
capabilities.drop         contains ALL
pod-template-revision     = "1"
```

It then executes inside each real RavenDB container:

```sh
test "$(id -u):$(id -g)" = "999:999"
touch /var/lib/ravendb/data/.upgrade-lab-write-check
rm /var/lib/ravendb/data/.upgrade-lab-write-check
/usr/lib/ravendb/server/Raven.Server --version
```

Those checks prove both runtime identity and PVC writability. Inspecting YAML
alone would not catch a mounted-volume ownership problem.

Before beginning the next phase, the test waits for all three rollout markers
to disappear. `Ready=True` and three Ready Pods are necessary, but they are not
the complete transaction boundary: the last node can already be serving while
its post-gates are still running.

Expected log:

```text
phase 3: an intentional image upgrade carries the current template revision A -> B -> C
```

## Observe a run while debugging

The Go test normally owns cleanup. While it is running, another shell can use
the Kind context:

```bash
kubectl config use-context kind-ravendb
kubectl -n lab get ravendbcluster,pod,sts,job,pvc
```

Watch images, revisions, and markers:

```bash
kubectl -n lab get sts -w \
  -o custom-columns='NAME:.metadata.name,IMAGE:.spec.template.spec.containers[0].image,REVISION:.spec.template.metadata.annotations.ravendb\.ravendb\.io/pod-template-revision,IN_FLIGHT:.metadata.annotations.ravendb\.ravendb\.io/upgrade-image'
```

Inspect the operator without exposing Secrets:

```bash
kubectl -n ravendb-operator-system logs \
  deployment/ravendb-operator-controller-manager \
  -c manager \
  --tail=200
```

Do not use `kubectl get secret -o yaml` in captured CI logs.

## Success criteria

The lab passes only if all of these are true:

- real RavenDB `6.2.11` reaches `Ready=True`;
- Kubernetes rejects a PodTemplate mutation of the completed bootstrap Job;
- a healthy legacy cluster stays untouched without an intentional image
  change;
- the real upgrade reaches `7.1.3` in `A -> B -> C` order;
- all final templates carry revision `"1"`;
- all real RavenDB processes run as `999:999`;
- all three RavenDB processes can write to their PVCs;
- all three binaries report RavenDB `7.1.3`;
- the cluster returns to `Ready=True`;
- the Go test exits with `PASS` and destroys Kind.

## Expected final output

The exact setup logs depend on image caches, but the test ends with:

```text
--- PASS: TestLab_PodTemplateMigrationOnRealKind
PASS
ok      ravendb-operator/test/e2e
```

After the test:

```bash
kind get clusters
```

must not list `ravendb`.

## Troubleshooting

### A StatefulSet is missing revision `"1"` immediately after creation

Use a unique `RAVEN_OPERATOR_IMAGE` tag. A floating local `latest` can make a
Kind node reuse an older image that predates the revision code.

Confirm the installed image:

```bash
kubectl -n ravendb-operator-system get deployment \
  ravendb-operator-controller-manager \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="manager")].image}{"\n"}'
```

### RavenDB images take a long time to start

The first run downloads both versioned server images and imports them into Kind.
Subsequent runs normally reuse Docker's cache. Allocate more Docker Desktop
memory if Pods are evicted or the Kind node becomes `NotReady`.

### A PVC remains Pending

Check the local-path provisioner:

```bash
kubectl -n local-path-storage get pod
kubectl -n lab describe pvc
```

`RAVEN_E2E_MINIMAL=1` skips ingress and MetalLB, but intentionally does not skip
local-path storage.

### A RavenDB Pod reports a certificate name error

Confirm that `spec.nodes[*].publicServerUrl` uses exactly the internal
`a.lab.svc.cluster.local`, `b...`, and `c...` names. The ephemeral certificate
contains those three DNS SANs and no public hostname.

### The test cannot read the license

Confirm the file mount and path without printing the file:

```bash
test -r "$RAVEN_LAB_LICENSE_PATH"
```

Inside the Windows runner:

```powershell
docker run --rm `
  -v "${license}:/run/secrets/license.json:ro" `
  alpine:3.22 `
  test -s /run/secrets/license.json
```

### A previous interrupted run left a Kind cluster

List it first:

```bash
kind get clusters
```

Then remove only the exact lab cluster:

```bash
kind delete cluster --name ravendb
```

Never script a broad Docker or Kubernetes cleanup for this lab.

## What this lab does not prove

The lab deliberately has a narrow release question. It does not prove:

- performance on production-size RavenDB data volumes;
- the duration of a real `fsGroup` ownership migration on every CSI driver;
- application-level compatibility between arbitrary RavenDB releases;
- external ingress, public DNS, or Let's Encrypt behavior;
- multi-zone availability under node or network failure;
- that every user-supplied sidecar can run as `999:999`;
- rollback from RavenDB `7.1` to `6.2`;
- behavior on Windows containers.

Those are separate test dimensions. Keeping them separate makes a failure in
this lab attributable to PodTemplate migration.

## Takeaway

A PodTemplate revision tells the operator what it has built. It does not grant
permission to restart a healthy database cluster.

The safe boundary is:

```text
new operator deployed
    != automatic data-plane rollout

intentional image upgrade
    = one node selected
    + current PodTemplate
    + successful rollout
```

The runnable test turns that release policy into executable evidence on a real
RavenDB cluster.
