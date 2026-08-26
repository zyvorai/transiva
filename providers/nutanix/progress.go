// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
)

var qemuImgProgressRE = regexp.MustCompile(`\((\d+(?:\.\d+)?)/100%\)`)

// PickupProgress describes pickup/export progress for one VM.
type PickupProgress struct {
	VMName          string
	DiskIndex       int
	DiskTotal       int
	DiskUUID        string
	Phase           string
	PercentComplete float64
	Message         string
}

// PickupProgressFunc receives pickup progress updates.
type PickupProgressFunc func(PickupProgress)

// parseQemuImgProgress extracts percent complete from qemu-img -p stderr output.
func parseQemuImgProgress(line string) (float64, bool) {
	m := qemuImgProgressRE.FindStringSubmatch(line)
	if len(m) != 2 {
		return 0, false
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return pct, true
}

func scanProgress(r io.Reader, onProgress func(percent float64)) {
	if r == nil || onProgress == nil {
		return
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if pct, ok := parseQemuImgProgress(scanner.Text()); ok {
			onProgress(pct)
		}
	}
}

func reportPickupProgress(opts PickupExecuteOptions, event PickupProgress) {
	if opts.Progress != nil {
		opts.Progress(event)
	}
}

func diskOverallPercent(diskIndex, diskTotal int, diskPercent float64) float64 {
	if diskTotal <= 0 {
		return diskPercent
	}
	return ((float64(diskIndex) + diskPercent/100.0) / float64(diskTotal)) * 100.0
}
