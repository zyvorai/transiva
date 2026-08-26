// SPDX-License-Identifier: Apache-2.0

package nutanix

import "testing"

func TestParseQemuImgProgress(t *testing.T) {
	tests := []struct {
		line string
		want float64
		ok   bool
	}{
		{"    (23.45/100%)", 23.45, true},
		{"(100/100%)", 100, true},
		{"no progress here", 0, false},
	}

	for _, tt := range tests {
		got, ok := parseQemuImgProgress(tt.line)
		if ok != tt.ok || (tt.ok && got != tt.want) {
			t.Fatalf("parseQemuImgProgress(%q) = (%v, %v), want (%v, %v)", tt.line, got, ok, tt.want, tt.ok)
		}
	}
}

func TestDiskOverallPercent(t *testing.T) {
	got := diskOverallPercent(1, 3, 50)
	if got < 49.9 || got > 50.1 {
		t.Fatalf("diskOverallPercent = %v", got)
	}
}
