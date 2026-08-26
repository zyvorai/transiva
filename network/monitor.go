// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/zyvorai/transiva/logger"
)

// State represents network connectivity state
type State int

const (
	StateUnknown  State = iota
	StateUp             // Network is available
	StateDown           // Network is unavailable
	StateDegraded       // Network is available but degraded (high latency, packet loss)
)

func (s State) String() string {
	switch s {
	case StateUp:
		return "up"
	case StateDown:
		return "down"
	case StateDegraded:
		return "degraded"
	default:
		return "unknown"
	}
}

// Monitor monitors network state changes using netlink
type Monitor struct {
	state     State
	stateMu   sync.RWMutex
	logger    logger.Logger
	listeners []chan State
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// Configuration
	checkInterval   time.Duration // How often to check connectivity
	checkTimeout    time.Duration // Timeout for connectivity checks
	checkHosts      []string      // Hosts to check for connectivity
	notifyOnChange  bool          // Only notify listeners on state change
	enableNetlink   bool          // Enable netlink monitoring (Linux only)
	preferredIfaces []string      // Preferred network interfaces to monitor
}

// MonitorConfig configures the network monitor
type MonitorConfig struct {
	CheckInterval   time.Duration // Default: 10s
	CheckTimeout    time.Duration // Default: 5s
	CheckHosts      []string      // Default: ["8.8.8.8", "1.1.1.1"]
	NotifyOnChange  bool          // Default: true
	EnableNetlink   bool          // Default: true (Linux only)
	PreferredIfaces []string      // Default: empty (monitor all)
}

// NewMonitor creates a new network monitor
func NewMonitor(cfg *MonitorConfig, log logger.Logger) *Monitor {
	useDefaults := false
	if cfg == nil {
		cfg = &MonitorConfig{}
		useDefaults = true
	}

	// Set defaults
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 10 * time.Second
	}
	if cfg.CheckTimeout == 0 {
		cfg.CheckTimeout = 5 * time.Second
	}
	if len(cfg.CheckHosts) == 0 {
		cfg.CheckHosts = []string{"8.8.8.8", "1.1.1.1", "8.8.4.4"}
	}
	// Only set these defaults if config was nil
	if useDefaults {
		if !cfg.NotifyOnChange {
			cfg.NotifyOnChange = true // Default to only notify on change
		}
		if !cfg.EnableNetlink {
			cfg.EnableNetlink = true // Default to enabled
		}
	}

	return &Monitor{
		state:           StateUnknown,
		logger:          log,
		stopCh:          make(chan struct{}),
		checkInterval:   cfg.CheckInterval,
		checkTimeout:    cfg.CheckTimeout,
		checkHosts:      cfg.CheckHosts,
		notifyOnChange:  cfg.NotifyOnChange,
		enableNetlink:   cfg.EnableNetlink,
		preferredIfaces: cfg.PreferredIfaces,
	}
}

// Start starts the network monitor
func (m *Monitor) Start(ctx context.Context) error {
	m.logger.Info("starting network monitor",
		"checkInterval", m.checkInterval,
		"netlinkEnabled", m.enableNetlink)

	// Initial connectivity check
	m.updateState(m.checkConnectivity())

	// Start netlink monitoring (Linux only)
	if m.enableNetlink {
		m.wg.Add(1)
		go m.monitorNetlinkEvents(ctx)
	}

	// Start periodic connectivity checks
	m.wg.Add(1)
	go m.periodicConnectivityCheck(ctx)

	return nil
}

// Stop stops the network monitor
func (m *Monitor) Stop() {
	m.logger.Info("stopping network monitor")
	close(m.stopCh)
	m.wg.Wait()

	// Close all listener channels
	m.stateMu.Lock()
	for _, ch := range m.listeners {
		close(ch)
	}
	m.listeners = nil
	m.stateMu.Unlock()
}

// GetState returns the current network state
func (m *Monitor) GetState() State {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state
}

// IsUp returns true if network is available
func (m *Monitor) IsUp() bool {
	return m.GetState() == StateUp
}

// Subscribe returns a channel that receives state changes
func (m *Monitor) Subscribe() <-chan State {
	ch := make(chan State, 10)
	m.stateMu.Lock()
	m.listeners = append(m.listeners, ch)
	m.stateMu.Unlock()
	return ch
}

// WaitForNetwork waits until network is available or context is cancelled
func (m *Monitor) WaitForNetwork(ctx context.Context) error {
	// Check current state first
	if m.IsUp() {
		return nil
	}

	m.logger.Info("waiting for network to become available")

	// Subscribe to state changes
	stateCh := m.Subscribe()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case state := <-stateCh:
			if state == StateUp {
				m.logger.Info("network is now available")
				return nil
			}
		}
	}
}

// updateState updates the network state and notifies listeners
func (m *Monitor) updateState(newState State) {
	m.stateMu.Lock()
	oldState := m.state
	m.state = newState
	m.stateMu.Unlock()

	// Notify listeners
	if !m.notifyOnChange || oldState != newState {
		if oldState != newState {
			m.logger.Info("network state changed",
				"from", oldState,
				"to", newState)
		}

		m.stateMu.RLock()
		for _, ch := range m.listeners {
			select {
			case ch <- newState:
			default:
				// Channel full, skip
			}
		}
		m.stateMu.RUnlock()
	}
}

// monitorNetlinkEvents monitors network interface state changes via netlink
func (m *Monitor) monitorNetlinkEvents(ctx context.Context) {
	defer m.wg.Done()

	// Subscribe to link updates
	linkUpdates := make(chan netlink.LinkUpdate)
	done := make(chan struct{})
	defer close(done)

	if err := netlink.LinkSubscribe(linkUpdates, done); err != nil {
		m.logger.Error("failed to subscribe to netlink events", "error", err)
		return
	}

	m.logger.Debug("netlink monitoring started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case update := <-linkUpdates:
			m.handleLinkUpdate(update)
		}
	}
}

// handleLinkUpdate handles a netlink link update
func (m *Monitor) handleLinkUpdate(update netlink.LinkUpdate) {
	link := update.Link
	attrs := link.Attrs()

	// Skip loopback and non-preferred interfaces
	if attrs.Flags&net.FlagLoopback != 0 {
		return
	}

	// If preferred interfaces are specified, only monitor those
	if len(m.preferredIfaces) > 0 {
		found := false
		for _, iface := range m.preferredIfaces {
			if attrs.Name == iface {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	isUp := attrs.Flags&net.FlagUp != 0

	m.logger.Debug("netlink event",
		"interface", attrs.Name,
		"state", map[bool]string{true: "up", false: "down"}[isUp],
		"flags", attrs.Flags)

	// When any monitored interface goes down, mark network as down
	// When any monitored interface comes up, perform connectivity check
	if !isUp {
		m.updateState(StateDown)
	} else {
		// Interface is up, but need to verify actual connectivity
		state := m.checkConnectivity()
		m.updateState(state)
	}
}

// periodicConnectivityCheck performs periodic connectivity checks
func (m *Monitor) periodicConnectivityCheck(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			state := m.checkConnectivity()
			m.updateState(state)
		}
	}
}

// checkConnectivity checks actual network connectivity by attempting to reach check hosts
func (m *Monitor) checkConnectivity() State {
	ctx, cancel := context.WithTimeout(context.Background(), m.checkTimeout)
	defer cancel()

	// Try to connect to check hosts
	successCount := 0
	for _, host := range m.checkHosts {
		if m.canReach(ctx, host) {
			successCount++
		}
	}

	// If we can reach at least one host, network is up
	if successCount > 0 {
		// If we can reach all hosts, fully up
		// If only some, consider degraded
		if successCount == len(m.checkHosts) {
			return StateUp
		}
		return StateDegraded
	}

	return StateDown
}

// canReach checks if we can reach a specific host
func (m *Monitor) canReach(ctx context.Context, host string) bool {
	// Try TCP connection to port 53 (DNS) or 443 (HTTPS)
	ports := []string{"53", "443"}

	for _, port := range ports {
		dialer := &net.Dialer{
			Timeout: m.checkTimeout,
		}

		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				m.logger.Debug("failed to close connectivity check connection", "host", host, "port", port, "error", closeErr)
			}
			m.logger.Debug("connectivity check succeeded", "host", host, "port", port)
			return true
		}
	}

	m.logger.Debug("connectivity check failed", "host", host)
	return false
}

// GetInterfaceStats returns statistics about network interfaces
func (m *Monitor) GetInterfaceStats() (map[string]InterfaceStats, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}

	stats := make(map[string]InterfaceStats)
	for _, link := range links {
		attrs := link.Attrs()

		// Skip loopback
		if attrs.Flags&net.FlagLoopback != 0 {
			continue
		}

		linkStats := attrs.Statistics
		if linkStats == nil {
			continue
		}

		stats[attrs.Name] = InterfaceStats{
			Name:       attrs.Name,
			IsUp:       attrs.Flags&net.FlagUp != 0,
			RxBytes:    linkStats.RxBytes,
			TxBytes:    linkStats.TxBytes,
			RxPackets:  linkStats.RxPackets,
			TxPackets:  linkStats.TxPackets,
			RxErrors:   linkStats.RxErrors,
			TxErrors:   linkStats.TxErrors,
			RxDropped:  linkStats.RxDropped,
			TxDropped:  linkStats.TxDropped,
			MTU:        attrs.MTU,
			MACAddress: attrs.HardwareAddr.String(),
		}
	}

	return stats, nil
}

// InterfaceStats contains statistics about a network interface
type InterfaceStats struct {
	Name       string
	IsUp       bool
	RxBytes    uint64
	TxBytes    uint64
	RxPackets  uint64
	TxPackets  uint64
	RxErrors   uint64
	TxErrors   uint64
	RxDropped  uint64
	TxDropped  uint64
	MTU        int
	MACAddress string
}
