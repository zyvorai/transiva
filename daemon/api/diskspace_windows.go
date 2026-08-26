// SPDX-License-Identifier: Apache-2.0

//go:build windows

package api

import "golang.org/x/sys/windows"

// getAvailableDiskSpace returns available disk space in bytes for the given path
func getAvailableDiskSpace(path string) (int64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeBytesAvailable uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, nil, nil); err != nil {
		return 0, err
	}
	return int64(freeBytesAvailable), nil
}
