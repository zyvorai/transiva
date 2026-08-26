// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DomainStats represents VM resource statistics
type DomainStats struct {
	Name      string      `json:"name"`
	State     string      `json:"state"`
	CPUStats  CPUStats    `json:"cpu"`
	MemStats  MemStats    `json:"memory"`
	DiskStats []DiskStats `json:"disks"`
	NetStats  []NetStats  `json:"networks"`
	Timestamp time.Time   `json:"timestamp"`
}

// CPUStats represents CPU usage statistics
type CPUStats struct {
	VCPUs      int     `json:"vcpus"`
	CPUTime    uint64  `json:"cpu_time_ns"`    // nanoseconds
	UserTime   uint64  `json:"user_time_ns"`   // nanoseconds
	SystemTime uint64  `json:"system_time_ns"` // nanoseconds
	Usage      float64 `json:"usage_percent"`  // percentage
}

// MemStats represents memory usage statistics
type MemStats struct {
	Total     uint64  `json:"total_kb"`      // KiB
	Used      uint64  `json:"used_kb"`       // KiB
	Available uint64  `json:"available_kb"`  // KiB
	Usage     float64 `json:"usage_percent"` // percentage
	SwapIn    uint64  `json:"swap_in_kb"`    // KiB
	SwapOut   uint64  `json:"swap_out_kb"`   // KiB
}

// DiskStats represents disk I/O statistics
type DiskStats struct {
	Device     string `json:"device"`
	ReadBytes  uint64 `json:"read_bytes"`
	ReadReqs   uint64 `json:"read_requests"`
	WriteBytes uint64 `json:"write_bytes"`
	WriteReqs  uint64 `json:"write_requests"`
	Errors     uint64 `json:"errors"`
}

// NetStats represents network I/O statistics
type NetStats struct {
	Interface string `json:"interface"`
	RxBytes   uint64 `json:"rx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	RxDropped uint64 `json:"rx_dropped"`
	TxBytes   uint64 `json:"tx_bytes"`
	TxPackets uint64 `json:"tx_packets"`
	TxErrors  uint64 `json:"tx_errors"`
	TxDropped uint64 `json:"tx_dropped"`
}

// handleGetDomainStats gets current resource statistics for a VM
func (s *Server) handleGetDomainStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}

	stats, err := s.getDomainStatistics(name)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to get stats: %v", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, stats)
}

// handleGetAllDomainStats gets statistics for all running VMs
func (s *Server) handleGetAllDomainStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get list of running domains
	cmd := exec.Command("virsh", "list", "--state-running", "--name")
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list domains: %v", err)
		return
	}

	lines := strings.Split(string(output), "\n")
	var allStats []DomainStats

	for _, name := range lines {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		stats, err := s.getDomainStatistics(name)
		if err == nil {
			allStats = append(allStats, *stats)
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"domains":   allStats,
		"total":     len(allStats),
		"timestamp": time.Now(),
	})
}

// handleGetCPUStats gets CPU usage for a VM
func (s *Server) handleGetCPUStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}
	if !isValidVMName(name) {
		http.Error(w, "invalid name parameter", http.StatusBadRequest)
		return
	}

	// #nosec G204,G702 -- name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cmd := exec.Command("virsh", "cpu-stats", name)
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to get CPU stats: %v", err)
		return
	}

	cpuStats := s.parseCPUStats(string(output))

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"name":      name,
		"cpu_stats": cpuStats,
		"timestamp": time.Now(),
	})
}

// handleGetMemoryStats gets memory usage for a VM
func (s *Server) handleGetMemoryStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}
	if !isValidVMName(name) {
		http.Error(w, "invalid name parameter", http.StatusBadRequest)
		return
	}

	// #nosec G204,G702 -- name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cmd := exec.Command("virsh", "dommemstat", name)
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to get memory stats: %v", err)
		return
	}

	memStats := s.parseMemoryStats(string(output))

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"name":      name,
		"memory":    memStats,
		"timestamp": time.Now(),
	})
}

// handleGetDiskIOStats gets disk I/O statistics for a VM
func (s *Server) handleGetDiskIOStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}

	diskStats, err := s.getDiskIOStats(name)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to get disk I/O stats: %v", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"name":      name,
		"disks":     diskStats,
		"timestamp": time.Now(),
	})
}

// handleGetNetworkIOStats gets network I/O statistics for a VM
func (s *Server) handleGetNetworkIOStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}

	netStats, err := s.getNetworkIOStats(name)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to get network I/O stats: %v", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"name":      name,
		"networks":  netStats,
		"timestamp": time.Now(),
	})
}

// getDomainStatistics collects all statistics for a domain
func (s *Server) getDomainStatistics(name string) (*DomainStats, error) {
	if !isValidVMName(name) {
		return nil, fmt.Errorf("invalid domain name: %s", name)
	}

	stats := &DomainStats{
		Name:      name,
		Timestamp: time.Now(),
	}

	// Get domain state
	// #nosec G204,G702 -- name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	stateCmd := exec.Command("virsh", "domstate", name)
	if stateOutput, err := stateCmd.Output(); err == nil {
		stats.State = strings.TrimSpace(string(stateOutput))
	}

	// Get CPU stats
	// #nosec G204,G702 -- name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cpuCmd := exec.Command("virsh", "cpu-stats", name, "--total")
	if cpuOutput, err := cpuCmd.Output(); err == nil {
		stats.CPUStats = s.parseCPUStats(string(cpuOutput))
	}

	// Get memory stats
	// #nosec G204,G702 -- name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	memCmd := exec.Command("virsh", "dommemstat", name)
	if memOutput, err := memCmd.Output(); err == nil {
		stats.MemStats = s.parseMemoryStats(string(memOutput))
	}

	// Get disk I/O stats
	if diskStats, err := s.getDiskIOStats(name); err == nil {
		stats.DiskStats = diskStats
	}

	// Get network I/O stats
	if netStats, err := s.getNetworkIOStats(name); err == nil {
		stats.NetStats = netStats
	}

	return stats, nil
}

// parseCPUStats parses virsh cpu-stats output
func (s *Server) parseCPUStats(output string) CPUStats {
	stats := CPUStats{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "cpu_time":
			val, _ := strconv.ParseFloat(value, 64)
			stats.CPUTime = uint64(val * 1000000000) // convert to nanoseconds
		case "user_time":
			val, _ := strconv.ParseFloat(value, 64)
			stats.UserTime = uint64(val * 1000000000)
		case "system_time":
			val, _ := strconv.ParseFloat(value, 64)
			stats.SystemTime = uint64(val * 1000000000)
		case "vcpu":
			stats.VCPUs, _ = strconv.Atoi(value)
		}
	}

	return stats
}

// parseMemoryStats parses virsh dommemstat output
func (s *Server) parseMemoryStats(output string) MemStats {
	stats := MemStats{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value, _ := strconv.ParseUint(parts[1], 10, 64)

		switch key {
		case "actual":
			stats.Total = value
		case "unused":
			stats.Available = value
		case "available":
			stats.Total = value
		case "usable":
			stats.Available = value
		case "swap_in":
			stats.SwapIn = value
		case "swap_out":
			stats.SwapOut = value
		}
	}

	if stats.Total > 0 && stats.Available > 0 {
		stats.Used = stats.Total - stats.Available
		stats.Usage = float64(stats.Used) / float64(stats.Total) * 100.0
	}

	return stats
}

// getDiskIOStats gets disk I/O statistics using virsh domblkstat
func (s *Server) getDiskIOStats(name string) ([]DiskStats, error) {
	if !isValidVMName(name) {
		return nil, fmt.Errorf("invalid domain name: %s", name)
	}

	// Get list of disk devices
	// #nosec G204,G702 -- name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	domblklistCmd := exec.Command("virsh", "domblklist", name)
	domblklistOutput, err := domblklistCmd.Output()
	if err != nil {
		return nil, err
	}

	var diskStats []DiskStats
	lines := strings.Split(string(domblklistOutput), "\n")

	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		device := fields[0]

		// Get stats for this device
		// #nosec G204,G702 -- name validated by isValidVMName(); device comes from trusted local virsh output
		statCmd := exec.Command("virsh", "domblkstat", name, device)
		statOutput, err := statCmd.Output()
		if err != nil {
			continue
		}

		stats := DiskStats{Device: device}
		statLines := strings.Split(string(statOutput), "\n")

		for _, statLine := range statLines {
			parts := strings.Fields(statLine)
			if len(parts) < 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value, _ := strconv.ParseUint(parts[1], 10, 64)

			switch key {
			case "rd_bytes":
				stats.ReadBytes = value
			case "rd_req":
				stats.ReadReqs = value
			case "wr_bytes":
				stats.WriteBytes = value
			case "wr_req":
				stats.WriteReqs = value
			case "errs":
				stats.Errors = value
			}
		}

		diskStats = append(diskStats, stats)
	}

	return diskStats, nil
}

// getNetworkIOStats gets network I/O statistics using virsh domifstat
func (s *Server) getNetworkIOStats(name string) ([]NetStats, error) {
	if !isValidVMName(name) {
		return nil, fmt.Errorf("invalid domain name: %s", name)
	}

	// Get domain XML to find network interfaces
	// #nosec G204,G702 -- name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	dumpCmd := exec.Command("virsh", "dumpxml", name)
	dumpOutput, err := dumpCmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse XML to extract interface names
	type Interface struct {
		Type   string `xml:"type,attr"`
		Target struct {
			Dev string `xml:"dev,attr"`
		} `xml:"target"`
	}
	type Domain struct {
		Interfaces []Interface `xml:"devices>interface"`
	}

	var domain Domain
	// #nosec G709 -- XML is generated by the local trusted virsh dumpxml command, not user-supplied data
	if err := xml.Unmarshal(dumpOutput, &domain); err != nil {
		return nil, err
	}

	var netStats []NetStats

	for _, iface := range domain.Interfaces {
		if iface.Target.Dev == "" {
			continue
		}

		device := iface.Target.Dev

		// Get stats for this interface
		// #nosec G204,G702 -- name validated by isValidVMName(); device comes from trusted local virsh output
		statCmd := exec.Command("virsh", "domifstat", name, device)
		statOutput, err := statCmd.Output()
		if err != nil {
			continue
		}

		stats := NetStats{Interface: device}
		statLines := strings.Split(string(statOutput), "\n")

		for _, statLine := range statLines {
			parts := strings.Fields(statLine)
			if len(parts) < 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value, _ := strconv.ParseUint(parts[1], 10, 64)

			switch key {
			case "rx_bytes":
				stats.RxBytes = value
			case "rx_packets":
				stats.RxPackets = value
			case "rx_errs":
				stats.RxErrors = value
			case "rx_drop":
				stats.RxDropped = value
			case "tx_bytes":
				stats.TxBytes = value
			case "tx_packets":
				stats.TxPackets = value
			case "tx_errs":
				stats.TxErrors = value
			case "tx_drop":
				stats.TxDropped = value
			}
		}

		netStats = append(netStats, stats)
	}

	return netStats, nil
}
