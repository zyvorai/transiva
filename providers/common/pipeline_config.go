// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
	"os"
)

// PipelineStage represents a conversion pipeline stage
type PipelineStage string

const (
	StageInspect  PipelineStage = "inspect"  // OS detection, driver analysis
	StageFix      PipelineStage = "fix"      // Driver injection, config fixes
	StageConvert  PipelineStage = "convert"  // Disk format conversion
	StageValidate PipelineStage = "validate" // Image validation
	StageOptimize PipelineStage = "optimize" // Image optimization
	StageCompress PipelineStage = "compress" // Image compression
)

// PipelineStageConfig holds configuration for a pipeline stage
type PipelineStageConfig struct {
	Enabled bool                   `json:"enabled"`
	Options map[string]interface{} `json:"options,omitempty"`
	Timeout int                    `json:"timeout,omitempty"` // Timeout in seconds
	Retry   int                    `json:"retry,omitempty"`   // Number of retries
}

// PipelineConfig defines a custom conversion pipeline
type PipelineConfig struct {
	Name        string                                 `json:"name"`
	Description string                                 `json:"description"`
	Stages      map[PipelineStage]*PipelineStageConfig `json:"stages"`
	Hooks       *PipelineHooks                         `json:"hooks,omitempty"`
}

// PipelineHooks defines custom hooks for pipeline stages
type PipelineHooks struct {
	PreConvert  string `json:"pre_convert,omitempty"`  // Script to run before conversion
	PostConvert string `json:"post_convert,omitempty"` // Script to run after conversion
	PreStage    string `json:"pre_stage,omitempty"`    // Script to run before each stage
	PostStage   string `json:"post_stage,omitempty"`   // Script to run after each stage
	OnError     string `json:"on_error,omitempty"`     // Script to run on error
}

// NewDefaultPipelineConfig creates a default pipeline configuration
func NewDefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		Name:        "default",
		Description: "Default conversion pipeline",
		Stages: map[PipelineStage]*PipelineStageConfig{
			StageInspect: {
				Enabled: true,
				Options: map[string]interface{}{
					"detect_os":      true,
					"check_drivers":  true,
					"analyze_config": true,
				},
			},
			StageFix: {
				Enabled: true,
				Options: map[string]interface{}{
					"inject_drivers": true,
					"fix_fstab":      true,
					"fix_grub":       true,
					"fix_network":    true,
				},
			},
			StageConvert: {
				Enabled: true,
				Options: map[string]interface{}{
					"target_format": "qcow2",
					"compression":   true,
				},
			},
			StageValidate: {
				Enabled: true,
				Options: map[string]interface{}{
					"check_integrity": true,
					"check_bootable":  true,
				},
			},
		},
	}
}

// NewMinimalPipelineConfig creates a minimal pipeline (convert only)
func NewMinimalPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		Name:        "minimal",
		Description: "Minimal conversion pipeline (convert only)",
		Stages: map[PipelineStage]*PipelineStageConfig{
			StageInspect: {
				Enabled: false,
			},
			StageFix: {
				Enabled: false,
			},
			StageConvert: {
				Enabled: true,
				Options: map[string]interface{}{
					"target_format": "qcow2",
					"compression":   false,
				},
			},
			StageValidate: {
				Enabled: false,
			},
		},
	}
}

// NewOptimizedPipelineConfig creates an optimized pipeline
func NewOptimizedPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		Name:        "optimized",
		Description: "Optimized conversion pipeline with compression",
		Stages: map[PipelineStage]*PipelineStageConfig{
			StageInspect: {
				Enabled: true,
			},
			StageFix: {
				Enabled: true,
			},
			StageConvert: {
				Enabled: true,
				Options: map[string]interface{}{
					"target_format": "qcow2",
					"compression":   true,
				},
			},
			StageValidate: {
				Enabled: true,
			},
			StageOptimize: {
				Enabled: true,
				Options: map[string]interface{}{
					"sparsify":    true,
					"trim_unused": true,
				},
			},
			StageCompress: {
				Enabled: true,
				Options: map[string]interface{}{
					"compression_level": 9,
				},
			},
		},
	}
}

// LoadPipelineConfig loads a pipeline configuration from a file
func LoadPipelineConfig(path string) (*PipelineConfig, error) {
	// #nosec G304 -- path is a locally-supplied pipeline config file path from the operator/CLI, not remote input.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pipeline config: %w", err)
	}

	var config PipelineConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse pipeline config: %w", err)
	}

	return &config, nil
}

// SavePipelineConfig saves a pipeline configuration to a file
func (pc *PipelineConfig) Save(path string) error {
	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pipeline config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write pipeline config: %w", err)
	}

	return nil
}

// Validate validates the pipeline configuration
func (pc *PipelineConfig) Validate() error {
	if pc.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}

	if len(pc.Stages) == 0 {
		return fmt.Errorf("pipeline must have at least one stage")
	}

	// Check that at least one stage is enabled
	hasEnabledStage := false
	for _, stage := range pc.Stages {
		if stage.Enabled {
			hasEnabledStage = true
			break
		}
	}

	if !hasEnabledStage {
		return fmt.Errorf("pipeline must have at least one enabled stage")
	}

	return nil
}

// IsStageEnabled checks if a stage is enabled
func (pc *PipelineConfig) IsStageEnabled(stage PipelineStage) bool {
	if config, ok := pc.Stages[stage]; ok {
		return config.Enabled
	}
	return false
}

// GetStageOptions returns options for a stage
func (pc *PipelineConfig) GetStageOptions(stage PipelineStage) map[string]interface{} {
	if config, ok := pc.Stages[stage]; ok {
		return config.Options
	}
	return nil
}

// GetStageOption returns a specific option for a stage
func (pc *PipelineConfig) GetStageOption(stage PipelineStage, key string) (interface{}, bool) {
	opts := pc.GetStageOptions(stage)
	if opts == nil {
		return nil, false
	}

	val, ok := opts[key]
	return val, ok
}

// EnableStage enables a pipeline stage
func (pc *PipelineConfig) EnableStage(stage PipelineStage) {
	if config, ok := pc.Stages[stage]; ok {
		config.Enabled = true
	} else {
		pc.Stages[stage] = &PipelineStageConfig{Enabled: true}
	}
}

// DisableStage disables a pipeline stage
func (pc *PipelineConfig) DisableStage(stage PipelineStage) {
	if config, ok := pc.Stages[stage]; ok {
		config.Enabled = false
	}
}

// SetStageOption sets an option for a stage
func (pc *PipelineConfig) SetStageOption(stage PipelineStage, key string, value interface{}) {
	if config, ok := pc.Stages[stage]; ok {
		if config.Options == nil {
			config.Options = make(map[string]interface{})
		}
		config.Options[key] = value
	} else {
		pc.Stages[stage] = &PipelineStageConfig{
			Enabled: true,
			Options: map[string]interface{}{
				key: value,
			},
		}
	}
}
