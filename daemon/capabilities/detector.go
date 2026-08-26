// SPDX-License-Identifier: Apache-2.0

package capabilities

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// ExportMethod represents an export tool/method
type ExportMethod string

const (
	ExportMethodCTL     ExportMethod = "ctl"
	ExportMethodGovc    ExportMethod = "govc"
	ExportMethodOvftool ExportMethod = "ovftool"
	ExportMethodWeb     ExportMethod = "web"
	ExportMethodNutanix ExportMethod = "nutanix"
)

// ExportCapability represents a detected export method
type ExportCapability struct {
	Method      ExportMethod `json:"method"`
	Available   bool         `json:"available"`
	Version     string       `json:"version"`
	Path        string       `json:"path"`
	Priority    int          `json:"priority"`
	LastChecked time.Time    `json:"last_checked"`
}

// Detector detects available export tools
type Detector struct {
	capabilities map[ExportMethod]*ExportCapability
	mu           sync.RWMutex
	logger       logger.Logger
}

// NewDetector creates a new capability detector
func NewDetector(log logger.Logger) *Detector {
	return &Detector{
		capabilities: make(map[ExportMethod]*ExportCapability),
		logger:       log,
	}
}

// Detect scans for available export tools
func (d *Detector) Detect(ctx context.Context) error {
	d.logger.Info("detecting export method capabilities")

	// Detect each method concurrently
	var wg sync.WaitGroup
	detections := make(chan *ExportCapability, 4)

	methods := []struct {
		method ExportMethod
		fn     func() *ExportCapability
	}{
		{ExportMethodCTL, d.detectCTL},
		{ExportMethodGovc, d.detectGovc},
		{ExportMethodOvftool, d.detectOvftool},
		{ExportMethodWeb, d.detectWeb},
	}

	for _, m := range methods {
		wg.Add(1)
		go func(method ExportMethod, detectFn func() *ExportCapability) {
			defer wg.Done()
			cap := detectFn()
			detections <- cap
		}(m.method, m.fn)
	}

	// Wait for all detections to complete
	go func() {
		wg.Wait()
		close(detections)
	}()

	// Collect results
	d.mu.Lock()
	for cap := range detections {
		d.capabilities[cap.Method] = cap
		if cap.Available {
			d.logger.Info("export method detected",
				"method", cap.Method,
				"version", cap.Version,
				"path", cap.Path,
				"priority", cap.Priority)
		} else {
			d.logger.Debug("export method not available", "method", cap.Method)
		}
	}
	d.mu.Unlock()

	return nil
}

// GetCapabilities returns all detected capabilities
func (d *Detector) GetCapabilities() map[ExportMethod]*ExportCapability {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a copy to prevent external modification
	caps := make(map[ExportMethod]*ExportCapability)
	for k, v := range d.capabilities {
		capCopy := *v
		caps[k] = &capCopy
	}
	return caps
}

// GetBestMethod returns the best available method based on priority
func (d *Detector) GetBestMethod() ExportMethod {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Priority order: CTL (1) > govc (2) > ovftool (3) > web (4)
	priorityOrder := []ExportMethod{
		ExportMethodCTL,
		ExportMethodGovc,
		ExportMethodOvftool,
		ExportMethodWeb,
	}

	for _, method := range priorityOrder {
		if cap, exists := d.capabilities[method]; exists && cap.Available {
			return method
		}
	}

	// Fallback to web (always available)
	return ExportMethodWeb
}

// GetDefaultMethod returns the default (best) available method
// Alias for GetBestMethod for API consistency
func (d *Detector) GetDefaultMethod() ExportMethod {
	return d.GetBestMethod()
}

// IsAvailable checks if a specific method is available
func (d *Detector) IsAvailable(method ExportMethod) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	cap, exists := d.capabilities[method]
	return exists && cap.Available
}

// detectCTL checks for hyperctl binary
func (d *Detector) detectCTL() *ExportCapability {
	cap := &ExportCapability{
		Method:      ExportMethodCTL,
		Available:   false,
		Priority:    1,
		LastChecked: time.Now(),
	}

	// Look for "hyperctl" in PATH
	path, err := exec.LookPath("hyperctl")
	if err != nil {
		return cap
	}

	cap.Path = path

	// Try to get version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "hyperctl", "version")
	output, err := cmd.Output()
	if err != nil {
		// Binary exists but version check failed - still mark as available
		cap.Available = true
		cap.Version = "unknown"
		return cap
	}

	// Parse version from output
	version := strings.TrimSpace(string(output))
	cap.Version = version
	cap.Available = true

	return cap
}

// detectGovc checks for govc binary
func (d *Detector) detectGovc() *ExportCapability {
	cap := &ExportCapability{
		Method:      ExportMethodGovc,
		Available:   false,
		Priority:    2,
		LastChecked: time.Now(),
	}

	// Look for "govc" in PATH
	path, err := exec.LookPath("govc")
	if err != nil {
		return cap
	}

	cap.Path = path

	// Try to get version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "govc", "version")
	output, err := cmd.Output()
	if err != nil {
		cap.Available = true
		cap.Version = "unknown"
		return cap
	}

	// Parse version from output (govc version output format: "govc <version>")
	version := strings.TrimSpace(string(output))
	cap.Version = version
	cap.Available = true

	return cap
}

// detectOvftool checks for ovftool binary
func (d *Detector) detectOvftool() *ExportCapability {
	cap := &ExportCapability{
		Method:      ExportMethodOvftool,
		Available:   false,
		Priority:    3,
		LastChecked: time.Now(),
	}

	// Look for "ovftool" in PATH
	path, err := exec.LookPath("ovftool")
	if err != nil {
		return cap
	}

	cap.Path = path

	// Try to get version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ovftool", "--version")
	output, err := cmd.Output()
	if err != nil {
		cap.Available = true
		cap.Version = "unknown"
		return cap
	}

	// Parse version from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "VMware ovftool") {
			cap.Version = strings.TrimSpace(line)
			break
		}
	}

	if cap.Version == "" {
		cap.Version = strings.TrimSpace(string(output))
	}

	cap.Available = true
	return cap
}

// detectWeb checks web/HTTP method (always available)
func (d *Detector) detectWeb() *ExportCapability {
	return &ExportCapability{
		Method:      ExportMethodWeb,
		Available:   true, // Always available (built-in via govmomi)
		Version:     "built-in",
		Path:        "internal",
		Priority:    4,
		LastChecked: time.Now(),
	}
}
