// Package mapper dataset resolution logic
package mapper

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fluid-cloudnative/fluid-resource-mapper/pkg/types"
)

// parseDataset converts an unstructured Dataset CR to a DatasetNode
func parseDataset(obj *unstructured.Unstructured) (*types.DatasetNode, error) {
	node := &types.DatasetNode{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Parse status
	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	if status != nil {
		if phase, ok := status["phase"].(string); ok {
			node.Phase = phase
		}

		if ufsTotal, ok := status["ufsTotal"].(string); ok {
			node.UfsTotal = ufsTotal
		}

		// Parse cache states
		if cacheStates, ok := status["cacheStates"].(map[string]interface{}); ok {
			if cached, ok := cacheStates["cached"].(string); ok {
				node.Cached = cached
			}
			if cachedPercentage, ok := cacheStates["cachedPercentage"].(string); ok {
				node.CachedPercentage = cachedPercentage
			}
		}

		// Parse conditions
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conditions {
				if cond, ok := c.(map[string]interface{}); ok {
					node.Conditions = append(node.Conditions, types.ConditionBrief{
						Type:               getStringField(cond, "type"),
						Status:             getStringField(cond, "status"),
						Reason:             getStringField(cond, "reason"),
						Message:            getStringField(cond, "message"),
						LastTransitionTime: getStringField(cond, "lastTransitionTime"),
					})
				}
			}
		}
	}

	// Parse spec for mount points
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	if spec != nil {
		if mounts, ok := spec["mounts"].([]interface{}); ok {
			for _, m := range mounts {
				if mount, ok := m.(map[string]interface{}); ok {
					if mp, ok := mount["mountPoint"].(string); ok {
						node.MountPoints = append(node.MountPoints, mp)
					}
				}
			}
		}
	}

	return node, nil
}

// getRuntimeTypeFromDataset extracts the runtime type from dataset status
func getRuntimeTypeFromDataset(obj *unstructured.Unstructured) (string, string, string, error) {
	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	if status == nil {
		return "", "", "", nil
	}

	runtimes, ok := status["runtimes"].([]interface{})
	if !ok || len(runtimes) == 0 {
		return "", "", "", nil
	}

	// Get the first runtime
	runtime, ok := runtimes[0].(map[string]interface{})
	if !ok {
		return "", "", "", nil
	}

	runtimeType := getStringField(runtime, "type")
	runtimeName := getStringField(runtime, "name")
	runtimeNamespace := getStringField(runtime, "namespace")

	return runtimeType, runtimeName, runtimeNamespace, nil
}

// getStringField safely extracts a string field from a map
func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
