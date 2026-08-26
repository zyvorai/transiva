// SPDX-License-Identifier: Apache-2.0

package cost

import (
	"testing"

	"github.com/zyvorai/transiva/logger"
)

func TestEstimateCost(t *testing.T) {
	log := logger.NewTestLogger(t)
	calc := NewCalculator(log)

	tests := []struct {
		name        string
		req         *CostEstimateRequest
		wantError   bool
		minCost     float64
		maxCost     float64
		description string
	}{
		{
			name: "S3 Standard Storage - 100GB for 30 days",
			req: &CostEstimateRequest{
				Provider:     ProviderS3,
				Region:       RegionUSEast1,
				StorageClass: StorageS3Standard,
				StorageGB:    100,
				TransferGB:   0,
				Requests:     0,
				DurationDays: 30,
			},
			wantError:   false,
			minCost:     2.0,
			maxCost:     3.0,
			description: "100GB * $0.023/GB * 1 month = $2.30",
		},
		{
			name: "S3 Glacier Deep Archive - 1TB for 180 days",
			req: &CostEstimateRequest{
				Provider:     ProviderS3,
				Region:       RegionUSEast1,
				StorageClass: StorageS3DeepArchive,
				StorageGB:    1024,
				TransferGB:   0,
				Requests:     0,
				DurationDays: 180,
			},
			wantError:   false,
			minCost:     5.0,
			maxCost:     8.0,
			description: "1024GB * $0.00099/GB * 6 months = ~$6",
		},
		{
			name: "Azure Hot Storage with Transfer - 500GB storage, 100GB transfer",
			req: &CostEstimateRequest{
				Provider:     ProviderAzureBlob,
				Region:       RegionAzureEastUS,
				StorageClass: StorageAzureHot,
				StorageGB:    500,
				TransferGB:   100,
				Requests:     1000,
				DurationDays: 30,
			},
			wantError:   false,
			minCost:     9.0,
			maxCost:     11.0,
			description: "Storage + transfer (first 100GB free) + requests",
		},
		{
			name: "GCS Standard Storage with Requests",
			req: &CostEstimateRequest{
				Provider:     ProviderGCS,
				Region:       RegionGCPUS,
				StorageClass: StorageGCSStandard,
				StorageGB:    250,
				TransferGB:   50,
				Requests:     10000,
				DurationDays: 30,
			},
			wantError:   false,
			minCost:     10.0,
			maxCost:     15.0,
			description: "250GB storage + 50GB transfer + 10k requests",
		},
		{
			name: "S3 IA Early Deletion - 200GB deleted after 15 days",
			req: &CostEstimateRequest{
				Provider:     ProviderS3,
				Region:       RegionUSEast1,
				StorageClass: StorageS3IA,
				StorageGB:    200,
				TransferGB:   0,
				Requests:     0,
				DurationDays: 15,
			},
			wantError:   false,
			minCost:     1.0,
			maxCost:     3.0,
			description: "Should include early deletion cost (minimum 30 days)",
		},
		{
			name: "Invalid Provider",
			req: &CostEstimateRequest{
				Provider:     CloudProvider("invalid"),
				Region:       RegionUSEast1,
				StorageClass: StorageS3Standard,
				StorageGB:    100,
				TransferGB:   0,
				Requests:     0,
				DurationDays: 30,
			},
			wantError:   true,
			description: "Should fail with invalid provider",
		},
		{
			name: "Invalid Storage Class",
			req: &CostEstimateRequest{
				Provider:     ProviderS3,
				Region:       RegionUSEast1,
				StorageClass: StorageClass("invalid"),
				StorageGB:    100,
				TransferGB:   0,
				Requests:     0,
				DurationDays: 30,
			},
			wantError:   true,
			description: "Should fail with invalid storage class",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := calc.EstimateCost(tt.req)

			if tt.wantError {
				if err == nil {
					t.Errorf("EstimateCost() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("EstimateCost() unexpected error: %v", err)
				return
			}

			if resp.TotalCost < tt.minCost || resp.TotalCost > tt.maxCost {
				t.Errorf("EstimateCost() cost = $%.2f, want between $%.2f and $%.2f (%s)",
					resp.TotalCost, tt.minCost, tt.maxCost, tt.description)
			}

			// Verify response fields
			if resp.Provider != tt.req.Provider {
				t.Errorf("EstimateCost() provider = %v, want %v", resp.Provider, tt.req.Provider)
			}

			if resp.Currency != "USD" {
				t.Errorf("EstimateCost() currency = %v, want USD", resp.Currency)
			}

			if resp.Breakdown == nil {
				t.Error("EstimateCost() breakdown is nil")
			}
		})
	}
}

func TestCompareProviders(t *testing.T) {
	log := logger.NewTestLogger(t)
	calc := NewCalculator(log)

	req := &CostEstimateRequest{
		StorageGB:    500,
		TransferGB:   100,
		Requests:     5000,
		DurationDays: 30,
	}

	comparison, err := calc.CompareProviders(req)
	if err != nil {
		t.Fatalf("CompareProviders() error: %v", err)
	}

	// Should have estimates for all 3 providers
	if len(comparison.Estimates) != 3 {
		t.Errorf("CompareProviders() got %d estimates, want 3", len(comparison.Estimates))
	}

	// Should identify cheapest provider
	if comparison.Cheapest == "" {
		t.Error("CompareProviders() cheapest is empty")
	}

	// Should calculate savings
	if comparison.SavingsVsExpensive <= 0 {
		t.Errorf("CompareProviders() savings = %.2f, want > 0", comparison.SavingsVsExpensive)
	}

	// Should have recommendation
	if comparison.Recommended == "" {
		t.Error("CompareProviders() recommended is empty")
	}

	// Verify all estimates are valid
	for _, estimate := range comparison.Estimates {
		if estimate.TotalCost <= 0 {
			t.Errorf("CompareProviders() provider %s has cost %.2f, want > 0",
				estimate.Provider, estimate.TotalCost)
		}
	}
}

func TestProjectYearlyCost(t *testing.T) {
	log := logger.NewTestLogger(t)
	calc := NewCalculator(log)

	req := &CostEstimateRequest{
		Provider:     ProviderS3,
		Region:       RegionUSEast1,
		StorageClass: StorageS3Standard,
		StorageGB:    100,
		TransferGB:   10,
		Requests:     1000,
		DurationDays: 30,
	}

	projection, err := calc.ProjectYearlyCost(req)
	if err != nil {
		t.Fatalf("ProjectYearlyCost() error: %v", err)
	}

	// Should have 12 monthly breakdowns
	if len(projection.MonthlyBreakdown) != 12 {
		t.Errorf("ProjectYearlyCost() got %d months, want 12", len(projection.MonthlyBreakdown))
	}

	// Total cost should be sum of all months
	var sum float64
	for _, month := range projection.MonthlyBreakdown {
		sum += month.TotalCost
		if month.TotalCost <= 0 {
			t.Errorf("ProjectYearlyCost() month %d has cost %.2f, want > 0",
				month.Month, month.TotalCost)
		}
	}

	if sum != projection.TotalCost {
		t.Errorf("ProjectYearlyCost() sum of months = %.2f, want %.2f",
			sum, projection.TotalCost)
	}

	// Monthly average should be total / 12
	expectedAverage := projection.TotalCost / 12
	if projection.MonthlyAverage != expectedAverage {
		t.Errorf("ProjectYearlyCost() average = %.2f, want %.2f",
			projection.MonthlyAverage, expectedAverage)
	}
}

func TestEstimateExportSize(t *testing.T) {
	log := logger.NewTestLogger(t)
	calc := NewCalculator(log)

	tests := []struct {
		name             string
		diskSizeGB       float64
		format           string
		includeSnapshots bool
		minSizeGB        float64
		maxSizeGB        float64
		description      string
	}{
		{
			name:             "OVA format - 100GB disk, no snapshots",
			diskSizeGB:       100,
			format:           "ova",
			includeSnapshots: false,
			minSizeGB:        65,
			maxSizeGB:        75,
			description:      "OVA has ~70% compression ratio = 70GB",
		},
		{
			name:             "QCOW2 format - 500GB disk, no snapshots",
			diskSizeGB:       500,
			format:           "qcow2",
			includeSnapshots: false,
			minSizeGB:        300,
			maxSizeGB:        350,
			description:      "QCOW2 has ~65% compression ratio = 325GB",
		},
		{
			name:             "RAW format - 200GB disk, no snapshots",
			diskSizeGB:       200,
			format:           "raw",
			includeSnapshots: false,
			minSizeGB:        195,
			maxSizeGB:        205,
			description:      "RAW has no compression = 200GB",
		},
		{
			name:             "OVA with snapshots - 100GB disk",
			diskSizeGB:       100,
			format:           "ova",
			includeSnapshots: true,
			minSizeGB:        80,
			maxSizeGB:        90,
			description:      "OVA 70GB + 20% for snapshots = 84GB",
		},
		{
			name:             "Large disk - 2TB QCOW2",
			diskSizeGB:       2048,
			format:           "qcow2",
			includeSnapshots: false,
			minSizeGB:        1200,
			maxSizeGB:        1400,
			description:      "2TB * 65% = ~1330GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimate := calc.EstimateExportSize(tt.diskSizeGB, tt.format, tt.includeSnapshots)

			if estimate.EstimatedExportGB < tt.minSizeGB || estimate.EstimatedExportGB > tt.maxSizeGB {
				t.Errorf("EstimateExportSize() size = %.2fGB, want between %.2fGB and %.2fGB (%s)",
					estimate.EstimatedExportGB, tt.minSizeGB, tt.maxSizeGB, tt.description)
			}

			if estimate.TotalDiskSizeGB != tt.diskSizeGB {
				t.Errorf("EstimateExportSize() disk size = %.2f, want %.2f",
					estimate.TotalDiskSizeGB, tt.diskSizeGB)
			}

			if estimate.Format != tt.format {
				t.Errorf("EstimateExportSize() format = %s, want %s",
					estimate.Format, tt.format)
			}

			if estimate.IncludeSnapshots != tt.includeSnapshots {
				t.Errorf("EstimateExportSize() snapshots = %v, want %v",
					estimate.IncludeSnapshots, tt.includeSnapshots)
			}
		})
	}
}

func TestCalculateTransferCost(t *testing.T) {
	log := logger.NewTestLogger(t)
	calc := NewCalculator(log)

	tests := []struct {
		name       string
		provider   CloudProvider
		transferGB float64
		minCost    float64
		maxCost    float64
	}{
		{
			name:       "S3 - No transfer",
			provider:   ProviderS3,
			transferGB: 0,
			minCost:    0,
			maxCost:    0,
		},
		{
			name:       "S3 - 1TB transfer",
			provider:   ProviderS3,
			transferGB: 1024,
			minCost:    85,
			maxCost:    95,
		},
		{
			name:       "Azure - 50GB transfer (under 100GB free)",
			provider:   ProviderAzureBlob,
			transferGB: 50,
			minCost:    0,
			maxCost:    0,
		},
		{
			name:       "Azure - 500GB transfer",
			provider:   ProviderAzureBlob,
			transferGB: 500,
			minCost:    30,
			maxCost:    40,
		},
		{
			name:       "GCS - 2TB transfer",
			provider:   ProviderGCS,
			transferGB: 2048,
			minCost:    230,
			maxCost:    240,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := calc.pricingData[tt.provider]
			cost := calc.calculateTransferCost(tt.transferGB, pricing)

			if cost < tt.minCost || cost > tt.maxCost {
				t.Errorf("calculateTransferCost() = %.2f, want between %.2f and %.2f",
					cost, tt.minCost, tt.maxCost)
			}
		})
	}
}

func TestCalculateRequestCost(t *testing.T) {
	log := logger.NewTestLogger(t)
	calc := NewCalculator(log)

	tests := []struct {
		name     string
		provider CloudProvider
		requests int64
		minCost  float64
		maxCost  float64
	}{
		{
			name:     "S3 - No requests",
			provider: ProviderS3,
			requests: 0,
			minCost:  0,
			maxCost:  0,
		},
		{
			name:     "S3 - 10,000 requests",
			provider: ProviderS3,
			requests: 10000,
			minCost:  0.02,
			maxCost:  0.06,
		},
		{
			name:     "Azure - 100,000 requests",
			provider: ProviderAzureBlob,
			requests: 100000,
			minCost:  0.2,
			maxCost:  0.6,
		},
		{
			name:     "GCS - 1,000,000 requests",
			provider: ProviderGCS,
			requests: 1000000,
			minCost:  20.0,
			maxCost:  25.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := calc.pricingData[tt.provider]
			cost := calc.calculateRequestCost(tt.requests, pricing)

			if cost < tt.minCost || cost > tt.maxCost {
				t.Errorf("calculateRequestCost() = %.2f, want between %.2f and %.2f",
					cost, tt.minCost, tt.maxCost)
			}
		})
	}
}

func TestCheckBudget(t *testing.T) {
	log := logger.NewTestLogger(t)
	calc := NewCalculator(log)

	tests := []struct {
		name            string
		currentSpending float64
		monthlyBudget   float64
		expectAlert     bool
		minPercentage   float64
		maxPercentage   float64
	}{
		{
			name:            "Under budget - 50%",
			currentSpending: 50,
			monthlyBudget:   100,
			expectAlert:     false,
			minPercentage:   49,
			maxPercentage:   51,
		},
		{
			name:            "At budget - 100%",
			currentSpending: 100,
			monthlyBudget:   100,
			expectAlert:     false,
			minPercentage:   99,
			maxPercentage:   101,
		},
		{
			name:            "Over budget - 150%",
			currentSpending: 150,
			monthlyBudget:   100,
			expectAlert:     true,
			minPercentage:   149,
			maxPercentage:   151,
		},
		{
			name:            "Way over budget - 200%",
			currentSpending: 200,
			monthlyBudget:   100,
			expectAlert:     true,
			minPercentage:   199,
			maxPercentage:   201,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert := calc.CheckBudget(tt.currentSpending, tt.monthlyBudget)

			if alert.Alert != tt.expectAlert {
				t.Errorf("CheckBudget() alert = %v, want %v", alert.Alert, tt.expectAlert)
			}

			if alert.PercentageUsed < tt.minPercentage || alert.PercentageUsed > tt.maxPercentage {
				t.Errorf("CheckBudget() percentage = %.1f%%, want between %.1f%% and %.1f%%",
					alert.PercentageUsed, tt.minPercentage, tt.maxPercentage)
			}

			if alert.CurrentSpending != roundTo(tt.currentSpending, 2) {
				t.Errorf("CheckBudget() spending = %.2f, want %.2f",
					alert.CurrentSpending, tt.currentSpending)
			}

			if alert.Threshold != tt.monthlyBudget {
				t.Errorf("CheckBudget() threshold = %.2f, want %.2f",
					alert.Threshold, tt.monthlyBudget)
			}
		})
	}
}

func TestRoundTo(t *testing.T) {
	tests := []struct {
		value    float64
		decimals int
		want     float64
	}{
		{1.2345, 2, 1.23},
		{1.2355, 2, 1.24},
		{10.999, 2, 11.00},
		{123.456, 1, 123.5},
		{0.1111, 3, 0.111},
	}

	for _, tt := range tests {
		got := roundTo(tt.value, tt.decimals)
		if got != tt.want {
			t.Errorf("roundTo(%.4f, %d) = %.4f, want %.4f",
				tt.value, tt.decimals, got, tt.want)
		}
	}
}

// Benchmark tests
func BenchmarkEstimateCost(b *testing.B) {
	log := logger.NewTestLogger(b)
	calc := NewCalculator(log)

	req := &CostEstimateRequest{
		Provider:     ProviderS3,
		Region:       RegionUSEast1,
		StorageClass: StorageS3Standard,
		StorageGB:    1000,
		TransferGB:   100,
		Requests:     10000,
		DurationDays: 30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = calc.EstimateCost(req)
	}
}

func BenchmarkCompareProviders(b *testing.B) {
	log := logger.NewTestLogger(b)
	calc := NewCalculator(log)

	req := &CostEstimateRequest{
		StorageGB:    500,
		TransferGB:   100,
		Requests:     5000,
		DurationDays: 30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = calc.CompareProviders(req)
	}
}
