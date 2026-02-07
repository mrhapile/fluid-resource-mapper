// Package mapper runtime resolution logic
package mapper

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fluid-cloudnative/fluid-resource-mapper/pkg/types"
)

// parseRuntime converts an unstructured Runtime CR to a RuntimeNode
func parseRuntime(obj *unstructured.Unstructured, runtimeType types.RuntimeType) (*types.RuntimeNode, error) {
	node := &types.RuntimeNode{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Type:      runtimeType,
	}

	// Parse status
	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	if status != nil {
		// Component phases
		if masterPhase, ok := status["masterPhase"].(string); ok {
			node.MasterPhase = masterPhase
		}
		if workerPhase, ok := status["workerPhase"].(string); ok {
			node.WorkerPhase = workerPhase
		}
		if fusePhase, ok := status["fusePhase"].(string); ok {
			node.FusePhase = fusePhase
		}

		// Master ready status
		masterCurrent := getInt64Field(status, "currentMasterNumberScheduled")
		masterDesired := getInt64Field(status, "desiredMasterNumberScheduled")
		if masterDesired > 0 {
			node.MasterReady = fmt.Sprintf("%d/%d", masterCurrent, masterDesired)
		}

		// Worker ready status
		workerCurrent := getInt64Field(status, "currentWorkerNumberScheduled")
		workerDesired := getInt64Field(status, "desiredWorkerNumberScheduled")
		if workerDesired > 0 {
			node.WorkerReady = fmt.Sprintf("%d/%d", workerCurrent, workerDesired)
		}

		// Fuse ready status
		fuseCurrent := getInt64Field(status, "currentFuseNumberScheduled")
		fuseDesired := getInt64Field(status, "desiredFuseNumberScheduled")
		if fuseDesired > 0 {
			node.FuseReady = fmt.Sprintf("%d/%d", fuseCurrent, fuseDesired)
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

	return node, nil
}

// getInt64Field safely extracts an int64 field from a map
func getInt64Field(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(int64); ok {
		return v
	}
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

// RuntimeComponents defines which components each runtime type supports
type RuntimeComponents struct {
	HasMaster bool
	HasWorker bool
	HasFuse   bool
}

// GetRuntimeComponents returns the component configuration for a runtime type
func GetRuntimeComponents(runtimeType types.RuntimeType) RuntimeComponents {
	switch runtimeType {
	case types.RuntimeTypeAlluxio, types.RuntimeTypeJindo, types.RuntimeTypeGooseFS, types.RuntimeTypeVineyard, types.RuntimeTypeEFC:
		return RuntimeComponents{HasMaster: true, HasWorker: true, HasFuse: true}
	case types.RuntimeTypeJuiceFS:
		return RuntimeComponents{HasMaster: false, HasWorker: true, HasFuse: true}
	case types.RuntimeTypeThin:
		return RuntimeComponents{HasMaster: false, HasWorker: false, HasFuse: true}
	default:
		return RuntimeComponents{HasMaster: true, HasWorker: true, HasFuse: true}
	}
}
