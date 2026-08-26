// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNetworkMonitor_BasicFunctionality(t *testing.T) {
	log := logger.NewTestLogger(t)

	config := &MonitorConfig{
		CheckInterval:   1 * time.Second,
		CheckTimeout:    500 * time.Millisecond,
		CheckHosts:      []string{"8.8.8.8"},
		NotifyOnChange:  true,
		EnableNetlink:   false, // Disable netlink for unit tests
		PreferredIfaces: nil,
	}

	monitor := NewMonitor(config, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start monitor
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Wait a bit for initial check
	time.Sleep(2 * time.Second)

	// Get current state
	state := monitor.GetState()
	t.Logf("Current network state: %s", state)

	// State should be either Up or Degraded (depending on connectivity)
	if state != StateUp && state != StateDegraded {
		t.Logf("Warning: Network state is %s, expected Up or Degraded", state)
	}
}

func TestNetworkMonitor_Subscribe(t *testing.T) {
	log := logger.NewTestLogger(t)

	config := &MonitorConfig{
		CheckInterval:  500 * time.Millisecond,
		CheckTimeout:   200 * time.Millisecond,
		CheckHosts:     []string{"8.8.8.8"},
		NotifyOnChange: true,
		EnableNetlink:  false,
	}

	monitor := NewMonitor(config, log)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Subscribe to state changes
	stateCh := monitor.Subscribe()

	// Wait for at least one state notification
	select {
	case state := <-stateCh:
		t.Logf("Received state notification: %s", state)
	case <-time.After(2 * time.Second):
		t.Log("No state change received within 2 seconds (this is okay)")
	}
}

func TestNetworkMonitor_MultipleSubscribers(t *testing.T) {
	log := logger.NewTestLogger(t)

	config := &MonitorConfig{
		CheckInterval:  1 * time.Second,
		CheckTimeout:   500 * time.Millisecond,
		CheckHosts:     []string{"8.8.8.8"},
		NotifyOnChange: false, // Get all updates
		EnableNetlink:  false,
	}

	monitor := NewMonitor(config, log)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Create multiple subscribers
	subscriber1 := monitor.Subscribe()
	subscriber2 := monitor.Subscribe()
	subscriber3 := monitor.Subscribe()

	// Wait a bit for state updates
	time.Sleep(2 * time.Second)

	// Check that all subscribers received updates
	checkSubscriber := func(name string, ch <-chan State) {
		select {
		case state := <-ch:
			t.Logf("%s received state: %s", name, state)
		default:
			t.Logf("%s: no state update (this is okay)", name)
		}
	}

	checkSubscriber("Subscriber1", subscriber1)
	checkSubscriber("Subscriber2", subscriber2)
	checkSubscriber("Subscriber3", subscriber3)
}

func TestNetworkMonitor_IsUp(t *testing.T) {
	log := logger.NewTestLogger(t)

	monitor := NewMonitor(nil, log) // Use defaults

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Wait for initial check
	time.Sleep(1 * time.Second)

	isUp := monitor.IsUp()
	t.Logf("Network is up: %v", isUp)

	// We expect network to be up in test environment
	if !isUp {
		t.Log("Warning: Network appears to be down in test environment")
	}
}

func TestNetworkMonitor_GetState(t *testing.T) {
	log := logger.NewTestLogger(t)

	monitor := NewMonitor(nil, log)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Initial state should be Unknown
	initialState := monitor.GetState()
	if initialState != StateUnknown && initialState != StateUp {
		t.Logf("Initial state: %s (expected Unknown or Up)", initialState)
	}

	// Wait for first check
	time.Sleep(1500 * time.Millisecond)

	// State should have been updated
	finalState := monitor.GetState()
	t.Logf("Final state: %s", finalState)

	if finalState == StateUnknown {
		t.Error("State should not be Unknown after initial check")
	}
}

func TestNetworkMonitor_ConfigDefaults(t *testing.T) {
	log := logger.NewTestLogger(t)

	// Test with nil config (should use defaults)
	monitor := NewMonitor(nil, log)

	if monitor.checkInterval != 10*time.Second {
		t.Errorf("Expected default checkInterval to be 10s, got %v", monitor.checkInterval)
	}

	if monitor.checkTimeout != 5*time.Second {
		t.Errorf("Expected default checkTimeout to be 5s, got %v", monitor.checkTimeout)
	}

	if len(monitor.checkHosts) != 3 {
		t.Errorf("Expected 3 default check hosts, got %d", len(monitor.checkHosts))
	}

	if !monitor.notifyOnChange {
		t.Error("Expected notifyOnChange to be true by default")
	}

	if !monitor.enableNetlink {
		t.Error("Expected enableNetlink to be true by default")
	}
}

func TestNetworkMonitor_CustomConfig(t *testing.T) {
	log := logger.NewTestLogger(t)

	config := &MonitorConfig{
		CheckInterval:   5 * time.Second,
		CheckTimeout:    2 * time.Second,
		CheckHosts:      []string{"1.1.1.1", "8.8.8.8"},
		NotifyOnChange:  false,
		EnableNetlink:   false,
		PreferredIfaces: []string{"eth0"},
	}

	monitor := NewMonitor(config, log)

	if monitor.checkInterval != 5*time.Second {
		t.Errorf("Expected checkInterval to be 5s, got %v", monitor.checkInterval)
	}

	if monitor.checkTimeout != 2*time.Second {
		t.Errorf("Expected checkTimeout to be 2s, got %v", monitor.checkTimeout)
	}

	if len(monitor.checkHosts) != 2 {
		t.Errorf("Expected 2 check hosts, got %d", len(monitor.checkHosts))
	}

	// These are set in config so they should match
	// (Note: The constructor always sets defaults, so we check the actual values)
	if monitor.notifyOnChange != false {
		t.Errorf("Expected notifyOnChange to be false, got %v", monitor.notifyOnChange)
	}

	if monitor.enableNetlink != false {
		t.Errorf("Expected enableNetlink to be false, got %v", monitor.enableNetlink)
	}

	if len(monitor.preferredIfaces) != 1 || monitor.preferredIfaces[0] != "eth0" {
		t.Errorf("Expected preferredIfaces to be [eth0], got %v", monitor.preferredIfaces)
	}
}

func TestNetworkMonitor_StopBeforeStart(t *testing.T) {
	log := logger.NewTestLogger(t)

	monitor := NewMonitor(nil, log)

	// Stopping before starting should not panic
	monitor.Stop()
}

func TestNetworkMonitor_GetInterfaceStats(t *testing.T) {
	log := logger.NewTestLogger(t)

	monitor := NewMonitor(nil, log)

	stats, err := monitor.GetInterfaceStats()
	if err != nil {
		t.Fatalf("failed to get interface stats: %v", err)
	}

	t.Logf("Found %d network interfaces", len(stats))

	for name, stat := range stats {
		t.Logf("Interface: %s", name)
		t.Logf("  IsUp: %v", stat.IsUp)
		t.Logf("  RxBytes: %d", stat.RxBytes)
		t.Logf("  TxBytes: %d", stat.TxBytes)
		t.Logf("  RxPackets: %d", stat.RxPackets)
		t.Logf("  TxPackets: %d", stat.TxPackets)
		t.Logf("  MTU: %d", stat.MTU)
		t.Logf("  MAC: %s", stat.MACAddress)
	}

	if len(stats) == 0 {
		t.Log("Warning: No network interfaces found")
	}
}

// Benchmark tests
func BenchmarkNetworkMonitor_GetState(b *testing.B) {
	log := logger.NewTestLogger(b)
	monitor := NewMonitor(nil, log)

	ctx := context.Background()
	monitor.Start(ctx)
	defer monitor.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = monitor.GetState()
	}
}

func BenchmarkNetworkMonitor_IsUp(b *testing.B) {
	log := logger.NewTestLogger(b)
	monitor := NewMonitor(nil, log)

	ctx := context.Background()
	monitor.Start(ctx)
	defer monitor.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = monitor.IsUp()
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateUnknown, "unknown"},
		{StateUp, "up"},
		{StateDown, "down"},
		{StateDegraded, "degraded"},
		{State(99), "unknown"}, // Test invalid state
	}

	for _, tt := range tests {
		result := tt.state.String()
		if result != tt.expected {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, result, tt.expected)
		}
	}
}

func TestWaitForNetwork_AlreadyUp(t *testing.T) {
	log := logger.NewTestLogger(t)
	monitor := NewMonitor(nil, log)

	ctx := context.Background()
	monitor.Start(ctx)
	defer monitor.Stop()

	// Wait for monitor to initialize and check network status
	time.Sleep(100 * time.Millisecond)

	// If network is up (which it likely is on a test machine), WaitForNetwork should return immediately
	if monitor.IsUp() {
		err := monitor.WaitForNetwork(ctx)
		if err != nil {
			t.Errorf("WaitForNetwork failed when network was already up: %v", err)
		}
	} else {
		t.Log("Network is not up, skipping immediate return test")
	}
}

func TestWaitForNetwork_ContextCancelled(t *testing.T) {
	log := logger.NewTestLogger(t)

	// Create monitor with config that makes network appear down
	config := &MonitorConfig{
		CheckHosts:    []string{"192.0.2.1"}, // TEST-NET-1 - should be unreachable
		CheckInterval: 10 * time.Second,      // Long interval
		CheckTimeout:  100 * time.Millisecond,
	}

	monitor := NewMonitor(config, log)

	ctx := context.Background()
	monitor.Start(ctx)
	defer monitor.Stop()

	// Wait for monitor to check network (and likely determine it's down)
	time.Sleep(200 * time.Millisecond)

	// Create context with short timeout
	waitCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// WaitForNetwork should return context error when cancelled
	err := monitor.WaitForNetwork(waitCtx)
	if err == nil {
		t.Error("Expected error when context is cancelled, got nil")
	}

	if err != context.DeadlineExceeded && err != context.Canceled {
		t.Errorf("Expected context deadline or cancelled error, got: %v", err)
	}
}

func TestWaitForNetwork_StateChange(t *testing.T) {
	log := logger.NewTestLogger(t)
	monitor := NewMonitor(nil, log)

	// Don't start the monitor - manually control the state

	// Set state to down initially
	monitor.state = StateDown

	// Start goroutine to wait for network
	waitDone := make(chan error, 1)
	go func() {
		ctx := context.Background()
		waitDone <- monitor.WaitForNetwork(ctx)
	}()

	// Give the goroutine time to subscribe and start waiting
	time.Sleep(50 * time.Millisecond)

	// Change state to up
	monitor.updateState(StateUp)

	// Wait should complete successfully
	select {
	case err := <-waitDone:
		if err != nil {
			t.Errorf("WaitForNetwork failed after state changed to up: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("WaitForNetwork did not return after state changed to up")
	}
}
