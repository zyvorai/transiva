// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/zyvorai/transiva/daemon/scheduler"
	"github.com/zyvorai/transiva/providers/carbon"
)

// CarbonHandlers provides HTTP handlers for carbon-aware features
type CarbonHandlers struct {
	carbonScheduler *scheduler.CarbonAwareScheduler
}

// NewCarbonHandlers creates carbon API handlers
func NewCarbonHandlers(carbonScheduler *scheduler.CarbonAwareScheduler) *CarbonHandlers {
	return &CarbonHandlers{
		carbonScheduler: carbonScheduler,
	}
}

// CarbonStatusRequest represents the request for carbon status
type CarbonStatusRequest struct {
	Zone      string  `json:"zone"`                // Geographic zone (e.g., "US-CAL-CISO")
	Threshold float64 `json:"threshold,omitempty"` // Carbon intensity threshold (gCO2/kWh)
}

// CarbonStatusResponse represents the grid carbon status response
type CarbonStatusResponse struct {
	Zone             string            `json:"zone"`
	CurrentIntensity float64           `json:"current_intensity"` // gCO2/kWh
	RenewablePercent float64           `json:"renewable_percent"` // 0-100
	OptimalForBackup bool              `json:"optimal_for_backup"`
	NextOptimalTime  *time.Time        `json:"next_optimal_time,omitempty"`
	Forecast         []carbon.Forecast `json:"forecast_next_4h"`
	Reasoning        string            `json:"reasoning"`
	Timestamp        time.Time         `json:"timestamp"`
	Quality          string            `json:"quality"` // excellent, good, moderate, poor, very poor
}

// HandleCarbonStatus returns current carbon status for a zone
// POST /carbon/status
func (h *CarbonHandlers) HandleCarbonStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CarbonStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate zone
	if req.Zone == "" {
		http.Error(w, "zone is required", http.StatusBadRequest)
		return
	}

	// Default threshold if not provided
	if req.Threshold == 0 {
		req.Threshold = 200.0 // Good/moderate threshold
	}

	// Get grid status
	status, err := h.carbonScheduler.GetCarbonStatus(req.Zone, req.Threshold)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Determine quality level
	quality := determineQuality(status.Current.CarbonIntensity)

	// Build response
	response := CarbonStatusResponse{
		Zone:             status.Current.Zone,
		CurrentIntensity: status.Current.CarbonIntensity,
		RenewablePercent: status.Current.FossilFreePercent,
		OptimalForBackup: status.OptimalForBackup,
		NextOptimalTime:  status.NextOptimalTime,
		Forecast:         status.Forecast,
		Reasoning:        status.Reasoning,
		Timestamp:        status.Current.Timestamp,
		Quality:          quality,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("carbon handler: failed to encode response: %v", err)
	}
}

// CarbonReportRequest represents the request for a carbon report
type CarbonReportRequest struct {
	JobID      string    `json:"job_id"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	DataSizeGB float64   `json:"data_size_gb"`
	Zone       string    `json:"zone"`
}

// CarbonReportResponse represents the carbon footprint report
type CarbonReportResponse struct {
	OperationID      string    `json:"operation_id"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	Duration         float64   `json:"duration_hours"`
	DataSize         float64   `json:"data_size_gb"`
	EnergyUsed       float64   `json:"energy_kwh"`
	CarbonIntensity  float64   `json:"carbon_intensity_gco2_kwh"`
	CarbonEmissions  float64   `json:"carbon_emissions_kg_co2"`
	SavingsVsWorst   float64   `json:"savings_vs_worst_kg_co2"`
	RenewablePercent float64   `json:"renewable_percent"`
	Equivalent       string    `json:"equivalent"` // Human-readable equivalent
}

// HandleCarbonReport generates a carbon footprint report for a job
// POST /carbon/report
func (h *CarbonHandlers) HandleCarbonReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CarbonReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.JobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}
	if req.DataSizeGB <= 0 {
		http.Error(w, "data_size_gb must be positive", http.StatusBadRequest)
		return
	}
	if req.Zone == "" {
		http.Error(w, "zone is required", http.StatusBadRequest)
		return
	}

	// Generate carbon report
	report, err := h.carbonScheduler.GenerateCarbonReport(
		req.JobID,
		req.StartTime,
		req.EndTime,
		req.DataSizeGB,
		req.Zone,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response with human-readable equivalent
	equivalent := generateEquivalent(report.CarbonEmissions)

	response := CarbonReportResponse{
		OperationID:      report.OperationID,
		StartTime:        report.StartTime,
		EndTime:          report.EndTime,
		Duration:         report.Duration,
		DataSize:         req.DataSizeGB,
		EnergyUsed:       report.EnergyUsed,
		CarbonIntensity:  report.CarbonIntensity,
		CarbonEmissions:  report.CarbonEmissions,
		SavingsVsWorst:   report.SavingsVsWorst,
		RenewablePercent: report.RenewablePercent,
		Equivalent:       equivalent,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("carbon handler: failed to encode response: %v", err)
	}
}

// CarbonZonesResponse represents available carbon zones
type CarbonZonesResponse struct {
	Zones []ZoneInfo `json:"zones"`
	Total int        `json:"total"`
}

// ZoneInfo represents information about a carbon zone
type ZoneInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Region      string  `json:"region"`
	Description string  `json:"description"`
	Typical     float64 `json:"typical_intensity"` // Typical carbon intensity
}

// HandleCarbonZones returns available carbon zones
// GET /carbon/zones
func (h *CarbonHandlers) HandleCarbonZones(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Define available zones with metadata
	zones := []ZoneInfo{
		{
			ID:          carbon.ZoneUSEast,
			Name:        "US California (CISO)",
			Region:      "North America",
			Description: "California Independent System Operator",
			Typical:     200.0,
		},
		{
			ID:          carbon.ZoneUSWest,
			Name:        "US Pacific Northwest",
			Region:      "North America",
			Description: "Pacific Northwest region (WA, OR)",
			Typical:     150.0,
		},
		{
			ID:          carbon.ZoneUSCentral,
			Name:        "US Midwest (MISO)",
			Region:      "North America",
			Description: "Midcontinent Independent System Operator",
			Typical:     450.0,
		},
		{
			ID:          carbon.ZoneEUCentral,
			Name:        "Germany",
			Region:      "Europe",
			Description: "German electricity grid",
			Typical:     350.0,
		},
		{
			ID:          carbon.ZoneEUWest,
			Name:        "United Kingdom",
			Region:      "Europe",
			Description: "UK National Grid",
			Typical:     250.0,
		},
		{
			ID:          carbon.ZoneEUNorth,
			Name:        "Sweden",
			Region:      "Europe",
			Description: "Swedish electricity grid (very clean)",
			Typical:     50.0,
		},
		{
			ID:          carbon.ZoneAPACSing,
			Name:        "Singapore",
			Region:      "Asia Pacific",
			Description: "Singapore electricity grid",
			Typical:     400.0,
		},
		{
			ID:          carbon.ZoneAPACTokyo,
			Name:        "Tokyo, Japan",
			Region:      "Asia Pacific",
			Description: "Tokyo Electric Power Company area",
			Typical:     500.0,
		},
		{
			ID:          carbon.ZoneAPACSydney,
			Name:        "Sydney, Australia",
			Region:      "Asia Pacific",
			Description: "New South Wales grid",
			Typical:     700.0,
		},
		{
			ID:          carbon.ZoneAPACIndia,
			Name:        "North India",
			Region:      "Asia Pacific",
			Description: "Northern India grid",
			Typical:     650.0,
		},
		{
			ID:          carbon.ZoneChinaBeijing,
			Name:        "Beijing, China",
			Region:      "Asia Pacific",
			Description: "Beijing electricity grid",
			Typical:     800.0,
		},
		{
			ID:          carbon.ZoneChinaShanghai,
			Name:        "Shanghai, China",
			Region:      "Asia Pacific",
			Description: "Shanghai electricity grid",
			Typical:     750.0,
		},
	}

	response := CarbonZonesResponse{
		Zones: zones,
		Total: len(zones),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("carbon handler: failed to encode response: %v", err)
	}
}

// CarbonEstimateRequest represents a carbon savings estimate request
type CarbonEstimateRequest struct {
	Zone          string  `json:"zone"`
	DataSizeGB    float64 `json:"data_size_gb"`
	DurationHours float64 `json:"duration_hours"`
}

// CarbonEstimateResponse represents estimated carbon savings
type CarbonEstimateResponse struct {
	CurrentIntensity float64    `json:"current_intensity_gco2_kwh"`
	CurrentEmissions float64    `json:"current_emissions_kg_co2"`
	BestIntensity    float64    `json:"best_intensity_gco2_kwh"`
	BestEmissions    float64    `json:"best_emissions_kg_co2"`
	BestTime         *time.Time `json:"best_time,omitempty"`
	SavingsKgCO2     float64    `json:"savings_kg_co2"`
	SavingsPercent   float64    `json:"savings_percent"`
	Recommendation   string     `json:"recommendation"`
	DelayMinutes     float64    `json:"delay_minutes,omitempty"`
	Forecast         []Forecast `json:"forecast"`
}

// Forecast represents a simplified forecast entry
type Forecast struct {
	Time      time.Time `json:"time"`
	Intensity float64   `json:"intensity_gco2_kwh"`
	Quality   string    `json:"quality"`
}

// HandleCarbonEstimate estimates carbon savings from delaying a job
// POST /carbon/estimate
func (h *CarbonHandlers) HandleCarbonEstimate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CarbonEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate
	if req.Zone == "" {
		http.Error(w, "zone is required", http.StatusBadRequest)
		return
	}
	if req.DataSizeGB <= 0 {
		http.Error(w, "data_size_gb must be positive", http.StatusBadRequest)
		return
	}
	if req.DurationHours <= 0 {
		http.Error(w, "duration_hours must be positive", http.StatusBadRequest)
		return
	}

	// Get estimate
	estimate, err := h.carbonScheduler.EstimateCarbonSavings(
		req.Zone,
		req.DataSizeGB,
		req.DurationHours,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get forecast for visualization
	status, err := h.carbonScheduler.GetCarbonStatus(req.Zone, 200.0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert forecast to simplified format
	forecastData := make([]Forecast, len(status.Forecast))
	for i, f := range status.Forecast {
		forecastData[i] = Forecast{
			Time:      f.Time,
			Intensity: f.CarbonIntensity,
			Quality:   determineQuality(f.CarbonIntensity),
		}
	}

	// Build response
	response := CarbonEstimateResponse{
		CurrentIntensity: estimate["current_intensity"].(float64),
		CurrentEmissions: estimate["current_emissions"].(float64),
		BestIntensity:    estimate["best_intensity"].(float64),
		BestEmissions:    estimate["best_emissions"].(float64),
		SavingsKgCO2:     estimate["savings_kg_co2"].(float64),
		SavingsPercent:   estimate["savings_percent"].(float64),
		Recommendation:   estimate["recommendation"].(string),
		Forecast:         forecastData,
	}

	// Add best time and delay if available
	if bestTime, ok := estimate["best_time"].(*time.Time); ok && bestTime != nil {
		response.BestTime = bestTime
		response.DelayMinutes = time.Until(*bestTime).Minutes()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("carbon handler: failed to encode response: %v", err)
	}
}

// determineQuality converts carbon intensity to quality level
func determineQuality(intensity float64) string {
	switch {
	case intensity < carbon.ThresholdExcellent:
		return "excellent"
	case intensity < carbon.ThresholdGood:
		return "good"
	case intensity < carbon.ThresholdModerate:
		return "moderate"
	case intensity < carbon.ThresholdPoor:
		return "poor"
	default:
		return "very poor"
	}
}

// generateEquivalent creates human-readable carbon equivalent
func generateEquivalent(kgCO2 float64) string {
	// 1 kg CO2 ≈ 4 km of driving (average car)
	kmDriving := kgCO2 * 4.0

	if kmDriving < 1 {
		return "Less than 1 km of driving"
	} else if kmDriving < 100 {
		return formatFloat(kmDriving) + " km of driving"
	} else {
		// Trees needed to offset (1 tree absorbs ~20 kg CO2/year)
		trees := kgCO2 / 20.0
		return formatFloat(kmDriving) + " km of driving, or " + formatFloat(trees) + " tree-years to offset"
	}
}

// formatFloat formats a float to 1 decimal place
func formatFloat(f float64) string {
	if f < 10 {
		return formatFloatPrecision(f, 1)
	}
	return formatFloatPrecision(f, 0)
}

// formatFloatPrecision formats float with specific precision
func formatFloatPrecision(f float64, prec int) string {
	// Use standard library for reliable formatting
	switch prec {
	case 0:
		return formatInt(int(f + 0.5)) // Round to nearest integer
	case 1:
		whole := int(f)
		decimal := int((f-float64(whole))*10+0.5) % 10
		return formatInt(whole) + "." + formatInt(decimal)
	default:
		return formatInt(int(f))
	}
}

// formatInt converts an integer to string
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + formatInt(-n)
	}

	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
