// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zyvorai/transiva/manifest"
)

// PickupResult summarizes pickup execution for one VM.
type PickupResult struct {
	VMName     string
	VMUUID     string
	Disks      []ConvertedDisk
	Manifest   *manifest.ArtifactManifest
	ManifestPath string
	Duration   time.Duration
}

// ExecutePickupVM converts disks for a single VM and builds an Artifact Manifest v1.0.
func ExecutePickupVM(ctx context.Context, vm PickupVM, opts PickupExecuteOptions) (*PickupResult, error) {
	start := time.Now()
	if opts.OutputDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}
	format := strings.ToLower(opts.Format)
	if format == "" {
		format = "qcow2"
	}
	if format != "qcow2" && format != "raw" {
		return nil, fmt.Errorf("unsupported format %q (use qcow2 or raw)", format)
	}

	qemuImg := opts.QemuImgPath
	if qemuImg == "" {
		qemuImg = "qemu-img"
	}

	vmOutDir := filepath.Join(opts.OutputDir, sanitizeFilename(vm.Name))
	if err := os.MkdirAll(vmOutDir, 0750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	result := &PickupResult{
		VMName: vm.Name,
		VMUUID: vm.UUID,
	}

	converted := make([]ConvertedDisk, 0, len(vm.Disks))
	diskTotal := len(vm.Disks)
	for i, disk := range vm.Disks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		mountRoot, err := opts.Mounts.ResolveMount(disk)
		if err != nil {
			return nil, fmt.Errorf("vm %q disk %q: %w", vm.Name, disk.UUID, err)
		}

		reportPickupProgress(opts, PickupProgress{
			VMName: vm.Name, DiskIndex: i, DiskTotal: diskTotal, DiskUUID: disk.UUID,
			Phase: "locating", PercentComplete: diskOverallPercent(i, diskTotal, 0),
			Message: fmt.Sprintf("Locating disk %d/%d", i+1, diskTotal),
		})

		src, err := FindDiskSource(mountRoot, disk)
		if err != nil {
			return nil, fmt.Errorf("vm %q disk %q: %w", vm.Name, disk.UUID, err)
		}

		dest := DiskLocalPath(opts.OutputDir, vm.Name, disk.UUID, format)
		if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
			return nil, fmt.Errorf("create disk output dir: %w", err)
		}

		if opts.DryRun {
			converted = append(converted, ConvertedDisk{
				PickupDisk: disk,
				SourcePath: src,
				OutputPath: dest,
			})
			continue
		}

		diskIndex := i
		if err := convertDisk(ctx, qemuImg, src, dest, format, func(pct float64) {
			reportPickupProgress(opts, PickupProgress{
				VMName: vm.Name, DiskIndex: diskIndex, DiskTotal: diskTotal, DiskUUID: disk.UUID,
				Phase: "converting", PercentComplete: diskOverallPercent(diskIndex, diskTotal, pct),
				Message: fmt.Sprintf("Converting disk %d/%d (%.1f%%)", diskIndex+1, diskTotal, pct),
			})
		}); err != nil {
			return nil, fmt.Errorf("vm %q disk %q: %w", vm.Name, disk.UUID, err)
		}

		info, err := os.Stat(dest)
		if err != nil {
			return nil, fmt.Errorf("stat converted disk %q: %w", dest, err)
		}

		converted = append(converted, ConvertedDisk{
			PickupDisk: disk,
			SourcePath: src,
			OutputPath: dest,
			Bytes:      info.Size(),
		})
	}

	result.Disks = converted
	result.Duration = time.Since(start)

	if opts.DryRun {
		return result, nil
	}

	reportPickupProgress(opts, PickupProgress{
		VMName: vm.Name, DiskTotal: diskTotal, Phase: "manifest",
		PercentComplete: 95, Message: "Building artifact manifest",
	})

	artifact, err := BuildArtifactManifest(vm, converted, format)
	if err != nil {
		return nil, err
	}
	result.Manifest = artifact

	manifestPath := filepath.Join(vmOutDir, "artifact-manifest.json")
	if err := manifest.WriteToFile(artifact, manifestPath); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}
	result.ManifestPath = manifestPath

	reportPickupProgress(opts, PickupProgress{
		VMName: vm.Name, DiskTotal: diskTotal, Phase: "completed",
		PercentComplete: 100, Message: "Export completed",
	})

	return result, nil
}

// ExecutePickupPlan runs pickup for all VMs in a plan.
func ExecutePickupPlan(ctx context.Context, plan PickupPlan, opts PickupExecuteOptions) ([]PickupResult, error) {
	results := make([]PickupResult, 0, len(plan.VMs))
	for _, vm := range plan.VMs {
		res, err := ExecutePickupVM(ctx, vm, opts)
		if err != nil {
			return results, err
		}
		results = append(results, *res)
	}
	return results, nil
}

func convertDisk(ctx context.Context, qemuImg, src, dest, format string, onProgress func(percent float64)) error {
	if _, err := exec.LookPath(qemuImg); err != nil {
		return fmt.Errorf("qemu-img not found: %w", err)
	}

	args := []string{"convert", "-p", "-O", format, src, dest}
	// #nosec G204 -- qemuImg is verified via exec.LookPath above; src/dest/format come from the local pickup plan, not remote input
	cmd := exec.CommandContext(ctx, qemuImg, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("qemu-img stderr pipe: %w", err)
	}
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("qemu-img convert start: %w", err)
	}

	scanProgress(stderr, onProgress)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("qemu-img convert: %w", err)
	}
	return nil
}
