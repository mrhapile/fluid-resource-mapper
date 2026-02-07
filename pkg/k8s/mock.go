// Package k8s mock client implementation for demo and testing purposes.
package k8s

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MockClient implements the Client interface with mock data for demos and testing
type MockClient struct {
	// Scenario determines which mock data to return
	Scenario MockScenario
}

// MockScenario defines different mock scenarios for testing
type MockScenario string

const (
	// ScenarioHealthy represents a fully healthy Fluid deployment
	ScenarioHealthy MockScenario = "healthy"

	// ScenarioPartialReady represents a deployment with some pods not ready
	ScenarioPartialReady MockScenario = "partial-ready"

	// ScenarioMissingRuntime represents a dataset with no bound runtime
	ScenarioMissingRuntime MockScenario = "missing-runtime"

	// ScenarioMissingFuse represents a deployment where fuse DaemonSet is missing
	ScenarioMissingFuse MockScenario = "missing-fuse"

	// ScenarioFailedPods represents a deployment with failed pods
	ScenarioFailedPods MockScenario = "failed-pods"

	// ScenarioOrphaned represents orphaned resources (no owner)
	ScenarioOrphaned MockScenario = "orphaned"

	// ScenarioMultipleDatasets represents multiple datasets in the namespace
	ScenarioMultipleDatasets MockScenario = "multiple"
)

// NewMockClient creates a new mock client with the specified scenario
func NewMockClient(scenario MockScenario) *MockClient {
	return &MockClient{Scenario: scenario}
}

// GetClusterName returns a mock cluster name
func (m *MockClient) GetClusterName() string {
	return "mock-cluster"
}

// GetDataset returns mock Dataset data
func (m *MockClient) GetDataset(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	if m.Scenario == ScenarioMissingRuntime {
		return createMockDataset(name, namespace, "NotBound", nil), nil
	}

	// Default: bound dataset
	runtimes := []interface{}{
		map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"type":      "alluxio",
		},
	}
	return createMockDataset(name, namespace, "Bound", runtimes), nil
}

// ListDatasets returns mock Dataset list
func (m *MockClient) ListDatasets(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	datasets := &unstructured.UnstructuredList{}
	datasets.SetAPIVersion("data.fluid.io/v1alpha1")
	datasets.SetKind("DatasetList")

	if m.Scenario == ScenarioMultipleDatasets {
		// Return multiple datasets
		for _, name := range []string{"dataset-alpha", "dataset-beta", "dataset-gamma"} {
			runtimes := []interface{}{
				map[string]interface{}{
					"name":      name,
					"namespace": namespace,
					"type":      "alluxio",
				},
			}
			datasets.Items = append(datasets.Items, *createMockDataset(name, namespace, "Bound", runtimes))
		}
	} else {
		// Single demo-data dataset
		runtimes := []interface{}{
			map[string]interface{}{
				"name":      "demo-data",
				"namespace": namespace,
				"type":      "alluxio",
			},
		}
		datasets.Items = append(datasets.Items, *createMockDataset("demo-data", namespace, "Bound", runtimes))
	}

	return datasets, nil
}

// GetRuntime returns mock Runtime data
func (m *MockClient) GetRuntime(ctx context.Context, runtimeType, name, namespace string) (*unstructured.Unstructured, error) {
	if m.Scenario == ScenarioMissingRuntime {
		return nil, fmt.Errorf("runtime not found: %s/%s", namespace, name)
	}

	runtime := &unstructured.Unstructured{}
	runtime.SetAPIVersion("data.fluid.io/v1alpha1")
	runtime.SetKind("AlluxioRuntime")
	runtime.SetName(name)
	runtime.SetNamespace(namespace)

	masterPhase := "Ready"
	workerPhase := "Ready"
	fusePhase := "Ready"
	masterCurrent := int64(1)
	masterDesired := int64(1)
	workerCurrent := int64(2)
	workerDesired := int64(2)
	fuseCurrent := int64(3)
	fuseDesired := int64(3)

	switch m.Scenario {
	case ScenarioPartialReady:
		workerPhase = "PartialReady"
		workerCurrent = 1
	case ScenarioMissingFuse:
		fusePhase = "NotReady"
		fuseCurrent = 0
	case ScenarioFailedPods:
		workerPhase = "Failed"
		workerCurrent = 0
	}

	runtime.Object["spec"] = map[string]interface{}{
		"replicas": 2,
		"master": map[string]interface{}{
			"replicas": 1,
		},
		"worker": map[string]interface{}{
			"replicas": 2,
		},
	}
	runtime.Object["status"] = map[string]interface{}{
		"masterPhase":                  masterPhase,
		"workerPhase":                  workerPhase,
		"fusePhase":                    fusePhase,
		"currentMasterNumberScheduled": masterCurrent,
		"desiredMasterNumberScheduled": masterDesired,
		"currentWorkerNumberScheduled": workerCurrent,
		"desiredWorkerNumberScheduled": workerDesired,
		"currentFuseNumberScheduled":   fuseCurrent,
		"desiredFuseNumberScheduled":   fuseDesired,
		"conditions": []interface{}{
			map[string]interface{}{
				"type":               "Ready",
				"status":             "True",
				"lastTransitionTime": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				"reason":             "RuntimeReady",
				"message":            "Runtime is ready",
			},
		},
	}

	return runtime, nil
}

// ListStatefulSets returns mock StatefulSet list
func (m *MockClient) ListStatefulSets(ctx context.Context, namespace string, labelSelector string) (*appsv1.StatefulSetList, error) {
	list := &appsv1.StatefulSetList{}

	// Parse release name from label selector
	releaseName := "demo-data" // default

	// Master StatefulSet
	masterSts := createMockStatefulSet(releaseName+"-master", namespace, releaseName, "alluxio-master", 1, 1)
	list.Items = append(list.Items, masterSts)

	// Worker StatefulSet
	workerReplicas := int32(2)
	workerReady := int32(2)
	if m.Scenario == ScenarioPartialReady {
		workerReady = 1
	} else if m.Scenario == ScenarioFailedPods {
		workerReady = 0
	}
	workerSts := createMockStatefulSet(releaseName+"-worker", namespace, releaseName, "alluxio-worker", workerReplicas, workerReady)
	list.Items = append(list.Items, workerSts)

	return list, nil
}

// ListDaemonSets returns mock DaemonSet list
func (m *MockClient) ListDaemonSets(ctx context.Context, namespace string, labelSelector string) (*appsv1.DaemonSetList, error) {
	list := &appsv1.DaemonSetList{}

	if m.Scenario == ScenarioMissingFuse {
		return list, nil // No fuse DaemonSet
	}

	releaseName := "demo-data"
	desired := int32(3)
	ready := int32(3)

	if m.Scenario == ScenarioPartialReady {
		ready = 2
	}

	fuseDs := createMockDaemonSet(releaseName+"-fuse", namespace, releaseName, "alluxio-fuse", desired, ready)
	list.Items = append(list.Items, fuseDs)

	return list, nil
}

// ListPods returns mock Pod list
func (m *MockClient) ListPods(ctx context.Context, namespace string, labelSelector string) (*corev1.PodList, error) {
	list := &corev1.PodList{}
	releaseName := "demo-data"

	// Master pod
	masterPod := createMockPod(releaseName+"-master-0", namespace, releaseName, "alluxio-master", corev1.PodRunning)
	list.Items = append(list.Items, masterPod)

	// Worker pods
	workerStatus := corev1.PodRunning
	if m.Scenario == ScenarioFailedPods {
		workerStatus = corev1.PodFailed
	}
	for i := 0; i < 2; i++ {
		status := workerStatus
		if m.Scenario == ScenarioPartialReady && i == 1 {
			status = corev1.PodPending
		}
		workerPod := createMockPod(fmt.Sprintf("%s-worker-%d", releaseName, i), namespace, releaseName, "alluxio-worker", status)
		list.Items = append(list.Items, workerPod)
	}

	// Fuse pods
	if m.Scenario != ScenarioMissingFuse {
		fuseCount := 3
		if m.Scenario == ScenarioPartialReady {
			fuseCount = 2
		}
		for i := 0; i < fuseCount; i++ {
			fusePod := createMockPod(fmt.Sprintf("%s-fuse-%s", releaseName, generateHash(i)), namespace, releaseName, "alluxio-fuse", corev1.PodRunning)
			list.Items = append(list.Items, fusePod)
		}
	}

	return list, nil
}

// ListPVCs returns mock PVC list
func (m *MockClient) ListPVCs(ctx context.Context, namespace string, labelSelector string) (*corev1.PersistentVolumeClaimList, error) {
	list := &corev1.PersistentVolumeClaimList{}
	releaseName := "demo-data"

	pvc := createMockPVC(releaseName, namespace, releaseName)
	list.Items = append(list.Items, pvc)

	return list, nil
}

// GetPV returns a mock PV
func (m *MockClient) GetPV(ctx context.Context, name string) (*corev1.PersistentVolume, error) {
	pv := createMockPV(name)
	return &pv, nil
}

// ListPVs returns mock PV list
func (m *MockClient) ListPVs(ctx context.Context, labelSelector string) (*corev1.PersistentVolumeList, error) {
	list := &corev1.PersistentVolumeList{}
	pv := createMockPV("demo-data-pv")
	list.Items = append(list.Items, pv)
	return list, nil
}

// ListConfigMaps returns mock ConfigMap list
func (m *MockClient) ListConfigMaps(ctx context.Context, namespace string, labelSelector string) (*corev1.ConfigMapList, error) {
	list := &corev1.ConfigMapList{}
	releaseName := "demo-data"

	for _, suffix := range []string{"config", "master-config", "worker-config"} {
		cm := createMockConfigMap(releaseName+"-"+suffix, namespace, releaseName)
		list.Items = append(list.Items, cm)
	}

	return list, nil
}

// ListSecrets returns mock Secret list
func (m *MockClient) ListSecrets(ctx context.Context, namespace string, labelSelector string) (*corev1.SecretList, error) {
	list := &corev1.SecretList{}
	releaseName := "demo-data"

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseName + "-secret",
			Namespace: namespace,
			Labels: map[string]string{
				"release": releaseName,
				"app":     "alluxio",
			},
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
		},
		Type: corev1.SecretTypeOpaque,
	}
	list.Items = append(list.Items, secret)

	return list, nil
}

// Helper functions to create mock resources

func createMockDataset(name, namespace, phase string, runtimes []interface{}) *unstructured.Unstructured {
	dataset := &unstructured.Unstructured{}
	dataset.SetAPIVersion("data.fluid.io/v1alpha1")
	dataset.SetKind("Dataset")
	dataset.SetName(name)
	dataset.SetNamespace(namespace)
	dataset.SetCreationTimestamp(metav1.Time{Time: time.Now().Add(-24 * time.Hour)})

	dataset.Object["spec"] = map[string]interface{}{
		"mounts": []interface{}{
			map[string]interface{}{
				"mountPoint": "s3://example-bucket/data",
				"name":       "data",
			},
		},
	}

	status := map[string]interface{}{
		"phase": phase,
		"conditions": []interface{}{
			map[string]interface{}{
				"type":               "Ready",
				"status":             "True",
				"lastTransitionTime": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				"reason":             "DatasetReady",
				"message":            "Dataset is ready",
			},
		},
	}

	if phase == "Bound" {
		status["ufsTotal"] = "100Gi"
		status["cacheStates"] = map[string]interface{}{
			"cacheCapacity":    "50Gi",
			"cached":           "25Gi",
			"cachedPercentage": "50%",
		}
	}

	if runtimes != nil {
		status["runtimes"] = runtimes
	}

	dataset.Object["status"] = status

	return dataset
}

func createMockStatefulSet(name, namespace, release, role string, replicas, ready int32) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"release": release,
				"app":     "alluxio",
				"role":    role,
			},
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "data.fluid.io/v1alpha1",
					Kind:       "AlluxioRuntime",
					Name:       release,
					UID:        "mock-uid-runtime",
				},
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      replicas,
			ReadyReplicas: ready,
		},
	}
}

func createMockDaemonSet(name, namespace, release, role string, desired, ready int32) appsv1.DaemonSet {
	return appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"release": release,
				"app":     "alluxio",
				"role":    role,
			},
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "data.fluid.io/v1alpha1",
					Kind:       "AlluxioRuntime",
					Name:       release,
					UID:        "mock-uid-runtime",
				},
			},
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: desired,
			NumberReady:            ready,
			CurrentNumberScheduled: ready,
		},
	}
}

func createMockPod(name, namespace, release, role string, phase corev1.PodPhase) corev1.Pod {
	containerStatus := corev1.ContainerStatus{
		Name:  "main",
		Ready: phase == corev1.PodRunning,
		State: corev1.ContainerState{},
	}
	if phase == corev1.PodRunning {
		containerStatus.State.Running = &corev1.ContainerStateRunning{
			StartedAt: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
		}
	}

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"release": release,
				"app":     "alluxio",
				"role":    role,
			},
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
		},
		Status: corev1.PodStatus{
			Phase:             phase,
			ContainerStatuses: []corev1.ContainerStatus{containerStatus},
		},
	}
}

func createMockPVC(name, namespace, release string) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"release": release,
			},
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: name + "-pv",
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("100Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
}

func createMockPV(name string) corev1.PersistentVolume {
	return corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("100Gi"),
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}
}

func createMockConfigMap(name, namespace, release string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"release": release,
				"app":     "alluxio",
			},
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
		},
		Data: map[string]string{
			"alluxio-site.properties": "alluxio.master.hostname=demo-data-master-0",
		},
	}
}

func generateHash(i int) string {
	hashes := []string{"a1b2c", "d3e4f", "g5h6i", "j7k8l", "m9n0p"}
	return hashes[i%len(hashes)]
}
