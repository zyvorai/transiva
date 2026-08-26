// SPDX-License-Identifier: Apache-2.0

package main

// contains returns the index of the first occurrence of val in slice, and
// whether it was found. Returns (-1, false) if val is not present.
func contains(slice []string, val string) (int, bool) {
	for i, s := range slice {
		if s == val {
			return i, true
		}
	}
	return -1, false
}
