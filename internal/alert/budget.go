// Package alert provides alerting functionality for continueplz.
// This file implements budget warning functionality.
package alert

import (
	"context"
	"fmt"
	"sync"

	"github.com/tmeurs/continueplz/internal/logging"
)

// BudgetThreshold represents a budget threshold level.
type BudgetThreshold int

const (
	// BudgetThresholdNone indicates the budget is well within limits.
	BudgetThresholdNone BudgetThreshold = iota
	// BudgetThreshold80 indicates 80% of budget has been used.
	BudgetThreshold80
	// BudgetThreshold100 indicates 100% of budget has been reached or exceeded.
	BudgetThreshold100
)

// String returns the string representation of the threshold.
func (t BudgetThreshold) String() string {
	switch t {
	case BudgetThreshold80:
		return "80%"
	case BudgetThreshold100:
		return "100%"
	default:
		return "none"
	}
}

// BudgetChecker monitors accumulated costs against a daily budget.
// It tracks which thresholds have already been alerted to avoid duplicate notifications.
type BudgetChecker struct {
	mu             sync.Mutex
	dailyBudgetEUR float64
	dispatcher     *Dispatcher
	logger         *logging.Logger

	// Track which thresholds have already triggered alerts (to avoid duplicates)
	alerted80  bool
	alerted100 bool
}

// BudgetCheckerOption is a functional option for configuring BudgetChecker.
type BudgetCheckerOption func(*BudgetChecker)

// WithBudgetDispatcher sets the dispatcher for budget alerts.
func WithBudgetDispatcher(d *Dispatcher) BudgetCheckerOption {
	return func(bc *BudgetChecker) {
		bc.dispatcher = d
	}
}

// WithBudgetLogger sets the logger for budget checking.
func WithBudgetLogger(logger *logging.Logger) BudgetCheckerOption {
	return func(bc *BudgetChecker) {
		bc.logger = logger
	}
}

// NewBudgetChecker creates a new BudgetChecker with the given daily budget.
// If dailyBudgetEUR is 0 or negative, budget checking is effectively disabled
// (no warnings will be triggered).
func NewBudgetChecker(dailyBudgetEUR float64, opts ...BudgetCheckerOption) *BudgetChecker {
	bc := &BudgetChecker{
		dailyBudgetEUR: dailyBudgetEUR,
		dispatcher:     GetDispatcher(),
		logger:         logging.Get(),
	}

	for _, opt := range opts {
		opt(bc)
	}

	return bc
}

// DailyBudget returns the configured daily budget in EUR.
func (bc *BudgetChecker) DailyBudget() float64 {
	return bc.dailyBudgetEUR
}

// IsEnabled returns true if budget checking is enabled (budget > 0).
func (bc *BudgetChecker) IsEnabled() bool {
	return bc.dailyBudgetEUR > 0
}

// GetThreshold returns the current threshold level based on accumulated cost.
// Does not trigger any alerts; use CheckAndAlert for that.
func (bc *BudgetChecker) GetThreshold(accumulatedCostEUR float64) BudgetThreshold {
	if !bc.IsEnabled() || accumulatedCostEUR <= 0 {
		return BudgetThresholdNone
	}

	percentage := (accumulatedCostEUR / bc.dailyBudgetEUR) * 100

	if percentage >= 100 {
		return BudgetThreshold100
	} else if percentage >= 80 {
		return BudgetThreshold80
	}
	return BudgetThresholdNone
}

// GetPercentage returns the percentage of budget used.
func (bc *BudgetChecker) GetPercentage(accumulatedCostEUR float64) float64 {
	if !bc.IsEnabled() || accumulatedCostEUR <= 0 {
		return 0
	}
	return (accumulatedCostEUR / bc.dailyBudgetEUR) * 100
}

// CheckAndAlert checks the accumulated cost against the budget and sends alerts
// if thresholds are crossed. It tracks which alerts have been sent to avoid duplicates.
//
// Returns the current threshold level (regardless of whether an alert was sent).
func (bc *BudgetChecker) CheckAndAlert(ctx context.Context, accumulatedCostEUR float64, alertCtx Context) BudgetThreshold {
	if !bc.IsEnabled() {
		return BudgetThresholdNone
	}

	bc.mu.Lock()
	defer bc.mu.Unlock()

	threshold := bc.GetThreshold(accumulatedCostEUR)
	percentage := bc.GetPercentage(accumulatedCostEUR)

	switch threshold {
	case BudgetThreshold100:
		if !bc.alerted100 {
			bc.alerted100 = true
			bc.alerted80 = true // Also mark 80% as alerted to avoid duplicate

			message := fmt.Sprintf(
				"Budget exceeded: €%.2f / €%.2f (%.1f%%)",
				accumulatedCostEUR, bc.dailyBudgetEUR, percentage,
			)

			bc.logger.Warn().
				Float64("accumulated_eur", accumulatedCostEUR).
				Float64("budget_eur", bc.dailyBudgetEUR).
				Float64("percentage", percentage).
				Msg("Daily budget exceeded")

			// Send CRITICAL alert for budget exceeded (100%+)
			if bc.dispatcher != nil {
				// Send webhook alert for budget exceeded
				if bc.dispatcher.webhookClient != nil {
					go func() {
						_ = bc.dispatcher.webhookClient.SendCritical(ctx, message, alertCtx)
					}()
				}
				// Dispatch through normal channels (log + TUI)
				bc.dispatcher.Dispatch(ctx, LevelCritical, message, alertCtx)
			}
		}

	case BudgetThreshold80:
		if !bc.alerted80 {
			bc.alerted80 = true

			message := fmt.Sprintf(
				"Budget warning: €%.2f / €%.2f (%.1f%%) - approaching daily limit",
				accumulatedCostEUR, bc.dailyBudgetEUR, percentage,
			)

			bc.logger.Warn().
				Float64("accumulated_eur", accumulatedCostEUR).
				Float64("budget_eur", bc.dailyBudgetEUR).
				Float64("percentage", percentage).
				Msg("Approaching daily budget (80%)")

			// Send WARN alert for approaching budget (80%)
			if bc.dispatcher != nil {
				// Send webhook alert for budget warning
				if bc.dispatcher.webhookClient != nil {
					go func() {
						_ = bc.dispatcher.webhookClient.SendWarn(ctx, message, alertCtx)
					}()
				}
				// Dispatch through normal channels (log + TUI)
				bc.dispatcher.Dispatch(ctx, LevelWarn, message, alertCtx)
			}
		}
	}

	return threshold
}

// Reset resets the alerted flags, allowing alerts to be sent again.
// This should be called at the start of a new day or new session.
func (bc *BudgetChecker) Reset() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.alerted80 = false
	bc.alerted100 = false
}

// HasAlerted80 returns true if the 80% threshold alert has been sent.
func (bc *BudgetChecker) HasAlerted80() bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.alerted80
}

// HasAlerted100 returns true if the 100% threshold alert has been sent.
func (bc *BudgetChecker) HasAlerted100() bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.alerted100
}

// Global budget checker instance
var (
	globalBudgetChecker     *BudgetChecker
	globalBudgetCheckerOnce sync.Once
)

// InitBudgetChecker initializes the global budget checker with the given daily budget.
func InitBudgetChecker(dailyBudgetEUR float64, opts ...BudgetCheckerOption) {
	globalBudgetCheckerOnce.Do(func() {
		globalBudgetChecker = NewBudgetChecker(dailyBudgetEUR, opts...)
	})
}

// GetBudgetChecker returns the global budget checker.
// Returns nil if not initialized.
func GetBudgetChecker() *BudgetChecker {
	return globalBudgetChecker
}

// ResetBudgetChecker resets the global budget checker for testing purposes.
func ResetBudgetChecker() {
	globalBudgetChecker = nil
	globalBudgetCheckerOnce = sync.Once{}
}

// CheckBudget checks the accumulated cost against the global budget checker.
// This is a convenience function for common use cases.
// Returns BudgetThresholdNone if the budget checker is not initialized.
func CheckBudget(ctx context.Context, accumulatedCostEUR float64, alertCtx Context) BudgetThreshold {
	if globalBudgetChecker == nil {
		return BudgetThresholdNone
	}
	return globalBudgetChecker.CheckAndAlert(ctx, accumulatedCostEUR, alertCtx)
}

// BudgetStatus represents the current budget status for display purposes.
type BudgetStatus struct {
	DailyBudgetEUR     float64
	AccumulatedEUR     float64
	Percentage         float64
	Threshold          BudgetThreshold
	Enabled            bool
	RemainingEUR       float64
}

// GetBudgetStatus returns the current budget status for the given accumulated cost.
// Useful for displaying budget information in the TUI.
func (bc *BudgetChecker) GetBudgetStatus(accumulatedCostEUR float64) BudgetStatus {
	if !bc.IsEnabled() {
		return BudgetStatus{Enabled: false}
	}

	percentage := bc.GetPercentage(accumulatedCostEUR)
	remaining := bc.dailyBudgetEUR - accumulatedCostEUR
	if remaining < 0 {
		remaining = 0
	}

	return BudgetStatus{
		DailyBudgetEUR: bc.dailyBudgetEUR,
		AccumulatedEUR: accumulatedCostEUR,
		Percentage:     percentage,
		Threshold:      bc.GetThreshold(accumulatedCostEUR),
		Enabled:        true,
		RemainingEUR:   remaining,
	}
}
