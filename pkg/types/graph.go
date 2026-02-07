// Package types defines the core data structures for the Fluid Resource Mapper.
// These types represent the mapping output in a structured, machine-readable format.
package types

import (
	"time"
)

// RuntimeType represents the type of Fluid runtime
type RuntimeType string

const (
	RuntimeTypeAlluxio  RuntimeType = "alluxio"
	RuntimeTypeJindo    RuntimeType = "jindo"
	RuntimeTypeJuiceFS  RuntimeType = "juicefs"
	RuntimeTypeGooseFS  RuntimeType = "goosefs"
	RuntimeTypeVineyard RuntimeType = "vineyard"
	RuntimeTypeEFC      RuntimeType = "efc"
	RuntimeTypeThin     RuntimeType = "thin"
	RuntimeTypeUnknown  RuntimeType = "unknown"
)

// ComponentType represents the type of runtime component
type ComponentType string

const (
	ComponentMaster  ComponentType = "master"
	ComponentWorker  ComponentType = "worker"
	ComponentFuse    ComponentType = "fuse"
	ComponentStorage ComponentType = "storage"
	ComponentConfig  ComponentType = "config"
)

// WarningLevel represents the severity of a mapping warning
type WarningLevel string

const (
	WarningLevelError   WarningLevel = "error"
	WarningLevelWarning WarningLevel = "warning"
	WarningLevelInfo    WarningLevel = "info"
)

// ResourcePhase represents the lifecycle phase of a resource
type ResourcePhase string

const (
	PhaseReady    ResourcePhase = "Ready"
	PhaseNotReady ResourcePhase = "NotReady"
	PhasePending  ResourcePhase = "Pending"
	PhaseFailed   ResourcePhase = "Failed"
	PhaseUnknown  ResourcePhase = "Unknown"
	PhaseBound    ResourcePhase = "Bound"
	PhaseNotBound ResourcePhase = "NotBound"
)

// ResourceGraph is the main output structure containing the complete
// mapping of a Fluid Dataset to its underlying Kubernetes resources.
type ResourceGraph struct {
	// Dataset is the root Dataset CR
	Dataset DatasetNode `json:"dataset"`

	// Runtime is the bound Runtime CR (nil if not bound)
	Runtime *RuntimeNode `json:"runtime,omitempty"`

	// Resources is the list of all discovered Kubernetes resources
	Resources []K8sResourceNode `json:"resources"`

	// Warnings contains detected issues during mapping
	Warnings []MappingWarning `json:"warnings"`

	// Metadata contains mapping execution metadata
	Metadata GraphMetadata `json:"metadata"`
}

// DatasetNode represents the Dataset Custom Resource
type DatasetNode struct {
	// Name of the Dataset
	Name string `json:"name"`

	// Namespace where the Dataset exists
	Namespace string `json:"namespace"`

	// Phase is the current lifecycle phase (Bound, NotBound, Pending, Failed)
	Phase string `json:"phase"`

	// UfsTotal is the total size of the underlying filesystem
	UfsTotal string `json:"ufsTotal,omitempty"`

	// Cached is the amount of data currently cached
	Cached string `json:"cached,omitempty"`

	// CachedPercentage is the percentage of data cached
	CachedPercentage string `json:"cachedPercentage,omitempty"`

	// Conditions are the current conditions of the Dataset
	Conditions []ConditionBrief `json:"conditions,omitempty"`

	// MountPoints lists the configured mount points
	MountPoints []string `json:"mountPoints,omitempty"`
}

// RuntimeNode represents a Runtime Custom Resource (AlluxioRuntime, JindoRuntime, etc.)
type RuntimeNode struct {
	// Name of the Runtime (same as Dataset name)
	Name string `json:"name"`

	// Namespace where the Runtime exists
	Namespace string `json:"namespace"`

	// Type is the runtime type (alluxio, jindo, juicefs, etc.)
	Type RuntimeType `json:"type"`

	// MasterPhase is the phase of the master component
	MasterPhase string `json:"masterPhase,omitempty"`

	// WorkerPhase is the phase of the worker component
	WorkerPhase string `json:"workerPhase,omitempty"`

	// FusePhase is the phase of the fuse component
	FusePhase string `json:"fusePhase,omitempty"`

	// MasterReady shows ready/desired master instances (e.g., "1/1")
	MasterReady string `json:"masterReady,omitempty"`

	// WorkerReady shows ready/desired worker instances (e.g., "2/3")
	WorkerReady string `json:"workerReady,omitempty"`

	// FuseReady shows ready/desired fuse instances (e.g., "5/5")
	FuseReady string `json:"fuseReady,omitempty"`

	// Conditions are the current conditions of the Runtime
	Conditions []ConditionBrief `json:"conditions,omitempty"`
}

// K8sResourceNode represents a discovered Kubernetes resource
type K8sResourceNode struct {
	// Kind of the Kubernetes resource (StatefulSet, DaemonSet, Pod, PVC, etc.)
	Kind string `json:"kind"`

	// APIVersion of the resource
	APIVersion string `json:"apiVersion,omitempty"`

	// Name of the resource
	Name string `json:"name"`

	// Namespace of the resource (empty for cluster-scoped resources)
	Namespace string `json:"namespace,omitempty"`

	// Component indicates which Fluid component this resource belongs to
	Component ComponentType `json:"component"`

	// Status contains the health status of the resource
	Status ResourceStatus `json:"status"`

	// Owner contains ownership information
	Owner *OwnerInfo `json:"owner,omitempty"`

	// Labels are selected relevant labels
	Labels map[string]string `json:"labels,omitempty"`

	// Details contains additional resource-specific information
	Details map[string]string `json:"details,omitempty"`

	// Children are resources owned by this resource (e.g., Pods owned by StatefulSet)
	Children []K8sResourceNode `json:"children,omitempty"`
}

// ResourceStatus indicates the health status of a Kubernetes resource
type ResourceStatus struct {
	// Phase is the current phase (Ready, NotReady, Pending, Failed, Unknown)
	Phase ResourcePhase `json:"phase"`

	// Ready shows ready/desired count for workloads (e.g., "3/3")
	Ready string `json:"ready,omitempty"`

	// Message provides additional context about the status
	Message string `json:"message,omitempty"`

	// Age is the age of the resource
	Age string `json:"age,omitempty"`
}

// OwnerInfo contains information about the resource's owner
type OwnerInfo struct {
	// Kind of the owner resource
	Kind string `json:"kind"`

	// Name of the owner resource
	Name string `json:"name"`

	// UID of the owner resource
	UID string `json:"uid,omitempty"`
}

// ConditionBrief is a simplified view of a Kubernetes condition
type ConditionBrief struct {
	// Type of the condition (e.g., Ready, Progressing)
	Type string `json:"type"`

	// Status of the condition (True, False, Unknown)
	Status string `json:"status"`

	// Reason is a brief machine-readable reason
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message
	Message string `json:"message,omitempty"`

	// LastTransitionTime is when the condition last changed
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
}

// MappingWarning represents a detected issue during the mapping process
type MappingWarning struct {
	// Level indicates severity (error, warning, info)
	Level WarningLevel `json:"level"`

	// Code is a machine-readable error code
	Code string `json:"code"`

	// Message is a human-readable description
	Message string `json:"message"`

	// Resource is the name of the affected resource
	Resource string `json:"resource,omitempty"`

	// Suggestion provides remediation guidance
	Suggestion string `json:"suggestion,omitempty"`
}

// GraphMetadata contains metadata about the mapping operation
type GraphMetadata struct {
	// MappedAt is when the mapping was performed
	MappedAt time.Time `json:"mappedAt"`

	// Duration is how long the mapping took
	Duration string `json:"duration,omitempty"`

	// ClusterName is the name of the Kubernetes cluster
	ClusterName string `json:"clusterName,omitempty"`

	// Version is the mapper version
	Version string `json:"version"`

	// MockMode indicates if mock data was used
	MockMode bool `json:"mockMode,omitempty"`
}

// WarningCodes defines standard warning codes for the mapper
var WarningCodes = struct {
	DatasetNotFound    string
	RuntimeNotBound    string
	RuntimeNotFound    string
	MasterMissing      string
	WorkerMissing      string
	FuseMissing        string
	PodsNotReady       string
	PVCMissing         string
	PVNotBound         string
	ConfigMapMissing   string
	OrphanedResource   string
	UnknownRuntimeType string
	PartialCreation    string
	ScalingInProgress  string
	DeletionInProgress string
}{
	DatasetNotFound:    "DATASET_NOT_FOUND",
	RuntimeNotBound:    "RUNTIME_NOT_BOUND",
	RuntimeNotFound:    "RUNTIME_NOT_FOUND",
	MasterMissing:      "MASTER_MISSING",
	WorkerMissing:      "WORKER_MISSING",
	FuseMissing:        "FUSE_MISSING",
	PodsNotReady:       "PODS_NOT_READY",
	PVCMissing:         "PVC_MISSING",
	PVNotBound:         "PV_NOT_BOUND",
	ConfigMapMissing:   "CONFIGMAP_MISSING",
	OrphanedResource:   "ORPHANED_RESOURCE",
	UnknownRuntimeType: "UNKNOWN_RUNTIME_TYPE",
	PartialCreation:    "PARTIAL_CREATION",
	ScalingInProgress:  "SCALING_IN_PROGRESS",
	DeletionInProgress: "DELETION_IN_PROGRESS",
}

// StatusIcon returns a visual indicator for the given phase
func (p ResourcePhase) StatusIcon() string {
	switch p {
	case PhaseReady, PhaseBound:
		return "✓"
	case PhaseNotReady, PhasePending:
		return "⚠"
	case PhaseFailed, PhaseNotBound:
		return "✗"
	default:
		return "?"
	}
}

// StatusIcon returns a visual indicator for the given warning level
func (w WarningLevel) StatusIcon() string {
	switch w {
	case WarningLevelError:
		return "🔴"
	case WarningLevelWarning:
		return "⚠️"
	case WarningLevelInfo:
		return "ℹ️"
	default:
		return "?"
	}
}

// IsHealthy returns true if the resource graph represents a healthy state
func (g *ResourceGraph) IsHealthy() bool {
	for _, w := range g.Warnings {
		if w.Level == WarningLevelError {
			return false
		}
	}
	return true
}

// HasWarnings returns true if any warnings exist
func (g *ResourceGraph) HasWarnings() bool {
	return len(g.Warnings) > 0
}

// GetResourcesByKind returns all resources of a specific kind
func (g *ResourceGraph) GetResourcesByKind(kind string) []K8sResourceNode {
	var result []K8sResourceNode
	for _, r := range g.Resources {
		if r.Kind == kind {
			result = append(result, r)
		}
	}
	return result
}

// GetResourcesByComponent returns all resources of a specific component type
func (g *ResourceGraph) GetResourcesByComponent(component ComponentType) []K8sResourceNode {
	var result []K8sResourceNode
	for _, r := range g.Resources {
		if r.Component == component {
			result = append(result, r)
		}
	}
	return result
}

// Summary returns a brief summary of the resource graph
func (g *ResourceGraph) Summary() string {
	if g.Runtime == nil {
		return "Dataset: " + g.Dataset.Name + " (No Runtime)"
	}
	return "Dataset: " + g.Dataset.Name + " → " + string(g.Runtime.Type) + " Runtime"
}
