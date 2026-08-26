// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pterm/pterm"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/formats"
)

const version = "1.0.0"

func main() {
	// Parse flags
	sourceFile := flag.String("source", "", "Source disk image path (required)")
	targetFile := flag.String("target", "", "Target disk image path (optional, auto-generated if not specified)")
	sourceFormat := flag.String("source-format", "", "Source format (auto-detected if not specified)")
	targetFormat := flag.String("target-format", "qcow2", "Target format (qcow2, raw, vmdk, vhd)")
	bufferSize := flag.Int("buffer-size", 4, "Buffer size in MB")
	info := flag.Bool("info", false, "Show information about a disk image")
	versionFlag := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("hyperconvert version %s\n", version)
		os.Exit(0)
	}

	if *sourceFile == "" {
		pterm.Error.Println("Source file required")
		flag.Usage()
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New("info")

	// Show info mode
	if *info {
		showInfo(*sourceFile)
		return
	}

	// Detect source format if not specified
	srcFormat := formats.ParseFormatString(*sourceFormat)
	if srcFormat == formats.FormatUnknown {
		detected, err := formats.DetectFormat(*sourceFile)
		if err != nil {
			pterm.Error.Printfln("Failed to detect source format: %v", err)
			os.Exit(1)
		}
		srcFormat = detected
		pterm.Info.Printfln("Detected source format: %s", srcFormat)
	}

	// Parse target format
	tgtFormat := formats.ParseFormatString(*targetFormat)
	if tgtFormat == formats.FormatUnknown {
		pterm.Error.Printfln("Invalid target format: %s", *targetFormat)
		os.Exit(1)
	}

	// Generate target path if not specified
	targetPath := *targetFile
	if targetPath == "" {
		targetPath = formats.SuggestTargetPath(*sourceFile, tgtFormat)
		pterm.Info.Printfln("Target path: %s", targetPath)
	}

	// Create converter
	converter := formats.NewConverter(log)

	// Setup conversion options
	opts := formats.DefaultConversionOptions()
	opts.SourceFormat = srcFormat
	opts.TargetFormat = tgtFormat
	opts.BufferSize = *bufferSize * 1024 * 1024

	// Progress bar
	var progressBar *pterm.ProgressbarPrinter
	var lastUpdate time.Time

	opts.ProgressCallback = func(progress float64, bytesProcessed int64) {
		// Update every 100ms to avoid too frequent updates
		if time.Since(lastUpdate) < 100*time.Millisecond {
			return
		}
		lastUpdate = time.Now()

		if progressBar == nil {
			progressBar, _ = pterm.DefaultProgressbar.
				WithTitle("Converting").
				WithTotal(100).
				Start()
		}

		progressBar.UpdateTitle(fmt.Sprintf("Converting (%.2f GB processed)", float64(bytesProcessed)/1024/1024/1024))
		progressBar.Current = int(progress)
	}

	// Convert
	pterm.Info.Printfln("Converting %s to %s", srcFormat, tgtFormat)
	pterm.Info.Printfln("Source: %s", *sourceFile)
	pterm.Info.Printfln("Target: %s", targetPath)

	ctx := context.Background()
	result, err := converter.Convert(ctx, *sourceFile, targetPath, opts)

	if progressBar != nil {
		if _, stopErr := progressBar.Stop(); stopErr != nil {
			log.Warn("failed to stop progress bar", "error", stopErr)
		}
	}

	if err != nil {
		pterm.Error.Printfln("Conversion failed: %v", err)
		os.Exit(1)
	}

	// Show results
	pterm.Success.Println("Conversion completed successfully!")
	if err := pterm.DefaultTable.WithHasHeader().WithData(pterm.TableData{
		{"Property", "Value"},
		{"Source Format", string(result.SourceFormat)},
		{"Target Format", string(result.TargetFormat)},
		{"Source Size", formatBytes(result.SourceSize)},
		{"Target Size", formatBytes(result.TargetSize)},
		{"Bytes Copied", formatBytes(result.BytesCopied)},
		{"Duration", result.Duration.String()},
		{"Speed", formatBytes(int64(float64(result.BytesCopied)/result.Duration.Seconds())) + "/s"},
	}).Render(); err != nil {
		log.Warn("failed to render results table", "error", err)
	}

	if result.SourceSize > 0 {
		compressionRatio := float64(result.TargetSize) / float64(result.SourceSize) * 100
		pterm.Info.Printfln("Compression ratio: %.2f%%", compressionRatio)
	}
}

func showInfo(path string) {
	info, err := formats.GetFormatInfo(path)
	if err != nil {
		pterm.Error.Printfln("Failed to get format info: %v", err)
		os.Exit(1)
	}

	pterm.DefaultHeader.Println("Disk Image Information")
	if err := pterm.DefaultTable.WithHasHeader().WithData(pterm.TableData{
		{"Property", "Value"},
		{"Format", string(info.Format)},
		{"File Size", formatBytes(info.Size)},
		{"Virtual Size", formatBytes(info.VirtualSize)},
		{"Compressed", fmt.Sprintf("%v", info.Compressed)},
	}).Render(); err != nil {
		pterm.Warning.Printfln("failed to render info table: %v", err)
	}

	if len(info.Metadata) > 0 {
		pterm.Println()
		pterm.DefaultHeader.Println("Metadata")
		for key, value := range info.Metadata {
			pterm.Printfln("  %s: %v", key, value)
		}
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
