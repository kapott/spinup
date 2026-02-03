package models

import (
	"testing"
)

func TestModelRegistryCount(t *testing.T) {
	// PRD Section 3.4 defines 11 models (originally said 10, but PRD actually has 11)
	allModels := GetAllModels()
	if len(allModels) != 11 {
		t.Errorf("expected 11 models in registry, got %d", len(allModels))
	}
}

func TestGetCompatibleModelsA100_40GB(t *testing.T) {
	// A100-40GB has 40GB VRAM
	compatible := GetCompatibleModels(40)

	// Should include:
	// - All small tier (VRAM: 8GB): 4 models
	// - qwen2.5-coder:14b (VRAM: 16GB)
	// - qwen2.5-coder:32b (VRAM: 35GB)
	// - deepseek-coder:33b (VRAM: 36GB)
	// - codellama:34b (VRAM: 36GB)
	// - codellama:70b (VRAM: 40GB) - exactly fits
	// Total: 9 models

	if len(compatible) != 9 {
		t.Errorf("expected 9 models compatible with 40GB VRAM, got %d", len(compatible))
		for _, m := range compatible {
			t.Logf("  - %s (VRAM: %dGB)", m.Name, m.VRAM)
		}
	}

	// Verify none exceed 40GB
	for _, m := range compatible {
		if m.VRAM > 40 {
			t.Errorf("model %s requires %dGB VRAM but was returned for 40GB", m.Name, m.VRAM)
		}
	}
}

func TestGetModelsByTier(t *testing.T) {
	smallModels := GetModelsByTier(TierSmall)
	mediumModels := GetModelsByTier(TierMedium)
	largeModels := GetModelsByTier(TierLarge)

	// PRD defines:
	// - Small: 4 models (7-14B range in description, but 4 actual models with 7B or less)
	// - Medium: 4 models (14-35B)
	// - Large: 3 models (70B+)

	if len(smallModels) != 4 {
		t.Errorf("expected 4 small tier models, got %d", len(smallModels))
	}

	if len(mediumModels) != 4 {
		t.Errorf("expected 4 medium tier models, got %d", len(mediumModels))
	}

	if len(largeModels) != 3 {
		t.Errorf("expected 3 large tier models, got %d", len(largeModels))
	}
}

func TestGetModelByName(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		wantVRAM  int
		wantErr   bool
	}{
		{
			name:      "find qwen2.5-coder:32b",
			modelName: "qwen2.5-coder:32b",
			wantVRAM:  35,
			wantErr:   false,
		},
		{
			name:      "find codellama:70b",
			modelName: "codellama:70b",
			wantVRAM:  40,
			wantErr:   false,
		},
		{
			name:      "not found",
			modelName: "nonexistent:model",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := GetModelByName(tt.modelName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for model %s, got nil", tt.modelName)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if model.VRAM != tt.wantVRAM {
				t.Errorf("expected VRAM %d, got %d", tt.wantVRAM, model.VRAM)
			}
		})
	}
}

func TestQualityStars(t *testing.T) {
	model := Model{Quality: 3}
	stars := model.QualityStars()

	expected := "★★★☆☆"
	if stars != expected {
		t.Errorf("expected quality stars %q, got %q", expected, stars)
	}
}

func TestParseTier(t *testing.T) {
	tests := []struct {
		input   string
		want    Tier
		wantErr bool
	}{
		{"small", TierSmall, false},
		{"medium", TierMedium, false},
		{"large", TierLarge, false},
		{"SMALL", TierSmall, false},  // case insensitive
		{"Medium", TierMedium, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTier(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("expected tier %v, got %v", tt.want, got)
			}
		})
	}
}

// GPU Registry Tests

func TestGPURegistryCount(t *testing.T) {
	// PRD Section 3.4 defines 4 GPU types
	allGPUs := GetAllGPUs()
	if len(allGPUs) != 4 {
		t.Errorf("expected 4 GPUs in registry, got %d", len(allGPUs))
	}
}

func TestGetGPUByName(t *testing.T) {
	tests := []struct {
		name     string
		gpuName  string
		wantVRAM int
		wantErr  bool
	}{
		{
			name:     "find A100-40GB",
			gpuName:  "A100-40GB",
			wantVRAM: 40,
			wantErr:  false,
		},
		{
			name:     "find A100-80GB",
			gpuName:  "A100-80GB",
			wantVRAM: 80,
			wantErr:  false,
		},
		{
			name:     "find A6000",
			gpuName:  "A6000",
			wantVRAM: 48,
			wantErr:  false,
		},
		{
			name:     "find H100-80GB",
			gpuName:  "H100-80GB",
			wantVRAM: 80,
			wantErr:  false,
		},
		{
			name:    "not found",
			gpuName: "RTX-4090",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpu, err := GetGPUByName(tt.gpuName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for GPU %s, got nil", tt.gpuName)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if gpu.VRAM != tt.wantVRAM {
				t.Errorf("expected VRAM %d, got %d", tt.wantVRAM, gpu.VRAM)
			}
		})
	}
}

func TestGetGPUsForProvider(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		wantCount  int
		wantGPUs   []string
	}{
		{
			name:       "vast.ai GPUs",
			providerID: "vast",
			wantCount:  3,
			wantGPUs:   []string{"A6000", "A100-40GB", "A100-80GB"},
		},
		{
			name:       "lambda GPUs",
			providerID: "lambda",
			wantCount:  3,
			wantGPUs:   []string{"A100-40GB", "A100-80GB", "H100-80GB"},
		},
		{
			name:       "runpod GPUs",
			providerID: "runpod",
			wantCount:  3,
			wantGPUs:   []string{"A6000", "A100-40GB", "A100-80GB"},
		},
		{
			name:       "coreweave GPUs",
			providerID: "coreweave",
			wantCount:  3,
			wantGPUs:   []string{"A100-40GB", "A100-80GB", "H100-80GB"},
		},
		{
			name:       "paperspace GPUs",
			providerID: "paperspace",
			wantCount:  2,
			wantGPUs:   []string{"A100-40GB", "A100-80GB"},
		},
		{
			name:       "unknown provider",
			providerID: "unknown",
			wantCount:  0,
			wantGPUs:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpus := GetGPUsForProvider(tt.providerID)
			if len(gpus) != tt.wantCount {
				t.Errorf("expected %d GPUs for provider %s, got %d", tt.wantCount, tt.providerID, len(gpus))
				for _, g := range gpus {
					t.Logf("  - %s", g.Name)
				}
			}

			// Verify expected GPUs are present
			for _, wantGPU := range tt.wantGPUs {
				found := false
				for _, g := range gpus {
					if g.Name == wantGPU {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected GPU %s for provider %s, but not found", wantGPU, tt.providerID)
				}
			}
		})
	}
}

func TestIsModelCompatible(t *testing.T) {
	a100_40, _ := GetGPUByName("A100-40GB")
	a100_80, _ := GetGPUByName("A100-80GB")
	a6000, _ := GetGPUByName("A6000")

	qwen32b, _ := GetModelByName("qwen2.5-coder:32b")   // VRAM: 35GB
	codellama70b, _ := GetModelByName("codellama:70b")  // VRAM: 40GB
	qwen72b, _ := GetModelByName("qwen2.5-coder:72b")   // VRAM: 45GB
	deepseek236b, _ := GetModelByName("deepseek-coder-v2:236b") // VRAM: 120GB

	tests := []struct {
		name       string
		gpu        *GPU
		model      *Model
		compatible bool
	}{
		{
			name:       "qwen32b on A100-40GB (35GB <= 40GB)",
			gpu:        a100_40,
			model:      qwen32b,
			compatible: true,
		},
		{
			name:       "codellama70b on A100-40GB (40GB <= 40GB)",
			gpu:        a100_40,
			model:      codellama70b,
			compatible: true,
		},
		{
			name:       "qwen72b on A100-40GB (45GB > 40GB)",
			gpu:        a100_40,
			model:      qwen72b,
			compatible: false,
		},
		{
			name:       "qwen72b on A100-80GB (45GB <= 80GB)",
			gpu:        a100_80,
			model:      qwen72b,
			compatible: true,
		},
		{
			name:       "deepseek236b on A100-80GB (120GB > 80GB)",
			gpu:        a100_80,
			model:      deepseek236b,
			compatible: false,
		},
		{
			name:       "qwen32b on A6000 (35GB <= 48GB)",
			gpu:        a6000,
			model:      qwen32b,
			compatible: true,
		},
		{
			name:       "nil GPU",
			gpu:        nil,
			model:      qwen32b,
			compatible: false,
		},
		{
			name:       "nil model",
			gpu:        a100_40,
			model:      nil,
			compatible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsModelCompatible(tt.gpu, tt.model)
			if result != tt.compatible {
				t.Errorf("expected IsModelCompatible to return %v, got %v", tt.compatible, result)
			}
		})
	}
}

func TestGetCompatibleGPUs(t *testing.T) {
	// qwen2.5-coder:7b requires 8GB VRAM - should fit on all 4 GPUs
	qwen7b, _ := GetModelByName("qwen2.5-coder:7b")
	compatibleGPUs := GetCompatibleGPUs(qwen7b)
	if len(compatibleGPUs) != 4 {
		t.Errorf("expected 4 compatible GPUs for 8GB model, got %d", len(compatibleGPUs))
	}

	// codellama:70b requires 40GB VRAM - should fit on all 4 GPUs (40, 48, 80, 80)
	codellama70b, _ := GetModelByName("codellama:70b")
	compatibleGPUs = GetCompatibleGPUs(codellama70b)
	if len(compatibleGPUs) != 4 {
		t.Errorf("expected 4 compatible GPUs for 40GB model, got %d", len(compatibleGPUs))
	}

	// qwen2.5-coder:72b requires 45GB VRAM - should fit on 3 GPUs (A6000-48GB, A100-80GB, H100-80GB)
	qwen72b, _ := GetModelByName("qwen2.5-coder:72b")
	compatibleGPUs = GetCompatibleGPUs(qwen72b)
	if len(compatibleGPUs) != 3 {
		t.Errorf("expected 3 compatible GPUs for 45GB model, got %d", len(compatibleGPUs))
	}

	// deepseek-coder-v2:236b requires 120GB VRAM - should fit on 0 GPUs (max is 80GB)
	deepseek236b, _ := GetModelByName("deepseek-coder-v2:236b")
	compatibleGPUs = GetCompatibleGPUs(deepseek236b)
	if len(compatibleGPUs) != 0 {
		t.Errorf("expected 0 compatible GPUs for 120GB model, got %d", len(compatibleGPUs))
	}

	// nil model
	compatibleGPUs = GetCompatibleGPUs(nil)
	if compatibleGPUs != nil {
		t.Errorf("expected nil for nil model, got %v", compatibleGPUs)
	}
}

func TestGPUSupportsProvider(t *testing.T) {
	a6000, _ := GetGPUByName("A6000")
	h100, _ := GetGPUByName("H100-80GB")

	// A6000 is available from vast and runpod only
	if !a6000.SupportsProvider("vast") {
		t.Errorf("A6000 should support vast")
	}
	if !a6000.SupportsProvider("runpod") {
		t.Errorf("A6000 should support runpod")
	}
	if a6000.SupportsProvider("lambda") {
		t.Errorf("A6000 should not support lambda")
	}

	// H100 is available from lambda and coreweave only
	if !h100.SupportsProvider("lambda") {
		t.Errorf("H100 should support lambda")
	}
	if !h100.SupportsProvider("coreweave") {
		t.Errorf("H100 should support coreweave")
	}
	if h100.SupportsProvider("vast") {
		t.Errorf("H100 should not support vast")
	}
}

// ============================================================================
// Additional Unit Tests for F057: Model/GPU Compatibility Edge Cases
// ============================================================================

// TestVRAMExactMatchBoundary tests edge case where VRAM exactly matches requirement
func TestVRAMExactMatchBoundary(t *testing.T) {
	tests := []struct {
		name       string
		modelName  string
		gpuName    string
		shouldFit  bool
		vramMatch  string // "exact", "over", "under"
	}{
		// Exact match: codellama:70b (40GB) on A100-40GB (40GB)
		{
			name:      "exact match - 40GB model on 40GB GPU",
			modelName: "codellama:70b",
			gpuName:   "A100-40GB",
			shouldFit: true,
			vramMatch: "exact",
		},
		// Just over: qwen2.5-coder:72b (45GB) on A100-40GB (40GB)
		{
			name:      "5GB over - 45GB model on 40GB GPU",
			modelName: "qwen2.5-coder:72b",
			gpuName:   "A100-40GB",
			shouldFit: false,
			vramMatch: "over",
		},
		// Just under: deepseek-coder:33b (36GB) on A100-40GB (40GB)
		{
			name:      "4GB under - 36GB model on 40GB GPU",
			modelName: "deepseek-coder:33b",
			gpuName:   "A100-40GB",
			shouldFit: true,
			vramMatch: "under",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := GetModelByName(tt.modelName)
			if err != nil {
				t.Fatalf("failed to get model %s: %v", tt.modelName, err)
			}

			gpu, err := GetGPUByName(tt.gpuName)
			if err != nil {
				t.Fatalf("failed to get GPU %s: %v", tt.gpuName, err)
			}

			result := IsModelCompatible(gpu, model)
			if result != tt.shouldFit {
				t.Errorf("IsModelCompatible(%s, %s) = %v, want %v (VRAM: %dGB vs %dGB, match type: %s)",
					gpu.Name, model.Name, result, tt.shouldFit, model.VRAM, gpu.VRAM, tt.vramMatch)
			}
		})
	}
}

// TestAllModelsAgainstAllGPUs tests compatibility matrix for all model/GPU combinations
func TestAllModelsAgainstAllGPUs(t *testing.T) {
	allModels := GetAllModels()
	allGPUs := GetAllGPUs()

	// Verify we have the expected counts
	if len(allModels) != 11 {
		t.Fatalf("expected 11 models, got %d", len(allModels))
	}
	if len(allGPUs) != 4 {
		t.Fatalf("expected 4 GPUs, got %d", len(allGPUs))
	}

	for _, model := range allModels {
		for _, gpu := range allGPUs {
			result := IsModelCompatible(&gpu, &model)
			expected := model.VRAM <= gpu.VRAM

			if result != expected {
				t.Errorf("IsModelCompatible(%s[%dGB], %s[%dGB]) = %v, want %v",
					gpu.Name, gpu.VRAM, model.Name, model.VRAM, result, expected)
			}
		}
	}
}

// TestCompatibleModelsVRAMThresholds tests GetCompatibleModels at specific VRAM boundaries
func TestCompatibleModelsVRAMThresholds(t *testing.T) {
	tests := []struct {
		vram      int
		wantCount int
		desc      string
	}{
		{vram: 0, wantCount: 0, desc: "no models fit in 0GB"},
		{vram: 7, wantCount: 0, desc: "no models fit in 7GB (all need 8GB+)"},
		{vram: 8, wantCount: 4, desc: "4 small models fit in exactly 8GB"},
		{vram: 15, wantCount: 4, desc: "still 4 models at 15GB (next needs 16GB)"},
		{vram: 16, wantCount: 5, desc: "5 models fit at 16GB (adds qwen2.5-coder:14b)"},
		{vram: 35, wantCount: 6, desc: "6 models fit at 35GB (adds qwen2.5-coder:32b)"},
		{vram: 36, wantCount: 8, desc: "8 models fit at 36GB (adds deepseek-coder:33b, codellama:34b)"},
		{vram: 39, wantCount: 8, desc: "still 8 at 39GB"},
		{vram: 40, wantCount: 9, desc: "9 models fit at 40GB (adds codellama:70b)"},
		{vram: 44, wantCount: 9, desc: "still 9 at 44GB"},
		{vram: 45, wantCount: 10, desc: "10 models fit at 45GB (adds qwen2.5-coder:72b)"},
		{vram: 80, wantCount: 10, desc: "still 10 at 80GB"},
		{vram: 119, wantCount: 10, desc: "still 10 at 119GB"},
		{vram: 120, wantCount: 11, desc: "all 11 models fit at 120GB (adds deepseek-coder-v2:236b)"},
		{vram: 256, wantCount: 11, desc: "all 11 models fit at 256GB"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			models := GetCompatibleModels(tt.vram)
			if len(models) != tt.wantCount {
				t.Errorf("GetCompatibleModels(%d) returned %d models, want %d",
					tt.vram, len(models), tt.wantCount)

				// Log which models were returned for debugging
				for _, m := range models {
					t.Logf("  - %s (VRAM: %dGB)", m.Name, m.VRAM)
				}
			}

			// Verify all returned models actually fit
			for _, m := range models {
				if m.VRAM > tt.vram {
					t.Errorf("model %s requires %dGB but was returned for %dGB VRAM",
						m.Name, m.VRAM, tt.vram)
				}
			}
		})
	}
}

// TestCompatibleGPUsForAllModels tests GetCompatibleGPUs for each model
func TestCompatibleGPUsForAllModels(t *testing.T) {
	tests := []struct {
		modelName    string
		wantGPUCount int
		wantGPUs     []string
	}{
		// Small models (8GB) - fit on all 4 GPUs
		{"qwen2.5-coder:7b", 4, []string{"A6000", "A100-40GB", "A100-80GB", "H100-80GB"}},
		{"deepseek-coder:6.7b", 4, []string{"A6000", "A100-40GB", "A100-80GB", "H100-80GB"}},
		{"codellama:7b", 4, []string{"A6000", "A100-40GB", "A100-80GB", "H100-80GB"}},
		{"starcoder2:7b", 4, []string{"A6000", "A100-40GB", "A100-80GB", "H100-80GB"}},

		// Medium models - varying compatibility
		{"qwen2.5-coder:14b", 4, []string{"A6000", "A100-40GB", "A100-80GB", "H100-80GB"}}, // 16GB
		{"qwen2.5-coder:32b", 4, []string{"A6000", "A100-40GB", "A100-80GB", "H100-80GB"}}, // 35GB
		{"deepseek-coder:33b", 4, []string{"A6000", "A100-40GB", "A100-80GB", "H100-80GB"}}, // 36GB
		{"codellama:34b", 4, []string{"A6000", "A100-40GB", "A100-80GB", "H100-80GB"}},      // 36GB

		// Large models - limited compatibility
		{"codellama:70b", 4, []string{"A6000", "A100-40GB", "A100-80GB", "H100-80GB"}},     // 40GB - exact match with A100-40GB
		{"qwen2.5-coder:72b", 3, []string{"A6000", "A100-80GB", "H100-80GB"}},              // 45GB - doesn't fit A100-40GB
		{"deepseek-coder-v2:236b", 0, nil},                                                  // 120GB - doesn't fit any
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			model, err := GetModelByName(tt.modelName)
			if err != nil {
				t.Fatalf("failed to get model %s: %v", tt.modelName, err)
			}

			gpus := GetCompatibleGPUs(model)
			if len(gpus) != tt.wantGPUCount {
				t.Errorf("GetCompatibleGPUs(%s) returned %d GPUs, want %d",
					tt.modelName, len(gpus), tt.wantGPUCount)
				for _, g := range gpus {
					t.Logf("  - %s (%dGB)", g.Name, g.VRAM)
				}
			}

			// Verify expected GPUs are present
			for _, wantGPU := range tt.wantGPUs {
				found := false
				for _, g := range gpus {
					if g.Name == wantGPU {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected GPU %s to be compatible with %s", wantGPU, tt.modelName)
				}
			}
		})
	}
}

// TestTierFilteringComprehensive tests GetModelsByTier for all tiers with detailed verification
func TestTierFilteringComprehensive(t *testing.T) {
	tests := []struct {
		tier       Tier
		wantCount  int
		wantModels []string
	}{
		{
			tier:      TierSmall,
			wantCount: 4,
			wantModels: []string{
				"qwen2.5-coder:7b",
				"deepseek-coder:6.7b",
				"codellama:7b",
				"starcoder2:7b",
			},
		},
		{
			tier:      TierMedium,
			wantCount: 4,
			wantModels: []string{
				"qwen2.5-coder:14b",
				"qwen2.5-coder:32b",
				"deepseek-coder:33b",
				"codellama:34b",
			},
		},
		{
			tier:      TierLarge,
			wantCount: 3,
			wantModels: []string{
				"codellama:70b",
				"qwen2.5-coder:72b",
				"deepseek-coder-v2:236b",
			},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			models := GetModelsByTier(tt.tier)

			if len(models) != tt.wantCount {
				t.Errorf("GetModelsByTier(%s) returned %d models, want %d",
					tt.tier, len(models), tt.wantCount)
			}

			// Verify all expected models are present
			for _, wantModel := range tt.wantModels {
				found := false
				for _, m := range models {
					if m.Name == wantModel {
						found = true
						// Also verify the model actually has the correct tier
						if m.Tier != tt.tier {
							t.Errorf("model %s has tier %s, expected %s",
								m.Name, m.Tier, tt.tier)
						}
						break
					}
				}
				if !found {
					t.Errorf("expected model %s in tier %s, but not found", wantModel, tt.tier)
				}
			}

			// Verify no models have wrong tier
			for _, m := range models {
				if m.Tier != tt.tier {
					t.Errorf("GetModelsByTier(%s) returned model %s with tier %s",
						tt.tier, m.Name, m.Tier)
				}
			}
		})
	}
}

// TestProviderGPUCompatibilityMatrix tests provider/GPU compatibility comprehensively
func TestProviderGPUCompatibilityMatrix(t *testing.T) {
	// Expected provider/GPU availability from PRD Section 3.4
	providerGPUs := map[string][]string{
		"vast":       {"A6000", "A100-40GB", "A100-80GB"},
		"lambda":     {"A100-40GB", "A100-80GB", "H100-80GB"},
		"runpod":     {"A6000", "A100-40GB", "A100-80GB"},
		"coreweave":  {"A100-40GB", "A100-80GB", "H100-80GB"},
		"paperspace": {"A100-40GB", "A100-80GB"},
	}

	// Test GetGPUsForProvider returns correct GPUs
	for provider, expectedGPUs := range providerGPUs {
		t.Run("GetGPUsForProvider_"+provider, func(t *testing.T) {
			gpus := GetGPUsForProvider(provider)

			if len(gpus) != len(expectedGPUs) {
				t.Errorf("GetGPUsForProvider(%s) returned %d GPUs, want %d",
					provider, len(gpus), len(expectedGPUs))
			}

			for _, wantGPU := range expectedGPUs {
				found := false
				for _, g := range gpus {
					if g.Name == wantGPU {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected GPU %s for provider %s", wantGPU, provider)
				}
			}
		})
	}

	// Test GPU.SupportsProvider returns correct results
	for _, gpu := range GetAllGPUs() {
		for provider, expectedGPUs := range providerGPUs {
			t.Run(gpu.Name+"_supports_"+provider, func(t *testing.T) {
				shouldSupport := false
				for _, g := range expectedGPUs {
					if g == gpu.Name {
						shouldSupport = true
						break
					}
				}

				result := gpu.SupportsProvider(provider)
				if result != shouldSupport {
					t.Errorf("GPU %s.SupportsProvider(%s) = %v, want %v",
						gpu.Name, provider, result, shouldSupport)
				}
			})
		}
	}
}

// TestModelQualityRatings verifies all models have valid quality ratings
func TestModelQualityRatings(t *testing.T) {
	for _, model := range GetAllModels() {
		t.Run(model.Name, func(t *testing.T) {
			// Quality should be 1-5
			if model.Quality < 1 || model.Quality > 5 {
				t.Errorf("model %s has invalid quality %d (should be 1-5)",
					model.Name, model.Quality)
			}

			// Test QualityStars output
			stars := model.QualityStars()
			expectedLen := 5 // Always 5 characters (filled + empty stars)
			if len([]rune(stars)) != expectedLen {
				t.Errorf("QualityStars() for %s returned %d chars, want %d",
					model.Name, len([]rune(stars)), expectedLen)
			}

			// Count filled stars - should match quality
			filledCount := 0
			for _, r := range stars {
				if r == '★' {
					filledCount++
				}
			}
			if filledCount != model.Quality {
				t.Errorf("QualityStars() for %s shows %d filled stars, expected %d",
					model.Name, filledCount, model.Quality)
			}
		})
	}
}

// TestModelVRAMRequirementsSanity verifies VRAM requirements are sensible
func TestModelVRAMRequirementsSanity(t *testing.T) {
	for _, model := range GetAllModels() {
		t.Run(model.Name, func(t *testing.T) {
			// VRAM should be positive
			if model.VRAM <= 0 {
				t.Errorf("model %s has invalid VRAM %d (should be positive)",
					model.Name, model.VRAM)
			}

			// VRAM should be reasonable (1-256GB)
			if model.VRAM > 256 {
				t.Errorf("model %s has unreasonably high VRAM %dGB",
					model.Name, model.VRAM)
			}

			// Larger parameter models should generally need more VRAM
			// (This is a sanity check, not a strict rule)
			if model.Params == "7B" && model.VRAM > 16 {
				t.Errorf("7B model %s has unexpectedly high VRAM %dGB",
					model.Name, model.VRAM)
			}
		})
	}
}

// TestGPUVRAMSanity verifies GPU VRAM values are sensible
func TestGPUVRAMSanity(t *testing.T) {
	for _, gpu := range GetAllGPUs() {
		t.Run(gpu.Name, func(t *testing.T) {
			// VRAM should be positive
			if gpu.VRAM <= 0 {
				t.Errorf("GPU %s has invalid VRAM %d (should be positive)",
					gpu.Name, gpu.VRAM)
			}

			// VRAM should match name for known GPUs
			switch gpu.Name {
			case "A100-40GB":
				if gpu.VRAM != 40 {
					t.Errorf("A100-40GB should have 40GB VRAM, got %dGB", gpu.VRAM)
				}
			case "A100-80GB":
				if gpu.VRAM != 80 {
					t.Errorf("A100-80GB should have 80GB VRAM, got %dGB", gpu.VRAM)
				}
			case "H100-80GB":
				if gpu.VRAM != 80 {
					t.Errorf("H100-80GB should have 80GB VRAM, got %dGB", gpu.VRAM)
				}
			case "A6000":
				if gpu.VRAM != 48 {
					t.Errorf("A6000 should have 48GB VRAM, got %dGB", gpu.VRAM)
				}
			}

			// Should have at least one provider
			if len(gpu.Providers) == 0 {
				t.Errorf("GPU %s has no providers", gpu.Name)
			}
		})
	}
}
