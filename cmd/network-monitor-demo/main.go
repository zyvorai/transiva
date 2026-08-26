// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/network"
	"github.com/zyvorai/transiva/retry"
)

func main() {
	log := logger.New("info")

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("   Network Monitor Demo - Real-time Network State Detection")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	// Create network monitor with custom config
	config := &network.MonitorConfig{
		CheckInterval:   5 * time.Second,
		CheckTimeout:    3 * time.Second,
		CheckHosts:      []string{"8.8.8.8", "1.1.1.1"},
		NotifyOnChange:  true,
		EnableNetlink:   true, // Enable netlink for real-time events
		PreferredIfaces: nil,  // Monitor all interfaces
	}

	monitor := network.NewMonitor(config, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitor
	fmt.Println("Starting network monitor...")
	if err := monitor.Start(ctx); err != nil {
		fmt.Printf("ERROR: Failed to start monitor: %v\n", err)
		os.Exit(1)
	}
	defer monitor.Stop()

	fmt.Println("Network monitor started!")
	fmt.Println()

	// Subscribe to state changes
	stateCh := monitor.Subscribe()

	// Show initial state
	initialState := monitor.GetState()
	fmt.Printf("Initial network state: %s\n", initialState)
	fmt.Println()

	// Show interface statistics
	stats, err := monitor.GetInterfaceStats()
	if err != nil {
		fmt.Printf("WARNING: Could not get interface stats: %v\n", err)
	} else {
		fmt.Println("Network Interfaces:")
		fmt.Println("───────────────────────────────────────────────────────────────")
		for name, stat := range stats {
			status := "DOWN"
			if stat.IsUp {
				status = "UP"
			}
			fmt.Printf("%-15s [%s]\n", name, status)
			fmt.Printf("  MAC:        %s\n", stat.MACAddress)
			fmt.Printf("  MTU:        %d\n", stat.MTU)
			fmt.Printf("  RX:         %d bytes (%d packets, %d errors)\n",
				stat.RxBytes, stat.RxPackets, stat.RxErrors)
			fmt.Printf("  TX:         %d bytes (%d packets, %d errors)\n",
				stat.TxBytes, stat.TxPackets, stat.TxErrors)
			fmt.Println()
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("Monitoring network state changes...")
	fmt.Println("Try disconnecting/reconnecting your network to see real-time detection!")
	fmt.Println("Press Ctrl+C to exit")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	// Demonstrate retry integration
	go demonstrateRetryIntegration(ctx, monitor, log)

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Monitor state changes
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Println("\nReceived interrupt signal, shutting down...")
			return

		case state := <-stateCh:
			timestamp := time.Now().Format("15:04:05")
			fmt.Printf("[%s] Network state changed: %s\n", timestamp, state)

			// Show current connectivity status
			if monitor.IsUp() {
				fmt.Printf("[%s] ✓ Network is available\n", timestamp)
			} else {
				fmt.Printf("[%s] ✗ Network is unavailable - operations will pause\n", timestamp)
			}
			fmt.Println()

		case <-ticker.C:
			// Periodic status update
			timestamp := time.Now().Format("15:04:05")
			currentState := monitor.GetState()
			fmt.Printf("[%s] Status check - Network: %s\n", timestamp, currentState)
		}
	}
}

func demonstrateRetryIntegration(ctx context.Context, monitor *network.Monitor, log logger.Logger) {
	// Wait a bit before starting demo
	time.Sleep(5 * time.Second)

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("   Demonstrating Retry Integration")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	// Create retry config with network awareness
	retryConfig := &retry.RetryConfig{
		MaxAttempts:    10,
		InitialDelay:   1 * time.Second,
		MaxDelay:       30 * time.Second,
		Multiplier:     2.0,
		Jitter:         true,
		WaitForNetwork: true, // Enable network-aware retry
	}

	// Create retryer and attach network monitor
	retryer := retry.NewRetryer(retryConfig, log)
	retryer.SetNetworkMonitor(monitor)

	// Simulate an operation that might fail due to network issues
	operationCount := 0
	err := retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		operationCount++
		timestamp := time.Now().Format("15:04:05")

		fmt.Printf("[%s] Simulated operation attempt %d (total: %d)\n",
			timestamp, attempt, operationCount)

		// Check if network is up
		if !monitor.IsUp() {
			fmt.Printf("[%s] Network is down - operation will wait for recovery\n", timestamp)
			return fmt.Errorf("network unavailable")
		}

		// Simulate success after network is up
		fmt.Printf("[%s] ✓ Simulated operation succeeded!\n", timestamp)
		return nil
	}, "simulated network operation")

	if err != nil {
		fmt.Printf("\nSimulated operation failed: %v\n", err)
	} else {
		fmt.Printf("\nSimulated operation completed successfully after %d total attempts\n", operationCount)
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}
