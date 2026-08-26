// SPDX-License-Identifier: Apache-2.0

package carbon

import "time"

// CarbonIntensity represents carbon intensity data from electricity grid
type CarbonIntensity struct {
	Zone              string    `json:"zone"`
	CarbonIntensity   float64   `json:"carbonIntensity"`   // gCO2/kWh
	FossilFreePercent float64   `json:"fossilFreePercent"` // 0-100
	Timestamp         time.Time `json:"timestamp"`
	Source            string    `json:"source"` // electricitymap, wattime, etc.
}

// Forecast represents predicted carbon intensity
type Forecast struct {
	Time            time.Time `json:"time"`
	CarbonIntensity float64   `json:"carbonIntensity"`
	Confidence      float64   `json:"confidence"` // 0-1
}

// CarbonSettings represents user configuration for carbon-aware operations
type CarbonSettings struct {
	Enabled          bool          `json:"enabled"`
	MaxIntensity     float64       `json:"max_intensity"`     // gCO2/kWh threshold
	MaxDelay         time.Duration `json:"max_delay"`         // Maximum delay allowed
	Zone             string        `json:"zone"`              // Geographic zone
	PreferRenewables bool          `json:"prefer_renewables"` // Prefer renewable energy times
}

// GridStatus represents the current status of the electricity grid
type GridStatus struct {
	Current          CarbonIntensity `json:"current"`
	OptimalForBackup bool            `json:"optimal_for_backup"`
	Forecast         []Forecast      `json:"forecast_next_4h"`
	NextOptimalTime  *time.Time      `json:"next_optimal_time,omitempty"`
	Reasoning        string          `json:"reasoning"`
}

// CarbonReport represents carbon footprint of an operation
type CarbonReport struct {
	OperationID      string    `json:"operation_id"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	Duration         float64   `json:"duration_hours"`
	EnergyUsed       float64   `json:"energy_kwh"`          // kWh consumed
	CarbonIntensity  float64   `json:"carbon_intensity"`    // Average gCO2/kWh
	CarbonEmissions  float64   `json:"carbon_emissions_kg"` // Total kg CO2
	SavingsVsWorst   float64   `json:"savings_vs_worst_kg"` // vs running at worst time
	RenewablePercent float64   `json:"renewable_percent"`
}

// Provider defines interface for carbon intensity providers
type Provider interface {
	// GetCurrentIntensity returns current carbon intensity for a zone
	GetCurrentIntensity(zone string) (*CarbonIntensity, error)

	// GetForecast returns carbon intensity forecast for next N hours
	GetForecast(zone string, hours int) ([]Forecast, error)

	// GetGridStatus returns comprehensive grid status
	GetGridStatus(zone string, threshold float64) (*GridStatus, error)

	// ListZones returns available geographic zones
	ListZones() ([]string, error)
}

// Zones for common data centers
const (
	ZoneUSEast      = "US-CAL-CISO"      // California
	ZoneUSWest      = "US-NW-PACW"       // Pacific Northwest
	ZoneUSCentral   = "US-MISO"          // Midwest
	ZoneEUCentral   = "DE"               // Germany
	ZoneEUWest      = "GB"               // UK
	ZoneEUNorth     = "SE"               // Sweden (very clean)
	ZoneAPACSing    = "SG"               // Singapore
	ZoneAPACTokyo   = "JP-TK"            // Tokyo
	ZoneAPACSydney  = "AUS-NSW"          // Sydney
	ZoneAPACIndia   = "IN-NO"            // North India
	ZoneChinaBeijing = "CN-BJ"           // Beijing
	ZoneChinaShanghai = "CN-SH"          // Shanghai
)

// Thresholds for carbon intensity (gCO2/kWh)
const (
	ThresholdExcellent = 100.0 // < 100: Very clean (renewables)
	ThresholdGood      = 200.0 // 100-200: Clean
	ThresholdModerate  = 400.0 // 200-400: Moderate
	ThresholdPoor      = 600.0 // 400-600: High carbon
	// > 600: Very high carbon (coal)
)
