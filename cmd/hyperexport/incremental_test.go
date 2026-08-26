// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGetIncrementalMetadataDir(t *testing.T) {
	dir := getIncrementalMetadataDir()
	if dir == "" {
		t.Fatal("getIncrementalMetadataDir returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("expected absolute path, got %q", dir)
	}
	if !strings.HasSuffix(dir, filepath.Join(".transiva", "incremental")) {
		t.Errorf("expected path to end with .transiva/incremental, got %q", dir)
	}
}
