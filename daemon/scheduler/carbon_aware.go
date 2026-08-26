// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"fmt"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/carbon"
)

// CarbonAwareScheduler wraps the regular scheduler with carbon-aware logic
type CarbonAwareScheduler struct {
	baseScheduler  *Scheduler
	carbonProvider carbon.Provider
	log            logger.Logger
	config         CarbonAwareConfig
}

// CarbonAwareConfig contains configuration for carbon-aware scheduling
type CarbonAwareConfig struct {
	Enabled             bool          `json:"enabled"`
	DefaultMaxIntensity float64       `json:"default_max_intensity"` // gCO2/kWh
	DefaultMaxDelay     time.Duration `json:"default_max_delay"`     // Maximum delay
	DefaultZone         string        `json:"default_zone"`          // Default datacenter zone
	CheckInterval       time.Duration `json:"check_interval"`        // How often to check grid
	FallbackOnError     bool          `json:"fallback_on_error"`     // Run job if carbon API fails
}

// DefaultCarbonAwareConfig returns sensible defaults
func DefaultCarbonAwareConfig() CarbonAwareConfig {
	return CarbonAwareConfig{
		Enabled:             false, // Opt-in
		DefaultMaxIntensity: 200.0, // 200 gCO2/kWh (clean/moderate threshold)
		DefaultMaxDelay:     4 * time.Hour,
		DefaultZone:         carbon.ZoneUSEast,
		CheckInterval:       15 * time.Minute,
		FallbackOnError:     true, // Don't block jobs if API is down
	}
}

// NewCarbonAwareScheduler creates a carbon-aware scheduler
func NewCarbonAwareScheduler(
	baseScheduler *Scheduler,
	carbonProvider carbon.Provider,
	config CarbonAwareConfig,
	log logger.Logger,
) *CarbonAwareScheduler {
	return &CarbonAwareScheduler{
		baseScheduler:  baseScheduler,
		carbonProvider: carbonProvider,
		config:         config,
		log:            log,
	}
}

// SubmitJob submits a job with carbon-awareness check
func (s *CarbonAwareScheduler) SubmitJob(def models.JobDefinition) (string, error) {
	// Check if carbon-awareness is enabled for this job
	if !s.shouldCheckCarbon(def) {
		// No carbon check needed, submit directly
		return s.baseScheduler.executor.SubmitJob(def)
	}

	// Get carbon settings for this job
	settings := s.getCarbonSettings(def)

	// Check current grid status
	status, err := s.carbonProvider.GetGridStatus(settings.Zone, settings.MaxIntensity)
	if err != nil {
		s.log.Warn("Carbon API error",
			"error", err,
			"fallback", s.config.FallbackOnError)

		if s.config.FallbackOnError {
			// Fallback: submit job anyway
			return s.baseScheduler.executor.SubmitJob(def)
		}
		return "", fmt.Errorf("carbon check failed: %w", err)
	}

	// Log carbon status
	s.log.Info("Carbon intensity check",
		"zone", settings.Zone,
		"current_intensity", status.Current.CarbonIntensity,
		"threshold", settings.MaxIntensity,
		"optimal", status.OptimalForBackup,
		"renewable_percent", status.Current.FossilFreePercent)

	// Decide whether to submit now or delay
	if status.OptimalForBackup {
		// Grid is clean, submit job now
		s.log.Info("Grid is clean, submitting job immediately",
			"intensity", status.Current.CarbonIntensity,
			"renewable", status.Current.FossilFreePercent)

		// Attach carbon metadata to job
		def.Metadata = s.enrichWithCarbonMetadata(def.Metadata, status.Current)

		return s.baseScheduler.executor.SubmitJob(def)
	}

	// Grid is dirty, check if we should delay
	if status.NextOptimalTime != nil {
		delay := time.Until(*status.NextOptimalTime)

		if delay > 0 && delay <= settings.MaxDelay {
			// Delay is acceptable, schedule for later
			s.log.Info("Grid is dirty, delaying job",
				"current_intensity", status.Current.CarbonIntensity,
				"delay", delay,
				"next_optimal", status.NextOptimalTime)

			// Schedule the job for the optimal time
			return s.scheduleForLater(def, *status.NextOptimalTime, status.Current)
		}

		s.log.Warn("Optimal time exceeds max delay",
			"delay", delay,
			"max_delay", settings.MaxDelay,
			"submitting_anyway", true)
	}

	// No optimal time found within delay window, submit anyway
	s.log.Warn("No clean grid period found, submitting job",
		"current_intensity", status.Current.CarbonIntensity,
		"threshold", settings.MaxIntensity)

	def.Metadata = s.enrichWithCarbonMetadata(def.Metadata, status.Current)
	return s.baseScheduler.executor.SubmitJob(def)
}

// shouldCheckCarbon determines if carbon check is needed for this job
func (s *CarbonAwareScheduler) shouldCheckCarbon(def models.JobDefinition) bool {
	// Global config disabled?
	if !s.config.Enabled {
		return false
	}

	// Check job-specific carbon settings
	if def.Metadata != nil {
		if carbonAware, ok := def.Metadata["carbon_aware"].(bool); ok {
			return carbonAware
		}
	}

	// Default: don't check (opt-in model)
	return false
}

// getCarbonSettings extracts carbon settings from job definition
func (s *CarbonAwareScheduler) getCarbonSettings(def models.JobDefinition) carbon.CarbonSettings {
	settings := carbon.CarbonSettings{
		Enabled:      true,
		MaxIntensity: s.config.DefaultMaxIntensity,
		MaxDelay:     s.config.DefaultMaxDelay,
		Zone:         s.config.DefaultZone,
	}

	// Override with job-specific settings if provided
	if def.Metadata != nil {
		if maxIntensity, ok := def.Metadata["carbon_max_intensity"].(float64); ok {
			settings.MaxIntensity = maxIntensity
		}
		if maxDelay, ok := def.Metadata["carbon_max_delay"].(time.Duration); ok {
			settings.MaxDelay = maxDelay
		}
		if zone, ok := def.Metadata["carbon_zone"].(string); ok {
			settings.Zone = zone
		}
	}

	return settings
}

// enrichWithCarbonMetadata adds carbon metrics to job metadata
func (s *CarbonAwareScheduler) enrichWithCarbonMetadata(
	metadata map[string]interface{},
	intensity carbon.CarbonIntensity,
) map[string]interface{} {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	metadata["carbon_intensity_at_submit"] = intensity.CarbonIntensity
	metadata["carbon_renewable_percent"] = intensity.FossilFreePercent
	metadata["carbon_zone"] = intensity.Zone
	metadata["carbon_timestamp"] = intensity.Timestamp

	return metadata
}

// scheduleForLater schedules a job for a specific time
func (s *CarbonAwareScheduler) scheduleForLater(
	def models.JobDefinition,
	scheduledTime time.Time,
	currentIntensity carbon.CarbonIntensity,
) (string, error) {
	delay := time.Until(scheduledTime)

	s.log.Info("Scheduling job for cleaner grid",
		"delay", delay,
		"scheduled_time", scheduledTime,
		"current_intensity", currentIntensity.CarbonIntensity,
		"vm", def.VMPath)

	// Create a unique ID for this delayed job
	scheduleID := fmt.Sprintf("carbon-delayed-%d", time.Now().UnixNano())

	// Add carbon delay metadata to job definition
	if def.Metadata == nil {
		def.Metadata = make(map[string]interface{})
	}
	def.Metadata["carbon_delayed"] = true
	def.Metadata["carbon_delay_duration"] = delay.String()
	def.Metadata["carbon_optimal_time"] = scheduledTime
	def.Metadata["carbon_submit_intensity"] = currentIntensity.CarbonIntensity

	// Schedule using a goroutine with timer
	go func() {
		s.log.Info("Waiting for optimal carbon time",
			"schedule_id", scheduleID,
			"wait_duration", delay)

		time.Sleep(delay)

		s.log.Info("Optimal time reached, submitting job",
			"schedule_id", scheduleID,
			"vm", def.VMPath)

		// Check carbon intensity again before submitting
		status, err := s.carbonProvider.GetCurrentIntensity(currentIntensity.Zone)
		if err != nil {
			s.log.Error("Failed to check carbon intensity before delayed submission",
				"error", err)
		} else {
			s.log.Info("Carbon intensity at optimal time",
				"intensity", status.CarbonIntensity,
				"renewable_percent", status.FossilFreePercent)
			def.Metadata = s.enrichWithCarbonMetadata(def.Metadata, *status)
		}

		// Submit the job
		jobID, err := s.baseScheduler.executor.SubmitJob(def)
		if err != nil {
			s.log.Error("Failed to submit carbon-delayed job",
				"schedule_id", scheduleID,
				"error", err)
			return
		}

		s.log.Info("Carbon-delayed job submitted successfully",
			"schedule_id", scheduleID,
			"job_id", jobID)
	}()

	return scheduleID, nil
}

// GetCarbonStatus returns current carbon status for a zone
func (s *CarbonAwareScheduler) GetCarbonStatus(zone string, threshold float64) (*carbon.GridStatus, error) {
	if s.carbonProvider == nil {
		return nil, fmt.Errorf("carbon provider not configured")
	}

	return s.carbonProvider.GetGridStatus(zone, threshold)
}

// GenerateCarbonReport generates a carbon footprint report for a completed job
func (s *CarbonAwareScheduler) GenerateCarbonReport(
	jobID string,
	startTime, endTime time.Time,
	dataSizeGB float64,
	zone string,
) (*carbon.CarbonReport, error) {
	// Get average carbon intensity during the job
	// For now, use current intensity (in production, would track over time)
	intensity, err := s.carbonProvider.GetCurrentIntensity(zone)
	if err != nil {
		return nil, fmt.Errorf("failed to get carbon intensity: %w", err)
	}

	report := carbon.GenerateCarbonReport(
		jobID,
		startTime,
		endTime,
		dataSizeGB,
		intensity.CarbonIntensity,
		intensity.FossilFreePercent,
	)

	return report, nil
}

// EstimateCarbonSavings estimates carbon savings if job is delayed
func (s *CarbonAwareScheduler) EstimateCarbonSavings(
	zone string,
	dataSizeGB float64,
	durationHours float64,
) (map[string]interface{}, error) {
	// Get current intensity
	current, err := s.carbonProvider.GetCurrentIntensity(zone)
	if err != nil {
		return nil, err
	}

	// Get forecast
	forecast, err := s.carbonProvider.GetForecast(zone, 4)
	if err != nil {
		return nil, err
	}

	// Find best time in forecast
	bestIntensity := current.CarbonIntensity
	var bestTime *time.Time
	for _, f := range forecast {
		if f.CarbonIntensity < bestIntensity {
			bestIntensity = f.CarbonIntensity
			bestTime = &f.Time
		}
	}

	// Calculate emissions for both scenarios
	energyKWh := carbon.EstimateEnergy(dataSizeGB, durationHours)
	currentEmissions := carbon.CalculateEmissions(energyKWh, current.CarbonIntensity)
	bestEmissions := carbon.CalculateEmissions(energyKWh, bestIntensity)
	savings := currentEmissions - bestEmissions

	result := map[string]interface{}{
		"current_intensity":  current.CarbonIntensity,
		"current_emissions":  currentEmissions,
		"best_intensity":     bestIntensity,
		"best_emissions":     bestEmissions,
		"savings_kg_co2":     savings,
		"savings_percent":    (savings / currentEmissions) * 100,
		"best_time":          bestTime,
		"recommendation":     "",
	}

	if savings > 0 {
		delay := time.Until(*bestTime)
		result["recommendation"] = fmt.Sprintf(
			"Delay %.0f minutes to save %.2f kg CO2 (%.0f%% reduction)",
			delay.Minutes(),
			savings,
			(savings/currentEmissions)*100,
		)
	} else {
		result["recommendation"] = "Grid is already clean, run job now"
	}

	return result, nil
}
