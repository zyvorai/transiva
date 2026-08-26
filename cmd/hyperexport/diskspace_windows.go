// SPDX-License-Identifier: Apache-2.0

//go:build windows

package main

import "golang.org/x/sys/windows"

// getDiskSpace returns available and total disk space in bytes for the given path.
func getDiskSpace(path string) (available, total int64, err error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return 0, 0, err
	}
	return int64(freeBytesAvailable), int64(totalBytes), nil
}
