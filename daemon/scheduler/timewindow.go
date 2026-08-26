// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"fmt"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

// TimeWindowManager manages time window restrictions for jobs
type TimeWindowManager struct {
	log logger.Logger
}

// NewTimeWindowManager creates a new time window manager
func NewTimeWindowManager(log logger.Logger) *TimeWindowManager {
	return &TimeWindowManager{
		log: log,
	}
}

// IsInTimeWindow checks if the current time is within any of the job's time windows
func (twm *TimeWindowManager) IsInTimeWindow(job *models.ScheduledJob) (bool, string) {
	if job.AdvancedConfig == nil || len(job.AdvancedConfig.TimeWindows) == 0 {
		return true, "" // No time windows = always allowed
	}

	now := time.Now()

	for i, window := range job.AdvancedConfig.TimeWindows {
		inWindow, err := twm.checkWindow(window, now)
		if err != nil {
			twm.log.Warn("time window check failed",
				"job", job.Name,
				"window", i,
				"error", err)
			continue
		}

		if inWindow {
			return true, ""
		}
	}

	// Not in any window
	nextWindow := twm.findNextWindow(job.AdvancedConfig.TimeWindows, now)
	if nextWindow != nil {
		return false, fmt.Sprintf("outside time window, next window: %s", nextWindow.Format(time.RFC3339))
	}

	return false, "outside all time windows"
}

// checkWindow checks if the given time is within a specific time window
func (twm *TimeWindowManager) checkWindow(window models.TimeWindow, t time.Time) (bool, error) {
	// Load timezone
	loc, err := time.LoadLocation(window.Timezone)
	if err != nil {
		return false, fmt.Errorf("invalid timezone %s: %w", window.Timezone, err)
	}

	// Convert to window timezone
	localTime := t.In(loc)

	// Check day of week
	dayMatch := false
	currentDay := localTime.Weekday().String()[:3] // Mon, Tue, Wed, etc.
	for _, day := range window.Days {
		if day == currentDay {
			dayMatch = true
			break
		}
	}

	if !dayMatch {
		return false, nil
	}

	// Parse start and end times
	startTime, err := time.Parse("15:04", window.StartTime)
	if err != nil {
		return false, fmt.Errorf("invalid start_time %s: %w", window.StartTime, err)
	}

	endTime, err := time.Parse("15:04", window.EndTime)
	if err != nil {
		return false, fmt.Errorf("invalid end_time %s: %w", window.EndTime, err)
	}

	// Create time bounds for today
	year, month, day := localTime.Date()
	start := time.Date(year, month, day, startTime.Hour(), startTime.Minute(), 0, 0, loc)
	end := time.Date(year, month, day, endTime.Hour(), endTime.Minute(), 0, 0, loc)

	// Handle overnight windows (e.g., 22:00 - 06:00)
	if end.Before(start) {
		// Check if we're after start OR before end
		if localTime.After(start) {
			return true, nil
		}
		// Adjust end to next day
		end = end.Add(24 * time.Hour)
	}

	// Check if current time is within window
	return (localTime.After(start) || localTime.Equal(start)) && localTime.Before(end), nil
}

// findNextWindow finds the next available time window
func (twm *TimeWindowManager) findNextWindow(windows []models.TimeWindow, from time.Time) *time.Time {
	var nextWindow *time.Time

	for _, window := range windows {
		next := twm.calculateNextWindowStart(window, from)
		if next != nil {
			if nextWindow == nil || next.Before(*nextWindow) {
				nextWindow = next
			}
		}
	}

	return nextWindow
}

// calculateNextWindowStart calculates when the next window starts
func (twm *TimeWindowManager) calculateNextWindowStart(window models.TimeWindow, from time.Time) *time.Time {
	// Load timezone
	loc, err := time.LoadLocation(window.Timezone)
	if err != nil {
		twm.log.Warn("invalid timezone", "timezone", window.Timezone, "error", err)
		return nil
	}

	// Convert to window timezone
	localTime := from.In(loc)

	// Parse start time
	startTime, err := time.Parse("15:04", window.StartTime)
	if err != nil {
		twm.log.Warn("invalid start_time", "start_time", window.StartTime, "error", err)
		return nil
	}

	// Try finding next window in the next 7 days
	for daysAhead := 0; daysAhead < 7; daysAhead++ {
		checkDate := localTime.Add(time.Duration(daysAhead) * 24 * time.Hour)
		dayName := checkDate.Weekday().String()[:3]

		// Check if this day is in the window
		for _, day := range window.Days {
			if day == dayName {
				year, month, day := checkDate.Date()
				windowStart := time.Date(year, month, day, startTime.Hour(), startTime.Minute(), 0, 0, loc)

				// If this is today, make sure the window hasn't passed
				if daysAhead == 0 && windowStart.Before(localTime) {
					continue
				}

				return &windowStart
			}
		}
	}

	return nil
}

// WaitForTimeWindow waits until a job is within its time window
func (twm *TimeWindowManager) WaitForTimeWindow(job *models.ScheduledJob, checkInterval time.Duration) <-chan bool {
	resultChan := make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()
		defer close(resultChan)

		for {
			inWindow, reason := twm.IsInTimeWindow(job)
			if inWindow {
				twm.log.Info("job entered time window", "job", job.Name)
				resultChan <- true
				return
			}

			twm.log.Debug("waiting for time window",
				"job", job.Name,
				"reason", reason)

			<-ticker.C
		}
	}()

	return resultChan
}

// GetTimeWindowStatus returns detailed status about time windows
func (twm *TimeWindowManager) GetTimeWindowStatus(job *models.ScheduledJob) *TimeWindowStatus {
	status := &TimeWindowStatus{
		JobID:      job.ID,
		JobName:    job.Name,
		InWindow:   false,
		Windows:    make([]WindowStatus, 0),
	}

	if job.AdvancedConfig == nil || len(job.AdvancedConfig.TimeWindows) == 0 {
		status.InWindow = true
		status.Message = "No time windows configured"
		return status
	}

	now := time.Now()

	for i, window := range job.AdvancedConfig.TimeWindows {
		ws := WindowStatus{
			Index:     i,
			StartTime: window.StartTime,
			EndTime:   window.EndTime,
			Days:      window.Days,
			Timezone:  window.Timezone,
		}

		inWindow, err := twm.checkWindow(window, now)
		if err != nil {
			ws.Error = err.Error()
		} else {
			ws.Active = inWindow
			if inWindow {
				status.InWindow = true
			}

			// Find next start time for this window
			nextStart := twm.calculateNextWindowStart(window, now)
			if nextStart != nil {
				ws.NextStart = *nextStart
			}
		}

		status.Windows = append(status.Windows, ws)
	}

	if status.InWindow {
		status.Message = "Job is within time window"
	} else {
		nextWindow := twm.findNextWindow(job.AdvancedConfig.TimeWindows, now)
		if nextWindow != nil {
			status.Message = fmt.Sprintf("Next window: %s", nextWindow.Format(time.RFC3339))
			status.NextWindowStart = *nextWindow
		} else {
			status.Message = "No upcoming time windows found"
		}
	}

	return status
}

// TimeWindowStatus contains detailed status about time windows
type TimeWindowStatus struct {
	JobID           string
	JobName         string
	InWindow        bool
	Message         string
	NextWindowStart time.Time
	Windows         []WindowStatus
}

// WindowStatus contains status for a single time window
type WindowStatus struct {
	Index     int
	StartTime string
	EndTime   string
	Days      []string
	Timezone  string
	Active    bool
	NextStart time.Time
	Error     string
}

// ValidateTimeWindow validates a time window configuration
func ValidateTimeWindow(window models.TimeWindow) error {
	// Validate timezone
	_, err := time.LoadLocation(window.Timezone)
	if err != nil {
		return fmt.Errorf("invalid timezone %s: %w", window.Timezone, err)
	}

	// Validate time format
	_, err = time.Parse("15:04", window.StartTime)
	if err != nil {
		return fmt.Errorf("invalid start_time format %s: %w", window.StartTime, err)
	}

	_, err = time.Parse("15:04", window.EndTime)
	if err != nil {
		return fmt.Errorf("invalid end_time format %s: %w", window.EndTime, err)
	}

	// Validate days
	validDays := map[string]bool{
		"Mon": true, "Tue": true, "Wed": true, "Thu": true,
		"Fri": true, "Sat": true, "Sun": true,
	}

	if len(window.Days) == 0 {
		return fmt.Errorf("at least one day must be specified")
	}

	for _, day := range window.Days {
		if !validDays[day] {
			return fmt.Errorf("invalid day: %s (must be Mon, Tue, Wed, Thu, Fri, Sat, or Sun)", day)
		}
	}

	return nil
}
