// SPDX-License-Identifier: Apache-2.0

package formats

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// Converter handles conversion between disk formats
type Converter struct {
	logger logger.Logger
}

// NewConverter creates a new format converter
func NewConverter(log logger.Logger) *Converter {
	return &Converter{
		logger: log,
	}
}

// ConversionOptions holds options for conversion
type ConversionOptions struct {
	SourceFormat      DiskFormat
	TargetFormat      DiskFormat
	Compress          bool
	CompressionLevel  int
	BufferSize        int
	PreallocateTarget bool
	ProgressCallback  func(progress float64, bytesProcessed int64)
}

// DefaultConversionOptions returns default conversion options
func DefaultConversionOptions() *ConversionOptions {
	return &ConversionOptions{
		BufferSize:        4 * 1024 * 1024, // 4MB buffer
		CompressionLevel:  6,
		PreallocateTarget: false,
	}
}

// ConversionResult holds the result of a conversion
type ConversionResult struct {
	SourcePath   string
	TargetPath   string
	SourceFormat DiskFormat
	TargetFormat DiskFormat
	SourceSize   int64
	TargetSize   int64
	Duration     time.Duration
	BytesCopied  int64
	Compressed   bool
}

// Convert converts a disk image from one format to another
func (c *Converter) Convert(ctx context.Context, sourcePath, targetPath string, opts *ConversionOptions) (*ConversionResult, error) {
	startTime := time.Now()

	c.logger.Info("starting format conversion",
		"source", sourcePath,
		"target", targetPath,
		"source_format", opts.SourceFormat,
		"target_format", opts.TargetFormat)

	// Auto-detect source format if not specified
	if opts.SourceFormat == FormatUnknown || opts.SourceFormat == "" {
		detected, err := DetectFormat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to detect source format: %w", err)
		}
		opts.SourceFormat = detected
		c.logger.Info("detected source format", "format", detected)
	}

	// Validate formats
	if err := c.validateFormats(opts.SourceFormat, opts.TargetFormat); err != nil {
		return nil, err
	}

	// Get source info
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat source: %w", err)
	}

	// Perform conversion based on format combination
	var bytesCopied int64
	var targetSize int64

	switch {
	case opts.SourceFormat == FormatRAW && opts.TargetFormat == FormatQCOW2:
		bytesCopied, err = c.convertRAWToQCOW2(ctx, sourcePath, targetPath, opts)
	case opts.SourceFormat == FormatQCOW2 && opts.TargetFormat == FormatRAW:
		bytesCopied, err = c.convertQCOW2ToRAW(ctx, sourcePath, targetPath, opts)
	case opts.SourceFormat == FormatVMDK && opts.TargetFormat == FormatQCOW2:
		bytesCopied, err = c.convertVMDKToQCOW2(ctx, sourcePath, targetPath, opts)
	case opts.SourceFormat == FormatVMDK && opts.TargetFormat == FormatRAW:
		bytesCopied, err = c.convertVMDKToRAW(ctx, sourcePath, targetPath, opts)
	case opts.SourceFormat == opts.TargetFormat:
		// Same format, just copy
		bytesCopied, err = c.copyFile(ctx, sourcePath, targetPath, opts)
	default:
		return nil, fmt.Errorf("unsupported conversion: %s to %s", opts.SourceFormat, opts.TargetFormat)
	}

	if err != nil {
		return nil, fmt.Errorf("conversion failed: %w", err)
	}

	// Get target size
	if targetInfo, err := os.Stat(targetPath); err == nil {
		targetSize = targetInfo.Size()
	}

	result := &ConversionResult{
		SourcePath:   sourcePath,
		TargetPath:   targetPath,
		SourceFormat: opts.SourceFormat,
		TargetFormat: opts.TargetFormat,
		SourceSize:   sourceInfo.Size(),
		TargetSize:   targetSize,
		Duration:     time.Since(startTime),
		BytesCopied:  bytesCopied,
		Compressed:   opts.Compress,
	}

	c.logger.Info("conversion completed",
		"duration", result.Duration,
		"source_size_mb", result.SourceSize/1024/1024,
		"target_size_mb", result.TargetSize/1024/1024,
		"compression_ratio", float64(result.TargetSize)/float64(result.SourceSize))

	return result, nil
}

// convertRAWToQCOW2 converts a RAW image to QCOW2
func (c *Converter) convertRAWToQCOW2(ctx context.Context, sourcePath, targetPath string, opts *ConversionOptions) (int64, error) {
	c.logger.Debug("converting RAW to QCOW2")

	// For now, use a simplified conversion
	// In production, you'd create a proper QCOW2 with header, L1/L2 tables, etc.
	// This is a placeholder that does RAW copy - real QCOW2 creation is complex

	// #nosec G304 -- sourcePath is supplied by the local hyperconvert CLI operator, not a remote caller
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open source: %w", err)
	}
	defer func() { _ = sourceFile.Close() }()

	// #nosec G304 -- targetPath is supplied by the local hyperconvert CLI operator, not a remote caller
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create target: %w", err)
	}
	defer func() { _ = targetFile.Close() }()

	// Write QCOW2 header (simplified - just magic for now)
	// Real implementation would write full QCOW2 header with L1/L2 tables
	if _, err := targetFile.Write(MagicQCOW2); err != nil {
		return 0, fmt.Errorf("failed to write QCOW2 header: %w", err)
	}

	// For demonstration, treat as RAW for now
	// TODO: Implement full QCOW2 format writing
	c.logger.Warn("QCOW2 conversion not fully implemented, using RAW copy")
	return c.copyWithProgress(ctx, sourceFile, targetFile, opts)
}

// convertQCOW2ToRAW converts a QCOW2 image to RAW
func (c *Converter) convertQCOW2ToRAW(ctx context.Context, sourcePath, targetPath string, opts *ConversionOptions) (int64, error) {
	c.logger.Debug("converting QCOW2 to RAW")

	// For now, use simplified conversion
	// Real implementation would parse QCOW2 L1/L2 tables and extract data
	// This is a placeholder

	// #nosec G304 -- sourcePath is supplied by the local hyperconvert CLI operator, not a remote caller
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open source: %w", err)
	}
	defer func() { _ = sourceFile.Close() }()

	// #nosec G304 -- targetPath is supplied by the local hyperconvert CLI operator, not a remote caller
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create target: %w", err)
	}
	defer func() { _ = targetFile.Close() }()

	// Skip QCOW2 header and copy data
	// TODO: Implement full QCOW2 parsing
	c.logger.Warn("QCOW2 parsing not fully implemented, using RAW copy")
	return c.copyWithProgress(ctx, sourceFile, targetFile, opts)
}

// convertVMDKToQCOW2 converts a VMDK image to QCOW2
func (c *Converter) convertVMDKToQCOW2(ctx context.Context, sourcePath, targetPath string, opts *ConversionOptions) (int64, error) {
	c.logger.Debug("converting VMDK to QCOW2")

	// Two-step conversion: VMDK → RAW → QCOW2
	tempPath := targetPath + ".tmp.raw"
	defer func() { _ = os.Remove(tempPath) }()

	// Step 1: VMDK to RAW
	if _, err := c.convertVMDKToRAW(ctx, sourcePath, tempPath, opts); err != nil {
		return 0, fmt.Errorf("VMDK to RAW failed: %w", err)
	}

	// Step 2: RAW to QCOW2
	return c.convertRAWToQCOW2(ctx, tempPath, targetPath, opts)
}

// convertVMDKToRAW converts a VMDK image to RAW
func (c *Converter) convertVMDKToRAW(ctx context.Context, sourcePath, targetPath string, opts *ConversionOptions) (int64, error) {
	c.logger.Debug("converting VMDK to RAW")

	// For streamedOptimized or sparse VMDK, need to parse grain tables
	// For flat VMDK, it's essentially RAW
	// This is a simplified version

	// #nosec G304 -- sourcePath is supplied by the local hyperconvert CLI operator, not a remote caller
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open source: %w", err)
	}
	defer func() { _ = sourceFile.Close() }()

	// #nosec G304 -- targetPath is supplied by the local hyperconvert CLI operator, not a remote caller
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create target: %w", err)
	}
	defer func() { _ = targetFile.Close() }()

	// TODO: Implement full VMDK parsing (descriptor, grain tables, etc.)
	c.logger.Warn("VMDK parsing not fully implemented, using RAW copy")
	return c.copyWithProgress(ctx, sourceFile, targetFile, opts)
}

// copyFile simply copies a file (for same-format conversions)
func (c *Converter) copyFile(ctx context.Context, sourcePath, targetPath string, opts *ConversionOptions) (int64, error) {
	c.logger.Debug("copying file (same format)")

	// #nosec G304 -- sourcePath is supplied by the local hyperconvert CLI operator, not a remote caller
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open source: %w", err)
	}
	defer func() { _ = sourceFile.Close() }()

	// #nosec G304 -- targetPath is supplied by the local hyperconvert CLI operator, not a remote caller
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create target: %w", err)
	}
	defer func() { _ = targetFile.Close() }()

	return c.copyWithProgress(ctx, sourceFile, targetFile, opts)
}

// copyWithProgress copies data with progress callbacks
func (c *Converter) copyWithProgress(ctx context.Context, src io.Reader, dst io.Writer, opts *ConversionOptions) (int64, error) {
	buffer := make([]byte, opts.BufferSize)
	var totalCopied int64

	// Get total size for progress calculation
	var totalSize int64
	if seeker, ok := src.(io.Seeker); ok {
		if size, err := seeker.Seek(0, io.SeekEnd); err == nil {
			totalSize = size
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				return 0, fmt.Errorf("failed to seek back to start: %w", err)
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return totalCopied, ctx.Err()
		default:
		}

		nr, err := src.Read(buffer)
		if nr > 0 {
			nw, err := dst.Write(buffer[0:nr])
			if err != nil {
				return totalCopied, fmt.Errorf("write error: %w", err)
			}
			if nr != nw {
				return totalCopied, fmt.Errorf("short write: %d != %d", nr, nw)
			}

			totalCopied += int64(nw)

			// Progress callback
			if opts.ProgressCallback != nil && totalSize > 0 {
				progress := float64(totalCopied) / float64(totalSize) * 100
				opts.ProgressCallback(progress, totalCopied)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return totalCopied, fmt.Errorf("read error: %w", err)
		}
	}

	return totalCopied, nil
}

// validateFormats validates source and target formats
func (c *Converter) validateFormats(source, target DiskFormat) error {
	if source == FormatUnknown {
		return fmt.Errorf("unknown source format")
	}
	if target == FormatUnknown {
		return fmt.Errorf("unknown target format")
	}

	// Check if conversion is supported
	supported := []struct {
		from, to DiskFormat
	}{
		{FormatRAW, FormatQCOW2},
		{FormatQCOW2, FormatRAW},
		{FormatVMDK, FormatRAW},
		{FormatVMDK, FormatQCOW2},
		{FormatRAW, FormatRAW},
		{FormatQCOW2, FormatQCOW2},
		{FormatVMDK, FormatVMDK},
	}

	for _, s := range supported {
		if s.from == source && s.to == target {
			return nil
		}
	}

	return fmt.Errorf("unsupported conversion: %s to %s", source, target)
}

// ConvertInPlace converts a file in place (creates temp file, then replaces)
func (c *Converter) ConvertInPlace(ctx context.Context, path string, targetFormat DiskFormat, opts *ConversionOptions) (*ConversionResult, error) {
	tempPath := path + ".converting"

	result, err := c.Convert(ctx, path, tempPath, opts)
	if err != nil {
		if rerr := os.Remove(tempPath); rerr != nil && !os.IsNotExist(rerr) {
			c.logger.Warn("failed to remove temp file", "path", tempPath, "error", rerr)
		}
		return nil, err
	}

	// Replace original with converted
	if err := os.Rename(tempPath, path); err != nil {
		if rerr := os.Remove(tempPath); rerr != nil && !os.IsNotExist(rerr) {
			c.logger.Warn("failed to remove temp file", "path", tempPath, "error", rerr)
		}
		return nil, fmt.Errorf("failed to replace original: %w", err)
	}

	result.TargetPath = path
	return result, nil
}

// SuggestTargetPath suggests an output path based on source path and target format
func SuggestTargetPath(sourcePath string, targetFormat DiskFormat) string {
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	nameWithoutExt := base[:len(base)-len(ext)]

	return filepath.Join(dir, nameWithoutExt+targetFormat.Extension())
}
