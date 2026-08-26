// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MountMap maps storage container name or UUID to a local NFS mount path.
type MountMap map[string]string

// PickupExecuteOptions controls offline disk pickup from mounted containers.
type PickupExecuteOptions struct {
	OutputDir   string
	Format      string // qcow2 or raw
	Mounts      MountMap
	QemuImgPath string
	DryRun      bool
	Progress    PickupProgressFunc
}

// ConvertedDisk records a converted disk artifact on the migration host.
type ConvertedDisk struct {
	PickupDisk
	SourcePath string
	OutputPath string
	Bytes      int64
}

// ResolveMount returns the mount path for a disk's container.
func (m MountMap) ResolveMount(disk PickupDisk) (string, error) {
	if len(m) == 0 {
		return "", fmt.Errorf("no container mounts configured")
	}
	if disk.ContainerName != "" {
		if p, ok := m[disk.ContainerName]; ok {
			return p, nil
		}
	}
	if disk.ContainerUUID != "" {
		if p, ok := m[disk.ContainerUUID]; ok {
			return p, nil
		}
	}
	return "", fmt.Errorf("no mount for container %q (uuid %q)", disk.ContainerName, disk.ContainerUUID)
}

// FindDiskSource locates the primary disk image under a container NFS mount.
func FindDiskSource(mountRoot string, disk PickupDisk) (string, error) {
	if mountRoot == "" {
		return "", fmt.Errorf("mount root is required")
	}
	base := filepath.Join(mountRoot, filepath.FromSlash(disk.NFSRelativePath))
	info, err := os.Stat(base)
	if err != nil {
		return "", fmt.Errorf("disk path not found %q: %w", base, err)
	}
	if !info.IsDir() {
		if info.Size() > 0 {
			return base, nil
		}
		return "", fmt.Errorf("disk path %q is empty", base)
	}

	var candidates []string
	err = filepath.Walk(base, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() {
			return nil
		}
		name := strings.ToLower(fi.Name())
		if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".xml") {
			return nil
		}
		if fi.Size() < 1024*1024 {
			return nil
		}
		candidates = append(candidates, path)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk disk directory %q: %w", base, err)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no disk image files found under %q", base)
	}

	best := candidates[0]
	var bestScore int
	for _, c := range candidates {
		score := scoreDiskCandidate(c)
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	return best, nil
}

func scoreDiskCandidate(path string) int {
	name := strings.ToLower(filepath.Base(path))
	score := 0
	switch {
	case strings.Contains(name, "flat"):
		score += 100
	case strings.HasSuffix(name, ".qcow2"):
		score += 80
	case strings.HasSuffix(name, ".vmdk"):
		score += 70
	case strings.HasSuffix(name, ".img"):
		score += 60
	case strings.HasSuffix(name, ".raw"):
		score += 50
	}
	if info, err := os.Stat(path); err == nil {
		score += int(info.Size() / (1024 * 1024))
	}
	return score
}

// DiskLocalPath builds the expected output path for a converted disk.
func DiskLocalPath(outputDir, vmName, diskUUID, format string) string {
	safeVM := sanitizeFilename(vmName)
	safeDisk := sanitizeFilename(diskUUID)
	ext := format
	if ext == "" {
		ext = "qcow2"
	}
	return filepath.Join(outputDir, safeVM, safeDisk+"."+ext)
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	out := replacer.Replace(strings.TrimSpace(name))
	if out == "" {
		return "unknown"
	}
	return out
}
