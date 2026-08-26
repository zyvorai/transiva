// SPDX-License-Identifier: Apache-2.0

package carbon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ElectricityMapClient implements Provider interface using ElectricityMap API
type ElectricityMapClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewElectricityMapClient creates a new ElectricityMap client
func NewElectricityMapClient(apiKey string) *ElectricityMapClient {
	return &ElectricityMapClient{
		apiKey:  apiKey,
		baseURL: "https://api.electricitymap.org/v3",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// electricityMapResponse represents the API response
type electricityMapResponse struct {
	Zone              string    `json:"zone"`
	CarbonIntensity   float64   `json:"carbonIntensity"`
	FossilFreePercent float64   `json:"fossilFreePercentage"`
	Datetime          time.Time `json:"datetime"`
}

// GetCurrentIntensity returns current carbon intensity for a zone
func (c *ElectricityMapClient) GetCurrentIntensity(zone string) (*CarbonIntensity, error) {
	url := fmt.Sprintf("%s/carbon-intensity/latest?zone=%s", c.baseURL, zone)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("auth-token", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result electricityMapResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &CarbonIntensity{
		Zone:              result.Zone,
		CarbonIntensity:   result.CarbonIntensity,
		FossilFreePercent: result.FossilFreePercent,
		Timestamp:         result.Datetime,
		Source:            "electricitymap",
	}, nil
}

// GetForecast returns carbon intensity forecast
func (c *ElectricityMapClient) GetForecast(zone string, hours int) ([]Forecast, error) {
	url := fmt.Sprintf("%s/carbon-intensity/forecast?zone=%s", c.baseURL, zone)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("auth-token", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch forecast: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result struct {
		Zone     string `json:"zone"`
		Forecast []struct {
			Datetime        time.Time `json:"datetime"`
			CarbonIntensity float64   `json:"carbonIntensity"`
		} `json:"forecast"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to our format and limit to requested hours
	forecasts := make([]Forecast, 0, hours)
	cutoff := time.Now().Add(time.Duration(hours) * time.Hour)

	for _, f := range result.Forecast {
		if f.Datetime.After(cutoff) {
			break
		}
		forecasts = append(forecasts, Forecast{
			Time:            f.Datetime,
			CarbonIntensity: f.CarbonIntensity,
			Confidence:      0.85, // ElectricityMap doesn't provide confidence
		})
	}

	return forecasts, nil
}

// GetGridStatus returns comprehensive grid status
func (c *ElectricityMapClient) GetGridStatus(zone string, threshold float64) (*GridStatus, error) {
	// Get current intensity
	current, err := c.GetCurrentIntensity(zone)
	if err != nil {
		return nil, err
	}

	// Get forecast
	forecast, err := c.GetForecast(zone, 4)
	if err != nil {
		// Forecast is optional, continue without it
		forecast = []Forecast{}
	}

	status := &GridStatus{
		Current:          *current,
		OptimalForBackup: current.CarbonIntensity <= threshold,
		Forecast:         forecast,
	}

	// Find next optimal time if current is not optimal
	if !status.OptimalForBackup && len(forecast) > 0 {
		for _, f := range forecast {
			if f.CarbonIntensity <= threshold {
				status.NextOptimalTime = &f.Time
				break
			}
		}
	}

	// Generate reasoning
	status.Reasoning = c.generateReasoning(current, threshold, status.NextOptimalTime)

	return status, nil
}

// generateReasoning creates human-readable reasoning
func (c *ElectricityMapClient) generateReasoning(current *CarbonIntensity, threshold float64, nextOptimal *time.Time) string {
	if current.CarbonIntensity <= threshold {
		quality := "EXCELLENT"
		if current.CarbonIntensity > ThresholdExcellent {
			quality = "GOOD"
		}
		return fmt.Sprintf("%s time to run backups (%.0f gCO2/kWh, %.0f%% renewable)",
			quality, current.CarbonIntensity, current.FossilFreePercent)
	}

	if nextOptimal != nil {
		delay := time.Until(*nextOptimal)
		return fmt.Sprintf("Grid is dirty (%.0f gCO2/kWh). Consider delaying %.0f minutes for cleaner energy.",
			current.CarbonIntensity, delay.Minutes())
	}

	return fmt.Sprintf("Grid carbon intensity is HIGH (%.0f gCO2/kWh, threshold: %.0f). No cleaner periods in next 4 hours.",
		current.CarbonIntensity, threshold)
}

// ListZones returns available geographic zones
func (c *ElectricityMapClient) ListZones() ([]string, error) {
	url := fmt.Sprintf("%s/zones", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("auth-token", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch zones: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var zones map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&zones); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract zone names
	zoneList := make([]string, 0, len(zones))
	for zone := range zones {
		zoneList = append(zoneList, zone)
	}

	return zoneList, nil
}

// CalculateEmissions calculates carbon emissions for an operation
func CalculateEmissions(energyKWh float64, carbonIntensity float64) float64 {
	// Carbon emissions in kg CO2
	return (energyKWh * carbonIntensity) / 1000.0
}

// EstimateEnergy estimates energy consumption for a backup operation
func EstimateEnergy(dataSizeGB float64, durationHours float64) float64 {
	// Rough estimate: 100W base + 50W per TB being transferred
	basePowerW := 100.0
	transferPowerW := (dataSizeGB / 1000.0) * 50.0
	totalPowerW := basePowerW + transferPowerW

	// Convert to kWh
	return (totalPowerW * durationHours) / 1000.0
}

// GenerateCarbonReport creates a carbon footprint report for an operation
func GenerateCarbonReport(
	operationID string,
	startTime, endTime time.Time,
	dataSizeGB float64,
	avgCarbonIntensity float64,
	renewablePercent float64,
) *CarbonReport {
	durationHours := endTime.Sub(startTime).Hours()
	energyKWh := EstimateEnergy(dataSizeGB, durationHours)
	emissions := CalculateEmissions(energyKWh, avgCarbonIntensity)

	// Calculate savings vs worst-case (coal power at 1000 gCO2/kWh)
	worstCaseEmissions := CalculateEmissions(energyKWh, 1000.0)
	savings := worstCaseEmissions - emissions

	return &CarbonReport{
		OperationID:      operationID,
		StartTime:        startTime,
		EndTime:          endTime,
		Duration:         durationHours,
		EnergyUsed:       energyKWh,
		CarbonIntensity:  avgCarbonIntensity,
		CarbonEmissions:  emissions,
		SavingsVsWorst:   savings,
		RenewablePercent: renewablePercent,
	}
}
