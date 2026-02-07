// Package mapper resource discovery helpers
package mapper

import (
	"github.com/fluid-cloudnative/fluid-resource-mapper/pkg/types"
)

// LabelSelectors contains standard Fluid label selectors
var LabelSelectors = struct {
	Release     func(name string) string
	RuntimeType func(runtimeType string) string
	Role        func(role string) string
}{
	Release: func(name string) string {
		return "release=" + name
	},
	RuntimeType: func(runtimeType string) string {
		return "app=" + runtimeType
	},
	Role: func(role string) string {
		return "role=" + role
	},
}

// NamingConventions contains standard Fluid naming patterns
var NamingConventions = struct {
	MasterStatefulSet func(name string) string
	WorkerStatefulSet func(name string) string
	FuseDaemonSet     func(name string) string
	MasterPod         func(name string, ordinal int) string
	WorkerPod         func(name string, ordinal int) string
	PVC               func(name string) string
	MasterConfig      func(name string) string
	WorkerConfig      func(name string) string
	FuseConfig        func(name string) string
}{
	MasterStatefulSet: func(name string) string { return name + "-master" },
	WorkerStatefulSet: func(name string) string { return name + "-worker" },
	FuseDaemonSet:     func(name string) string { return name + "-fuse" },
	MasterPod:         func(name string, ordinal int) string { return name + "-master-" + string(rune('0'+ordinal)) },
	WorkerPod:         func(name string, ordinal int) string { return name + "-worker-" + string(rune('0'+ordinal)) },
	PVC:               func(name string) string { return name },
	MasterConfig:      func(name string) string { return name + "-master-config" },
	WorkerConfig:      func(name string) string { return name + "-worker-config" },
	FuseConfig:        func(name string) string { return name + "-fuse-config" },
}

// ResourceKinds defines the Kubernetes resource kinds we discover
var ResourceKinds = struct {
	StatefulSet           string
	DaemonSet             string
	Pod                   string
	PersistentVolumeClaim string
	PersistentVolume      string
	ConfigMap             string
	Secret                string
	Service               string
}{
	StatefulSet:           "StatefulSet",
	DaemonSet:             "DaemonSet",
	Pod:                   "Pod",
	PersistentVolumeClaim: "PersistentVolumeClaim",
	PersistentVolume:      "PersistentVolume",
	ConfigMap:             "ConfigMap",
	Secret:                "Secret",
	Service:               "Service",
}

// FluidLabels defines standard Fluid labels
var FluidLabels = struct {
	Release          string
	App              string
	Role             string
	Component        string
	Dataset          string
	DatasetNamespace string
	RuntimeType      string
}{
	Release:          "release",
	App:              "app",
	Role:             "role",
	Component:        "component",
	Dataset:          "fluid.io/dataset",
	DatasetNamespace: "fluid.io/dataset-namespace",
	RuntimeType:      "fluid.io/runtime-type",
}

// ComponentRoles maps components to their expected label values
var ComponentRoles = map[types.ComponentType][]string{
	types.ComponentMaster: {"alluxio-master", "jindo-master", "juicefs-master", "goosefs-master", "vineyard-master", "efc-master"},
	types.ComponentWorker: {"alluxio-worker", "jindo-worker", "juicefs-worker", "goosefs-worker", "vineyard-worker", "efc-worker"},
	types.ComponentFuse:   {"alluxio-fuse", "jindo-fuse", "juicefs-fuse", "goosefs-fuse", "vineyard-fuse", "efc-fuse"},
}
