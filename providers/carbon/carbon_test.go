// SPDX-License-Identifier: Apache-2.0

package carbon

import (
	"testing"
	"time"
)

func TestMockProvider(t *testing.T) {
	provider := NewMockProvider()

	// Test GetCurrentIntensity
	t.Run("GetCurrentIntensity", func(t *testing.T) {
		intensity, err := provider.GetCurrentIntensity(ZoneUSEast)
		if err != nil {
			t.Fatalf("GetCurrentIntensity failed: %v", err)
		}

		if intensity.Zone != ZoneUSEast {
			t.Errorf("Expected zone %s, got %s", ZoneUSEast, intensity.Zone)
		}

		if intensity.CarbonIntensity <= 0 {
			t.Errorf("Invalid carbon intensity: %f", intensity.CarbonIntensity)
		}

		if intensity.Source != "mock" {
			t.Errorf("Expected source 'mock', got '%s'", intensity.Source)
		}
	})

	// Test GetForecast
	t.Run("GetForecast", func(t *testing.T) {
		now := time.Now()
		forecast, err := provider.GetForecast(ZoneUSEast, 4)
		if err != nil {
			t.Fatalf("GetForecast failed: %v", err)
		}

		if len(forecast) != 4 {
			t.Errorf("Expected 4 forecast entries, got %d", len(forecast))
		}

		// Check that forecast times are at or after the time we started the call
		for i, f := range forecast {
			if f.Time.Before(now.Add(-1 * time.Second)) {
				t.Errorf("Forecast[%d] time is too far in the past: %v (now: %v)", i, f.Time, now)
			}

			if f.CarbonIntensity <= 0 {
				t.Errorf("Forecast[%d] has invalid intensity: %f", i, f.CarbonIntensity)
			}
		}
	})

	// Test GetGridStatus
	t.Run("GetGridStatus", func(t *testing.T) {
		status, err := provider.GetGridStatus(ZoneUSEast, 200.0)
		if err != nil {
			t.Fatalf("GetGridStatus failed: %v", err)
		}

		if status.Current.Zone != ZoneUSEast {
			t.Errorf("Expected zone %s, got %s", ZoneUSEast, status.Current.Zone)
		}

		if status.Reasoning == "" {
			t.Error("Expected non-empty reasoning")
		}
	})

	// Test ListZones
	t.Run("ListZones", func(t *testing.T) {
		zones, err := provider.ListZones()
		if err != nil {
			t.Fatalf("ListZones failed: %v", err)
		}

		if len(zones) == 0 {
			t.Error("Expected non-empty zone list")
		}
	})

	// Test invalid zone
	t.Run("InvalidZone", func(t *testing.T) {
		_, err := provider.GetCurrentIntensity("INVALID-ZONE")
		if err == nil {
			t.Error("Expected error for invalid zone")
		}
	})
}

func TestCalculateEmissions(t *testing.T) {
	tests := []struct {
		name            string
		energyKWh       float64
		carbonIntensity float64
		expectedKgCO2   float64
	}{
		{
			name:            "clean energy",
			energyKWh:       10.0,
			carbonIntensity: 100.0, // 100 gCO2/kWh
			expectedKgCO2:   1.0,   // 10 * 100 / 1000
		},
		{
			name:            "dirty energy",
			energyKWh:       10.0,
			carbonIntensity: 1000.0, // 1000 gCO2/kWh (coal)
			expectedKgCO2:   10.0,   // 10 * 1000 / 1000
		},
		{
			name:            "zero energy",
			energyKWh:       0.0,
			carbonIntensity: 500.0,
			expectedKgCO2:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateEmissions(tt.energyKWh, tt.carbonIntensity)
			if result != tt.expectedKgCO2 {
				t.Errorf("Expected %.2f kg CO2, got %.2f", tt.expectedKgCO2, result)
			}
		})
	}
}

func TestEstimateEnergy(t *testing.T) {
	tests := []struct {
		name          string
		dataSizeGB    float64
		durationHours float64
		minExpected   float64 // kWh
		maxExpected   float64 // kWh
	}{
		{
			name:          "small backup",
			dataSizeGB:    100.0,  // 100 GB
			durationHours: 1.0,    // 1 hour
			minExpected:   0.08,   // At least 80 Wh
			maxExpected:   0.15,   // At most 150 Wh
		},
		{
			name:          "large backup",
			dataSizeGB:    1000.0, // 1 TB
			durationHours: 4.0,    // 4 hours
			minExpected:   0.5,    // At least 500 Wh
			maxExpected:   0.8,    // At most 800 Wh
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateEnergy(tt.dataSizeGB, tt.durationHours)
			if result < tt.minExpected || result > tt.maxExpected {
				t.Errorf("Expected energy between %.3f and %.3f kWh, got %.3f",
					tt.minExpected, tt.maxExpected, result)
			}
		})
	}
}

func TestGenerateCarbonReport(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(2 * time.Hour)

	report := GenerateCarbonReport(
		"job-123",
		startTime,
		endTime,
		500.0,  // 500 GB
		200.0,  // 200 gCO2/kWh
		65.0,   // 65% renewable
	)

	if report.OperationID != "job-123" {
		t.Errorf("Expected operation ID 'job-123', got '%s'", report.OperationID)
	}

	if report.Duration <= 0 {
		t.Errorf("Expected positive duration, got %f", report.Duration)
	}

	if report.EnergyUsed <= 0 {
		t.Errorf("Expected positive energy usage, got %f", report.EnergyUsed)
	}

	if report.CarbonEmissions <= 0 {
		t.Errorf("Expected positive emissions, got %f", report.CarbonEmissions)
	}

	if report.SavingsVsWorst <= 0 {
		t.Errorf("Expected positive savings, got %f", report.SavingsVsWorst)
	}

	if report.RenewablePercent != 65.0 {
		t.Errorf("Expected 65%% renewable, got %f%%", report.RenewablePercent)
	}
}

func TestCarbonIntensityThresholds(t *testing.T) {
	tests := []struct {
		intensity float64
		quality   string
	}{
		{50.0, "excellent"},
		{150.0, "good"},
		{300.0, "moderate"},
		{500.0, "poor"},
		{800.0, "very poor"},
	}

	for _, tt := range tests {
		t.Run(tt.quality, func(t *testing.T) {
			var actual string
			switch {
			case tt.intensity < ThresholdExcellent:
				actual = "excellent"
			case tt.intensity < ThresholdGood:
				actual = "good"
			case tt.intensity < ThresholdModerate:
				actual = "moderate"
			case tt.intensity < ThresholdPoor:
				actual = "poor"
			default:
				actual = "very poor"
			}

			if actual != tt.quality {
				t.Errorf("Intensity %.0f: expected '%s', got '%s'",
					tt.intensity, tt.quality, actual)
			}
		})
	}
}
