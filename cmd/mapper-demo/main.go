// Package main provides the demo CLI for the Fluid Resource Mapper
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fluid-cloudnative/fluid-resource-mapper/pkg/k8s"
	"github.com/fluid-cloudnative/fluid-resource-mapper/pkg/mapper"
	"github.com/fluid-cloudnative/fluid-resource-mapper/pkg/types"
)

// reorderArgs moves flags before positional arguments so flag.Parse works correctly
func reorderArgs(args []string) []string {
	var flags []string
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			// It's a flag
			flags = append(flags, arg)
			// Check if it's a flag with value (not a boolean)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				// Check if it's not one of the known boolean flags
				flagName := strings.TrimLeft(arg, "-")
				if flagName != "mock" && flagName != "pods" && flagName != "help" && flagName != "version" {
					i++
					flags = append(flags, args[i])
				}
			}
		} else {
			positional = append(positional, arg)
		}
		i++
	}

	return append(flags, positional...)
}

// Version information
const (
	version = "1.0.0"
	banner  = `
╭───────────────────────────────────────────────────────────────╮
│        Fluid Resource Mapper - Dataset Discovery Tool         │
│                        Version %s                           │
╰───────────────────────────────────────────────────────────────╯
`
)

// CLI flags
var (
	namespace    = flag.String("n", "default", "Kubernetes namespace")
	outputFormat = flag.String("o", "tree", "Output format: tree, json, wide")
	mockMode     = flag.Bool("mock", false, "Use mock data (no cluster required)")
	mockScenario = flag.String("scenario", "healthy", "Mock scenario: healthy, partial-ready, missing-runtime, missing-fuse, failed-pods")
	includePods  = flag.Bool("pods", true, "Include individual pods in output")
	kubeconfig   = flag.String("kubeconfig", "", "Path to kubeconfig file")
	showHelp     = flag.Bool("help", false, "Show help")
	showVersion  = flag.Bool("version", false, "Show version")
)

func main() {
	// Reorder args to allow flags after positional arguments
	args := reorderArgs(os.Args[1:])
	os.Args = append([]string{os.Args[0]}, args...)

	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("fluid-resource-mapper version %s\n", version)
		os.Exit(0)
	}

	if *showHelp || flag.NArg() < 1 {
		usage()
		os.Exit(0)
	}

	command := flag.Arg(0)
	resourceName := ""
	if flag.NArg() >= 2 {
		resourceName = flag.Arg(1)
	}

	switch command {
	case "dataset":
		mapDataset(resourceName)
	case "list":
		listDatasets()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Printf(banner, version)
	fmt.Println(`
USAGE:
    mapper-demo <command> <name> [flags]

COMMANDS:
    dataset <name>    Map resources for a Dataset
    list              List all Datasets in namespace

FLAGS:`)
	flag.PrintDefaults()
	fmt.Println(`
EXAMPLES:
    # Map a dataset in default namespace
    mapper-demo dataset demo-data

    # Map a dataset in specific namespace
    mapper-demo dataset demo-data -n fluid-system

    # Use mock mode for demo (no cluster needed)
    mapper-demo dataset demo-data --mock

    # Try different mock scenarios
    mapper-demo dataset demo-data --mock --scenario partial-ready
    mapper-demo dataset demo-data --mock --scenario missing-fuse
    mapper-demo dataset demo-data --mock --scenario failed-pods

    # Output as JSON
    mapper-demo dataset demo-data --mock -o json

MOCK SCENARIOS:
    healthy          Fully healthy deployment (default)
    partial-ready    Some pods/workers not ready
    missing-runtime  Dataset without bound Runtime
    missing-fuse     Fuse DaemonSet is missing
    failed-pods      Worker pods in failed state`)
}

func mapDataset(name string) {
	ctx := context.Background()

	// Create client
	var client k8s.Client
	if *mockMode {
		scenario := k8s.MockScenario(*mockScenario)
		client = k8s.NewMockClient(scenario)
		fmt.Println("🔧 Using MOCK mode - no cluster connection required")
		fmt.Printf("📋 Scenario: %s\n\n", *mockScenario)
	} else {
		realClient, err := k8s.NewClient(k8s.ClientConfig{
			KubeconfigPath: *kubeconfig,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to create Kubernetes client: %v\n", err)
			fmt.Fprintf(os.Stderr, "\n💡 Tip: Use --mock flag to run without a cluster\n")
			os.Exit(1)
		}
		client = realClient
	}

	// Create mapper
	m := mapper.New(client)

	// Map the dataset
	opts := mapper.Options{
		IncludePods:    *includePods,
		IncludeConfigs: true,
		IncludeStorage: true,
	}

	graph, err := m.MapFromDataset(ctx, name, *namespace, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Mapping failed: %v\n", err)
		os.Exit(1)
	}

	// Output
	switch *outputFormat {
	case "json":
		outputJSON(graph)
	case "wide":
		outputWide(graph)
	default:
		outputTree(graph)
	}

	// Exit with error code if unhealthy
	if !graph.IsHealthy() {
		os.Exit(1)
	}
}

func listDatasets() {
	fmt.Println("📋 Listing datasets in namespace:", *namespace)
	fmt.Println("(Not yet implemented - use 'dataset <name>' to map a specific dataset)")
}

func outputJSON(graph *types.ResourceGraph) {
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func outputTree(graph *types.ResourceGraph) {
	// Print header
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("📊 Resource Map for Dataset: %s/%s\n", graph.Dataset.Namespace, graph.Dataset.Name)
	fmt.Println(strings.Repeat("─", 60))

	// Dataset info
	datasetIcon := phaseIcon(graph.Dataset.Phase)
	fmt.Printf("\n%s Dataset: %s (%s)\n", datasetIcon, graph.Dataset.Name, graph.Dataset.Phase)
	if graph.Dataset.UfsTotal != "" {
		fmt.Printf("   📁 UFS Total: %s", graph.Dataset.UfsTotal)
		if graph.Dataset.Cached != "" {
			fmt.Printf(" | Cached: %s (%s)", graph.Dataset.Cached, graph.Dataset.CachedPercentage)
		}
		fmt.Println()
	}

	// Runtime info
	if graph.Runtime != nil {
		fmt.Printf("│\n└── 🔧 Runtime: %s (%s)\n", graph.Runtime.Name, graph.Runtime.Type)

		// Group resources by component
		masters := graph.GetResourcesByComponent(types.ComponentMaster)
		workers := graph.GetResourcesByComponent(types.ComponentWorker)
		fuses := graph.GetResourcesByComponent(types.ComponentFuse)
		storage := graph.GetResourcesByComponent(types.ComponentStorage)
		configs := graph.GetResourcesByComponent(types.ComponentConfig)

		// Print Master
		if len(masters) > 0 {
			for i, r := range masters {
				prefix := "    ├──"
				if i == len(masters)-1 && len(workers) == 0 && len(fuses) == 0 && len(storage) == 0 {
					prefix = "    └──"
				}
				fmt.Printf("%s %s %s: %s %s\n", prefix, r.Status.Phase.StatusIcon(), r.Kind, r.Name, colorReady(r.Status.Ready))
				printPodChildren(r.Children, "    │")
			}
		} else if graph.Runtime.MasterPhase != "" {
			fmt.Printf("    ├── ✗ Master: MISSING\n")
		}

		// Print Workers
		if len(workers) > 0 {
			for i, r := range workers {
				prefix := "    ├──"
				if i == len(workers)-1 && len(fuses) == 0 && len(storage) == 0 {
					prefix = "    └──"
				}
				fmt.Printf("%s %s %s: %s %s\n", prefix, r.Status.Phase.StatusIcon(), r.Kind, r.Name, colorReady(r.Status.Ready))
				printPodChildren(r.Children, "    │")
			}
		} else {
			fmt.Printf("    ├── ✗ Worker: MISSING\n")
		}

		// Print Fuse
		if len(fuses) > 0 {
			for i, r := range fuses {
				prefix := "    ├──"
				if i == len(fuses)-1 && len(storage) == 0 && len(configs) == 0 {
					prefix = "    └──"
				}
				fmt.Printf("%s %s %s: %s %s\n", prefix, r.Status.Phase.StatusIcon(), r.Kind, r.Name, colorReady(r.Status.Ready))
			}
		} else {
			fmt.Printf("    ├── ⚠ Fuse: Not deployed (on-demand)\n")
		}

		// Print Storage
		if len(storage) > 0 {
			fmt.Printf("    │\n")
			fmt.Printf("    ├── 💾 Storage\n")
			for i, r := range storage {
				prefix := "    │   ├──"
				if i == len(storage)-1 && len(configs) == 0 {
					prefix = "    │   └──"
				}
				fmt.Printf("%s %s %s: %s\n", prefix, r.Status.Phase.StatusIcon(), r.Kind, r.Name)
			}
		}

		// Print Configs
		if len(configs) > 0 {
			fmt.Printf("    │\n")
			fmt.Printf("    └── ⚙️  Configuration\n")
			for i, r := range configs {
				prefix := "        ├──"
				if i == len(configs)-1 {
					prefix = "        └──"
				}
				fmt.Printf("%s %s %s: %s\n", prefix, r.Status.Phase.StatusIcon(), r.Kind, r.Name)
			}
		}
	} else {
		fmt.Printf("│\n└── ⚠ No Runtime bound\n")
	}

	// Print warnings
	if len(graph.Warnings) > 0 {
		fmt.Printf("\n%s\n", strings.Repeat("─", 60))
		fmt.Printf("⚠️  Warnings (%d)\n", len(graph.Warnings))
		fmt.Println(strings.Repeat("─", 60))
		for _, w := range graph.Warnings {
			fmt.Printf("%s [%s] %s\n", w.Level.StatusIcon(), w.Code, w.Message)
			if w.Suggestion != "" {
				fmt.Printf("   💡 %s\n", w.Suggestion)
			}
		}
	}

	// Print summary
	fmt.Printf("\n%s\n", strings.Repeat("─", 60))
	fmt.Printf("📈 Summary: %d resources mapped in %s\n", len(graph.Resources), graph.Metadata.Duration)
	if graph.IsHealthy() {
		fmt.Println("✅ Status: HEALTHY")
	} else {
		fmt.Println("❌ Status: UNHEALTHY")
	}
	fmt.Println(strings.Repeat("─", 60))
}

func outputWide(graph *types.ResourceGraph) {
	outputTree(graph)
	fmt.Println("\n📋 Detailed Resource List:")
	fmt.Println(strings.Repeat("─", 100))
	fmt.Printf("%-20s %-30s %-15s %-10s %-15s\n", "KIND", "NAME", "COMPONENT", "STATUS", "AGE")
	fmt.Println(strings.Repeat("─", 100))
	for _, r := range graph.Resources {
		fmt.Printf("%-20s %-30s %-15s %-10s %-15s\n",
			r.Kind,
			truncate(r.Name, 28),
			r.Component,
			r.Status.Ready,
			r.Status.Age,
		)
	}
	fmt.Println(strings.Repeat("─", 100))
}

func printPodChildren(children []types.K8sResourceNode, indent string) {
	for i, pod := range children {
		prefix := indent + "   ├──"
		if i == len(children)-1 {
			prefix = indent + "   └──"
		}
		icon := "🟢"
		if pod.Status.Phase != types.PhaseReady && string(pod.Status.Phase) != "Running" {
			icon = "🟡"
			if pod.Status.Phase == types.PhaseFailed {
				icon = "🔴"
			}
		}
		fmt.Printf("%s %s Pod: %s (%s)\n", prefix, icon, pod.Name, pod.Status.Message)
	}
}

func phaseIcon(phase string) string {
	switch phase {
	case "Bound", "Ready":
		return "✓"
	case "NotBound", "NotReady", "Pending":
		return "⚠"
	case "Failed":
		return "✗"
	default:
		return "?"
	}
}

func colorReady(ready string) string {
	if ready == "" {
		return ""
	}
	return fmt.Sprintf("(%s)", ready)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}
