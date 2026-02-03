// Package models provides model and GPU registry for continueplz.
package models

import (
	"fmt"
	"strings"
)

// Tier represents the model size tier.
type Tier string

const (
	TierSmall  Tier = "small"
	TierMedium Tier = "medium"
	TierLarge  Tier = "large"
)

// Model represents a code-assist LLM with its requirements.
type Model struct {
	Name    string // Full model name including tag (e.g., "qwen2.5-coder:32b")
	Params  string // Parameter count (e.g., "32B")
	VRAM    int    // Required VRAM in GB
	Quality int    // Quality rating 1-5
	Tier    Tier   // Model tier (small/medium/large)
}

// QualityStars returns the quality as a star rating string.
func (m Model) QualityStars() string {
	filled := strings.Repeat("★", m.Quality)
	empty := strings.Repeat("☆", 5-m.Quality)
	return filled + empty
}

// ModelRegistry contains all supported code-assist models.
// Models are from PRD Section 3.4.
var ModelRegistry = []Model{
	// Small tier (7-14B)
	{Name: "qwen2.5-coder:7b", Params: "7B", VRAM: 8, Quality: 3, Tier: TierSmall},
	{Name: "deepseek-coder:6.7b", Params: "6.7B", VRAM: 8, Quality: 3, Tier: TierSmall},
	{Name: "codellama:7b", Params: "7B", VRAM: 8, Quality: 2, Tier: TierSmall},
	{Name: "starcoder2:7b", Params: "7B", VRAM: 8, Quality: 3, Tier: TierSmall},

	// Medium tier (14-35B)
	{Name: "qwen2.5-coder:14b", Params: "14B", VRAM: 16, Quality: 4, Tier: TierMedium},
	{Name: "qwen2.5-coder:32b", Params: "32B", VRAM: 35, Quality: 5, Tier: TierMedium},
	{Name: "deepseek-coder:33b", Params: "33B", VRAM: 36, Quality: 5, Tier: TierMedium},
	{Name: "codellama:34b", Params: "34B", VRAM: 36, Quality: 4, Tier: TierMedium},

	// Large tier (70B+)
	{Name: "codellama:70b", Params: "70B", VRAM: 40, Quality: 5, Tier: TierLarge},
	{Name: "qwen2.5-coder:72b", Params: "72B", VRAM: 45, Quality: 5, Tier: TierLarge},
	{Name: "deepseek-coder-v2:236b", Params: "236B", VRAM: 120, Quality: 5, Tier: TierLarge},
}

// ErrModelNotFound is returned when a model is not found in the registry.
var ErrModelNotFound = fmt.Errorf("model not found")

// GetModelByName returns a model by its exact name.
func GetModelByName(name string) (*Model, error) {
	for i := range ModelRegistry {
		if ModelRegistry[i].Name == name {
			return &ModelRegistry[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrModelNotFound, name)
}

// GetModelsByTier returns all models matching the specified tier.
func GetModelsByTier(tier Tier) []Model {
	var models []Model
	for _, m := range ModelRegistry {
		if m.Tier == tier {
			models = append(models, m)
		}
	}
	return models
}

// GetCompatibleModels returns all models that can run on a GPU with the specified VRAM.
// A model is compatible if its VRAM requirement is <= the available VRAM.
func GetCompatibleModels(vram int) []Model {
	var models []Model
	for _, m := range ModelRegistry {
		if m.VRAM <= vram {
			models = append(models, m)
		}
	}
	return models
}

// GetAllModels returns all models in the registry.
func GetAllModels() []Model {
	result := make([]Model, len(ModelRegistry))
	copy(result, ModelRegistry)
	return result
}

// ParseTier parses a tier string into a Tier constant.
func ParseTier(s string) (Tier, error) {
	switch strings.ToLower(s) {
	case "small":
		return TierSmall, nil
	case "medium":
		return TierMedium, nil
	case "large":
		return TierLarge, nil
	default:
		return "", fmt.Errorf("invalid tier: %s (must be small, medium, or large)", s)
	}
}

// GPU represents a GPU type with its specifications and provider availability.
type GPU struct {
	Name      string   // GPU name (e.g., "A100-40GB")
	VRAM      int      // VRAM in GB
	Providers []string // List of provider IDs that offer this GPU
}

// GPURegistry contains all supported GPU types.
// GPUs are from PRD Section 3.4.
var GPURegistry = []GPU{
	{Name: "A6000", VRAM: 48, Providers: []string{"vast", "runpod"}},
	{Name: "A100-40GB", VRAM: 40, Providers: []string{"vast", "lambda", "runpod", "coreweave", "paperspace"}},
	{Name: "A100-80GB", VRAM: 80, Providers: []string{"vast", "lambda", "runpod", "coreweave", "paperspace"}},
	{Name: "H100-80GB", VRAM: 80, Providers: []string{"lambda", "coreweave"}},
}

// ErrGPUNotFound is returned when a GPU is not found in the registry.
var ErrGPUNotFound = fmt.Errorf("GPU not found")

// GetGPUByName returns a GPU by its exact name.
func GetGPUByName(name string) (*GPU, error) {
	for i := range GPURegistry {
		if GPURegistry[i].Name == name {
			return &GPURegistry[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrGPUNotFound, name)
}

// GetGPUsForProvider returns all GPUs available for the specified provider.
func GetGPUsForProvider(providerID string) []GPU {
	var gpus []GPU
	for _, g := range GPURegistry {
		for _, p := range g.Providers {
			if p == providerID {
				gpus = append(gpus, g)
				break
			}
		}
	}
	return gpus
}

// GetAllGPUs returns all GPUs in the registry.
func GetAllGPUs() []GPU {
	result := make([]GPU, len(GPURegistry))
	copy(result, GPURegistry)
	return result
}

// IsModelCompatible checks if a model can run on a specific GPU.
// A model is compatible if its VRAM requirement is <= the GPU's VRAM.
func IsModelCompatible(gpu *GPU, model *Model) bool {
	if gpu == nil || model == nil {
		return false
	}
	return model.VRAM <= gpu.VRAM
}

// GetCompatibleGPUs returns all GPUs that can run the specified model.
func GetCompatibleGPUs(model *Model) []GPU {
	if model == nil {
		return nil
	}
	var gpus []GPU
	for _, g := range GPURegistry {
		if model.VRAM <= g.VRAM {
			gpus = append(gpus, g)
		}
	}
	return gpus
}

// SupportsProvider checks if this GPU is available from the specified provider.
func (g GPU) SupportsProvider(providerID string) bool {
	for _, p := range g.Providers {
		if p == providerID {
			return true
		}
	}
	return false
}
