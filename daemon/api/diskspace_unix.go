// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package api

import (
	"fmt"
	"math"
	"syscall"
)

// getAvailableDiskSpace returns available disk space in bytes for the given path
func getAvailableDiskSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	if stat.Bsize < 0 {
		return 0, fmt.Errorf("invalid negative block size %d from statfs", stat.Bsize)
	}

	// Available blocks * block size
	available := stat.Bavail * uint64(stat.Bsize)
	if available > math.MaxInt64 {
		return 0, fmt.Errorf("available disk space %d exceeds int64 range", available)
	}
	return int64(available), nil
}
