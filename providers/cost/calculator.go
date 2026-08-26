// SPDX-License-Identifier: Apache-2.0

package cost

import (
	"fmt"
	"math"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// Calculator estimates cloud storage and transfer costs
type Calculator struct {
	pricingData map[CloudProvider]*PricingData
	log         logger.Logger
}

// NewCalculator creates a new cost calculator
func NewCalculator(log logger.Logger) *Calculator {
	calc := &Calculator{
		pricingData: make(map[CloudProvider]*PricingData),
		log:         log,
	}

	// Initialize pricing data for major providers
	calc.initializePricing()

	return calc
}

// initializePricing initializes pricing data for major cloud providers
// Prices as of 2026-01 (approximate)
func (c *Calculator) initializePricing() {
	// Amazon S3 (US East)
	c.pricingData[ProviderS3] = &PricingData{
		Provider: ProviderS3,
		Region:   "us-east-1",
		StoragePricing: map[StorageClass]float64{
			StorageS3Standard:    0.023,  // per GB/month
			StorageS3IA:          0.0125, // per GB/month
			StorageS3OneZoneIA:   0.01,   // per GB/month
			StorageS3Glacier:     0.004,  // per GB/month
			StorageS3DeepArchive: 0.00099, // per GB/month
		},
		TransferPricing: map[string]float64{
			"first_10tb":  0.09,  // per GB
			"next_40tb":   0.085, // per GB
			"next_100tb":  0.07,  // per GB
			"over_150tb":  0.05,  // per GB
		},
		RequestPricing: &RequestPricing{
			PutCost:    0.005,  // per 1000 requests
			GetCost:    0.0004, // per 1000 requests
			ListCost:   0.005,  // per 1000 requests
			DeleteCost: 0,      // free
		},
		RetrievalPricing: map[StorageClass]float64{
			StorageS3Glacier:     0.01,   // per GB
			StorageS3DeepArchive: 0.02,   // per GB
		},
		MinimumStorage: map[StorageClass]int{
			StorageS3IA:        30,  // days
			StorageS3Glacier:   90,  // days
			StorageS3DeepArchive: 180, // days
		},
		UpdatedAt: time.Now(),
	}

	// Azure Blob Storage (US East)
	c.pricingData[ProviderAzureBlob] = &PricingData{
		Provider: ProviderAzureBlob,
		Region:   "eastus",
		StoragePricing: map[StorageClass]float64{
			StorageAzureHot:     0.0184, // per GB/month
			StorageAzureCool:    0.01,   // per GB/month
			StorageAzureArchive: 0.002,  // per GB/month
		},
		TransferPricing: map[string]float64{
			"first_100gb":  0,     // free
			"first_10tb":   0.087, // per GB
			"next_40tb":    0.083, // per GB
			"over_50tb":    0.07,  // per GB
		},
		RequestPricing: &RequestPricing{
			PutCost:    0.005,  // per 10000 requests
			GetCost:    0.0004, // per 10000 requests
			ListCost:   0.005,  // per 10000 requests
			DeleteCost: 0,
		},
		RetrievalPricing: map[StorageClass]float64{
			StorageAzureCool:    0.01,  // per GB
			StorageAzureArchive: 0.02,  // per GB
		},
		MinimumStorage: map[StorageClass]int{
			StorageAzureCool:    30,  // days
			StorageAzureArchive: 180, // days
		},
		UpdatedAt: time.Now(),
	}

	// Google Cloud Storage (US)
	c.pricingData[ProviderGCS] = &PricingData{
		Provider: ProviderGCS,
		Region:   "us",
		StoragePricing: map[StorageClass]float64{
			StorageGCSStandard: 0.02,   // per GB/month
			StorageGCSNearline: 0.01,   // per GB/month
			StorageGCSColdline: 0.004,  // per GB/month
			StorageGCSArchive:  0.0012, // per GB/month
		},
		TransferPricing: map[string]float64{
			"first_1tb":   0.12,  // per GB
			"next_9tb":    0.11,  // per GB
			"over_10tb":   0.08,  // per GB
		},
		RequestPricing: &RequestPricing{
			PutCost:    0.05,  // per 10000 requests
			GetCost:    0.004, // per 10000 requests
			ListCost:   0.05,  // per 10000 requests
			DeleteCost: 0,
		},
		RetrievalPricing: map[StorageClass]float64{
			StorageGCSNearline: 0.01,  // per GB
			StorageGCSColdline: 0.02,  // per GB
			StorageGCSArchive:  0.05,  // per GB
		},
		MinimumStorage: map[StorageClass]int{
			StorageGCSNearline: 30,  // days
			StorageGCSColdline: 90,  // days
			StorageGCSArchive:  365, // days
		},
		UpdatedAt: time.Now(),
	}
}

// EstimateCost calculates the estimated cost for cloud storage
func (c *Calculator) EstimateCost(req *CostEstimateRequest) (*CostEstimateResponse, error) {
	pricing, exists := c.pricingData[req.Provider]
	if !exists {
		return nil, fmt.Errorf("pricing data not available for provider: %s", req.Provider)
	}

	breakdown := &CostBreakdown{}

	// Calculate storage cost
	storagePrice, exists := pricing.StoragePricing[req.StorageClass]
	if !exists {
		return nil, fmt.Errorf("pricing not available for storage class: %s", req.StorageClass)
	}

	monthsFloat := float64(req.DurationDays) / 30.0
	breakdown.StorageCost = req.StorageGB * storagePrice * monthsFloat

	// Calculate transfer cost
	breakdown.TransferCost = c.calculateTransferCost(req.TransferGB, pricing)

	// Calculate request cost
	breakdown.RequestCost = c.calculateRequestCost(req.Requests, pricing)

	// Calculate retrieval cost (for archive storage classes)
	if retrievalPrice, exists := pricing.RetrievalPricing[req.StorageClass]; exists {
		breakdown.RetrievalCost = req.TransferGB * retrievalPrice
	}

	// Calculate early deletion cost
	if minDays, exists := pricing.MinimumStorage[req.StorageClass]; exists {
		if req.DurationDays < minDays {
			remainingDays := minDays - req.DurationDays
			breakdown.EarlyDeleteCost = req.StorageGB * storagePrice * (float64(remainingDays) / 30.0)
		}
	}

	totalCost := breakdown.StorageCost +
		breakdown.TransferCost +
		breakdown.RequestCost +
		breakdown.RetrievalCost +
		breakdown.EarlyDeleteCost

	return &CostEstimateResponse{
		Provider:       req.Provider,
		Region:         string(req.Region),
		StorageClass:   string(req.StorageClass),
		Breakdown:      breakdown,
		TotalCost:      roundTo(totalCost, 2),
		Currency:       "USD",
		EstimatedAt:    time.Now(),
		PricingVersion: "2026-01",
	}, nil
}

// calculateTransferCost calculates data transfer costs
func (c *Calculator) calculateTransferCost(transferGB float64, pricing *PricingData) float64 {
	if transferGB == 0 {
		return 0
	}

	var cost float64
	remaining := transferGB

	// Simple tiered pricing
	if pricing.Provider == ProviderS3 {
		// S3 tiered pricing
		tiers := []struct {
			limit float64
			price float64
		}{
			{10 * 1024, pricing.TransferPricing["first_10tb"]},
			{40 * 1024, pricing.TransferPricing["next_40tb"]},
			{100 * 1024, pricing.TransferPricing["next_100tb"]},
			{math.MaxFloat64, pricing.TransferPricing["over_150tb"]},
		}

		for _, tier := range tiers {
			if remaining <= 0 {
				break
			}
			amount := math.Min(remaining, tier.limit)
			cost += amount * tier.price
			remaining -= amount
		}
	} else if pricing.Provider == ProviderAzureBlob {
		// Azure: first 100GB free
		if transferGB <= 100 {
			return 0
		}
		remaining = transferGB - 100

		tiers := []struct {
			limit float64
			price float64
		}{
			{10 * 1024, pricing.TransferPricing["first_10tb"]},
			{40 * 1024, pricing.TransferPricing["next_40tb"]},
			{math.MaxFloat64, pricing.TransferPricing["over_50tb"]},
		}

		for _, tier := range tiers {
			if remaining <= 0 {
				break
			}
			amount := math.Min(remaining, tier.limit)
			cost += amount * tier.price
			remaining -= amount
		}
	} else if pricing.Provider == ProviderGCS {
		// GCS tiered pricing
		tiers := []struct {
			limit float64
			price float64
		}{
			{1024, pricing.TransferPricing["first_1tb"]},
			{9 * 1024, pricing.TransferPricing["next_9tb"]},
			{math.MaxFloat64, pricing.TransferPricing["over_10tb"]},
		}

		for _, tier := range tiers {
			if remaining <= 0 {
				break
			}
			amount := math.Min(remaining, tier.limit)
			cost += amount * tier.price
			remaining -= amount
		}
	}

	return cost
}

// calculateRequestCost calculates API request costs
func (c *Calculator) calculateRequestCost(requests int64, pricing *PricingData) float64 {
	if requests == 0 {
		return 0
	}

	// Assume 60% GET, 30% PUT, 10% LIST
	getRequests := float64(requests) * 0.6
	putRequests := float64(requests) * 0.3
	listRequests := float64(requests) * 0.1

	cost := (getRequests/1000)*pricing.RequestPricing.GetCost +
		(putRequests/1000)*pricing.RequestPricing.PutCost +
		(listRequests/1000)*pricing.RequestPricing.ListCost

	return cost
}

// CompareProviders compares costs across multiple providers
func (c *Calculator) CompareProviders(req *CostEstimateRequest) (*CostComparison, error) {
	comparison := &CostComparison{
		Request:   req,
		Estimates: make([]*CostEstimateResponse, 0),
	}

	providers := []CloudProvider{ProviderS3, ProviderAzureBlob, ProviderGCS}
	storageClasses := map[CloudProvider]StorageClass{
		ProviderS3:        StorageS3Standard,
		ProviderAzureBlob: StorageAzureHot,
		ProviderGCS:       StorageGCSStandard,
	}

	var cheapest *CostEstimateResponse
	var mostExpensive *CostEstimateResponse

	for _, provider := range providers {
		providerReq := *req
		providerReq.Provider = provider
		providerReq.StorageClass = storageClasses[provider]

		estimate, err := c.EstimateCost(&providerReq)
		if err != nil {
			c.log.Warn("failed to estimate cost", "provider", provider, "error", err)
			continue
		}

		comparison.Estimates = append(comparison.Estimates, estimate)

		if cheapest == nil || estimate.TotalCost < cheapest.TotalCost {
			cheapest = estimate
			comparison.Cheapest = provider
		}

		if mostExpensive == nil || estimate.TotalCost > mostExpensive.TotalCost {
			mostExpensive = estimate
		}
	}

	if cheapest != nil && mostExpensive != nil {
		comparison.SavingsVsExpensive = mostExpensive.TotalCost - cheapest.TotalCost
	}

	// Recommend based on cost and reliability
	comparison.Recommended = comparison.Cheapest

	return comparison, nil
}

// ProjectYearlyCost projects costs for a year
func (c *Calculator) ProjectYearlyCost(req *CostEstimateRequest) (*YearlyCostProjection, error) {
	projection := &YearlyCostProjection{
		Year:             time.Now().Year(),
		MonthlyBreakdown: make([]*MonthlyProjection, 12),
	}

	for month := 1; month <= 12; month++ {
		monthReq := *req
		monthReq.DurationDays = 30

		estimate, err := c.EstimateCost(&monthReq)
		if err != nil {
			return nil, err
		}

		projection.MonthlyBreakdown[month-1] = &MonthlyProjection{
			Month:     month,
			TotalCost: estimate.TotalCost,
			Breakdown: estimate.Breakdown,
		}

		projection.TotalCost += estimate.TotalCost
	}

	projection.MonthlyAverage = projection.TotalCost / 12

	return projection, nil
}

// EstimateExportSize estimates the size of a VM export
func (c *Calculator) EstimateExportSize(diskSizeGB float64, format string, includeSnapshots bool) *ExportSizeEstimate {
	estimate := &ExportSizeEstimate{
		TotalDiskSizeGB:  diskSizeGB,
		Format:           format,
		IncludeSnapshots: includeSnapshots,
	}

	// Estimate compression ratio based on format
	var compressionRatio float64
	switch format {
	case "ova":
		compressionRatio = 0.7 // 30% compression
	case "qcow2":
		compressionRatio = 0.65 // 35% compression
	case "raw":
		compressionRatio = 1.0 // No compression
	default:
		compressionRatio = 0.8
	}

	estimate.CompressionRatio = compressionRatio
	estimate.EstimatedExportGB = diskSizeGB * compressionRatio

	if includeSnapshots {
		// Snapshots add ~20% more data
		estimate.EstimatedExportGB *= 1.2
	}

	return estimate
}

// CheckBudget checks if costs are within budget
func (c *Calculator) CheckBudget(currentSpending, monthlyBudget float64) *BudgetAlert {
	percentageUsed := (currentSpending / monthlyBudget) * 100

	return &BudgetAlert{
		Threshold:        monthlyBudget,
		CurrentSpending:  roundTo(currentSpending, 2),
		ProjectedMonthly: roundTo(currentSpending, 2),
		Alert:            currentSpending > monthlyBudget,
		PercentageUsed:   roundTo(percentageUsed, 1),
	}
}

// roundTo rounds a float to a specified number of decimal places
func roundTo(value float64, decimals int) float64 {
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(value*multiplier) / multiplier
}
