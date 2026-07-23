# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 2.0.x   | :white_check_mark: |

## Runtime security posture

The operator provisions RavenDB pods with a hardened, PodSecurity `restricted`
compatible security context:

- runs as the non-root RavenDB user (uid/gid 999, matching the image's `USER`),
- `runAsNonRoot`, `allowPrivilegeEscalation: false`, all capabilities dropped,
  `seccompProfile: RuntimeDefault`,
- `fsGroup: 999` so mounted PVCs are writable by the non-root process.

See the operator chart's [Pod security](helm/chart/README.md#pod-security)
section for details.

## Reporting a Vulnerability
Please report security vulnerabilities to security@ravendb.net