// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"fmt"
	"math"
	"syscall"
)

// getDiskSpace returns available and total disk space in bytes for the given path.
func getDiskSpace(path string) (available, total int64, err error) {
	var stat syscall.Statfs_t
	if err = syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	if stat.Bavail > uint64(math.MaxInt64) {
		return 0, 0, fmt.Errorf("available block count for %q overflows int64", path)
	}
	if stat.Blocks > uint64(math.MaxInt64) {
		return 0, 0, fmt.Errorf("total block count for %q overflows int64", path)
	}
	available = int64(stat.Bavail) * int64(stat.Bsize)
	total = int64(stat.Blocks) * int64(stat.Bsize)
	return available, total, nil
}
