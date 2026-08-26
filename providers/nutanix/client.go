// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"
	"sync"

	ntnxapi "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/api"
	ntnxclient "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/client"
	ahvconfig "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"

	"github.com/zyvorai/transiva/logger"
)

// Client wraps the Nutanix v4 VMM API for VM discovery.
type Client struct {
	apiClient *ntnxclient.ApiClient
	vmAPI     *ntnxapi.VmApi
	cfg       *ClientConfig
	logger    logger.Logger
}

// NewClient creates a Nutanix Prism Central client.
func NewClient(cfg *ClientConfig, log logger.Logger) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nutanix config is required")
	}
	if cfg.Host == "" {
		return nil, fmt.Errorf("nutanix host is required")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, fmt.Errorf("nutanix username and password are required")
	}

	port := cfg.Port
	if port == 0 {
		port = 9440
	}

	apiClient := ntnxclient.NewApiClient()
	apiClient.Host = cfg.Host
	apiClient.Port = port
	apiClient.Username = cfg.Username
	apiClient.Password = cfg.Password
	apiClient.VerifySSL = cfg.VerifySSL

	if log != nil {
		log.Info("nutanix client configured", "host", cfg.Host, "port", port)
	}

	return &Client{
		apiClient: apiClient,
		vmAPI:     ntnxapi.NewVmApi(apiClient),
		cfg:       cfg,
		logger:    log,
	}, nil
}

// ListVMs returns VM inventory from Prism Central.
func (c *Client) ListVMs(ctx context.Context, detailed bool) ([]VMInventory, error) {
	pageSize := c.cfg.PageSize
	if pageSize <= 0 {
		pageSize = 500
	}

	var summaries []VMInventory
	page := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pageArg := page
		limitArg := pageSize
		resp, err := c.vmAPI.ListVms(&pageArg, &limitArg, nil, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("list VMs page %d: %w", page, err)
		}
		if resp == nil {
			break
		}

		data := resp.GetData()
		if data == nil {
			break
		}

		vms, ok := data.([]ahvconfig.Vm)
		if !ok {
			return nil, fmt.Errorf("unexpected list VMs response type %T", data)
		}

		for i := range vms {
			inv := vmToInventory(&vms[i], detailed)
			if c.cfg.ClusterFilter != "" && !clusterMatches(inv, c.cfg.ClusterFilter) {
				continue
			}
			summaries = append(summaries, inv)
		}

		if len(vms) < pageSize {
			break
		}
		if resp.Metadata != nil && resp.Metadata.TotalAvailableResults != nil {
			if len(summaries) >= *resp.Metadata.TotalAvailableResults {
				break
			}
		}
		page++
	}

	if !detailed || len(summaries) == 0 {
		return summaries, nil
	}

	return c.enrichDetails(ctx, summaries)
}

// GetVM returns detailed inventory for a single VM by extId.
func (c *Client) GetVM(ctx context.Context, extID string) (*VMInventory, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	resp, err := c.vmAPI.GetVmById(&extID)
	if err != nil {
		return nil, fmt.Errorf("get VM %s: %w", extID, err)
	}
	if resp == nil || resp.GetData() == nil {
		return nil, fmt.Errorf("VM %s not found", extID)
	}

	vm, ok := resp.GetData().(ahvconfig.Vm)
	if !ok {
		return nil, fmt.Errorf("unexpected get VM response type %T", resp.GetData())
	}

	inv := vmToInventory(&vm, true)
	return &inv, nil
}

func (c *Client) enrichDetails(ctx context.Context, summaries []VMInventory) ([]VMInventory, error) {
	workers := c.cfg.DetailWorkers
	if workers <= 0 {
		workers = 10
	}

	type result struct {
		index int
		vm    VMInventory
		err   error
	}

	jobs := make(chan int, len(summaries))
	results := make(chan result, len(summaries))

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				select {
				case <-ctx.Done():
					results <- result{index: idx, err: ctx.Err()}
					return
				default:
				}

				detail, err := c.GetVM(ctx, summaries[idx].UUID)
				if err != nil {
					results <- result{index: idx, err: err}
					continue
				}
				results <- result{index: idx, vm: *detail}
			}
		}()
	}

	for i := range summaries {
		jobs <- i
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	out := make([]VMInventory, len(summaries))
	copy(out, summaries)

	for res := range results {
		if res.err != nil {
			if c.logger != nil {
				c.logger.Warn("failed to fetch VM details",
					"uuid", summaries[res.index].UUID,
					"name", summaries[res.index].Name,
					"error", res.err)
			}
			continue
		}
		out[res.index] = res.vm
	}

	return out, nil
}

func vmToInventory(vm *ahvconfig.Vm, includeDisks bool) VMInventory {
	inv := VMInventory{
		Name:       derefString(vm.Name),
		UUID:       derefString(vm.ExtId),
		PowerState: powerStateString(vm.PowerState),
		VCPUs:      vmVCPUs(vm),
		MemoryGiB:  bytesToGiB(vm.MemorySizeBytes),
		NICCount:   len(vm.Nics),
	}

	if vm.Cluster != nil {
		inv.ClusterUUID = derefString(vm.Cluster.ExtId)
	}

	if includeDisks {
		inv.Disks = extractDisks(vm.Disks)
		inv.DiskCount = len(inv.Disks)
		for _, d := range inv.Disks {
			inv.TotalDiskGiB += d.SizeGiB
		}
	} else {
		inv.DiskCount = len(vm.Disks)
	}

	return inv
}

func extractDisks(disks []ahvconfig.Disk) []DiskInfo {
	out := make([]DiskInfo, 0, len(disks))
	for _, disk := range disks {
		info := DiskInfo{
			UUID:       derefString(disk.ExtId),
			DeviceType: "DISK",
		}
		if disk.DiskAddress != nil {
			bus := "SCSI"
			if disk.DiskAddress.BusType != nil {
				bus = disk.DiskAddress.BusType.GetName()
			}
			index := 0
			if disk.DiskAddress.Index != nil {
				index = *disk.DiskAddress.Index
			}
			info.DiskAddress = fmt.Sprintf("%s:%d", bus, index)
		}

		if disk.BackingInfo != nil {
			switch backing := disk.BackingInfo.GetValue().(type) {
			case ahvconfig.VmDisk:
				if info.UUID == "" {
					info.UUID = derefString(backing.DiskExtId)
				}
				info.SizeGiB = bytesToGiB(backing.DiskSizeBytes)
				if backing.StorageContainer != nil {
					info.ContainerUUID = derefString(backing.StorageContainer.ExtId)
				}
			case *ahvconfig.VmDisk:
				if backing != nil {
					if info.UUID == "" {
						info.UUID = derefString(backing.DiskExtId)
					}
					info.SizeGiB = bytesToGiB(backing.DiskSizeBytes)
					if backing.StorageContainer != nil {
						info.ContainerUUID = derefString(backing.StorageContainer.ExtId)
					}
				}
			default:
				info.DeviceType = "OTHER"
			}
		}

		if info.UUID != "" {
			out = append(out, info)
		}
	}
	return out
}

func vmVCPUs(vm *ahvconfig.Vm) int {
	sockets := 1
	cores := 1
	threads := 1
	if vm.NumSockets != nil && *vm.NumSockets > 0 {
		sockets = *vm.NumSockets
	}
	if vm.NumCoresPerSocket != nil && *vm.NumCoresPerSocket > 0 {
		cores = *vm.NumCoresPerSocket
	}
	if vm.NumThreadsPerCore != nil && *vm.NumThreadsPerCore > 0 {
		threads = *vm.NumThreadsPerCore
	}
	return sockets * cores * threads
}

func powerStateString(state *ahvconfig.PowerState) string {
	if state == nil {
		return "UNKNOWN"
	}
	name := state.GetName()
	if name == "$UNKNOWN" || name == "$REDACTED" {
		return "UNKNOWN"
	}
	return name
}

func clusterMatches(inv VMInventory, filter string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	if strings.EqualFold(inv.ClusterUUID, filter) {
		return true
	}
	if inv.ClusterName != "" && strings.EqualFold(inv.ClusterName, filter) {
		return true
	}
	return false
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func bytesToGiB(b *int64) float64 {
	if b == nil || *b <= 0 {
		return 0
	}
	return float64(*b) / (1024 * 1024 * 1024)
}
