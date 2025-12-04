# RavenDB Storage Configuration Guide

This document explains all storage-related capabilities supported by the RavenDB Kubernetes Operator.  
Each section includes explanations, YAML examples, and commands to verify that your configuration works as expected.

---

## 1. Basic Data Volume

Every RavenDB node requires a persistent data volume.  
This is the minimal storage configuration required to run the operator.

```yaml 
storage:
  data:
    size: 10Gi
    storageClassName: local-path
```

**What happens:**  
- Each node in the cluster receives its own PVC.  
- The PVC name follows the pattern:  
  `ravendb-data-<cluster>-<nodeTag>-0`

**Verify:**

```bash
kubectl get pvc -n ravendb
```

---

## 2. Persistent Logs

RavenDB supports storing logs on separate PVCs.  
You may define:
- RavenDB logs
- Audit logs

```yaml
storage:
  data:
    size: 10Gi
    storageClassName: local-path
  logs:
    ravendb:
      size: 1Gi
      storageClassName: local-path
    audit:
      size: 1Gi
      storageClassName: local-path
```

**What happens:**  
- Two additional PVCs are created per node:  
  `ravendb-logs-<tag>-0` and `ravendb-audit-<tag>-0`

**Verify mount paths:**

```bash
kubectl exec -it ravendb-a-0 -n ravendb -- mount | grep log
```

---

## 3. Persistent Logs with Custom Paths

You may override where logs are written.  
If you do - **you must set `RAVEN_LogPath`** to match.

```yaml
env:
  RAVEN_LogPath: "/ravendb/logs"

storage:
  data:
    size: 10Gi
    storageClassName: local-path
  logs:
    ravendb:
      size: 1Gi
      storageClassName: local-path
      path: /ravendb/logs
    audit:
      size: 1Gi
      storageClassName: local-path
      path: /ravendb/logs
```

**Verify:**

```bash
kubectl exec -it ravendb-a-0 -n ravendb -- mount | grep /ravendb/logs
```

---

## 4. Additional Volumes

These allow mounting external files into your RavenDB container.  
Supported types:
- **ConfigMap** (single or multiple files)
- **Secret**
- **Existing PVC**

### 4.1 Single File from ConfigMap

```yaml
storage:
  data:
    size: 10Gi
  additionalVolumes:
    - name: csv-import
      mountPath: /tmp/orders.csv
      subPath: orders.csv
      volumeSource:
        configMap:
          name: orders-doc
```

**Example ConfigMap:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: orders-doc
data:
  orders.csv: |
    @id,Customer,Amount,Status
    1001,Contoso,49.99,Completed
```

---

### 4.2 Multiple Files from ConfigMap

Mount a directory of scripts:

```yaml
storage:
  data:
    size: 10Gi
  additionalVolumes:
    - name: scripts
      mountPath: /tmp/scripts
      volumeSource:
        configMap:
          name: myscripts
```

---

### 4.3 Mount Secret Files

Use this for sensitive imports (e.g., `.ravendump`):

```yaml
storage:
  additionalVolumes:
    - name: import-volume
      mountPath: /tmp/import
      volumeSource:
        secret:
          secretName: salaries-ravendump
```

Create the secret:

```bash
kubectl -n ravendb create secret generic salaries-ravendump --from-file=salaries.ravendump
```

---

### 4.4 Mount an Existing PVC

```yaml
storage:
  data:
    size: 10Gi
  additionalVolumes:
    - name: restore-volume
      mountPath: /tmp/restore
      volumeSource:
        persistentVolumeClaim:
          claimName: restore-backup-pvc
```

---

## 5. Access Modes

Each volume may define access modes.  
RavenDB typically uses:

- `ReadWriteOnce`

```yaml
storage:
  data:
    size: 10Gi
    storageClassName: local-path
    accessModes:
      - ReadWriteOnce
```

---

## 6. VolumeAttributesClass (Advanced / Alpha Feature)

Only use this if you know your CSI driver supports it.

```yaml
storage:
  data:
    size: 10Gi
    storageClassName: local-path
    volumeAttributesClassName: raven-default
```

Example:

```yaml
apiVersion: storage.k8s.io/v1alpha1
kind: VolumeAttributesClass
metadata:
  name: raven-default
parameters:
  fstype: xfs
  iops: "3000"
  throughput: "128"
```

**Important:**  
Most clusters disable this feature by default.

---

## Notes

- All PVCs, ConfigMaps, and Secrets **must be in the same namespace** as the cluster (e.g., `ravendb`).  
- If you override log paths, ensure they match `RAVEN_LogPath`.  
- `ReadWriteMany` is allowed by schema but may not be supported by your StorageClass.  
- The Operator automatically handles PVC naming and mounting per-node.

---
