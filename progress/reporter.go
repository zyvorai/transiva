// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"time"

	"github.com/schollz/progressbar/v3"
)

type ProgressReporter interface {
	Start(total int64, description string)
	Update(current int64)
	Finish()
	SetTotal(total int64)
	Add(count int64)
	Close() error
	Describe(description string)
}

type BarProgress struct {
	bar *progressbar.ProgressBar
}

func NewBarProgress(writer io.Writer, options ...progressbar.Option) *BarProgress {
	defaultOptions := []progressbar.Option{
		progressbar.OptionSetWriter(writer),
		progressbar.OptionSetWidth(50),
		progressbar.OptionThrottle(65 * time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("bytes"),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(writer, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	}

	// Apply custom options
	allOptions := append(defaultOptions, options...)

	return &BarProgress{
		bar: progressbar.NewOptions64(0, allOptions...),
	}
}

func (b *BarProgress) Start(total int64, description string) {
	if b == nil || b.bar == nil {
		return
	}
	b.bar.ChangeMax64(total)
	b.bar.Describe(description)
	b.bar.Reset()
}

func (b *BarProgress) Update(current int64) {
	if b == nil || b.bar == nil {
		return
	}
	_ = b.bar.Set64(current)
}

func (b *BarProgress) Add(count int64) {
	if b == nil || b.bar == nil {
		return
	}
	_ = b.bar.Add64(count)
}

func (b *BarProgress) Finish() {
	if b == nil || b.bar == nil {
		return
	}
	_ = b.bar.Finish()
}

func (b *BarProgress) SetTotal(total int64) {
	if b == nil || b.bar == nil {
		return
	}
	b.bar.ChangeMax64(total)
}

func (b *BarProgress) Describe(description string) {
	if b == nil || b.bar == nil {
		return
	}
	b.bar.Describe(description)
}

func (b *BarProgress) Close() error {
	if b == nil || b.bar == nil {
		return nil
	}
	return b.bar.Close()
}

// NewDownloadProgress creates a progress bar optimized for file downloads
func NewDownloadProgress(writer io.Writer, filename string, totalSize int64) *BarProgress {
	bar := progressbar.NewOptions64(totalSize,
		progressbar.OptionSetWriter(writer),
		progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s:", filename)),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetElapsedTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(writer, "\n")
		}),
	)
	return &BarProgress{bar: bar}
}

// NewOverallProgress creates a progress bar for overall export progress
func NewOverallProgress(writer io.Writer, vmName string, fileCount int) *BarProgress {
	return NewBarProgress(writer,
		progressbar.OptionSetDescription(fmt.Sprintf("Exporting %s:", vmName)),
		progressbar.OptionSetItsString("files"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowIts(),
		progressbar.OptionSetElapsedTime(true),
		progressbar.OptionShowCount(),
	)
}

// NewMultiProgress creates a manager for multiple progress bars
type MultiProgress struct {
	bars []*BarProgress
	done chan struct{}
}

func NewMultiProgress() *MultiProgress {
	return &MultiProgress{
		bars: make([]*BarProgress, 0),
		done: make(chan struct{}),
	}
}

func (m *MultiProgress) AddBar(bar *BarProgress) {
	m.bars = append(m.bars, bar)
}

func (m *MultiProgress) Wait() {
	<-m.done
}

func (m *MultiProgress) Close() {
	for _, bar := range m.bars {
		_ = bar.Close()
	}
	close(m.done)
}
