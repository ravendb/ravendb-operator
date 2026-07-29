# v2.1.0 — RavenDB Pod security and PVC permissions

## Compatibility

- Kubernetes 1.23 or newer is required.
- Official RavenDB images continue to run as UID/GID `999:999`.
- Custom images must support the same runtime identity and restricted
  PodSecurity settings.

## What changed

- RavenDB containers run as non-root UID/GID `999:999`.
- Mounted PVCs use `fsGroup: 999`.
- Privilege escalation is disabled.
- All Linux capabilities are dropped.
- The runtime-default seccomp profile is enabled.
- The same policy applies to node StatefulSets and the bootstrap Job.

## Existing clusters

The operator assigns revision `1` to the hardened PodTemplate.

- Updating only the operator does not restart a healthy RavenDB cluster whose
  current template has no revision.
- The hardened template is applied one node at a time with the next intentional
  `spec.image` upgrade.
- Before bootstrap completes, legacy templates can be migrated one node per
  reconciliation without RavenDB HTTP health gates.

## Bootstrap Job

The bootstrap Job template is immutable.

- Active legacy Jobs are allowed to finish.
- Completed legacy Jobs are retained.
- Only a terminal failed legacy Job is deleted for recreation.
- Failed Jobs already using revision `1` are retained for diagnostics.

## Upgrade checklist

1. Confirm the Kubernetes cluster is version 1.23 or newer.
2. Confirm custom RavenDB images run as UID/GID `999:999`.
3. Schedule a RavenDB image upgrade if existing clusters must receive the
   hardened PodTemplate immediately.
4. Verify every RavenDB Pod becomes Ready and can write to its PVC.
