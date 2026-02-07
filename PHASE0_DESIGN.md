# Fluid Resource Mapper - Phase 0 Design Document

> **Version**: 1.0  
> **Author**: Resource Mapper Team  
> **Status**: Ready for Review  
> **Last Updated**: 2026-02-08

---

## Executive Summary

The **Fluid Resource Mapper** is a read-only, deterministic mapping engine that discovers and visualizes the relationships between Fluid's high-level CRDs (`Dataset`, `Runtime`) and their underlying Kubernetes resources (`StatefulSet`, `DaemonSet`, `Pod`, `PVC`, `PV`, `ConfigMap`, `Secret`).

This document defines:
1. CRD relationship analysis
2. Resource mapping rules
3. Failure-aware mapping strategies
4. Architecture decisions

---

## 1. CRD Relationship Analysis

### 1.1 Dataset Custom Resource

The `Dataset` CR represents a **logical view of data** in Fluid. It abstracts the underlying storage and caching layer.

```yaml
apiVersion: data.fluid.io/v1alpha1
kind: Dataset
metadata:
  name: my-dataset
  namespace: fluid-system
spec:
  mounts:
    - mountPoint: s3://bucket/path
      name: data
      options:
        aws.accessKeyId: "..."
        aws.secretKey: "..."
  nodeAffinity:
    required:
      nodeSelectorTerms:
        - matchExpressions:
            - key: fluid.io/s-my-dataset
              operator: Exists
status:
  phase: Bound           # NotBound, Bound, Failed, Pending
  runtimes:
    - name: my-dataset
      namespace: fluid-system
      type: alluxio       # alluxio, jindo, juicefs, goosefs, vineyard, efc
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2026-02-07T10:00:00Z"
      reason: DatasetReady
      message: Dataset is ready
  ufsTotal: "100Gi"
  cacheStates:
    cacheCapacity: "50Gi"
    cached: "25Gi"
    cachedPercentage: "50%"
```

**Key Status Fields for Mapping:**
| Field | Purpose | Mapping Impact |
|-------|---------|----------------|
| `.status.phase` | Lifecycle phase | Determines if Runtime is bound |
| `.status.runtimes[]` | Bound runtime references | **Primary link to Runtime CRs** |
| `.status.runtimes[].type` | Runtime type | Determines which Runtime CR to query |
| `.status.conditions` | Health conditions | Used for warning detection |
| `.spec.nodeAffinity` | Data locality labels | Helps identify cache-enabled nodes |

### 1.2 Runtime Custom Resources

Fluid supports multiple runtime types, each implementing the same logical pattern but with engine-specific configurations.

#### 1.2.1 AlluxioRuntime

```yaml
apiVersion: data.fluid.io/v1alpha1
kind: AlluxioRuntime
metadata:
  name: my-dataset       # Must match Dataset name
  namespace: fluid-system
spec:
  replicas: 2
  master:
    replicas: 1
    resources:
      requests:
        cpu: "1"
        memory: "2Gi"
  worker:
    replicas: 2
    resources:
      requests:
        cpu: "2"
        memory: "8Gi"
  fuse:
    resources:
      requests:
        cpu: "100m"
        memory: "512Mi"
  tieredstore:
    levels:
      - mediumtype: MEM
        path: /dev/shm
        quota: 8Gi
        high: "0.95"
        low: "0.7"
status:
  masterPhase: Ready      # Pending, Ready, NotReady, Failed
  workerPhase: Ready
  fusePhase: Ready
  currentMasterNumberScheduled: 1
  currentWorkerNumberScheduled: 2
  currentFuseNumberScheduled: 3
  desiredMasterNumberScheduled: 1
  desiredWorkerNumberScheduled: 2
  desiredFuseNumberScheduled: 3
  conditions:
    - type: Ready
      status: "True"
      lastProbeTime: "..."
```

#### 1.2.2 JindoRuntime

```yaml
apiVersion: data.fluid.io/v1alpha1
kind: JindoRuntime
metadata:
  name: my-dataset
  namespace: fluid-system
spec:
  replicas: 2
  master:
    replicas: 1
  worker:
    replicas: 2
  fuse: {}
status:
  # Same structure as AlluxioRuntime
```

#### 1.2.3 JuiceFSRuntime

```yaml
apiVersion: data.fluid.io/v1alpha1
kind: JuiceFSRuntime
metadata:
  name: my-dataset
  namespace: fluid-system
spec:
  replicas: 2
  worker:
    replicas: 2
  fuse: {}
  # Note: JuiceFS doesn't have a master component
status:
  workerPhase: Ready
  fusePhase: Ready
  # No masterPhase
```

### 1.3 Name Binding & Namespace Coupling

**Critical Design Decision in Fluid:**

> **Dataset and Runtime must share the same name and namespace.**

This 1:1 binding relationship is enforced by Fluid's controllers:

```
Dataset(name=foo, ns=default) ↔ Runtime(name=foo, ns=default)
```

This simplifies resource discovery but has implications:
- Multiple runtimes cannot back a single dataset
- Renaming requires recreation
- This constraint is exploitable for deterministic mapping

### 1.4 OwnerReferences vs Labels

Fluid uses **both** mechanisms for resource correlation:

| Mechanism | Use Case | Resources Using It |
|-----------|----------|--------------------|
| **OwnerReferences** | Garbage collection, controller tracking | StatefulSets, DaemonSets, ConfigMaps, Secrets |
| **Labels** | Selection, filtering, identification | All resources including Pods |

**Standard Labels Used by Fluid:**

```yaml
labels:
  # Release identification (dataset/runtime name)
  release: my-dataset
  
  # Application identification
  app: alluxio             # or jindo, juicefs, etc.
  
  # Role identification
  role: alluxio-master     # or alluxio-worker, alluxio-fuse
  
  # Fluid-specific labels
  fluid.io/dataset: my-dataset
  fluid.io/dataset-namespace: fluid-system
```

---

## 2. Resource Mapping Rules

### 2.1 Component-to-Resource Mapping Table

| Fluid Component | Kubernetes Resource | Discovery Method | Naming Convention |
|-----------------|---------------------|------------------|-------------------|
| Master | StatefulSet | Label: `role=<runtime>-master` | `{dataset}-master` |
| Worker | StatefulSet | Label: `role=<runtime>-worker` | `{dataset}-worker` |
| Fuse | DaemonSet | Label: `role=<runtime>-fuse` | `{dataset}-fuse` |
| Master Pod(s) | Pod | Owner: Master StatefulSet | `{dataset}-master-{ordinal}` |
| Worker Pod(s) | Pod | Owner: Worker StatefulSet | `{dataset}-worker-{ordinal}` |
| Fuse Pod(s) | Pod | Owner: Fuse DaemonSet | `{dataset}-fuse-{hash}` |
| Data Volume | PVC | Label: `release={dataset}` | `{dataset}` |
| Data Volume | PV | Bound to PVC | auto-generated |
| Master Config | ConfigMap | Owner: Runtime CR | `{dataset}-master-config` |
| Worker Config | ConfigMap | Owner: Runtime CR | `{dataset}-worker-config` |
| Fuse Config | ConfigMap | Owner: Runtime CR | `{dataset}-fuse-config` |
| Secrets | Secret | Owner: Runtime CR | `{dataset}-*-secret` |

### 2.2 Label Selectors for Discovery

**Primary Label Selector (Most Reliable):**
```yaml
release: {dataset-name}
```

**Supplementary Label Selectors:**
```yaml
# By specific role
role: alluxio-master
role: alluxio-worker
role: alluxio-fuse

# By application type
app: alluxio

# Fluid dataset label
fluid.io/dataset: {dataset-name}
fluid.io/dataset-namespace: {namespace}
```

### 2.3 Discovery Strategy (Priority Order)

```
1. Start with Dataset CR
   ├── Check .status.phase
   ├── Extract .status.runtimes[]
   │
2. Resolve Runtime CR by type + name
   ├── Extract component phases
   │
3. Discover workloads by labels
   ├── StatefulSets (label: release={name})
   ├── DaemonSets (label: release={name})
   │
4. Discover pods by ownership
   ├── Pods owned by StatefulSets
   ├── Pods owned by DaemonSets
   │
5. Discover storage resources
   ├── PVCs (label: release={name})
   ├── PVs (bound to PVCs)
   │
6. Discover configuration resources
   ├── ConfigMaps (ownerRef: Runtime)
   ├── Secrets (ownerRef: Runtime)
```

### 2.4 Runtime-Specific Variations

| Runtime Type | Has Master | Has Worker | Has Fuse | Special Resources |
|--------------|------------|------------|----------|-------------------|
| AlluxioRuntime | ✓ | ✓ | ✓ | Job Format ConfigMap |
| JindoRuntime | ✓ | ✓ | ✓ | - |
| JuiceFSRuntime | ✗ | ✓ | ✓ | Redis/TiKV for metadata |
| GooseFSRuntime | ✓ | ✓ | ✓ | - |
| VineyardRuntime | ✓ | ✓ | ✓ | etcd for metadata |
| EFCRuntime | ✓ | ✓ | ✓ | - |
| ThinRuntime | ✗ | ✗ | ✓ | Only Fuse DaemonSet |

---

## 3. Failure-Aware Mapping

### 3.1 Missing Resource Scenarios

| Scenario | Detection | Warning Level | User Message |
|----------|-----------|---------------|--------------|
| Dataset exists, no Runtime | `.status.runtimes` empty | ⚠️ Warning | "No runtime bound to dataset" |
| Runtime exists, components missing | StatefulSet not found | 🔴 Error | "Master StatefulSet missing" |
| StatefulSet exists, Pods missing | No pods with ownerRef | ⚠️ Warning | "Pods not yet scheduled" |
| Pods exist, not Ready | Pod status != Running | ⚠️ Warning | "Pods not healthy (1/3 ready)" |
| PVC missing | PVC not found | 🔴 Error | "Data PVC not created" |
| PV not bound | PVC.status.phase != Bound | ⚠️ Warning | "PV provisioning pending" |

### 3.2 Partial Creation Scenarios

Fluid creates resources in a specific order. Partial states occur during:

1. **Initial Creation**: Runtime CR created → Master → Worker → Fuse → PVC
2. **Scaling Operations**: New pods being scheduled
3. **Failure Recovery**: Some components restarting
4. **Deletion**: Resources being garbage collected

**Detection Strategy:**
```go
type CreationProgress struct {
    ExpectedMaster    int
    ActualMaster      int
    ExpectedWorker    int
    ActualWorker      int
    ExpectedFuse      int
    ActualFuse        int
    Phase             string // Creating, Scaling, Ready, Degraded, Failed
}
```

### 3.3 Orphaned Resources

Orphaned resources occur when:
- Owner CR deleted but finalizer failed
- Manual intervention corrupted ownership
- Controller crash during cleanup

**Detection:**
```go
// Resource has Fluid labels but no valid ownerReference
func isOrphaned(resource client.Object) bool {
    labels := resource.GetLabels()
    if labels["release"] == "" {
        return false // Not a Fluid resource
    }
    
    for _, ref := range resource.GetOwnerReferences() {
        if ref.Kind == "AlluxioRuntime" || 
           ref.Kind == "JindoRuntime" {
            return false // Has valid owner
        }
    }
    return true
}
```

### 3.4 State Matrix

```
Dataset Phase    | Runtime Phase | Expected State              | Mapping Action
-----------------|---------------|-----------------------------|-----------------
NotBound         | -             | No runtime yet              | Show dataset only
Pending          | Creating      | Resources being created     | Show with progress
Bound            | Ready         | All healthy                 | Full resource tree
Bound            | PartialReady  | Some components degraded    | Tree with warnings
Failed           | Failed        | Error state                 | Tree with errors
-                | Exists        | Orphaned runtime            | Warning + tree
```

---

## 4. Architecture Design

### 4.1 Conceptual Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Fluid Resource Mapper                        │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐       │
│  │   Mapper    │────▶│  Resolver   │────▶│  Discovery  │       │
│  │   (Entry)   │     │  (CRD→CRD)  │     │  (K8s API)  │       │
│  └─────────────┘     └─────────────┘     └─────────────┘       │
│         │                  │                   │                │
│         ▼                  ▼                   ▼                │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐       │
│  │  Resource   │     │   Graph     │     │  Warning    │       │
│  │   Graph     │◀────│  Builder    │◀────│  Detector   │       │
│  └─────────────┘     └─────────────┘     └─────────────┘       │
├─────────────────────────────────────────────────────────────────┤
│                     Kubernetes Client                           │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ GET datasets, runtimes, statefulsets, daemonsets, pods,    ││
│  │     pvcs, pvs, configmaps, secrets                         ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### 4.2 Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **Mapper** | Entry point, orchestrates mapping workflow |
| **Resolver** | Resolves CRD relationships (Dataset → Runtime) |
| **Discovery** | Discovers K8s resources by labels/ownerRefs |
| **Graph Builder** | Constructs the resource graph with relationships |
| **Warning Detector** | Identifies missing/unhealthy resources |

### 4.3 Data Flow

```
Input: MapFromDataset("demo-data", "fluid-system")
                    │
                    ▼
        ┌─────────────────────┐
        │  Get Dataset CR     │
        │  GET /apis/data.fluid.io/v1alpha1/namespaces/fluid-system/datasets/demo-data
        └─────────────────────┘
                    │
                    ▼
        ┌─────────────────────┐
        │  Check .status      │
        │  .runtimes[].type   │
        └─────────────────────┘
                    │
                    ▼
        ┌─────────────────────┐
        │  Get Runtime CR     │
        │  (based on type)    │
        │  GET /apis/data.fluid.io/v1alpha1/namespaces/fluid-system/alluxioruntimes/demo-data
        └─────────────────────┘
                    │
                    ▼
        ┌─────────────────────┐
        │  Discover Resources │
        │  by labels          │
        │  release=demo-data  │
        └─────────────────────┘
                    │
                    ▼
        ┌─────────────────────┐
        │  Build Resource     │
        │  Graph + Detect     │
        │  Warnings           │
        └─────────────────────┘
                    │
                    ▼
           ResourceGraph{
             Dataset, Runtime,
             Resources[], Warnings[]
           }
```

---

## 5. Output Model Design

### 5.1 Core Types

```go
// ResourceGraph is the main output structure
type ResourceGraph struct {
    // Root dataset
    Dataset DatasetNode `json:"dataset"`
    
    // Bound runtime (if any)
    Runtime *RuntimeNode `json:"runtime,omitempty"`
    
    // All discovered Kubernetes resources
    Resources []K8sResourceNode `json:"resources"`
    
    // Detected issues
    Warnings []MappingWarning `json:"warnings"`
    
    // Mapping metadata
    Metadata GraphMetadata `json:"metadata"`
}

// DatasetNode represents the Dataset CR
type DatasetNode struct {
    Name       string            `json:"name"`
    Namespace  string            `json:"namespace"`
    Phase      string            `json:"phase"`
    UfsTotal   string            `json:"ufsTotal,omitempty"`
    Cached     string            `json:"cached,omitempty"`
    Conditions []ConditionBrief  `json:"conditions,omitempty"`
}

// RuntimeNode represents the Runtime CR
type RuntimeNode struct {
    Name         string `json:"name"`
    Namespace    string `json:"namespace"`
    Type         string `json:"type"` // alluxio, jindo, juicefs, etc.
    MasterPhase  string `json:"masterPhase,omitempty"`
    WorkerPhase  string `json:"workerPhase,omitempty"`
    FusePhase    string `json:"fusePhase,omitempty"`
    MasterReady  string `json:"masterReady,omitempty"`  // "1/1"
    WorkerReady  string `json:"workerReady,omitempty"`  // "2/3"
    FuseReady    string `json:"fuseReady,omitempty"`    // "5/5"
}

// K8sResourceNode represents a discovered Kubernetes resource
type K8sResourceNode struct {
    Kind             string             `json:"kind"`
    Name             string             `json:"name"`
    Namespace        string             `json:"namespace,omitempty"`
    Component        string             `json:"component"` // master, worker, fuse, config, storage
    Status           ResourceStatus     `json:"status"`
    Owner            *OwnerInfo         `json:"owner,omitempty"`
    Details          map[string]string  `json:"details,omitempty"`
}

// ResourceStatus indicates health
type ResourceStatus struct {
    Phase   string `json:"phase"`   // Ready, NotReady, Pending, Failed, Unknown
    Ready   string `json:"ready"`   // "3/3" for deployments/statefulsets
    Message string `json:"message,omitempty"`
}

// MappingWarning represents a detected issue
type MappingWarning struct {
    Level    string `json:"level"`    // error, warning, info
    Code     string `json:"code"`     // e.g., "MISSING_MASTER", "PODS_NOT_READY"
    Message  string `json:"message"`
    Resource string `json:"resource,omitempty"` // affected resource name
}

// GraphMetadata contains mapping metadata
type GraphMetadata struct {
    MappedAt    time.Time `json:"mappedAt"`
    ClusterName string    `json:"clusterName,omitempty"`
    Version     string    `json:"version"`
}
```

### 5.2 Example Output

```json
{
  "dataset": {
    "name": "demo-data",
    "namespace": "fluid-system",
    "phase": "Bound",
    "ufsTotal": "100Gi",
    "cached": "25Gi"
  },
  "runtime": {
    "name": "demo-data",
    "namespace": "fluid-system",
    "type": "alluxio",
    "masterPhase": "Ready",
    "workerPhase": "Ready",
    "fusePhase": "Ready",
    "masterReady": "1/1",
    "workerReady": "2/2",
    "fuseReady": "3/3"
  },
  "resources": [
    {
      "kind": "StatefulSet",
      "name": "demo-data-master",
      "namespace": "fluid-system",
      "component": "master",
      "status": {
        "phase": "Ready",
        "ready": "1/1"
      }
    },
    {
      "kind": "StatefulSet",
      "name": "demo-data-worker",
      "namespace": "fluid-system",
      "component": "worker",
      "status": {
        "phase": "Ready",
        "ready": "2/2"
      }
    },
    {
      "kind": "DaemonSet",
      "name": "demo-data-fuse",
      "namespace": "fluid-system",
      "component": "fuse",
      "status": {
        "phase": "Ready",
        "ready": "3/3"
      }
    },
    {
      "kind": "PersistentVolumeClaim",
      "name": "demo-data",
      "namespace": "fluid-system",
      "component": "storage",
      "status": {
        "phase": "Bound"
      }
    }
  ],
  "warnings": [],
  "metadata": {
    "mappedAt": "2026-02-08T10:30:00Z",
    "version": "1.0.0"
  }
}
```

---

## 6. Assumptions

1. **Name Coupling**: Dataset and Runtime always share the same name and namespace
2. **Label Consistency**: Fluid controllers always apply the `release={name}` label
3. **Single Runtime**: Each Dataset has at most one bound Runtime
4. **Read-Only**: The mapper never modifies cluster state
5. **Namespace Scoped**: Operations are namespace-scoped by default
6. **API Availability**: Fluid CRDs are installed and API server is accessible
7. **Controller Semantics**: OwnerReferences follow standard Kubernetes conventions

---

## 7. Known Edge Cases

| Edge Case | Behavior | Mitigation |
|-----------|----------|------------|
| Runtime type not recognized | Return warning, skip specific resource discovery | Extensible runtime registry |
| Multiple runtimes in status | Use first runtime only | Document limitation, log warning |
| Cross-namespace references | Not supported in Fluid | Validate same namespace |
| CRD not installed | API returns 404 | Return clear error with installation guidance |
| Deleted resources with finalizers | May appear in list but fail to read | Handle 404 gracefully |
| Very large number of resources | Performance degradation | Pagination support |
| Cluster-scoped vs namespace-scoped | Some resources (PV) are cluster-scoped | Handle appropriately |

---

## 8. Future Extensibility

### 8.1 Planned Extensions

1. **Multi-Runtime Discovery**: Support for discovering all runtimes in a namespace
2. **Cluster-Wide Scanning**: Scan all datasets across namespaces
3. **Event Correlation**: Include recent events for debugging
4. **Metric Integration**: Pull metrics from Prometheus/Fluid exporter
5. **Interactive Mode**: Stream updates via watch

### 8.2 Plugin Architecture (Future)

```go
// RuntimePlugin interface for extensibility
type RuntimePlugin interface {
    Name() string
    SupportedKinds() []string
    DiscoverResources(ctx context.Context, name, namespace string) ([]K8sResourceNode, error)
    GetComponents() []ComponentSpec
}
```

### 8.3 Integration Points

| Integration | How |
|-------------|-----|
| `kubectl-fluid inspect` | Import as Go library |
| `kubectl-fluid diagnose` | Use ResourceGraph for analysis |
| CI/CD Tools | JSON output + exit codes |
| AI Pipelines | Structured graph for LLM context |
| Dashboards | REST API wrapper |

---

## 9. Security Considerations

1. **RBAC Requirements**:
   - Read access to `datasets.data.fluid.io`
   - Read access to `*runtimes.data.fluid.io`
   - Read access to core resources (pods, statefulsets, daemonsets, pvcs, pvs, configmaps, secrets)

2. **Minimal Permissions**: Per-resource-type RBAC rules for least privilege

3. **No Secrets Exposure**: Secret contents are never included in output, only metadata

4. **Audit Logging**: All API calls use standard K8s audit logging

---

## Appendix A: Label Reference

```yaml
# Standard Fluid Labels
release: {dataset-name}               # Primary correlation label
app: {runtime-type}                   # alluxio, jindo, juicefs, etc.
role: {runtime-type}-{component}      # e.g., alluxio-master

# Fluid Custom Labels
fluid.io/dataset: {dataset-name}
fluid.io/dataset-namespace: {namespace}
fluid.io/runtime-type: {type}

# Component Identification
component: master | worker | fuse
```

---

## Appendix B: API Resources Reference

| API Group | Version | Kind | Namespaced |
|-----------|---------|------|------------|
| data.fluid.io | v1alpha1 | Dataset | Yes |
| data.fluid.io | v1alpha1 | AlluxioRuntime | Yes |
| data.fluid.io | v1alpha1 | JindoRuntime | Yes |
| data.fluid.io | v1alpha1 | JuiceFSRuntime | Yes |
| data.fluid.io | v1alpha1 | GooseFSRuntime | Yes |
| data.fluid.io | v1alpha1 | VineyardRuntime | Yes |
| data.fluid.io | v1alpha1 | EFCRuntime | Yes |
| data.fluid.io | v1alpha1 | ThinRuntime | Yes |
| apps | v1 | StatefulSet | Yes |
| apps | v1 | DaemonSet | Yes |
| core | v1 | Pod | Yes |
| core | v1 | PersistentVolumeClaim | Yes |
| core | v1 | PersistentVolume | No |
| core | v1 | ConfigMap | Yes |
| core | v1 | Secret | Yes |

---

*End of Phase 0 Design Document*
