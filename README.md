# Fluid Resource Mapper

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Fluid](https://img.shields.io/badge/Fluid-CNCF%20Incubating-9cf)](https://github.com/fluid-cloudnative/fluid)

> **A read-only, deterministic mapping engine that discovers and visualizes relationships between Fluid's high-level CRDs and their underlying Kubernetes resources.**

---

## 🎯 The Problem

When working with Fluid, users and operators struggle to answer a simple question:

> **"Given a Dataset, what Kubernetes resources actually exist, and how are they related?"**

While Fluid abstracts complexity behind `Dataset` and `Runtime` CRDs, debugging issues requires understanding the underlying resources:

| Fluid Abstraction | Hidden Kubernetes Resources |
|-------------------|-----------------------------|
| Dataset | PVC, PV, Labels on Nodes |
| Runtime | StatefulSets (Master, Worker), DaemonSets (Fuse), ConfigMaps, Secrets |
| Components | Individual Pods, Container statuses, Events |

**`kubectl get all` is insufficient** because:
1. It doesn't show CRDs (Dataset, Runtime)
2. It doesn't show PVCs/PVs in the same view
3. It doesn't show relationships/ownership
4. It can't identify missing or orphaned resources
5. It doesn't correlate health across the stack

---

## 💡 The Solution

The **Fluid Resource Mapper** provides:

✅ **Complete Discovery** — Start from Dataset, discover everything  
✅ **Relationship Mapping** — Show owner references and component roles  
✅ **Health Analysis** — Identify missing, unhealthy, or orphaned resources  
✅ **Multiple Outputs** — Human-readable tree, machine-readable JSON  
✅ **Mock Mode** — Demo without a cluster  
✅ **Library-First** — Embed in CLI tools, CI pipelines, AI agents  

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Fluid Resource Mapper                        │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐        │
│  │   Mapper    │────▶│  Resolver   │────▶│  Discovery  │        │
│  │   (Entry)   │     │  (CRD→CRD)  │     │  (K8s API)  │        │
│  └─────────────┘     └─────────────┘     └─────────────┘        │
│         │                  │                   │                │
│         ▼                  ▼                   ▼                │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐        │
│  │  Resource   │     │   Graph     │     │  Warning    │        │
│  │   Graph     │◀────│  Builder    │◀────│  Detector   │        │
│  └─────────────┘     └─────────────┘     └─────────────┘        │
├─────────────────────────────────────────────────────────────────┤
│                     Kubernetes Client                           │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ GET datasets, runtimes, statefulsets, daemonsets, pods,     ││
│  │     pvcs, pvs, configmaps, secrets                          ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

---

## 📦 Project Structure

```
fluid-resource-mapper/
├── cmd/
│   └── mapper-demo/        # Demo CLI binary
│       └── main.go
├── pkg/
│   ├── mapper/             # Core mapping logic
│   │   ├── mapper.go       # Main orchestrator
│   │   ├── dataset.go      # Dataset CR parsing
│   │   ├── runtime.go      # Runtime CR parsing
│   │   └── resources.go    # Discovery helpers
│   ├── k8s/                # Kubernetes client
│   │   ├── client.go       # Real K8s client
│   │   └── mock.go         # Mock client for demos
│   └── types/              # Data structures
│       └── graph.go        # Output type definitions
├── examples/
│   └── mock_output.json    # Example JSON output
├── PHASE0_DESIGN.md        # Design document
├── README.md               # This file
└── go.mod                  # Go module
```

---

## 🚀 Quick Start

### Demo Mode (No Cluster Required!)

```bash
# Build
go build -o mapper-demo ./cmd/mapper-demo

# Run with mock data
./mapper-demo dataset demo-data --mock
```

**Example Output:**

```
🔧 Using MOCK mode - no cluster connection required
📋 Scenario: healthy

────────────────────────────────────────────────────────────
📊 Resource Map for Dataset: default/demo-data
────────────────────────────────────────────────────────────

✓ Dataset: demo-data (Bound)
   📁 UFS Total: 100Gi | Cached: 25Gi (50%)
│
└── 🔧 Runtime: demo-data (alluxio)
    ├── ✓ StatefulSet: demo-data-master (1/1)
    │   └── 🟢 Pod: demo-data-master-0 (Running)
    ├── ✓ StatefulSet: demo-data-worker (2/2)
    │   ├── 🟢 Pod: demo-data-worker-0 (Running)
    │   └── 🟢 Pod: demo-data-worker-1 (Running)
    ├── ✓ DaemonSet: demo-data-fuse (3/3)
    │
    ├── 💾 Storage
    │   ├── ✓ PersistentVolumeClaim: demo-data
    │   └── ✓ PersistentVolume: demo-data-pv
    │
    └── ⚙️  Configuration
        ├── ✓ ConfigMap: demo-data-config
        ├── ✓ ConfigMap: demo-data-master-config
        ├── ✓ ConfigMap: demo-data-worker-config
        └── ✓ Secret: demo-data-secret

────────────────────────────────────────────────────────────
📈 Summary: 9 resources mapped in 1.234ms
✅ Status: HEALTHY
────────────────────────────────────────────────────────────
```

### Try Different Scenarios

```bash
# Partial readiness (some pods not ready)
./mapper-demo dataset demo-data --mock --scenario partial-ready

# Missing fuse DaemonSet
./mapper-demo dataset demo-data --mock --scenario missing-fuse

# Failed pods
./mapper-demo dataset demo-data --mock --scenario failed-pods

# Runtime not bound
./mapper-demo dataset demo-data --mock --scenario missing-runtime
```

### JSON Output

```bash
./mapper-demo dataset demo-data --mock -o json
```

### Real Cluster Mode

```bash
# Uses current kubeconfig context
./mapper-demo dataset my-dataset -n my-namespace

# Specify kubeconfig
./mapper-demo dataset my-dataset -n my-namespace --kubeconfig=/path/to/kubeconfig
```

---

## 📊 Output Formats

### Tree (Default)
Human-readable hierarchical view with icons and color-coded status.

### JSON
Machine-readable format for CI pipelines and tools:

```json
{
  "dataset": {
    "name": "demo-data",
    "namespace": "fluid-system",
    "phase": "Bound"
  },
  "runtime": {
    "name": "demo-data",
    "type": "alluxio",
    "masterReady": "1/1",
    "workerReady": "2/2",
    "fuseReady": "3/3"
  },
  "resources": [...],
  "warnings": [],
  "metadata": {
    "mappedAt": "2026-02-08T10:30:00Z",
    "duration": "45ms"
  }
}
```

### Wide
Table format with detailed resource information.

---

## 🔗 Integration Points

This mapper is designed to be embedded into:

| Tool | Use Case |
|------|----------|
| `kubectl-fluid inspect` | Visual resource inspection |
| `kubectl-fluid diagnose` | Automated problem detection |
| CI/CD Pipelines | Deployment validation |
| AI Diagnostic Agents | Context for LLM analysis |
| Monitoring Dashboards | Resource relationship views |

### Library Usage

```go
import (
    "github.com/fluid-cloudnative/fluid-resource-mapper/pkg/k8s"
    "github.com/fluid-cloudnative/fluid-resource-mapper/pkg/mapper"
)

// Create a client
client, _ := k8s.NewClient(k8s.ClientConfig{})

// Create the mapper
m := mapper.New(client)

// Map from a Dataset
graph, _ := m.MapFromDataset(ctx, "my-dataset", "my-namespace", mapper.DefaultOptions())

// Use the result
if !graph.IsHealthy() {
    for _, w := range graph.Warnings {
        log.Printf("Warning: %s - %s", w.Code, w.Message)
    }
}
```

---

## 🎭 Mock Scenarios

For demos and testing without a real cluster:

| Scenario | Description |
|----------|-------------|
| `healthy` | Fully healthy deployment (default) |
| `partial-ready` | Workers/Fuse not fully ready |
| `missing-runtime` | Dataset exists without bound Runtime |
| `missing-fuse` | Fuse DaemonSet is missing |
| `failed-pods` | Worker pods in failed state |
| `orphaned` | Resources without valid owner references |

---

## 📋 Supported Resources

| Fluid Component | Kubernetes Resource | Discovery Method |
|-----------------|---------------------|------------------|
| Master | StatefulSet | Label: `role=*-master` |
| Worker | StatefulSet | Label: `role=*-worker` |
| Fuse | DaemonSet | Label: `role=*-fuse` |
| Master Pods | Pod | Owner: Master StatefulSet |
| Worker Pods | Pod | Owner: Worker StatefulSet |
| Fuse Pods | Pod | Owner: Fuse DaemonSet |
| Data Volume | PVC | Label: `release={name}` |
| Data Volume | PV | Bound to PVC |
| Configs | ConfigMap | Label: `release={name}` |
| Secrets | Secret | Label: `release={name}` |

---

## ⚠️ Warning Detection

The mapper automatically detects:

| Issue | Code | Level |
|-------|------|-------|
| Dataset not found | `DATASET_NOT_FOUND` | Error |
| Runtime not bound | `RUNTIME_NOT_BOUND` | Warning |
| Master missing | `MASTER_MISSING` | Error |
| Worker missing | `WORKER_MISSING` | Error |
| Fuse missing | `FUSE_MISSING` | Warning |
| Pods not ready | `PODS_NOT_READY` | Warning |
| PVC missing | `PVC_MISSING` | Error |
| PV not bound | `PV_NOT_BOUND` | Warning |
| Orphaned resource | `ORPHANED_RESOURCE` | Warning |

---

## 🛠️ Development

### Prerequisites

- Go 1.21+
- (Optional) Kubernetes cluster with Fluid installed

### Build

```bash
go mod download
go build -o mapper-demo ./cmd/mapper-demo
```

### Test

```bash
go test ./...
```

### Lint

```bash
golangci-lint run
```

---

## 📖 Design Document

See [PHASE0_DESIGN.md](PHASE0_DESIGN.md) for detailed design decisions, CRD analysis, and architecture documentation.

---

## 🗺️ Roadmap

- [x] Phase 0: Design & Architecture
- [x] Phase 1: Core Mapper Engine
- [ ] Phase 2: `kubectl-fluid inspect` integration
- [ ] Phase 3: `kubectl-fluid diagnose` integration
- [ ] Phase 4: Event correlation
- [ ] Phase 5: Prometheus metrics integration

---

## 📄 License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

## 🤝 Contributing

Contributions are welcome! Please read our contributing guidelines and submit PRs to the main repository.

---

<p align="center">
  <b>Built for the Fluid community</b><br>
  <a href="https://github.com/fluid-cloudnative/fluid">Fluid</a> •
  <a href="https://fluid-cloudnative.github.io/">Documentation</a> •
  <a href="https://cloud-native.slack.com/messages/fluid">Slack</a>
</p>
