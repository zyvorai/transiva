// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	"context"
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"

	"github.com/zyvorai/transiva/progress"
)

// ExportOptions contains options for exporting an Alibaba Cloud ECS instance
type ExportOptions struct {
	OutputDir         string                    // Local output directory
	Format            string                    // Export format: qcow2, raw
	ImageName         string                    // Custom image name
	UploadToOSS       bool                      // Upload to OSS storage
	Bucket            string                    // OSS bucket name
	DeleteAfterExport bool                      // Delete custom image after export
	ProgressReporter  progress.ProgressReporter // Progress reporter
}

// ExportResult contains the result of an export operation
type ExportResult struct {
	InstanceID   string
	InstanceName string
	ImageID      string
	ImageName    string
	ExportFormat string
	OSSURL       string
	Duration     time.Duration
}

// ExportInstance exports an Alibaba Cloud ECS instance to a custom image
func (c *Client) ExportInstance(ctx context.Context, instanceID string, opts *ExportOptions) (*ExportResult, error) {
	startTime := time.Now()

	c.logger.Info("starting Alibaba Cloud instance export",
		"instance_id", instanceID,
		"format", opts.Format)

	// Step 1: Get instance details
	c.logger.Info("retrieving instance details", "instance_id", instanceID)
	instance, err := c.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get instance: %w", err)
	}

	// Step 2: Create custom image
	imageName := opts.ImageName
	if imageName == "" {
		imageName = fmt.Sprintf("%s-export-%d", instance.Name, time.Now().Unix())
	}

	c.logger.Info("creating custom image", "image_name", imageName)
	imageID, err := c.CreateImage(ctx, instanceID, imageName)
	if err != nil {
		return nil, fmt.Errorf("create image: %w", err)
	}

	result := &ExportResult{
		InstanceID:   instanceID,
		InstanceName: instance.Name,
		ImageID:      imageID,
		ImageName:    imageName,
		ExportFormat: opts.Format,
	}

	// Cleanup image if requested
	if opts.DeleteAfterExport {
		defer func() {
			c.logger.Info("deleting custom image", "image_id", imageID)
			if err := c.DeleteImage(context.Background(), imageID); err != nil {
				c.logger.Error("failed to delete image", "image_id", imageID, "error", err)
			}
		}()
	}

	// Step 3: Wait for image to be ready
	c.logger.Info("waiting for image to be ready", "image_id", imageID)
	if err := c.WaitForImageReady(ctx, imageID, 30*time.Minute); err != nil {
		return nil, fmt.Errorf("wait for image: %w", err)
	}

	// Step 4: Export to OSS if requested
	if opts.UploadToOSS {
		c.logger.Info("exporting image to OSS",
			"image_id", imageID,
			"bucket", opts.Bucket)

		ossURL, err := c.exportImageToOSS(ctx, imageID, opts)
		if err != nil {
			return nil, fmt.Errorf("export to OSS: %w", err)
		}

		result.OSSURL = ossURL
	}

	result.Duration = time.Since(startTime)

	c.logger.Info("instance export completed",
		"instance_id", instanceID,
		"duration", result.Duration)

	return result, nil
}

// exportImageToOSS exports an image to OSS
func (c *Client) exportImageToOSS(ctx context.Context, imageID string, opts *ExportOptions) (string, error) {
	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying export to OSS", "image_id", imageID, "attempt", attempt)
		}

		// Create export task
		request := ecs.CreateExportImageRequest()
		request.Scheme = "https"
		request.ImageId = imageID
		request.OSSBucket = opts.Bucket
		request.OSSPrefix = fmt.Sprintf("exports/%s", opts.ImageName)
		request.ImageFormat = opts.Format

		response, err := c.ecsClient.ExportImage(request)
		if err != nil {
			return nil, fmt.Errorf("export image: %w", err)
		}

		taskID := response.TaskId
		c.logger.Info("export task created", "task_id", taskID)

		// Wait for export task to complete
		if err := c.waitForExportTask(ctx, taskID, 60*time.Minute); err != nil {
			return nil, fmt.Errorf("wait for export task: %w", err)
		}

		ossURL := fmt.Sprintf("oss://%s/exports/%s.%s", opts.Bucket, opts.ImageName, opts.Format)
		return ossURL, nil
	}, fmt.Sprintf("export image %s to OSS", imageID))

	if err != nil {
		return "", err
	}

	return result.(string), nil
}

// waitForExportTask waits for an export task to complete
func (c *Client) waitForExportTask(ctx context.Context, taskID string, timeout time.Duration) error {
	c.logger.Info("waiting for export task",
		"task_id", taskID,
		"timeout", timeout)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for export task")
			}

			request := ecs.CreateDescribeTasksRequest()
			request.Scheme = "https"
			request.TaskIds = fmt.Sprintf("[\"%s\"]", taskID)

			response, err := c.ecsClient.DescribeTasks(request)
			if err != nil {
				return fmt.Errorf("describe task: %w", err)
			}

			if len(response.TaskSet.Task) == 0 {
				return fmt.Errorf("task not found: %s", taskID)
			}

			task := response.TaskSet.Task[0]
			c.logger.Debug("export task status",
				"task_id", taskID,
				"status", task.TaskStatus)

			if task.TaskStatus == "Finished" {
				c.logger.Info("export task completed", "task_id", taskID)
				return nil
			}

			if task.TaskStatus == "Failed" {
				return fmt.Errorf("export task failed")
			}
		}
	}
}
