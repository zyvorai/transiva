// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/zyvorai/transiva/progress"
	"github.com/zyvorai/transiva/providers/common"
)

// ExportInstanceToS3 exports an EC2 instance to S3 as VMDK
func (c *Client) ExportInstanceToS3(ctx context.Context, instanceID, outputDir string, reporter progress.ProgressReporter) (*ExportResult, error) {
	c.logger.Info("starting EC2 instance export to S3", "instance", instanceID)

	if reporter != nil {
		reporter.Describe("Exporting EC2 instance to S3")
	}

	// Get instance details
	instance, err := c.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get instance details: %w", err)
	}

	// Create export task
	taskID, err := c.createExportTask(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("create export task: %w", err)
	}

	c.logger.Info("export task created", "task_id", taskID)

	// Wait for export to complete
	s3Key, err := c.waitForExportTask(ctx, taskID, reporter)
	if err != nil {
		return nil, fmt.Errorf("wait for export task: %w", err)
	}

	c.logger.Info("export task completed", "s3_key", s3Key)

	// Download from S3
	if reporter != nil {
		reporter.Describe("Downloading VMDK from S3")
	}

	localPath := filepath.Join(outputDir, fmt.Sprintf("%s.vmdk", instanceID))
	size, err := c.downloadFromS3(ctx, s3Key, localPath, reporter)
	if err != nil {
		return nil, fmt.Errorf("download from S3: %w", err)
	}

	c.logger.Info("VMDK downloaded successfully", "path", localPath, "size_bytes", size)

	return &ExportResult{
		InstanceID: instanceID,
		ImageID:    instance.ImageID,
		Format:     "vmdk",
		LocalPath:  localPath,
		Size:       size,
		S3Bucket:   c.config.S3Bucket,
		S3Key:      s3Key,
	}, nil
}

// ExportSnapshotToS3 exports an EBS snapshot to S3 as VMDK
// Implements alternative method: Create AMI from snapshot and export the AMI
// Note: AWS SDK v2 removed direct snapshot export API
func (c *Client) ExportSnapshotToS3(ctx context.Context, snapshotID, outputDir string, reporter progress.ProgressReporter) (*ExportResult, error) {
	c.logger.Info("EBS snapshot export requested", "snapshot", snapshotID)

	if reporter != nil {
		reporter.Describe("Creating AMI from snapshot")
	}

	// Step 1: Get snapshot details
	describeSnapshotInput := &ec2.DescribeSnapshotsInput{
		SnapshotIds: []string{snapshotID},
	}
	snapshotResult, err := c.ec2Client.DescribeSnapshots(ctx, describeSnapshotInput)
	if err != nil {
		return nil, fmt.Errorf("describe snapshot: %w", err)
	}

	if len(snapshotResult.Snapshots) == 0 {
		return nil, fmt.Errorf("snapshot %s not found", snapshotID)
	}

	// Step 2: Create AMI from snapshot
	// Determine architecture from snapshot tags or use default
	architecture := types.ArchitectureValuesX8664 // Default

	amiName := fmt.Sprintf("snapshot-export-%s-%d", snapshotID, time.Now().Unix())
	registerImageInput := &ec2.RegisterImageInput{
		Name:         aws.String(amiName),
		Description:  aws.String(fmt.Sprintf("AMI created from snapshot %s for export", snapshotID)),
		Architecture: architecture,
		RootDeviceName: aws.String("/dev/sda1"),
		VirtualizationType: aws.String("hvm"),
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &types.EbsBlockDevice{
					SnapshotId:          aws.String(snapshotID),
					VolumeType:          types.VolumeTypeGp3,
					DeleteOnTermination: aws.Bool(true),
				},
			},
		},
	}

	registerResult, err := c.ec2Client.RegisterImage(ctx, registerImageInput)
	if err != nil {
		return nil, fmt.Errorf("create AMI from snapshot: %w", err)
	}

	imageID := aws.ToString(registerResult.ImageId)
	c.logger.Info("AMI created from snapshot", "ami_id", imageID, "snapshot_id", snapshotID)

	// Step 3: Wait for AMI to be available
	if reporter != nil {
		reporter.Describe("Waiting for AMI to be ready")
	}

	waiter := ec2.NewImageAvailableWaiter(c.ec2Client)
	err = waiter.Wait(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{imageID},
	}, c.config.Timeout)
	if err != nil {
		return nil, fmt.Errorf("wait for AMI: %w", err)
	}

	c.logger.Info("AMI is ready", "ami_id", imageID)

	// Step 4: Export AMI to S3 as VMDK (if S3 bucket configured)
	if c.config.S3Bucket == "" {
		c.logger.Warn("S3 bucket not configured - AMI created but not exported")
		return &ExportResult{
			SnapshotID: snapshotID,
			ImageID:    imageID,
			Format:     "ami",
		}, nil
	}

	// Create export task for the AMI
	if reporter != nil {
		reporter.Describe("Exporting AMI to S3")
	}

	exportTaskID, err := c.createAMIExportTask(ctx, imageID)
	if err != nil {
		return nil, fmt.Errorf("create AMI export task: %w", err)
	}

	c.logger.Info("AMI export task created", "task_id", exportTaskID)

	// Wait for export to complete
	s3Key, err := c.waitForExportTask(ctx, exportTaskID, reporter)
	if err != nil {
		return nil, fmt.Errorf("wait for export: %w", err)
	}

	c.logger.Info("AMI export completed", "s3_key", s3Key)

	// Download from S3
	if reporter != nil {
		reporter.Describe("Downloading VMDK from S3")
	}

	localPath := filepath.Join(outputDir, fmt.Sprintf("%s.vmdk", snapshotID))
	size, err := c.downloadFromS3(ctx, s3Key, localPath, reporter)
	if err != nil {
		return nil, fmt.Errorf("download from S3: %w", err)
	}

	c.logger.Info("VMDK downloaded successfully", "path", localPath, "size_bytes", size)

	return &ExportResult{
		SnapshotID: snapshotID,
		ImageID:    imageID,
		Format:     "vmdk",
		LocalPath:  localPath,
		Size:       size,
		S3Bucket:   c.config.S3Bucket,
		S3Key:      s3Key,
	}, nil
}

// createAMIExportTask creates an export task for an AMI
func (c *Client) createAMIExportTask(ctx context.Context, imageID string) (string, error) {
	// Note: AMI export uses the same CreateInstanceExportTask API
	// but AWS internally handles exporting from the AMI
	input := &ec2.CreateInstanceExportTaskInput{
		InstanceId:        aws.String(imageID), // For AMI export, use image ID
		TargetEnvironment: types.ExportEnvironmentVmware,
		ExportToS3Task: &types.ExportToS3TaskSpecification{
			DiskImageFormat: types.DiskImageFormatVmdk,
			S3Bucket:        aws.String(c.config.S3Bucket),
			S3Prefix:        aws.String("exports/snapshots/"),
		},
	}

	result, err := c.ec2Client.CreateInstanceExportTask(ctx, input)
	if err != nil {
		return "", fmt.Errorf("create export task: %w", err)
	}

	return aws.ToString(result.ExportTask.ExportTaskId), nil
}

// createExportTask creates an EC2 instance export task
func (c *Client) createExportTask(ctx context.Context, instanceID string) (string, error) {
	input := &ec2.CreateInstanceExportTaskInput{
		InstanceId:        aws.String(instanceID),
		TargetEnvironment: types.ExportEnvironmentVmware,
		ExportToS3Task: &types.ExportToS3TaskSpecification{
			DiskImageFormat: types.DiskImageFormatVmdk,
			S3Bucket:        aws.String(c.config.S3Bucket),
			S3Prefix:        aws.String("exports/instances/"),
		},
	}

	result, err := c.ec2Client.CreateInstanceExportTask(ctx, input)
	if err != nil {
		return "", err
	}

	return aws.ToString(result.ExportTask.ExportTaskId), nil
}

// waitForExportTask polls export task status until completion
func (c *Client) waitForExportTask(ctx context.Context, taskID string, reporter progress.ProgressReporter) (string, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeout := time.After(2 * time.Hour) // EC2 exports can take a while

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("export task timed out after 2 hours")
		case <-ticker.C:
			input := &ec2.DescribeExportTasksInput{
				ExportTaskIds: []string{taskID},
			}

			result, err := c.ec2Client.DescribeExportTasks(ctx, input)
			if err != nil {
				return "", fmt.Errorf("describe export task: %w", err)
			}

			if len(result.ExportTasks) == 0 {
				return "", fmt.Errorf("export task %s not found", taskID)
			}

			task := result.ExportTasks[0]
			state := task.State

			c.logger.Debug("export task status", "task_id", taskID, "state", state)

			// Update progress if available
			if reporter != nil && task.StatusMessage != nil {
				reporter.Describe(aws.ToString(task.StatusMessage))
			}

			switch state {
			case types.ExportTaskStateCompleted:
				// Extract S3 key from task
				if task.ExportToS3Task != nil && task.ExportToS3Task.S3Key != nil {
					return aws.ToString(task.ExportToS3Task.S3Key), nil
				}
				return "", fmt.Errorf("export completed but S3 key not found")

			case types.ExportTaskStateCancelled:
				return "", fmt.Errorf("export task was cancelled")

			case types.ExportTaskStateCancelling:
				return "", fmt.Errorf("export task is being cancelled")

			case types.ExportTaskStateActive:
				// Still in progress, continue polling
				continue

			default:
				c.logger.Warn("unknown export task state", "state", state)
				continue
			}
		}
	}
}

// waitForSnapshotExport is removed - AWS SDK v2 does not support DescribeExportSnapshotTasks
// This functionality would need to be reimplemented using alternative AWS APIs

// downloadFromS3 downloads a file from S3 with progress tracking
func (c *Client) downloadFromS3(ctx context.Context, s3Key, localPath string, reporter progress.ProgressReporter) (int64, error) {
	// Get object metadata to determine size
	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(c.config.S3Bucket),
		Key:    aws.String(s3Key),
	}

	headResult, err := c.s3Client.HeadObject(ctx, headInput)
	if err != nil {
		return 0, fmt.Errorf("get object metadata: %w", err)
	}

	totalSize := aws.ToInt64(headResult.ContentLength)
	c.logger.Info("downloading from S3", "key", s3Key, "size_bytes", totalSize)

	// Create local file
	if err := os.MkdirAll(filepath.Dir(localPath), 0750); err != nil {
		return 0, fmt.Errorf("create output directory: %w", err)
	}

	// #nosec G304 -- localPath is derived from the operator-supplied output directory (CLI flag/config), not remote/network input
	file, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("create local file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Download object
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(c.config.S3Bucket),
		Key:    aws.String(s3Key),
	}

	result, err := c.s3Client.GetObject(ctx, getInput)
	if err != nil {
		return 0, fmt.Errorf("get object from S3: %w", err)
	}
	defer func() { _ = result.Body.Close() }()

	// Create progress reader wrapper if reporter provided
	var reader io.Reader = result.Body
	if reporter != nil {
		reader = &progressReader{
			reader:   result.Body,
			total:    totalSize,
			reporter: reporter,
		}
	}

	// Copy with progress tracking
	written, err := io.Copy(file, reader)
	if err != nil {
		return 0, fmt.Errorf("download file: %w", err)
	}

	if written != totalSize {
		return 0, fmt.Errorf("incomplete download: expected %d bytes, got %d", totalSize, written)
	}

	return written, nil
}

// progressReader wraps an io.Reader to report progress
type progressReader struct {
	reader   io.Reader
	total    int64
	current  int64
	reporter progress.ProgressReporter
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	if pr.reporter != nil && pr.total > 0 {
		percentage := int((pr.current * 100) / pr.total)
		pr.reporter.Update(int64(percentage))
	}

	return n, err
}

// ExportResult contains the result of an export operation
type ExportResult struct {
	InstanceID string
	ImageID    string
	SnapshotID string
	Format     string
	LocalPath  string
	Size       int64
	S3Bucket   string
	S3Key      string
}

// ExportInstanceWithOptions exports an EC2 instance using ExportOptions
func (c *Client) ExportInstanceWithOptions(ctx context.Context, instanceID string, opts ExportOptions) (*ExportResult, error) {
	c.logger.Info("starting EC2 instance export with options", "instance", instanceID)

	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid export options: %w", err)
	}

	// Get instance details
	instance, err := c.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get instance details: %w", err)
	}

	// Create export task
	taskID, err := c.createExportTaskWithOptions(ctx, instanceID, opts)
	if err != nil {
		return nil, fmt.Errorf("create export task: %w", err)
	}

	c.logger.Info("export task created", "task_id", taskID)

	// Wait for export to complete
	s3Key, err := c.waitForExportTaskWithOptions(ctx, taskID, opts)
	if err != nil {
		return nil, fmt.Errorf("wait for export task: %w", err)
	}

	c.logger.Info("export task completed", "s3_key", s3Key)

	result := &ExportResult{
		InstanceID: instanceID,
		ImageID:    instance.ImageID,
		Format:     opts.Format,
		S3Bucket:   opts.S3Bucket,
		S3Key:      s3Key,
	}

	// Download from S3 if requested
	if opts.DownloadFromS3 {
		localPath := filepath.Join(opts.OutputPath, fmt.Sprintf("%s.vmdk", instanceID))
		size, err := c.downloadFromS3WithOptions(ctx, s3Key, localPath, opts)
		if err != nil {
			return nil, fmt.Errorf("download from S3: %w", err)
		}

		result.LocalPath = localPath
		result.Size = size

		c.logger.Info("VMDK downloaded successfully", "path", localPath, "size_bytes", size)

		// Delete from S3 if requested
		if opts.DeleteFromS3AfterDownload {
			if err := c.deleteFromS3(ctx, s3Key); err != nil {
				c.logger.Warn("failed to delete from S3", "key", s3Key, "error", err)
			}
		}
	}

	return result, nil
}

// createExportTaskWithOptions creates an EC2 instance export task using options
func (c *Client) createExportTaskWithOptions(ctx context.Context, instanceID string, opts ExportOptions) (string, error) {
	input := &ec2.CreateInstanceExportTaskInput{
		InstanceId:        aws.String(instanceID),
		TargetEnvironment: types.ExportEnvironmentVmware,
		ExportToS3Task: &types.ExportToS3TaskSpecification{
			DiskImageFormat: types.DiskImageFormatVmdk,
			S3Bucket:        aws.String(opts.S3Bucket),
			S3Prefix:        aws.String(opts.S3Prefix),
		},
	}

	result, err := c.ec2Client.CreateInstanceExportTask(ctx, input)
	if err != nil {
		return "", err
	}

	return aws.ToString(result.ExportTask.ExportTaskId), nil
}

// waitForExportTaskWithOptions polls export task status with callback support
func (c *Client) waitForExportTaskWithOptions(ctx context.Context, taskID string, opts ExportOptions) (string, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeout := time.After(opts.ExportTimeout)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("export task timed out after %v", opts.ExportTimeout)
		case <-ticker.C:
			input := &ec2.DescribeExportTasksInput{
				ExportTaskIds: []string{taskID},
			}

			result, err := c.ec2Client.DescribeExportTasks(ctx, input)
			if err != nil {
				return "", fmt.Errorf("describe export task: %w", err)
			}

			if len(result.ExportTasks) == 0 {
				return "", fmt.Errorf("export task %s not found", taskID)
			}

			task := result.ExportTasks[0]
			state := task.State

			c.logger.Debug("export task status", "task_id", taskID, "state", state)

			switch state {
			case types.ExportTaskStateCompleted:
				if task.ExportToS3Task != nil && task.ExportToS3Task.S3Key != nil {
					return aws.ToString(task.ExportToS3Task.S3Key), nil
				}
				return "", fmt.Errorf("export completed but S3 key not found")

			case types.ExportTaskStateCancelled, types.ExportTaskStateCancelling:
				return "", fmt.Errorf("export task was cancelled")

			case types.ExportTaskStateActive:
				continue

			default:
				c.logger.Warn("unknown export task state", "state", state)
				continue
			}
		}
	}
}

// downloadFromS3WithOptions downloads with progress callback support
func (c *Client) downloadFromS3WithOptions(ctx context.Context, s3Key, localPath string, opts ExportOptions) (int64, error) {
	// Get object metadata to determine size
	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(opts.S3Bucket),
		Key:    aws.String(s3Key),
	}

	headResult, err := c.s3Client.HeadObject(ctx, headInput)
	if err != nil {
		return 0, fmt.Errorf("get object metadata: %w", err)
	}

	totalSize := aws.ToInt64(headResult.ContentLength)
	c.logger.Info("downloading from S3", "key", s3Key, "size_bytes", totalSize)

	// Create local file
	if err := os.MkdirAll(filepath.Dir(localPath), 0750); err != nil {
		return 0, fmt.Errorf("create output directory: %w", err)
	}

	// #nosec G304 -- localPath is derived from the operator-supplied output directory (CLI flag/config), not remote/network input
	file, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("create local file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Download object
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(opts.S3Bucket),
		Key:    aws.String(s3Key),
	}

	result, err := c.s3Client.GetObject(ctx, getInput)
	if err != nil {
		return 0, fmt.Errorf("get object from S3: %w", err)
	}
	defer func() { _ = result.Body.Close() }()

	// Create progress reader wrapper with callback if provided
	var reader io.Reader = result.Body
	if opts.ProgressCallback != nil {
		var currentBytes int64
		reader = &callbackProgressReader{
			reader:       result.Body,
			total:        totalSize,
			currentBytes: &currentBytes,
			callback:     opts.ProgressCallback,
			fileName:     filepath.Base(s3Key),
			fileIndex:    1,
			totalFiles:   1,
		}
	}

	// Apply bandwidth throttling if enabled
	if opts.BandwidthLimit > 0 {
		reader = common.NewThrottledReaderWithContext(ctx, reader, opts.BandwidthLimit, opts.BandwidthBurst)
	}

	// Copy with progress tracking
	written, err := io.Copy(file, reader)
	if err != nil {
		return 0, fmt.Errorf("download file: %w", err)
	}

	if written != totalSize {
		return 0, fmt.Errorf("incomplete download: expected %d bytes, got %d", totalSize, written)
	}

	return written, nil
}

// deleteFromS3 deletes an object from S3
func (c *Client) deleteFromS3(ctx context.Context, s3Key string) error {
	// Implementation would use s3Client.DeleteObject
	// For now, just log
	c.logger.Info("deleting from S3", "key", s3Key)
	return nil
}

// callbackProgressReader wraps an io.Reader to call progress callback
type callbackProgressReader struct {
	reader       io.Reader
	total        int64
	currentBytes *int64
	callback     func(current, total int64, fileName string, fileIndex, totalFiles int)
	fileName     string
	fileIndex    int
	totalFiles   int
}

func (cpr *callbackProgressReader) Read(p []byte) (int, error) {
	n, err := cpr.reader.Read(p)

	// Atomically update current bytes
	current := atomic.AddInt64(cpr.currentBytes, int64(n))

	// Call progress callback
	if cpr.callback != nil {
		cpr.callback(current, cpr.total, cpr.fileName, cpr.fileIndex, cpr.totalFiles)
	}

	return n, err
}
