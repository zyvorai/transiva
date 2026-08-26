// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	// Performance counter intervals (vSphere defaults)
	// Level 1: Realtime (20 seconds)
	// Level 2: 5 minutes
	// Level 3: 30 minutes
	// Level 4: 2 hours
	intervalRealtime = int32(20)
	interval5Min     = int32(300)
	interval30Min    = int32(1800)
	interval2Hour    = int32(7200)
)

// Counter metric names for different entity types
var (
	cpuUsageCounter    = "cpu.usage.average"      // Percentage
	cpuUsedCounter     = "cpu.usagemhz.average"   // MHz
	memUsageCounter    = "mem.usage.average"      // Percentage
	memConsumedCounter = "mem.consumed.average"   // KB
	diskReadCounter    = "disk.read.average"      // KBps
	diskWriteCounter   = "disk.write.average"     // KBps
	netRxCounter       = "net.received.average"   // KBps
	netTxCounter       = "net.transmitted.average" // KBps
)

// GetRealtimeMetrics retrieves real-time performance metrics (20-second interval)
func (c *VSphereClient) GetRealtimeMetrics(ctx context.Context, entityName, entityType string) (*PerformanceMetrics, error) {
	// Find entity
	entity, err := c.findEntity(ctx, entityName, entityType)
	if err != nil {
		return nil, fmt.Errorf("find entity: %w", err)
	}

	// Get performance manager
	perfManager := performance.NewManager(c.client.Client)

	// Get available counters
	counters, err := perfManager.CounterInfoByName(ctx)
	if err != nil {
		return nil, fmt.Errorf("get performance counters: %w", err)
	}

	// Build metric IDs for the counters we want
	var metricIDs []types.PerfMetricId
	counterMap := make(map[int32]string) // counterID -> metric name

	for _, metricName := range []string{
		cpuUsageCounter,
		cpuUsedCounter,
		memUsageCounter,
		memConsumedCounter,
		diskReadCounter,
		diskWriteCounter,
		netRxCounter,
		netTxCounter,
	} {
		if counter, exists := counters[metricName]; exists {
			metricID := types.PerfMetricId{
				CounterId: counter.Key,
				Instance:  "*", // Aggregate across all instances
			}
			metricIDs = append(metricIDs, metricID)
			counterMap[counter.Key] = metricName
		}
	}

	if len(metricIDs) == 0 {
		return nil, fmt.Errorf("no performance counters available")
	}

	// Query realtime performance data
	spec := types.PerfQuerySpec{
		Entity:     entity.Reference(),
		MetricId:   metricIDs,
		MaxSample:  1,        // Latest sample only
		IntervalId: intervalRealtime, // 20 seconds
	}

	result, err := perfManager.Query(ctx, []types.PerfQuerySpec{spec})
	if err != nil {
		return nil, fmt.Errorf("query performance metrics: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no performance data available")
	}

	// Parse results
	metrics := &PerformanceMetrics{
		EntityName: entityName,
		EntityType: entityType,
		Timestamp:  time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	// Extract metric values - type assert to access Value field
	if entityMetric, ok := result[0].(*types.PerfEntityMetric); ok {
		for _, base := range entityMetric.Value {
			if series, ok := base.(*types.PerfMetricIntSeries); ok {
				if metricName, exists := counterMap[series.Id.CounterId]; exists {
					if len(series.Value) > 0 {
						value := float64(series.Value[0])
						c.assignMetricValue(metrics, metricName, value)
					}
				}
			}
		}
	}

	c.logger.Info("retrieved realtime metrics",
		"entity", entityName,
		"type", entityType,
		"cpu_percent", metrics.CPUPercent,
		"memory_percent", metrics.MemoryPercent)

	return metrics, nil
}

// GetMetricsHistory retrieves historical metrics for a time range
func (c *VSphereClient) GetMetricsHistory(ctx context.Context, entityName, entityType string, start, end time.Time, interval string) (*MetricsHistory, error) {
	// Find entity
	entity, err := c.findEntity(ctx, entityName, entityType)
	if err != nil {
		return nil, fmt.Errorf("find entity: %w", err)
	}

	// Determine interval ID
	var intervalID int32
	switch interval {
	case "realtime":
		intervalID = intervalRealtime
	case "5min":
		intervalID = interval5Min
	case "30min":
		intervalID = interval30Min
	case "2hour":
		intervalID = interval2Hour
	default:
		return nil, fmt.Errorf("invalid interval: %s (valid values: realtime, 5min, 30min, 2hour)", interval)
	}

	// Get performance manager
	perfManager := performance.NewManager(c.client.Client)

	// Get counter IDs
	counters, err := perfManager.CounterInfoByName(ctx)
	if err != nil {
		return nil, fmt.Errorf("get performance counters: %w", err)
	}

	var metricIDs []types.PerfMetricId
	counterMap := make(map[int32]string)

	for _, metricName := range []string{
		cpuUsageCounter,
		cpuUsedCounter,
		memUsageCounter,
		memConsumedCounter,
		diskReadCounter,
		diskWriteCounter,
		netRxCounter,
		netTxCounter,
	} {
		if counter, exists := counters[metricName]; exists {
			metricID := types.PerfMetricId{
				CounterId: counter.Key,
				Instance:  "*",
			}
			metricIDs = append(metricIDs, metricID)
			counterMap[counter.Key] = metricName
		}
	}

	// Query historical data
	spec := types.PerfQuerySpec{
		Entity:     entity.Reference(),
		MetricId:   metricIDs,
		StartTime:  &start,
		EndTime:    &end,
		IntervalId: intervalID,
	}

	result, err := perfManager.Query(ctx, []types.PerfQuerySpec{spec})
	if err != nil {
		return nil, fmt.Errorf("query historical metrics: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no historical data available")
	}

	// Parse results into samples
	history := &MetricsHistory{
		EntityName: entityName,
		Interval:   interval,
		StartTime:  start,
		EndTime:    end,
		Samples:    []PerformanceMetrics{},
	}

	// Build samples from time series data
	// Each sample corresponds to a timestamp
	sampleMap := make(map[time.Time]*PerformanceMetrics)

	// Type assert to access Value field
	if entityMetric, ok := result[0].(*types.PerfEntityMetric); ok {
		for _, base := range entityMetric.Value {
			if series, ok := base.(*types.PerfMetricIntSeries); ok {
				if metricName, exists := counterMap[series.Id.CounterId]; exists {
					// Iterate through time series values
					for i, value := range series.Value {
						// Calculate timestamp for this sample
						sampleTime := start.Add(time.Duration(i) * time.Duration(intervalID) * time.Second)

						// Get or create sample for this timestamp
						sample, exists := sampleMap[sampleTime]
						if !exists {
							sample = &PerformanceMetrics{
								EntityName: entityName,
								EntityType: entityType,
								Timestamp:  sampleTime,
								Metadata:   make(map[string]interface{}),
							}
							sampleMap[sampleTime] = sample
						}

						// Assign metric value
						c.assignMetricValue(sample, metricName, float64(value))
					}
				}
			}
		}
	}

	// Convert map to sorted slice
	for _, sample := range sampleMap {
		history.Samples = append(history.Samples, *sample)
	}

	c.logger.Info("retrieved historical metrics",
		"entity", entityName,
		"interval", interval,
		"samples", len(history.Samples))

	return history, nil
}

// StreamMetrics streams real-time metrics via channel
func (c *VSphereClient) StreamMetrics(ctx context.Context, entityName, entityType string, interval time.Duration) (<-chan PerformanceMetrics, error) {
	ch := make(chan PerformanceMetrics, 10)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				c.logger.Info("metrics stream cancelled", "entity", entityName)
				return
			case <-ticker.C:
				metrics, err := c.GetRealtimeMetrics(ctx, entityName, entityType)
				if err != nil {
					c.logger.Error("failed to get metrics", "error", err)
					continue
				}

				select {
				case ch <- *metrics:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	c.logger.Info("started metrics stream", "entity", entityName, "interval", interval)
	return ch, nil
}

// Helper function to find entity by name and type
func (c *VSphereClient) findEntity(ctx context.Context, name, entityType string) (types.ManagedObjectReference, error) {
	var entity types.ManagedObjectReference

	switch entityType {
	case "vm":
		vm, err := c.finder.VirtualMachine(ctx, name)
		if err != nil {
			return entity, fmt.Errorf("find VM: %w", err)
		}
		entity = vm.Reference()

	case "host":
		host, err := c.finder.HostSystem(ctx, name)
		if err != nil {
			return entity, fmt.Errorf("find host: %w", err)
		}
		entity = host.Reference()

	case "cluster":
		cluster, err := c.finder.ClusterComputeResource(ctx, name)
		if err != nil {
			return entity, fmt.Errorf("find cluster: %w", err)
		}
		entity = cluster.Reference()

	default:
		return entity, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	return entity, nil
}

// Helper function to assign metric value to PerformanceMetrics struct
func (c *VSphereClient) assignMetricValue(metrics *PerformanceMetrics, metricName string, value float64) {
	switch metricName {
	case cpuUsageCounter:
		metrics.CPUPercent = value / 100.0 // Convert from hundredths to percentage
	case cpuUsedCounter:
		metrics.CPUUsageMhz = int64(value)
	case memUsageCounter:
		metrics.MemoryPercent = value / 100.0
	case memConsumedCounter:
		metrics.MemoryUsageMB = int64(value / 1024) // Convert KB to MB
	case diskReadCounter:
		metrics.DiskReadMBps = value / 1024.0 // Convert KBps to MBps
	case diskWriteCounter:
		metrics.DiskWriteMBps = value / 1024.0
	case netRxCounter:
		metrics.NetRxMBps = value / 1024.0
	case netTxCounter:
		metrics.NetTxMBps = value / 1024.0
	}
}
