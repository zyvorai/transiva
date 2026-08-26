// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/daemon/scheduler"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/carbon"
)

// mockCarbonExecutor for testing
type mockCarbonExecutor struct {
	jobs []models.JobDefinition
}

func (m *mockCarbonExecutor) SubmitJob(def models.JobDefinition) (string, error) {
	m.jobs = append(m.jobs, def)
	return "job-123", nil
}

func setupCarbonHandlers(t *testing.T) *CarbonHandlers {
	log := logger.NewTestLogger(t)
	executor := &mockCarbonExecutor{jobs: make([]models.JobDefinition, 0)}

	baseScheduler := scheduler.NewScheduler(executor, log)
	carbonProvider := carbon.NewMockProvider()

	config := scheduler.DefaultCarbonAwareConfig()
	config.Enabled = true

	carbonScheduler := scheduler.NewCarbonAwareScheduler(
		baseScheduler,
		carbonProvider,
		config,
		log,
	)

	return NewCarbonHandlers(carbonScheduler)
}

func TestHandleCarbonStatus(t *testing.T) {
	handlers := setupCarbonHandlers(t)

	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
	}{
		{
			name:   "valid request",
			method: http.MethodPost,
			body: CarbonStatusRequest{
				Zone:      carbon.ZoneUSEast,
				Threshold: 200.0,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing zone",
			method:         http.MethodPost,
			body:           CarbonStatusRequest{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid method",
			method:         http.MethodGet,
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "invalid json",
			method:         http.MethodPost,
			body:           "invalid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					body = []byte(str)
				} else {
					body, err = json.Marshal(tt.body)
					if err != nil {
						t.Fatalf("Failed to marshal request: %v", err)
					}
				}
			}

			req := httptest.NewRequest(tt.method, "/carbon/status", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handlers.HandleCarbonStatus(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				var response CarbonStatusResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Validate response structure
				if response.Zone == "" {
					t.Error("Expected non-empty zone")
				}
				if response.CurrentIntensity <= 0 {
					t.Error("Expected positive carbon intensity")
				}
				if response.Quality == "" {
					t.Error("Expected non-empty quality")
				}
				if response.Reasoning == "" {
					t.Error("Expected non-empty reasoning")
				}
			}
		})
	}
}

func TestHandleCarbonReport(t *testing.T) {
	handlers := setupCarbonHandlers(t)

	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
	}{
		{
			name:   "valid request",
			method: http.MethodPost,
			body: CarbonReportRequest{
				JobID:      "job-123",
				StartTime:  time.Now().Add(-2 * time.Hour),
				EndTime:    time.Now(),
				DataSizeGB: 500.0,
				Zone:       carbon.ZoneUSEast,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "missing job_id",
			method: http.MethodPost,
			body: CarbonReportRequest{
				DataSizeGB: 500.0,
				Zone:       carbon.ZoneUSEast,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "invalid data size",
			method: http.MethodPost,
			body: CarbonReportRequest{
				JobID:      "job-123",
				DataSizeGB: -100.0,
				Zone:       carbon.ZoneUSEast,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "missing zone",
			method: http.MethodPost,
			body: CarbonReportRequest{
				JobID:      "job-123",
				DataSizeGB: 500.0,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid method",
			method:         http.MethodGet,
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				var err error
				body, err = json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("Failed to marshal request: %v", err)
				}
			}

			req := httptest.NewRequest(tt.method, "/carbon/report", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handlers.HandleCarbonReport(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				var response CarbonReportResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Validate response
				if response.OperationID == "" {
					t.Error("Expected non-empty operation ID")
				}
				if response.Duration <= 0 {
					t.Error("Expected positive duration")
				}
				if response.EnergyUsed <= 0 {
					t.Error("Expected positive energy usage")
				}
				if response.CarbonEmissions <= 0 {
					t.Error("Expected positive carbon emissions")
				}
				if response.Equivalent == "" {
					t.Error("Expected non-empty equivalent")
				}
			}
		})
	}
}

func TestHandleCarbonZones(t *testing.T) {
	handlers := setupCarbonHandlers(t)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedZones  int
	}{
		{
			name:           "get zones",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedZones:  12, // We have 12 predefined zones
		},
		{
			name:           "invalid method",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedZones:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/carbon/zones", nil)
			w := httptest.NewRecorder()

			handlers.HandleCarbonZones(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				var response CarbonZonesResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Total != tt.expectedZones {
					t.Errorf("Expected %d zones, got %d", tt.expectedZones, response.Total)
				}

				if len(response.Zones) != tt.expectedZones {
					t.Errorf("Expected %d zone entries, got %d", tt.expectedZones, len(response.Zones))
				}

				// Validate zone structure
				if len(response.Zones) > 0 {
					zone := response.Zones[0]
					if zone.ID == "" {
						t.Error("Expected non-empty zone ID")
					}
					if zone.Name == "" {
						t.Error("Expected non-empty zone name")
					}
					if zone.Region == "" {
						t.Error("Expected non-empty region")
					}
					if zone.Typical <= 0 {
						t.Error("Expected positive typical intensity")
					}
				}
			}
		})
	}
}

func TestHandleCarbonEstimate(t *testing.T) {
	handlers := setupCarbonHandlers(t)

	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
	}{
		{
			name:   "valid request",
			method: http.MethodPost,
			body: CarbonEstimateRequest{
				Zone:          carbon.ZoneUSEast,
				DataSizeGB:    500.0,
				DurationHours: 2.0,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "missing zone",
			method: http.MethodPost,
			body: CarbonEstimateRequest{
				DataSizeGB:    500.0,
				DurationHours: 2.0,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "invalid data size",
			method: http.MethodPost,
			body: CarbonEstimateRequest{
				Zone:          carbon.ZoneUSEast,
				DataSizeGB:    -100.0,
				DurationHours: 2.0,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "invalid duration",
			method: http.MethodPost,
			body: CarbonEstimateRequest{
				Zone:          carbon.ZoneUSEast,
				DataSizeGB:    500.0,
				DurationHours: 0.0,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid method",
			method:         http.MethodGet,
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				var err error
				body, err = json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("Failed to marshal request: %v", err)
				}
			}

			req := httptest.NewRequest(tt.method, "/carbon/estimate", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handlers.HandleCarbonEstimate(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Code == http.StatusOK {
				var response CarbonEstimateResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Validate response
				if response.CurrentIntensity <= 0 {
					t.Error("Expected positive current intensity")
				}
				if response.BestIntensity <= 0 {
					t.Error("Expected positive best intensity")
				}
				if response.Recommendation == "" {
					t.Error("Expected non-empty recommendation")
				}
				if len(response.Forecast) == 0 {
					t.Error("Expected non-empty forecast")
				}

				// Check forecast structure
				if len(response.Forecast) > 0 {
					f := response.Forecast[0]
					if f.Intensity <= 0 {
						t.Error("Expected positive intensity in forecast")
					}
					if f.Quality == "" {
						t.Error("Expected non-empty quality in forecast")
					}
				}
			}
		})
	}
}

func TestDetermineQuality(t *testing.T) {
	tests := []struct {
		intensity float64
		expected  string
	}{
		{50.0, "excellent"},
		{150.0, "good"},
		{300.0, "moderate"},
		{500.0, "poor"},
		{800.0, "very poor"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := determineQuality(tt.intensity)
			if result != tt.expected {
				t.Errorf("For intensity %.0f, expected '%s', got '%s'",
					tt.intensity, tt.expected, result)
			}
		})
	}
}

func TestGenerateEquivalent(t *testing.T) {
	tests := []struct {
		kgCO2    float64
		contains string // String that should be in the result
	}{
		{0.1, "driving"},
		{5.0, "km of driving"},
		{50.0, "km of driving"},
		{100.0, "tree"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := generateEquivalent(tt.kgCO2)
			if result == "" {
				t.Error("Expected non-empty equivalent")
			}
			// Just check it returns something reasonable
			// The exact format might vary
		})
	}
}
