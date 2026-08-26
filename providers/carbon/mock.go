// SPDX-License-Identifier: Apache-2.0

package carbon

import (
	"fmt"
	"math/rand"
	"time"
)

// MockProvider implements Provider interface for testing
type MockProvider struct {
	zones               []string
	baseIntensity       float64
	fossilFreePercent   float64
	variancePercent     float64 // How much intensity varies
	simulateRenewables  bool    // Simulate renewable energy peaks
}

// NewMockProvider creates a mock carbon provider for testing
func NewMockProvider() *MockProvider {
	return &MockProvider{
		zones: []string{
			ZoneUSEast, ZoneUSWest, ZoneEUCentral,
			ZoneEUNorth, ZoneAPACSing, ZoneAPACTokyo,
		},
		baseIntensity:      250.0, // Moderate baseline
		fossilFreePercent:  45.0,
		variancePercent:    30.0,
		simulateRenewables: true,
	}
}

// GetCurrentIntensity returns simulated current carbon intensity
func (m *MockProvider) GetCurrentIntensity(zone string) (*CarbonIntensity, error) {
	if !m.isValidZone(zone) {
		return nil, fmt.Errorf("unknown zone: %s", zone)
	}

	intensity := m.calculateIntensity(time.Now())

	return &CarbonIntensity{
		Zone:              zone,
		CarbonIntensity:   intensity,
		FossilFreePercent: m.fossilFreePercent,
		Timestamp:         time.Now(),
		Source:            "mock",
	}, nil
}

// GetForecast returns simulated forecast
func (m *MockProvider) GetForecast(zone string, hours int) ([]Forecast, error) {
	if !m.isValidZone(zone) {
		return nil, fmt.Errorf("unknown zone: %s", zone)
	}

	forecasts := make([]Forecast, hours)
	now := time.Now()

	for i := 0; i < hours; i++ {
		forecastTime := now.Add(time.Duration(i) * time.Hour)
		intensity := m.calculateIntensity(forecastTime)

		forecasts[i] = Forecast{
			Time:            forecastTime,
			CarbonIntensity: intensity,
			Confidence:      0.90 - (float64(i) * 0.05), // Decreasing confidence
		}
	}

	return forecasts, nil
}

// GetGridStatus returns simulated grid status
func (m *MockProvider) GetGridStatus(zone string, threshold float64) (*GridStatus, error) {
	current, err := m.GetCurrentIntensity(zone)
	if err != nil {
		return nil, err
	}

	forecast, err := m.GetForecast(zone, 4)
	if err != nil {
		return nil, err
	}

	status := &GridStatus{
		Current:          *current,
		OptimalForBackup: current.CarbonIntensity <= threshold,
		Forecast:         forecast,
	}

	// Find next optimal time
	if !status.OptimalForBackup {
		for _, f := range forecast {
			if f.CarbonIntensity <= threshold {
				status.NextOptimalTime = &f.Time
				break
			}
		}
	}

	status.Reasoning = m.generateReasoning(current, threshold, status.NextOptimalTime)

	return status, nil
}

// ListZones returns available zones
func (m *MockProvider) ListZones() ([]string, error) {
	return m.zones, nil
}

// calculateIntensity simulates carbon intensity based on time of day
func (m *MockProvider) calculateIntensity(t time.Time) float64 {
	hour := t.Hour()

	// Base intensity
	intensity := m.baseIntensity

	// Simulate daily pattern
	if m.simulateRenewables {
		// Solar peaks during day (10am-4pm)
		if hour >= 10 && hour <= 16 {
			// More renewable energy during sunny hours
			intensity *= 0.6 // 40% reduction
		} else if hour >= 0 && hour <= 5 {
			// Wind often peaks at night
			intensity *= 0.75 // 25% reduction
		} else {
			// Peak demand hours (evening)
			intensity *= 1.3 // 30% increase
		}
	}

	// Add some random variance
	// #nosec G404 -- non-cryptographic randomness used only to simulate test/mock carbon-intensity data, not for any security-sensitive purpose
	variance := (rand.Float64()*2 - 1) * (m.variancePercent / 100.0)
	intensity *= (1.0 + variance)

	// Ensure it stays positive
	if intensity < 0 {
		intensity = 50.0
	}

	return intensity
}

// generateReasoning creates human-readable reasoning
func (m *MockProvider) generateReasoning(current *CarbonIntensity, threshold float64, nextOptimal *time.Time) string {
	if current.CarbonIntensity <= threshold {
		return fmt.Sprintf("GOOD time to run backups (%.0f gCO2/kWh, %.0f%% renewable)",
			current.CarbonIntensity, current.FossilFreePercent)
	}

	if nextOptimal != nil {
		delay := time.Until(*nextOptimal)
		return fmt.Sprintf("Consider delaying %.0f minutes for cleaner energy (current: %.0f gCO2/kWh)",
			delay.Minutes(), current.CarbonIntensity)
	}

	return fmt.Sprintf("Grid carbon intensity is HIGH (%.0f gCO2/kWh). No cleaner periods in next 4 hours.",
		current.CarbonIntensity)
}

// isValidZone checks if zone is valid
func (m *MockProvider) isValidZone(zone string) bool {
	for _, z := range m.zones {
		if z == zone {
			return true
		}
	}
	return false
}
