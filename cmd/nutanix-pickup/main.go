// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
	"github.com/zyvorai/transiva/providers/nutanix"
)

func main() {
	prism := flag.String("prism", "", "Prism Central/Element IP or FQDN")
	user := flag.String("user", "admin", "Prism username")
	pass := flag.String("pass", "", "Prism password")
	cluster := flag.String("cluster", "", "Filter by cluster name or UUID (optional)")
	insecure := flag.Bool("insecure", true, "Skip TLS verify (common in labs)")
	detailed := flag.Bool("detailed", true, "Fetch per-VM disk UUIDs and container info")
	resolveContainers := flag.Bool("resolve-containers", false, "Resolve storage container names via Prism API")
	format := flag.String("format", "table", "Output format: table, json, or pickup-plan")
	outFile := flag.String("out", "", "Write JSON output to file (optional)")

	planIn := flag.String("plan-in", "", "Execute an existing pickup plan JSON file (skips discovery)")
	execute := flag.Bool("execute", false, "Convert disks from mounted containers with qemu-img")
	outputDir := flag.String("output-dir", "", "Output directory for converted disk images")
	diskFormat := flag.String("disk-format", "qcow2", "Converted disk format: qcow2 or raw")
	mountsSpec := flag.String("mounts", "", "Comma-separated container mounts: name1:/path1,name2:/path2")
	dryRun := flag.Bool("dry-run", false, "Resolve sources and print actions without converting")
	vmFilter := flag.String("vm", "", "Process only this VM name or UUID (with --execute)")
	flag.Parse()

	if *planIn == "" && (*prism == "" || *pass == "") {
		fmt.Fprintln(os.Stderr, "Error: --prism and --pass are required unless --plan-in is set")
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var plan nutanix.PickupPlan
	if *planIn != "" {
		data, err := os.ReadFile(*planIn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading plan: %v\n", err)
			os.Exit(1)
		}
		if err := json.Unmarshal(data, &plan); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing plan: %v\n", err)
			os.Exit(1)
		}
	} else {
		plan = discoverPlan(ctx, *prism, *user, *pass, *cluster, *insecure, *detailed, *resolveContainers, *format)
	}

	if *execute || *dryRun {
		runExecute(ctx, plan, *outputDir, *diskFormat, *mountsSpec, *dryRun, *vmFilter)
		return
	}

	emitPlanOutput(plan, *format, *outFile)
}

func discoverPlan(ctx context.Context, prism, user, pass, cluster string, insecure, detailed, resolveContainers bool, format string) nutanix.PickupPlan {
	log := logger.New("warn")
	providerCfg := providers.ProviderConfig{
		Type:     providers.ProviderNutanix,
		Host:     prism,
		Port:     9440,
		Username: user,
		Password: pass,
		Insecure: insecure,
		Metadata: map[string]interface{}{
			"detailed": detailed,
			"cluster":  cluster,
		},
	}

	provider, err := nutanix.NewProvider(providerCfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Connecting to https://%s:9440 ...\n", prism)

	filter := providers.VMFilter{}
	if cluster != "" {
		filter.Location = cluster
	}

	vms, err := provider.ListVMs(ctx, filter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	inventory := make([]nutanix.VMInventory, 0, len(vms))
	for _, vm := range vms {
		inventory = append(inventory, nutanix.InventoryFromVMInfo(vm))
	}
	fmt.Fprintf(os.Stderr, "Fetched %d VMs\n", len(inventory))

	var containerNames map[string]string
	if resolveContainers || format == "pickup-plan" {
		nxProvider, ok := provider.(*nutanix.Provider)
		if ok {
			fmt.Fprintln(os.Stderr, "Resolving storage container names...")
			containerNames, err = nxProvider.ResolveContainerNames(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: container name resolution failed: %v\n", err)
			}
		}
	}

	return nutanix.BuildPickupPlan(prism, inventory, containerNames)
}

func runExecute(ctx context.Context, plan nutanix.PickupPlan, outputDir, diskFormat, mountsSpec string, dryRun bool, vmFilter string) {
	if outputDir == "" {
		fmt.Fprintln(os.Stderr, "Error: --output-dir is required with --execute")
		os.Exit(1)
	}

	specs := strings.Split(mountsSpec, ",")
	mounts, err := nutanix.ParseMountMap(specs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	opts := nutanix.PickupExecuteOptions{
		OutputDir: outputDir,
		Format:    diskFormat,
		Mounts:    mounts,
		DryRun:    dryRun,
	}

	vms := plan.VMs
	if vmFilter != "" {
		vms = filterPickupVMs(vms, vmFilter)
		if len(vms) == 0 {
			fmt.Fprintf(os.Stderr, "No VMs matched filter %q\n", vmFilter)
			os.Exit(1)
		}
	}

	for _, vm := range vms {
		fmt.Fprintf(os.Stderr, "Processing VM %s (%s)...\n", vm.Name, vm.UUID)
		result, err := nutanix.ExecutePickupVM(ctx, vm, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		for _, d := range result.Disks {
			fmt.Printf("  %s -> %s (source: %s)\n", d.UUID, d.OutputPath, d.SourcePath)
		}
		if result.ManifestPath != "" {
			fmt.Printf("  manifest: %s\n", result.ManifestPath)
		}
	}
}

func filterPickupVMs(vms []nutanix.PickupVM, filter string) []nutanix.PickupVM {
	filter = strings.ToLower(strings.TrimSpace(filter))
	var out []nutanix.PickupVM
	for _, vm := range vms {
		if strings.EqualFold(vm.UUID, filter) || strings.Contains(strings.ToLower(vm.Name), filter) {
			out = append(out, vm)
		}
	}
	return out
}

func emitPlanOutput(plan nutanix.PickupPlan, format, outFile string) {
	switch format {
	case "json":
		writeJSON(plan.VMs, outFile)
	case "pickup-plan":
		writeJSON(plan, outFile)
	default:
		printTableFromPlan(plan)
	}
}

func writeJSON(v interface{}, outFile string) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if outFile != "" {
		if err := os.WriteFile(outFile, data, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote output to %s\n", outFile)
		return
	}
	fmt.Println(string(data))
}

func printTableFromPlan(plan nutanix.PickupPlan) {
	fmt.Printf("%-30s %-36s %-20s %-10s %6s %10s %8s %8s\n",
		"NAME", "UUID", "CLUSTER", "STATE", "vCPU", "MEM(GiB)", "DISKS", "DISK(GiB)")
	fmt.Println(strings.Repeat("-", 140))
	for _, v := range plan.VMs {
		clusterDisp := v.ClusterName
		if clusterDisp == "" && v.ClusterUUID != "" {
			clusterDisp = v.ClusterUUID
			if len(clusterDisp) > 8 {
				clusterDisp = clusterDisp[:8] + "..."
			}
		}
		fmt.Printf("%-30s %-36s %-20s %-10s %6d %10.1f %8d %8.1f\n",
			truncate(v.Name, 28), v.UUID, truncate(clusterDisp, 18), v.PowerState,
			v.VCPUs, v.MemoryGiB, v.DiskCount, v.TotalDiskGiB)
	}
	fmt.Printf("\nTotal VMs: %d\n", plan.VMCount)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
