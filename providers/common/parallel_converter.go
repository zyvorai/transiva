// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// ParallelConverter handles parallel conversion of multiple disks
type ParallelConverter struct {
	converter   Converter
	maxParallel int
	logger      logger.Logger
}

// NewParallelConverter creates a new parallel converter
func NewParallelConverter(converter Converter, maxParallel int, log logger.Logger) *ParallelConverter {
	if maxParallel <= 0 {
		maxParallel = 1
	}

	return &ParallelConverter{
		converter:   converter,
		maxParallel: maxParallel,
		logger:      log,
	}
}

// DiskConversionTask represents a single disk conversion task
type DiskConversionTask struct {
	ManifestPath string
	DiskIndex    int
	Options      ConvertOptions
}

// DiskConversionResult represents the result of a single disk conversion
type DiskConversionResult struct {
	DiskIndex int
	Result    *ConversionResult
	Error     error
	Duration  time.Duration
}

// ConvertParallel converts multiple disks in parallel
func (pc *ParallelConverter) ConvertParallel(ctx context.Context, tasks []*DiskConversionTask) ([]*DiskConversionResult, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no conversion tasks provided")
	}

	pc.logger.Info("starting parallel conversion", "tasks", len(tasks), "max_parallel", pc.maxParallel)

	// Create semaphore for limiting parallelism
	sem := make(chan struct{}, pc.maxParallel)

	// Results channel
	results := make(chan *DiskConversionResult, len(tasks))

	// Error group
	var wg sync.WaitGroup

	// Launch conversion tasks
	for _, task := range tasks {
		wg.Add(1)
		go func(t *DiskConversionTask) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Convert disk
			result := pc.convertDisk(ctx, t)
			results <- result
		}(task)
	}

	// Wait for all tasks to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []*DiskConversionResult
	for result := range results {
		allResults = append(allResults, result)

		if result.Error != nil {
			pc.logger.Error("disk conversion failed",
				"disk_index", result.DiskIndex,
				"error", result.Error)
		} else {
			pc.logger.Info("disk conversion completed",
				"disk_index", result.DiskIndex,
				"duration", result.Duration,
				"files", len(result.Result.ConvertedFiles))
		}
	}

	// Check if all conversions succeeded
	var failedCount int
	for _, result := range allResults {
		if result.Error != nil {
			failedCount++
		}
	}

	if failedCount > 0 {
		pc.logger.Warn("parallel conversion completed with errors",
			"total", len(allResults),
			"failed", failedCount)
	} else {
		pc.logger.Info("parallel conversion completed successfully",
			"total", len(allResults))
	}

	return allResults, nil
}

// convertDisk converts a single disk
func (pc *ParallelConverter) convertDisk(ctx context.Context, task *DiskConversionTask) *DiskConversionResult {
	startTime := time.Now()

	pc.logger.Info("starting disk conversion", "disk_index", task.DiskIndex)

	result, err := pc.converter.Convert(ctx, task.ManifestPath, task.Options)

	return &DiskConversionResult{
		DiskIndex: task.DiskIndex,
		Result:    result,
		Error:     err,
		Duration:  time.Since(startTime),
	}
}

// ConvertBatch converts multiple VMs in batch mode
func (pc *ParallelConverter) ConvertBatch(ctx context.Context, manifests []string, opts ConvertOptions) ([]*ConversionResult, error) {
	pc.logger.Info("starting batch conversion", "vms", len(manifests))

	var tasks []*DiskConversionTask
	for i, manifestPath := range manifests {
		tasks = append(tasks, &DiskConversionTask{
			ManifestPath: manifestPath,
			DiskIndex:    i,
			Options:      opts,
		})
	}

	diskResults, err := pc.ConvertParallel(ctx, tasks)
	if err != nil {
		return nil, err
	}

	// Extract conversion results
	var results []*ConversionResult
	for _, dr := range diskResults {
		if dr.Error != nil {
			// Create failed result
			results = append(results, &ConversionResult{
				Success: false,
				Error:   dr.Error.Error(),
			})
		} else {
			results = append(results, dr.Result)
		}
	}

	return results, nil
}

// ParallelConversionStats holds statistics about parallel conversion
type ParallelConversionStats struct {
	TotalTasks      int
	SuccessfulTasks int
	FailedTasks     int
	TotalDuration   time.Duration
	AverageDuration time.Duration
	MaxDuration     time.Duration
	MinDuration     time.Duration
}

// GetStats computes statistics from conversion results
func GetStats(results []*DiskConversionResult) *ParallelConversionStats {
	stats := &ParallelConversionStats{
		TotalTasks: len(results),
	}

	if len(results) == 0 {
		return stats
	}

	stats.MinDuration = results[0].Duration

	for _, result := range results {
		if result.Error == nil {
			stats.SuccessfulTasks++
		} else {
			stats.FailedTasks++
		}

		stats.TotalDuration += result.Duration

		if result.Duration > stats.MaxDuration {
			stats.MaxDuration = result.Duration
		}

		if result.Duration < stats.MinDuration {
			stats.MinDuration = result.Duration
		}
	}

	if stats.TotalTasks > 0 {
		stats.AverageDuration = stats.TotalDuration / time.Duration(stats.TotalTasks)
	}

	return stats
}
