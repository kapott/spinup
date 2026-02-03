# retry-patterns

Generate robust retry logic with exponential backoff for API calls.

## Trigger
User invokes `/retry-patterns [function-name]` or asks to add retry logic.

## Arguments
- `[function-name]` - Name of function to wrap with retry

## Instructions

1. **Generate retry wrapper**:

```go
package [package]

import (
    "context"
    "errors"
    "math"
    "time"
)

// RetryConfig holds retry parameters
type RetryConfig struct {
    MaxRetries  int           // Maximum number of retry attempts
    BaseDelay   time.Duration // Initial delay between retries
    MaxDelay    time.Duration // Maximum delay between retries
    Multiplier  float64       // Delay multiplier for exponential backoff
}

// DefaultRetryConfig returns sensible defaults matching PRD requirements
func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxRetries: 5,
        BaseDelay:  2 * time.Second,
        MaxDelay:   30 * time.Second,
        Multiplier: 2.0,
    }
}

// RetryableError indicates an error that should trigger a retry
type RetryableError struct {
    Err error
}

func (e RetryableError) Error() string { return e.Err.Error() }
func (e RetryableError) Unwrap() error { return e.Err }

// IsRetryable checks if an error should be retried
func IsRetryable(err error) bool {
    var retryable RetryableError
    return errors.As(err, &retryable)
}

// WithRetry executes fn with retry logic
func WithRetry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
    var result T
    var lastErr error

    for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
        // Check context cancellation
        if ctx.Err() != nil {
            return result, ctx.Err()
        }

        result, lastErr = fn()
        if lastErr == nil {
            return result, nil
        }

        // Don't retry non-retryable errors
        if !IsRetryable(lastErr) {
            return result, lastErr
        }

        // Don't sleep after last attempt
        if attempt == cfg.MaxRetries {
            break
        }

        // Calculate delay with exponential backoff
        delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt)))
        if delay > cfg.MaxDelay {
            delay = cfg.MaxDelay
        }

        // Add jitter (10% random variation)
        // jitter := time.Duration(rand.Float64() * 0.1 * float64(delay))
        // delay += jitter

        select {
        case <-ctx.Done():
            return result, ctx.Err()
        case <-time.After(delay):
            // Continue to next attempt
        }
    }

    return result, fmt.Errorf("max retries (%d) exceeded: %w", cfg.MaxRetries, lastErr)
}
```

2. **Generate usage example**:

```go
// Example: Wrap provider termination with retry
func (c *Client) TerminateWithRetry(ctx context.Context, instanceID string) error {
    _, err := WithRetry(ctx, DefaultRetryConfig(), func() (struct{}, error) {
        err := c.provider.TerminateInstance(ctx, instanceID)
        if err != nil {
            // Mark network errors as retryable
            if isNetworkError(err) || isRateLimitError(err) {
                return struct{}{}, RetryableError{Err: err}
            }
            return struct{}{}, err // Not retryable
        }
        return struct{}{}, nil
    })
    return err
}
```

3. **Add logging integration**:

```go
// WithRetryLogged adds logging to retry attempts
func WithRetryLogged[T any](ctx context.Context, cfg RetryConfig, logger *zerolog.Logger, operation string, fn func() (T, error)) (T, error) {
    attempt := 0
    return WithRetry(ctx, cfg, func() (T, error) {
        attempt++
        result, err := fn()
        if err != nil && IsRetryable(err) {
            logger.Warn().
                Err(err).
                Str("operation", operation).
                Int("attempt", attempt).
                Int("max_retries", cfg.MaxRetries).
                Msg("Operation failed, retrying")
        }
        return result, err
    })
}
```

## Output

Creates retry utility in specified package with:
- Generic retry function
- Configurable backoff parameters
- Context cancellation support
- Retryable error type
- Logging integration
