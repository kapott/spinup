// Package config provides configuration and state management for continueplz.
package config

import (
	"math"
	"testing"
	"time"
)

// floatEquals compares two floats with tolerance.
func floatEquals(a, b float64) bool {
	const epsilon = 0.0001
	return math.Abs(a-b) < epsilon
}

// TestCostCalculationHourlyToDaily tests hourly to daily cost conversion.
func TestCostCalculationHourlyToDaily(t *testing.T) {
	tests := []struct {
		name            string
		hourlyRate      float64
		hoursRunning    float64
		expectedCost    float64
		description     string
	}{
		{
			name:         "standard 8 hour workday",
			hourlyRate:   0.50,
			hoursRunning: 8.0,
			expectedCost: 4.0,
			description:  "0.50 EUR/hr * 8 hours = 4.00 EUR",
		},
		{
			name:         "full 24 hour day",
			hourlyRate:   0.50,
			hoursRunning: 24.0,
			expectedCost: 12.0,
			description:  "0.50 EUR/hr * 24 hours = 12.00 EUR",
		},
		{
			name:         "partial hour",
			hourlyRate:   1.0,
			hoursRunning: 0.5,
			expectedCost: 0.5,
			description:  "1.00 EUR/hr * 0.5 hours = 0.50 EUR",
		},
		{
			name:         "high hourly rate",
			hourlyRate:   3.50,
			hoursRunning: 8.0,
			expectedCost: 28.0,
			description:  "3.50 EUR/hr (H100) * 8 hours = 28.00 EUR",
		},
		{
			name:         "zero hourly rate",
			hourlyRate:   0.0,
			hoursRunning: 8.0,
			expectedCost: 0.0,
			description:  "Free tier with 0 EUR/hr",
		},
		{
			name:         "zero hours",
			hourlyRate:   0.50,
			hoursRunning: 0.0,
			expectedCost: 0.0,
			description:  "Instance just started (0 hours)",
		},
		{
			name:         "very small duration",
			hourlyRate:   0.50,
			hoursRunning: 0.001, // 3.6 seconds
			expectedCost: 0.0005,
			description:  "Very short run time",
		},
		{
			name:         "multi-day run",
			hourlyRate:   0.50,
			hoursRunning: 72.0, // 3 days
			expectedCost: 36.0,
			description:  "3 days continuous running",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate cost using simple multiplication (as used in CalculateAccumulatedCost)
			cost := tc.hourlyRate * tc.hoursRunning

			if !floatEquals(cost, tc.expectedCost) {
				t.Errorf("%s: expected %.4f, got %.4f", tc.description, tc.expectedCost, cost)
			}
		})
	}
}

// TestCostCalculationAccumulatedWithDuration tests CalculateAccumulatedCost method.
func TestCostCalculationAccumulatedWithDuration(t *testing.T) {
	tests := []struct {
		name           string
		hourlyRate     float64
		createdAt      time.Time
		expectedMinCost float64
		expectedMaxCost float64
	}{
		{
			name:           "1 hour running",
			hourlyRate:     0.50,
			createdAt:      time.Now().Add(-1 * time.Hour),
			expectedMinCost: 0.49, // Allow slight tolerance
			expectedMaxCost: 0.51,
		},
		{
			name:           "2 hours running",
			hourlyRate:     0.50,
			createdAt:      time.Now().Add(-2 * time.Hour),
			expectedMinCost: 0.99,
			expectedMaxCost: 1.01,
		},
		{
			name:           "30 minutes running",
			hourlyRate:     1.0,
			createdAt:      time.Now().Add(-30 * time.Minute),
			expectedMinCost: 0.49,
			expectedMaxCost: 0.51,
		},
		{
			name:           "just started",
			hourlyRate:     0.50,
			createdAt:      time.Now(),
			expectedMinCost: 0.0,
			expectedMaxCost: 0.01, // Allow for small time delta
		},
		{
			name:           "high rate 1 hour",
			hourlyRate:     3.50, // H100 price range
			createdAt:      time.Now().Add(-1 * time.Hour),
			expectedMinCost: 3.49,
			expectedMaxCost: 3.51,
		},
		{
			name:           "zero rate",
			hourlyRate:     0.0,
			createdAt:      time.Now().Add(-10 * time.Hour),
			expectedMinCost: 0.0,
			expectedMaxCost: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := &State{
				Instance: &InstanceState{
					CreatedAt: tc.createdAt,
				},
				Cost: &CostState{
					HourlyRate: tc.hourlyRate,
					Currency:   "EUR",
				},
			}

			cost := state.CalculateAccumulatedCost()

			if cost < tc.expectedMinCost || cost > tc.expectedMaxCost {
				t.Errorf("expected cost between %.2f and %.2f, got %.4f",
					tc.expectedMinCost, tc.expectedMaxCost, cost)
			}
		})
	}
}

// TestCostCalculationEdgeCases tests edge cases in cost calculations.
func TestCostCalculationEdgeCases(t *testing.T) {
	t.Run("nil state returns zero", func(t *testing.T) {
		var state *State
		if state.CalculateAccumulatedCost() != 0 {
			t.Error("nil state should return 0 cost")
		}
	})

	t.Run("nil instance returns zero", func(t *testing.T) {
		state := &State{
			Cost: &CostState{HourlyRate: 1.0},
		}
		if state.CalculateAccumulatedCost() != 0 {
			t.Error("state without instance should return 0 cost")
		}
	})

	t.Run("nil cost returns zero", func(t *testing.T) {
		state := &State{
			Instance: &InstanceState{
				CreatedAt: time.Now().Add(-1 * time.Hour),
			},
		}
		if state.CalculateAccumulatedCost() != 0 {
			t.Error("state without cost should return 0 cost")
		}
	})

	t.Run("both nil returns zero", func(t *testing.T) {
		state := &State{Version: 1}
		if state.CalculateAccumulatedCost() != 0 {
			t.Error("state without instance and cost should return 0 cost")
		}
	})

	t.Run("future created_at returns zero or negative", func(t *testing.T) {
		// This is an edge case - instance created "in the future"
		// The cost should be 0 or negative (both are acceptable as defensive behavior)
		state := &State{
			Instance: &InstanceState{
				CreatedAt: time.Now().Add(1 * time.Hour), // Future
			},
			Cost: &CostState{
				HourlyRate: 0.50,
			},
		}
		cost := state.CalculateAccumulatedCost()
		// Future creation time results in negative duration, hence negative cost
		// This is technically correct behavior - the formula is duration * rate
		if cost > 0.01 { // Allow tiny positive due to timing
			t.Errorf("future created_at should not produce positive cost, got %.4f", cost)
		}
	})

	t.Run("very long duration", func(t *testing.T) {
		// Test 30 days running
		state := &State{
			Instance: &InstanceState{
				CreatedAt: time.Now().Add(-30 * 24 * time.Hour), // 30 days ago
			},
			Cost: &CostState{
				HourlyRate: 0.50,
				Currency:   "EUR",
			},
		}
		cost := state.CalculateAccumulatedCost()
		// 30 days * 24 hours * 0.50 = 360 EUR
		expectedCost := 30.0 * 24.0 * 0.50
		if cost < expectedCost*0.99 || cost > expectedCost*1.01 {
			t.Errorf("expected ~%.2f, got %.2f", expectedCost, cost)
		}
	})

	t.Run("very small hourly rate", func(t *testing.T) {
		state := &State{
			Instance: &InstanceState{
				CreatedAt: time.Now().Add(-1 * time.Hour),
			},
			Cost: &CostState{
				HourlyRate: 0.0001, // Very cheap
			},
		}
		cost := state.CalculateAccumulatedCost()
		if !floatEquals(cost, 0.0001) {
			t.Errorf("expected ~0.0001, got %.6f", cost)
		}
	})

	t.Run("very high hourly rate", func(t *testing.T) {
		state := &State{
			Instance: &InstanceState{
				CreatedAt: time.Now().Add(-1 * time.Hour),
			},
			Cost: &CostState{
				HourlyRate: 10.0, // Expensive multi-GPU
			},
		}
		cost := state.CalculateAccumulatedCost()
		if cost < 9.99 || cost > 10.01 {
			t.Errorf("expected ~10.0, got %.2f", cost)
		}
	})
}

// TestCostStateValues tests CostState struct field values.
func TestCostStateValues(t *testing.T) {
	t.Run("standard EUR cost state", func(t *testing.T) {
		cost := &CostState{
			HourlyRate:  0.65,
			Accumulated: 5.20,
			Currency:    "EUR",
		}

		if cost.HourlyRate != 0.65 {
			t.Errorf("HourlyRate: expected 0.65, got %f", cost.HourlyRate)
		}
		if cost.Accumulated != 5.20 {
			t.Errorf("Accumulated: expected 5.20, got %f", cost.Accumulated)
		}
		if cost.Currency != "EUR" {
			t.Errorf("Currency: expected EUR, got %s", cost.Currency)
		}
	})

	t.Run("zero accumulated cost", func(t *testing.T) {
		cost := &CostState{
			HourlyRate:  0.50,
			Accumulated: 0.0,
			Currency:    "EUR",
		}

		// Verify all fields are accessible and have expected values
		if cost.Accumulated != 0.0 {
			t.Errorf("Accumulated: expected 0.0, got %f", cost.Accumulated)
		}
		if cost.HourlyRate != 0.50 {
			t.Errorf("HourlyRate: expected 0.50, got %f", cost.HourlyRate)
		}
		if cost.Currency != "EUR" {
			t.Errorf("Currency: expected EUR, got %s", cost.Currency)
		}
	})

	t.Run("USD currency", func(t *testing.T) {
		cost := &CostState{
			HourlyRate:  0.70, // USD typically higher
			Accumulated: 5.60,
			Currency:    "USD",
		}

		// Verify all fields are accessible and have expected values
		if cost.Currency != "USD" {
			t.Errorf("Currency: expected USD, got %s", cost.Currency)
		}
		if cost.HourlyRate != 0.70 {
			t.Errorf("HourlyRate: expected 0.70, got %f", cost.HourlyRate)
		}
		if cost.Accumulated != 5.60 {
			t.Errorf("Accumulated: expected 5.60, got %f", cost.Accumulated)
		}
	})
}

// TestCostCalculationPrecision tests floating point precision in cost calculations.
func TestCostCalculationPrecision(t *testing.T) {
	t.Run("small increments accumulate correctly", func(t *testing.T) {
		// Simulate 100 small increments
		rate := 0.01
		var total float64
		for range 100 {
			total += rate
		}

		expected := 1.0
		if !floatEquals(total, expected) {
			t.Errorf("expected %.2f, got %.10f (floating point precision)", expected, total)
		}
	})

	t.Run("hourly calculation maintains precision", func(t *testing.T) {
		// 1.5 hours at 0.33 EUR/hr = 0.495 EUR
		hours := 1.5
		rate := 0.33
		expected := 0.495
		actual := hours * rate

		if !floatEquals(actual, expected) {
			t.Errorf("expected %.4f, got %.10f", expected, actual)
		}
	})

	t.Run("storage cost calculation", func(t *testing.T) {
		// Storage typically charged per GB-hour
		// 100 GB at 0.0005 EUR/GB-hr for 8 hours
		storageGB := 100.0
		ratePerGBHour := 0.0005
		hours := 8.0

		storageCost := storageGB * ratePerGBHour * hours
		expected := 0.40

		if !floatEquals(storageCost, expected) {
			t.Errorf("storage cost: expected %.2f, got %.6f", expected, storageCost)
		}
	})

	t.Run("total cost with compute and storage", func(t *testing.T) {
		// Compute: 0.65 EUR/hr * 8 hours = 5.20 EUR
		// Storage: 100 GB * 0.0005 EUR/GB-hr * 8 hours = 0.40 EUR
		// Total: 5.60 EUR
		computeRate := 0.65
		storageRate := 0.0005
		storageGB := 100.0
		hours := 8.0

		computeCost := computeRate * hours
		storageCost := storageRate * storageGB * hours
		totalCost := computeCost + storageCost

		expectedTotal := 5.60
		if !floatEquals(totalCost, expectedTotal) {
			t.Errorf("total cost: expected %.2f, got %.6f", expectedTotal, totalCost)
		}
	})
}

// TestCostCalculationBudgetTracking tests budget threshold tracking.
func TestCostCalculationBudgetTracking(t *testing.T) {
	t.Run("under budget", func(t *testing.T) {
		dailyBudget := 10.0
		currentCost := 5.20
		isOverBudget := currentCost > dailyBudget

		if isOverBudget {
			t.Error("5.20 EUR should be under 10.00 EUR budget")
		}
	})

	t.Run("exactly at budget", func(t *testing.T) {
		dailyBudget := 10.0
		currentCost := 10.0
		isOverBudget := currentCost > dailyBudget

		if isOverBudget {
			t.Error("10.00 EUR should not be over 10.00 EUR budget (equal)")
		}
	})

	t.Run("over budget", func(t *testing.T) {
		dailyBudget := 10.0
		currentCost := 10.01
		isOverBudget := currentCost > dailyBudget

		if !isOverBudget {
			t.Error("10.01 EUR should be over 10.00 EUR budget")
		}
	})

	t.Run("percentage of budget", func(t *testing.T) {
		dailyBudget := 20.0
		currentCost := 15.0
		percentUsed := (currentCost / dailyBudget) * 100

		expected := 75.0
		if !floatEquals(percentUsed, expected) {
			t.Errorf("budget percentage: expected %.1f%%, got %.1f%%", expected, percentUsed)
		}
	})

	t.Run("estimated daily cost", func(t *testing.T) {
		// If current cost is 2.60 EUR after 4 hours, estimate daily (8 hours)
		currentCost := 2.60
		hoursElapsed := 4.0
		workingHours := 8.0

		hourlyRate := currentCost / hoursElapsed
		estimatedDaily := hourlyRate * workingHours

		expected := 5.20
		if !floatEquals(estimatedDaily, expected) {
			t.Errorf("estimated daily: expected %.2f, got %.2f", expected, estimatedDaily)
		}
	})
}

// TestCostCalculationSpotVsOnDemand tests spot vs on-demand pricing calculations.
func TestCostCalculationSpotVsOnDemand(t *testing.T) {
	t.Run("spot savings calculation", func(t *testing.T) {
		spotPrice := 0.65
		onDemandPrice := 0.85
		hours := 8.0

		spotCost := spotPrice * hours      // 5.20
		onDemandCost := onDemandPrice * hours // 6.80
		savings := onDemandCost - spotCost   // 1.60

		expectedSpotCost := 5.20
		expectedOnDemandCost := 6.80
		expectedSavings := 1.60

		if !floatEquals(spotCost, expectedSpotCost) {
			t.Errorf("spot cost: expected %.2f, got %.2f", expectedSpotCost, spotCost)
		}
		if !floatEquals(onDemandCost, expectedOnDemandCost) {
			t.Errorf("on-demand cost: expected %.2f, got %.2f", expectedOnDemandCost, onDemandCost)
		}
		if !floatEquals(savings, expectedSavings) {
			t.Errorf("savings: expected %.2f, got %.2f", expectedSavings, savings)
		}
	})

	t.Run("spot savings percentage", func(t *testing.T) {
		spotPrice := 0.65
		onDemandPrice := 0.85

		savingsPercent := ((onDemandPrice - spotPrice) / onDemandPrice) * 100

		// (0.85 - 0.65) / 0.85 * 100 = 23.53%
		expected := 23.53
		if math.Abs(savingsPercent-expected) > 0.01 {
			t.Errorf("savings percent: expected ~%.2f%%, got %.2f%%", expected, savingsPercent)
		}
	})
}

// TestInstanceDurationCalculation tests the Duration() method.
func TestInstanceDurationCalculation(t *testing.T) {
	t.Run("exact 1 hour", func(t *testing.T) {
		inst := &InstanceState{
			CreatedAt: time.Now().Add(-1 * time.Hour),
		}
		dur := inst.Duration()

		// Allow 1 second tolerance for test execution
		if dur < 59*time.Minute+59*time.Second || dur > 1*time.Hour+1*time.Second {
			t.Errorf("expected ~1 hour, got %v", dur)
		}
	})

	t.Run("15 minutes", func(t *testing.T) {
		inst := &InstanceState{
			CreatedAt: time.Now().Add(-15 * time.Minute),
		}
		dur := inst.Duration()

		// Convert to hours for cost calculation
		hours := dur.Hours()
		expectedHours := 0.25

		if math.Abs(hours-expectedHours) > 0.01 {
			t.Errorf("expected ~0.25 hours, got %.4f", hours)
		}
	})

	t.Run("nil instance returns zero", func(t *testing.T) {
		var inst *InstanceState
		if inst.Duration() != 0 {
			t.Error("nil instance should return 0 duration")
		}
	})

	t.Run("hours conversion", func(t *testing.T) {
		// 90 minutes = 1.5 hours
		inst := &InstanceState{
			CreatedAt: time.Now().Add(-90 * time.Minute),
		}
		hours := inst.Duration().Hours()

		if math.Abs(hours-1.5) > 0.01 {
			t.Errorf("expected ~1.5 hours, got %.4f", hours)
		}
	})
}
