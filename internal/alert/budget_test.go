package alert

import (
	"context"
	"testing"
)

func TestBudgetThresholdString(t *testing.T) {
	tests := []struct {
		threshold BudgetThreshold
		expected  string
	}{
		{BudgetThresholdNone, "none"},
		{BudgetThreshold80, "80%"},
		{BudgetThreshold100, "100%"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.threshold.String(); got != tt.expected {
				t.Errorf("BudgetThreshold.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBudgetCheckerIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		budget   float64
		expected bool
	}{
		{"positive budget", 20.0, true},
		{"zero budget", 0, false},
		{"negative budget", -10.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBudgetChecker(tt.budget)
			if got := bc.IsEnabled(); got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBudgetCheckerGetThreshold(t *testing.T) {
	bc := NewBudgetChecker(100.0) // €100 budget

	tests := []struct {
		name        string
		accumulated float64
		expected    BudgetThreshold
	}{
		{"0%", 0, BudgetThresholdNone},
		{"50%", 50, BudgetThresholdNone},
		{"79%", 79, BudgetThresholdNone},
		{"80%", 80, BudgetThreshold80},
		{"90%", 90, BudgetThreshold80},
		{"99%", 99, BudgetThreshold80},
		{"100%", 100, BudgetThreshold100},
		{"150%", 150, BudgetThreshold100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bc.GetThreshold(tt.accumulated); got != tt.expected {
				t.Errorf("GetThreshold(%.0f) = %v, want %v", tt.accumulated, got, tt.expected)
			}
		})
	}
}

func TestBudgetCheckerGetPercentage(t *testing.T) {
	bc := NewBudgetChecker(100.0) // €100 budget

	tests := []struct {
		name        string
		accumulated float64
		expected    float64
	}{
		{"0%", 0, 0},
		{"50%", 50, 50},
		{"100%", 100, 100},
		{"150%", 150, 150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bc.GetPercentage(tt.accumulated)
			if got != tt.expected {
				t.Errorf("GetPercentage(%.0f) = %.0f, want %.0f", tt.accumulated, got, tt.expected)
			}
		})
	}
}

func TestBudgetCheckerDisabledBudget(t *testing.T) {
	bc := NewBudgetChecker(0) // Disabled

	// Should always return none when disabled
	if got := bc.GetThreshold(1000); got != BudgetThresholdNone {
		t.Errorf("Disabled checker GetThreshold() = %v, want BudgetThresholdNone", got)
	}

	if got := bc.GetPercentage(1000); got != 0 {
		t.Errorf("Disabled checker GetPercentage() = %v, want 0", got)
	}
}

func TestBudgetCheckerCheckAndAlert(t *testing.T) {
	// Reset global dispatcher for clean test
	ResetDispatcher()

	bc := NewBudgetChecker(100.0)
	ctx := context.Background()
	alertCtx := Context{}

	// First check at 50% - no alert
	threshold := bc.CheckAndAlert(ctx, 50, alertCtx)
	if threshold != BudgetThresholdNone {
		t.Errorf("50%% threshold = %v, want BudgetThresholdNone", threshold)
	}
	if bc.HasAlerted80() || bc.HasAlerted100() {
		t.Error("Should not have alerted at 50%")
	}

	// Check at 85% - should trigger 80% alert
	threshold = bc.CheckAndAlert(ctx, 85, alertCtx)
	if threshold != BudgetThreshold80 {
		t.Errorf("85%% threshold = %v, want BudgetThreshold80", threshold)
	}
	if !bc.HasAlerted80() {
		t.Error("Should have alerted at 80%")
	}
	if bc.HasAlerted100() {
		t.Error("Should not have alerted 100% yet")
	}

	// Check again at 85% - should not re-alert
	bc.CheckAndAlert(ctx, 85, alertCtx)
	// Just verifying no panic or duplicate alerts (would need mock to verify)

	// Check at 105% - should trigger 100% alert
	threshold = bc.CheckAndAlert(ctx, 105, alertCtx)
	if threshold != BudgetThreshold100 {
		t.Errorf("105%% threshold = %v, want BudgetThreshold100", threshold)
	}
	if !bc.HasAlerted100() {
		t.Error("Should have alerted at 100%")
	}

	// Check again at 120% - should not re-alert
	bc.CheckAndAlert(ctx, 120, alertCtx)
	// Just verifying no panic or duplicate alerts
}

func TestBudgetCheckerReset(t *testing.T) {
	bc := NewBudgetChecker(100.0)
	ctx := context.Background()
	alertCtx := Context{}

	// Trigger both alerts
	bc.CheckAndAlert(ctx, 100, alertCtx)
	if !bc.HasAlerted80() || !bc.HasAlerted100() {
		t.Error("Both alerts should have triggered")
	}

	// Reset
	bc.Reset()

	if bc.HasAlerted80() || bc.HasAlerted100() {
		t.Error("Alerts should be reset")
	}
}

func TestBudgetCheckerGetBudgetStatus(t *testing.T) {
	bc := NewBudgetChecker(100.0)

	status := bc.GetBudgetStatus(75)

	if !status.Enabled {
		t.Error("Status should be enabled")
	}
	if status.DailyBudgetEUR != 100 {
		t.Errorf("DailyBudgetEUR = %v, want 100", status.DailyBudgetEUR)
	}
	if status.AccumulatedEUR != 75 {
		t.Errorf("AccumulatedEUR = %v, want 75", status.AccumulatedEUR)
	}
	if status.Percentage != 75 {
		t.Errorf("Percentage = %v, want 75", status.Percentage)
	}
	if status.RemainingEUR != 25 {
		t.Errorf("RemainingEUR = %v, want 25", status.RemainingEUR)
	}
	if status.Threshold != BudgetThresholdNone {
		t.Errorf("Threshold = %v, want BudgetThresholdNone", status.Threshold)
	}
}

func TestBudgetCheckerGetBudgetStatusOverBudget(t *testing.T) {
	bc := NewBudgetChecker(100.0)

	status := bc.GetBudgetStatus(120)

	if status.RemainingEUR != 0 {
		t.Errorf("RemainingEUR should be 0 when over budget, got %v", status.RemainingEUR)
	}
	if status.Threshold != BudgetThreshold100 {
		t.Errorf("Threshold = %v, want BudgetThreshold100", status.Threshold)
	}
}

func TestBudgetCheckerDisabledStatus(t *testing.T) {
	bc := NewBudgetChecker(0) // Disabled

	status := bc.GetBudgetStatus(100)

	if status.Enabled {
		t.Error("Status should not be enabled for zero budget")
	}
}

func TestGlobalBudgetChecker(t *testing.T) {
	// Reset for clean test
	ResetBudgetChecker()

	// Before init, should be nil
	if GetBudgetChecker() != nil {
		t.Error("GetBudgetChecker should be nil before init")
	}

	// Init
	InitBudgetChecker(50.0)

	bc := GetBudgetChecker()
	if bc == nil {
		t.Fatal("GetBudgetChecker should not be nil after init")
	}
	if bc.DailyBudget() != 50.0 {
		t.Errorf("DailyBudget = %v, want 50.0", bc.DailyBudget())
	}

	// CheckBudget should work
	ctx := context.Background()
	threshold := CheckBudget(ctx, 45, Context{})
	if threshold != BudgetThreshold80 {
		t.Errorf("CheckBudget at 90%% = %v, want BudgetThreshold80", threshold)
	}

	// Reset for other tests
	ResetBudgetChecker()
}

func TestCheckBudgetWithoutInit(t *testing.T) {
	ResetBudgetChecker()

	ctx := context.Background()
	threshold := CheckBudget(ctx, 100, Context{})
	if threshold != BudgetThresholdNone {
		t.Errorf("CheckBudget without init = %v, want BudgetThresholdNone", threshold)
	}
}
