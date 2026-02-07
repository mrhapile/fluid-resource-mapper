// Package k8s provides a Kubernetes client wrapper for the Fluid Resource Mapper.
// It handles client initialization, API calls, and provides a mockable interface
// for testing without a real cluster.
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// FluidAPI group and version constants
const (
	FluidAPIGroup   = "data.fluid.io"
	FluidAPIVersion = "v1alpha1"
)

// FluidGVR returns the GroupVersionResource for a Fluid resource kind
func FluidGVR(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    FluidAPIGroup,
		Version:  FluidAPIVersion,
		Resource: resource,
	}
}

// Common Fluid GVRs
var (
	DatasetGVR         = FluidGVR("datasets")
	AlluxioRuntimeGVR  = FluidGVR("alluxioruntimes")
	JindoRuntimeGVR    = FluidGVR("jindoruntimes")
	JuiceFSRuntimeGVR  = FluidGVR("juicefsruntimes")
	GooseFSRuntimeGVR  = FluidGVR("goosefsruntimes")
	VineyardRuntimeGVR = FluidGVR("vineyardruntimes")
	EFCRuntimeGVR      = FluidGVR("efcruntimes")
	ThinRuntimeGVR     = FluidGVR("thinruntimes")
)

// RuntimeTypeToGVR maps runtime type strings to their GVRs
var RuntimeTypeToGVR = map[string]schema.GroupVersionResource{
	"alluxio":  AlluxioRuntimeGVR,
	"jindo":    JindoRuntimeGVR,
	"juicefs":  JuiceFSRuntimeGVR,
	"goosefs":  GooseFSRuntimeGVR,
	"vineyard": VineyardRuntimeGVR,
	"efc":      EFCRuntimeGVR,
	"thin":     ThinRuntimeGVR,
}

// Client provides a high-level interface for Kubernetes API operations
// needed by the Fluid Resource Mapper.
type Client interface {
	// Dataset operations
	GetDataset(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error)
	ListDatasets(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error)

	// Runtime operations
	GetRuntime(ctx context.Context, runtimeType, name, namespace string) (*unstructured.Unstructured, error)

	// Workload operations
	ListStatefulSets(ctx context.Context, namespace string, labelSelector string) (*appsv1.StatefulSetList, error)
	ListDaemonSets(ctx context.Context, namespace string, labelSelector string) (*appsv1.DaemonSetList, error)
	ListPods(ctx context.Context, namespace string, labelSelector string) (*corev1.PodList, error)

	// Storage operations
	ListPVCs(ctx context.Context, namespace string, labelSelector string) (*corev1.PersistentVolumeClaimList, error)
	GetPV(ctx context.Context, name string) (*corev1.PersistentVolume, error)
	ListPVs(ctx context.Context, labelSelector string) (*corev1.PersistentVolumeList, error)

	// Configuration operations
	ListConfigMaps(ctx context.Context, namespace string, labelSelector string) (*corev1.ConfigMapList, error)
	ListSecrets(ctx context.Context, namespace string, labelSelector string) (*corev1.SecretList, error)

	// Cluster info
	GetClusterName() string
}

// RealClient implements the Client interface using the real Kubernetes API
type RealClient struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	clusterName   string
}

// ClientConfig holds configuration for creating a Kubernetes client
type ClientConfig struct {
	// KubeconfigPath is the path to the kubeconfig file (optional, defaults to in-cluster or ~/.kube/config)
	KubeconfigPath string

	// Context is the kubeconfig context to use (optional)
	Context string

	// InCluster forces in-cluster configuration
	InCluster bool
}

// NewClient creates a new Kubernetes client with the given configuration
func NewClient(cfg ClientConfig) (*RealClient, error) {
	var restConfig *rest.Config
	var err error

	if cfg.InCluster {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	} else {
		kubeconfigPath := cfg.KubeconfigPath
		if kubeconfigPath == "" {
			// Try default locations
			if home := os.Getenv("HOME"); home != "" {
				kubeconfigPath = filepath.Join(home, ".kube", "config")
			}
			if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
				kubeconfigPath = envKubeconfig
			}
		}

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		loadingRules.ExplicitPath = kubeconfigPath

		configOverrides := &clientcmd.ConfigOverrides{}
		if cfg.Context != "" {
			configOverrides.CurrentContext = cfg.Context
		}

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

		restConfig, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Try to determine cluster name
	clusterName := "unknown"
	if cfg.Context != "" {
		clusterName = cfg.Context
	}

	return &RealClient{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		clusterName:   clusterName,
	}, nil
}

// GetClusterName returns the cluster name
func (c *RealClient) GetClusterName() string {
	return c.clusterName
}

// GetDataset retrieves a Dataset CR by name and namespace
func (c *RealClient) GetDataset(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	return c.dynamicClient.Resource(DatasetGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListDatasets lists all Datasets in a namespace
func (c *RealClient) ListDatasets(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	return c.dynamicClient.Resource(DatasetGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// GetRuntime retrieves a Runtime CR by type, name, and namespace
func (c *RealClient) GetRuntime(ctx context.Context, runtimeType, name, namespace string) (*unstructured.Unstructured, error) {
	gvr, ok := RuntimeTypeToGVR[runtimeType]
	if !ok {
		return nil, fmt.Errorf("unknown runtime type: %s", runtimeType)
	}
	return c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListStatefulSets lists StatefulSets in a namespace with optional label selector
func (c *RealClient) ListStatefulSets(ctx context.Context, namespace string, labelSelector string) (*appsv1.StatefulSetList, error) {
	return c.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// ListDaemonSets lists DaemonSets in a namespace with optional label selector
func (c *RealClient) ListDaemonSets(ctx context.Context, namespace string, labelSelector string) (*appsv1.DaemonSetList, error) {
	return c.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// ListPods lists Pods in a namespace with optional label selector
func (c *RealClient) ListPods(ctx context.Context, namespace string, labelSelector string) (*corev1.PodList, error) {
	return c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// ListPVCs lists PersistentVolumeClaims in a namespace with optional label selector
func (c *RealClient) ListPVCs(ctx context.Context, namespace string, labelSelector string) (*corev1.PersistentVolumeClaimList, error) {
	return c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// GetPV retrieves a PersistentVolume by name
func (c *RealClient) GetPV(ctx context.Context, name string) (*corev1.PersistentVolume, error) {
	return c.clientset.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
}

// ListPVs lists PersistentVolumes with optional label selector
func (c *RealClient) ListPVs(ctx context.Context, labelSelector string) (*corev1.PersistentVolumeList, error) {
	return c.clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// ListConfigMaps lists ConfigMaps in a namespace with optional label selector
func (c *RealClient) ListConfigMaps(ctx context.Context, namespace string, labelSelector string) (*corev1.ConfigMapList, error) {
	return c.clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// ListSecrets lists Secrets in a namespace with optional label selector
func (c *RealClient) ListSecrets(ctx context.Context, namespace string, labelSelector string) (*corev1.SecretList, error) {
	return c.clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}
