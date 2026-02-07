// Package mapper provides the core resource mapping logic for Fluid.
// It discovers all Kubernetes resources related to a Dataset or Runtime
// and builds a structured graph showing their relationships.
package mapper

import (
	"context"
	"fmt"
	"time"

	"github.com/fluid-cloudnative/fluid-resource-mapper/pkg/k8s"
	"github.com/fluid-cloudnative/fluid-resource-mapper/pkg/types"
)

const (
	// MapperVersion is the current version of the mapper
	MapperVersion = "1.0.0"
)

// Mapper is the main resource mapping engine
type Mapper struct {
	client k8s.Client
}

// Options configures the mapper behavior
type Options struct {
	// IncludePods includes individual pods in the resource graph
	IncludePods bool

	// IncludeConfigs includes ConfigMaps and Secrets
	IncludeConfigs bool

	// IncludeStorage includes PVCs and PVs
	IncludeStorage bool
}

// DefaultOptions returns sensible default options
func DefaultOptions() Options {
	return Options{
		IncludePods:    true,
		IncludeConfigs: true,
		IncludeStorage: true,
	}
}

// New creates a new Mapper with the given Kubernetes client
func New(client k8s.Client) *Mapper {
	return &Mapper{
		client: client,
	}
}

// MapFromDataset maps all resources starting from a Dataset CR
func (m *Mapper) MapFromDataset(ctx context.Context, name, namespace string, opts Options) (*types.ResourceGraph, error) {
	startTime := time.Now()

	graph := &types.ResourceGraph{
		Metadata: types.GraphMetadata{
			MappedAt:    startTime,
			ClusterName: m.client.GetClusterName(),
			Version:     MapperVersion,
		},
	}

	// Step 1: Fetch the Dataset
	dataset, err := m.resolveDataset(ctx, name, namespace)
	if err != nil {
		graph.Warnings = append(graph.Warnings, types.MappingWarning{
			Level:      types.WarningLevelError,
			Code:       types.WarningCodes.DatasetNotFound,
			Message:    fmt.Sprintf("Failed to get Dataset %s/%s: %v", namespace, name, err),
			Resource:   name,
			Suggestion: "Verify the Dataset name and namespace are correct",
		})
		graph.Metadata.Duration = time.Since(startTime).String()
		return graph, nil
	}
	graph.Dataset = *dataset

	// Step 2: Resolve the Runtime
	runtime, err := m.resolveRuntime(ctx, *dataset)
	if err != nil {
		graph.Warnings = append(graph.Warnings, types.MappingWarning{
			Level:      types.WarningLevelWarning,
			Code:       types.WarningCodes.RuntimeNotBound,
			Message:    fmt.Sprintf("No Runtime bound to Dataset: %v", err),
			Resource:   name,
			Suggestion: "Create a Runtime CR with the same name as the Dataset",
		})
	} else {
		graph.Runtime = runtime
	}

	// Step 3: Discover Kubernetes resources
	resources, warnings := m.discoverResources(ctx, name, namespace, runtime, opts)
	graph.Resources = resources
	graph.Warnings = append(graph.Warnings, warnings...)

	// Step 4: Detect additional warnings
	graph.Warnings = append(graph.Warnings, m.detectWarnings(graph, runtime)...)

	graph.Metadata.Duration = time.Since(startTime).String()

	return graph, nil
}

// resolveDataset fetches and parses a Dataset CR
func (m *Mapper) resolveDataset(ctx context.Context, name, namespace string) (*types.DatasetNode, error) {
	obj, err := m.client.GetDataset(ctx, name, namespace)
	if err != nil {
		return nil, err
	}

	return parseDataset(obj)
}

// resolveRuntime resolves the Runtime CR from the Dataset
func (m *Mapper) resolveRuntime(ctx context.Context, dataset types.DatasetNode) (*types.RuntimeNode, error) {
	// Check if dataset is bound
	if dataset.Phase != "Bound" {
		return nil, fmt.Errorf("dataset is not bound (phase: %s)", dataset.Phase)
	}

	// For now, use the dataset name to find the runtime
	// In a real implementation, we'd check .status.runtimes
	runtimeType := "alluxio" // Default to alluxio, in reality parse from status.runtimes

	obj, err := m.client.GetRuntime(ctx, runtimeType, dataset.Name, dataset.Namespace)
	if err != nil {
		return nil, err
	}

	return parseRuntime(obj, types.RuntimeType(runtimeType))
}

// discoverResources discovers all K8s resources related to the dataset
func (m *Mapper) discoverResources(ctx context.Context, name, namespace string, runtime *types.RuntimeNode, opts Options) ([]types.K8sResourceNode, []types.MappingWarning) {
	var resources []types.K8sResourceNode
	var warnings []types.MappingWarning

	labelSelector := fmt.Sprintf("release=%s", name)

	// Discover StatefulSets (Master, Worker)
	stsResources, stsWarnings := m.discoverStatefulSets(ctx, namespace, labelSelector, opts)
	resources = append(resources, stsResources...)
	warnings = append(warnings, stsWarnings...)

	// Discover DaemonSets (Fuse)
	dsResources, dsWarnings := m.discoverDaemonSets(ctx, namespace, labelSelector, opts)
	resources = append(resources, dsResources...)
	warnings = append(warnings, dsWarnings...)

	// Discover Storage resources
	if opts.IncludeStorage {
		storageResources, storageWarnings := m.discoverStorage(ctx, namespace, labelSelector)
		resources = append(resources, storageResources...)
		warnings = append(warnings, storageWarnings...)
	}

	// Discover Config resources
	if opts.IncludeConfigs {
		configResources, configWarnings := m.discoverConfigs(ctx, namespace, labelSelector)
		resources = append(resources, configResources...)
		warnings = append(warnings, configWarnings...)
	}

	return resources, warnings
}

// discoverStatefulSets discovers StatefulSet resources (master, worker)
func (m *Mapper) discoverStatefulSets(ctx context.Context, namespace, labelSelector string, opts Options) ([]types.K8sResourceNode, []types.MappingWarning) {
	var resources []types.K8sResourceNode
	var warnings []types.MappingWarning

	stsList, err := m.client.ListStatefulSets(ctx, namespace, labelSelector)
	if err != nil {
		warnings = append(warnings, types.MappingWarning{
			Level:   types.WarningLevelWarning,
			Code:    "STS_LIST_FAILED",
			Message: fmt.Sprintf("Failed to list StatefulSets: %v", err),
		})
		return resources, warnings
	}

	for _, sts := range stsList.Items {
		component := determineComponent(sts.Labels)
		phase := types.PhaseReady
		if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
			phase = types.PhaseNotReady
		}

		node := types.K8sResourceNode{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
			Name:       sts.Name,
			Namespace:  sts.Namespace,
			Component:  component,
			Status: types.ResourceStatus{
				Phase: phase,
				Ready: fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, *sts.Spec.Replicas),
				Age:   formatAge(sts.CreationTimestamp.Time),
			},
			Labels: filterLabels(sts.Labels),
		}

		// Include owner info
		if len(sts.OwnerReferences) > 0 {
			node.Owner = &types.OwnerInfo{
				Kind: sts.OwnerReferences[0].Kind,
				Name: sts.OwnerReferences[0].Name,
				UID:  string(sts.OwnerReferences[0].UID),
			}
		}

		// Include pods as children if requested
		if opts.IncludePods {
			pods, _ := m.discoverPodsForWorkload(ctx, namespace, sts.Name)
			node.Children = pods
		}

		resources = append(resources, node)
	}

	return resources, warnings
}

// discoverDaemonSets discovers DaemonSet resources (fuse)
func (m *Mapper) discoverDaemonSets(ctx context.Context, namespace, labelSelector string, opts Options) ([]types.K8sResourceNode, []types.MappingWarning) {
	var resources []types.K8sResourceNode
	var warnings []types.MappingWarning

	dsList, err := m.client.ListDaemonSets(ctx, namespace, labelSelector)
	if err != nil {
		warnings = append(warnings, types.MappingWarning{
			Level:   types.WarningLevelWarning,
			Code:    "DS_LIST_FAILED",
			Message: fmt.Sprintf("Failed to list DaemonSets: %v", err),
		})
		return resources, warnings
	}

	for _, ds := range dsList.Items {
		phase := types.PhaseReady
		if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
			phase = types.PhaseNotReady
		}

		node := types.K8sResourceNode{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
			Name:       ds.Name,
			Namespace:  ds.Namespace,
			Component:  types.ComponentFuse,
			Status: types.ResourceStatus{
				Phase: phase,
				Ready: fmt.Sprintf("%d/%d", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled),
				Age:   formatAge(ds.CreationTimestamp.Time),
			},
			Labels: filterLabels(ds.Labels),
		}

		// Include owner info
		if len(ds.OwnerReferences) > 0 {
			node.Owner = &types.OwnerInfo{
				Kind: ds.OwnerReferences[0].Kind,
				Name: ds.OwnerReferences[0].Name,
				UID:  string(ds.OwnerReferences[0].UID),
			}
		}

		resources = append(resources, node)
	}

	return resources, warnings
}

// discoverPodsForWorkload discovers pods owned by a workload
func (m *Mapper) discoverPodsForWorkload(ctx context.Context, namespace, workloadName string) ([]types.K8sResourceNode, []types.MappingWarning) {
	var resources []types.K8sResourceNode
	var warnings []types.MappingWarning

	// Get all pods and filter by owner
	podList, err := m.client.ListPods(ctx, namespace, "")
	if err != nil {
		return resources, warnings
	}

	for _, pod := range podList.Items {
		// Check if pod is owned by this workload
		isOwned := false
		for _, ref := range pod.OwnerReferences {
			if ref.Name == workloadName {
				isOwned = true
				break
			}
		}
		// For statefulsets, check naming convention
		if !isOwned && len(pod.Name) > len(workloadName) && pod.Name[:len(workloadName)] == workloadName {
			isOwned = true
		}

		if !isOwned {
			continue
		}

		phase := types.PhaseReady
		if pod.Status.Phase != "Running" {
			phase = types.ResourcePhase(pod.Status.Phase)
		}

		node := types.K8sResourceNode{
			Kind:       "Pod",
			APIVersion: "v1",
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			Component:  determineComponent(pod.Labels),
			Status: types.ResourceStatus{
				Phase:   phase,
				Message: string(pod.Status.Phase),
				Age:     formatAge(pod.CreationTimestamp.Time),
			},
			Labels: filterLabels(pod.Labels),
		}

		resources = append(resources, node)
	}

	return resources, warnings
}

// discoverStorage discovers PVC and PV resources
func (m *Mapper) discoverStorage(ctx context.Context, namespace, labelSelector string) ([]types.K8sResourceNode, []types.MappingWarning) {
	var resources []types.K8sResourceNode
	var warnings []types.MappingWarning

	pvcList, err := m.client.ListPVCs(ctx, namespace, labelSelector)
	if err != nil {
		warnings = append(warnings, types.MappingWarning{
			Level:   types.WarningLevelWarning,
			Code:    "PVC_LIST_FAILED",
			Message: fmt.Sprintf("Failed to list PVCs: %v", err),
		})
		return resources, warnings
	}

	for _, pvc := range pvcList.Items {
		phase := types.PhaseBound
		if pvc.Status.Phase != "Bound" {
			phase = types.PhaseNotBound
		}

		node := types.K8sResourceNode{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
			Name:       pvc.Name,
			Namespace:  pvc.Namespace,
			Component:  types.ComponentStorage,
			Status: types.ResourceStatus{
				Phase: phase,
				Age:   formatAge(pvc.CreationTimestamp.Time),
			},
			Details: map[string]string{
				"volumeName": pvc.Spec.VolumeName,
			},
		}

		resources = append(resources, node)

		// If PVC is bound, include the PV
		if pvc.Spec.VolumeName != "" {
			pv, err := m.client.GetPV(ctx, pvc.Spec.VolumeName)
			if err == nil {
				pvNode := types.K8sResourceNode{
					Kind:       "PersistentVolume",
					APIVersion: "v1",
					Name:       pv.Name,
					Component:  types.ComponentStorage,
					Status: types.ResourceStatus{
						Phase: types.ResourcePhase(pv.Status.Phase),
						Age:   formatAge(pv.CreationTimestamp.Time),
					},
					Owner: &types.OwnerInfo{
						Kind: "PersistentVolumeClaim",
						Name: pvc.Name,
					},
				}
				resources = append(resources, pvNode)
			}
		}
	}

	return resources, warnings
}

// discoverConfigs discovers ConfigMap and Secret resources
func (m *Mapper) discoverConfigs(ctx context.Context, namespace, labelSelector string) ([]types.K8sResourceNode, []types.MappingWarning) {
	var resources []types.K8sResourceNode
	var warnings []types.MappingWarning

	// ConfigMaps
	cmList, err := m.client.ListConfigMaps(ctx, namespace, labelSelector)
	if err != nil {
		warnings = append(warnings, types.MappingWarning{
			Level:   types.WarningLevelWarning,
			Code:    "CM_LIST_FAILED",
			Message: fmt.Sprintf("Failed to list ConfigMaps: %v", err),
		})
	} else {
		for _, cm := range cmList.Items {
			node := types.K8sResourceNode{
				Kind:       "ConfigMap",
				APIVersion: "v1",
				Name:       cm.Name,
				Namespace:  cm.Namespace,
				Component:  types.ComponentConfig,
				Status: types.ResourceStatus{
					Phase: types.PhaseReady,
					Age:   formatAge(cm.CreationTimestamp.Time),
				},
				Details: map[string]string{
					"keys": fmt.Sprintf("%d", len(cm.Data)),
				},
			}
			resources = append(resources, node)
		}
	}

	// Secrets
	secretList, err := m.client.ListSecrets(ctx, namespace, labelSelector)
	if err != nil {
		warnings = append(warnings, types.MappingWarning{
			Level:   types.WarningLevelWarning,
			Code:    "SECRET_LIST_FAILED",
			Message: fmt.Sprintf("Failed to list Secrets: %v", err),
		})
	} else {
		for _, secret := range secretList.Items {
			node := types.K8sResourceNode{
				Kind:       "Secret",
				APIVersion: "v1",
				Name:       secret.Name,
				Namespace:  secret.Namespace,
				Component:  types.ComponentConfig,
				Status: types.ResourceStatus{
					Phase: types.PhaseReady,
					Age:   formatAge(secret.CreationTimestamp.Time),
				},
				Details: map[string]string{
					"type": string(secret.Type),
					"keys": fmt.Sprintf("%d", len(secret.Data)),
				},
			}
			resources = append(resources, node)
		}
	}

	return resources, warnings
}

// detectWarnings analyzes the graph and detects additional warnings
func (m *Mapper) detectWarnings(graph *types.ResourceGraph, runtime *types.RuntimeNode) []types.MappingWarning {
	var warnings []types.MappingWarning

	// Check for missing master
	masters := graph.GetResourcesByComponent(types.ComponentMaster)
	if len(masters) == 0 && runtime != nil {
		warnings = append(warnings, types.MappingWarning{
			Level:      types.WarningLevelError,
			Code:       types.WarningCodes.MasterMissing,
			Message:    "No Master StatefulSet found",
			Suggestion: "Check if the runtime controller is running correctly",
		})
	}

	// Check for missing workers
	workers := graph.GetResourcesByComponent(types.ComponentWorker)
	if len(workers) == 0 && runtime != nil {
		warnings = append(warnings, types.MappingWarning{
			Level:      types.WarningLevelError,
			Code:       types.WarningCodes.WorkerMissing,
			Message:    "No Worker StatefulSet found",
			Suggestion: "Check if the runtime controller is running correctly",
		})
	}

	// Check for missing fuse
	fuseResources := graph.GetResourcesByComponent(types.ComponentFuse)
	if len(fuseResources) == 0 && runtime != nil {
		warnings = append(warnings, types.MappingWarning{
			Level:      types.WarningLevelWarning,
			Code:       types.WarningCodes.FuseMissing,
			Message:    "No Fuse DaemonSet found",
			Suggestion: "Fuse pods are created on-demand when data is accessed",
		})
	}

	// Check for unhealthy resources
	for _, res := range graph.Resources {
		if res.Status.Phase == types.PhaseNotReady || res.Status.Phase == types.PhaseFailed {
			warnings = append(warnings, types.MappingWarning{
				Level:    types.WarningLevelWarning,
				Code:     types.WarningCodes.PodsNotReady,
				Message:  fmt.Sprintf("%s %s is not ready (%s)", res.Kind, res.Name, res.Status.Ready),
				Resource: res.Name,
			})
		}
	}

	return warnings
}

// Helper functions

func determineComponent(labels map[string]string) types.ComponentType {
	role := labels["role"]
	switch {
	case contains(role, "master"):
		return types.ComponentMaster
	case contains(role, "worker"):
		return types.ComponentWorker
	case contains(role, "fuse"):
		return types.ComponentFuse
	default:
		return types.ComponentType("")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && s[len(s)-len(substr):] == substr || s[:len(substr)] == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func filterLabels(labels map[string]string) map[string]string {
	filtered := make(map[string]string)
	for k, v := range labels {
		// Only include relevant labels
		switch k {
		case "release", "app", "role", "component":
			filtered[k] = v
		}
	}
	return filtered
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
