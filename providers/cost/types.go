// SPDX-License-Identifier: Apache-2.0

package cost

import (
	"time"
)

// CloudProvider represents a cloud storage provider
type CloudProvider string

const (
	ProviderS3         CloudProvider = "s3"          // Amazon S3
	ProviderAzureBlob  CloudProvider = "azure_blob"  // Azure Blob Storage
	ProviderGCS        CloudProvider = "gcs"         // Google Cloud Storage
	ProviderOCI        CloudProvider = "oci"         // Oracle Cloud Infrastructure
	ProviderBackblaze  CloudProvider = "backblaze"   // Backblaze B2
	ProviderWasabi     CloudProvider = "wasabi"      // Wasabi
)

// StorageClass represents different storage tiers
type StorageClass string

const (
	// S3 Storage Classes
	StorageS3Standard      StorageClass = "s3_standard"
	StorageS3IA            StorageClass = "s3_ia"             // Infrequent Access
	StorageS3OneZoneIA     StorageClass = "s3_onezone_ia"
	StorageS3Glacier       StorageClass = "s3_glacier"
	StorageS3DeepArchive   StorageClass = "s3_deep_archive"

	// Azure Storage Classes
	StorageAzureHot        StorageClass = "azure_hot"
	StorageAzureCool       StorageClass = "azure_cool"
	StorageAzureArchive    StorageClass = "azure_archive"

	// GCS Storage Classes
	StorageGCSStandard     StorageClass = "gcs_standard"
	StorageGCSNearline     StorageClass = "gcs_nearline"
	StorageGCSColdline     StorageClass = "gcs_coldline"
	StorageGCSArchive      StorageClass = "gcs_archive"
)

// Region represents a cloud region
type Region string

const (
	// AWS Regions
	RegionUSEast1      Region = "us-east-1"
	RegionUSWest1      Region = "us-west-1"
	RegionUSWest2      Region = "us-west-2"
	RegionEUWest1      Region = "eu-west-1"
	RegionAPSoutheast1 Region = "ap-southeast-1"

	// Azure Regions
	RegionAzureEastUS   Region = "eastus"
	RegionAzureWestUS   Region = "westus"
	RegionAzureWestEU   Region = "westeurope"
	RegionAzureSouthEU  Region = "southeurope"

	// GCP Regions
	RegionGCPUS          Region = "us"
	RegionGCPEurope      Region = "europe"
	RegionGCPAsia        Region = "asia"
	RegionGCPUSCentral1  Region = "us-central1"
	RegionGCPUSEast1     Region = "us-east1"
)

// CostEstimateRequest represents a request to estimate costs
type CostEstimateRequest struct {
	Provider      CloudProvider `json:"provider"`
	Region        Region        `json:"region"`
	StorageClass  StorageClass  `json:"storage_class"`
	StorageGB     float64       `json:"storage_gb"`      // Amount of data in GB
	TransferGB    float64       `json:"transfer_gb"`     // Data transfer out in GB
	Requests      int64         `json:"requests"`        // Number of API requests
	DurationDays  int           `json:"duration_days"`   // Duration in days
}

// CostEstimateResponse represents the cost estimation result
type CostEstimateResponse struct {
	Provider         CloudProvider     `json:"provider"`
	Region           string            `json:"region"`
	StorageClass     string            `json:"storage_class"`
	Breakdown        *CostBreakdown    `json:"breakdown"`
	TotalCost        float64           `json:"total_cost"`
	Currency         string            `json:"currency"`
	EstimatedAt      time.Time         `json:"estimated_at"`
	PricingVersion   string            `json:"pricing_version"`
}

// CostBreakdown provides detailed cost breakdown
type CostBreakdown struct {
	StorageCost     float64 `json:"storage_cost"`      // Monthly storage cost
	TransferCost    float64 `json:"transfer_cost"`     // Data transfer cost
	RequestCost     float64 `json:"request_cost"`      // API request cost
	RetrievalCost   float64 `json:"retrieval_cost"`    // Data retrieval cost (for archive)
	EarlyDeleteCost float64 `json:"early_delete_cost"` // Early deletion fee (if applicable)
}

// CostComparison compares costs across multiple providers
type CostComparison struct {
	Request     *CostEstimateRequest   `json:"request"`
	Estimates   []*CostEstimateResponse `json:"estimates"`
	Cheapest    CloudProvider          `json:"cheapest"`
	Recommended CloudProvider          `json:"recommended"`
	SavingsVsExpensive float64         `json:"savings_vs_expensive"`
}

// PricingData represents pricing information for a provider
type PricingData struct {
	Provider           CloudProvider              `json:"provider"`
	Region             string                     `json:"region"`
	StoragePricing     map[StorageClass]float64   `json:"storage_pricing"`     // USD per GB per month
	TransferPricing    map[string]float64         `json:"transfer_pricing"`    // USD per GB
	RequestPricing     *RequestPricing            `json:"request_pricing"`     // Request costs
	RetrievalPricing   map[StorageClass]float64   `json:"retrieval_pricing"`   // USD per GB
	MinimumStorage     map[StorageClass]int       `json:"minimum_storage"`     // Minimum storage days
	UpdatedAt          time.Time                  `json:"updated_at"`
}

// RequestPricing represents API request costs
type RequestPricing struct {
	PutCost    float64 `json:"put_cost"`    // Cost per 1000 PUT requests
	GetCost    float64 `json:"get_cost"`    // Cost per 1000 GET requests
	ListCost   float64 `json:"list_cost"`   // Cost per 1000 LIST requests
	DeleteCost float64 `json:"delete_cost"` // Cost per 1000 DELETE requests
}

// MonthlyProjection represents monthly cost projection
type MonthlyProjection struct {
	Month     int     `json:"month"`
	TotalCost float64 `json:"total_cost"`
	Breakdown *CostBreakdown `json:"breakdown"`
}

// YearlyCostProjection represents yearly cost projection
type YearlyCostProjection struct {
	Year             int                  `json:"year"`
	TotalCost        float64              `json:"total_cost"`
	MonthlyAverage   float64              `json:"monthly_average"`
	MonthlyBreakdown []*MonthlyProjection `json:"monthly_breakdown"`
}

// ExportSizeEstimate estimates the size of an export
type ExportSizeEstimate struct {
	VMID               string  `json:"vm_id"`
	VMName             string  `json:"vm_name"`
	TotalDiskSizeGB    float64 `json:"total_disk_size_gb"`
	EstimatedExportGB  float64 `json:"estimated_export_gb"`  // After compression
	CompressionRatio   float64 `json:"compression_ratio"`
	Format             string  `json:"format"`
	IncludeSnapshots   bool    `json:"include_snapshots"`
}

// BudgetAlert represents a budget threshold alert
type BudgetAlert struct {
	Threshold        float64 `json:"threshold"`          // USD
	CurrentSpending  float64 `json:"current_spending"`   // USD
	ProjectedMonthly float64 `json:"projected_monthly"`  // USD
	Alert            bool    `json:"alert"`              // True if over budget
	PercentageUsed   float64 `json:"percentage_used"`
}
