package reliability

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// BackoffStrategy defines the backoff strategy for retries
type BackoffStrategy int

const (
	BackoffStrategyLinear BackoffStrategy = iota
	BackoffStrategyExponential
	BackoffStrategyFixed
)

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	Strategy        BackoffStrategy
	RetryableErrors []error
	ShouldRetry     func(error) bool
}

// RetryOption is a function that configures retry options
type RetryOption func(*RetryConfig)

// Retry provides retry functionality with configurable backoff strategies
type Retry struct {
	config RetryConfig
}

// NewRetry creates a new Retry instance with default configuration
func NewRetry(opts ...RetryOption) *Retry {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Strategy:     BackoffStrategyExponential,
		ShouldRetry: func(err error) bool {
			return err != nil
		},
	}

	for _, opt := range opts {
		opt(&config)
	}

	return &Retry{config: config}
}

// WithMaxAttempts sets the maximum number of retry attempts
func WithMaxAttempts(attempts int) RetryOption {
	return func(c *RetryConfig) {
		c.MaxAttempts = attempts
	}
}

// WithInitialDelay sets the initial delay between retries
func WithInitialDelay(delay time.Duration) RetryOption {
	return func(c *RetryConfig) {
		c.InitialDelay = delay
	}
}

// WithMaxDelay sets the maximum delay between retries
func WithMaxDelay(delay time.Duration) RetryOption {
	return func(c *RetryConfig) {
		c.MaxDelay = delay
	}
}

// WithMultiplier sets the backoff multiplier for exponential strategy
func WithMultiplier(multiplier float64) RetryOption {
	return func(c *RetryConfig) {
		c.Multiplier = multiplier
	}
}

// WithBackoffStrategy sets the backoff strategy
func WithBackoffStrategy(strategy BackoffStrategy) RetryOption {
	return func(c *RetryConfig) {
		c.Strategy = strategy
	}
}

// WithRetryableErrors sets specific errors that should be retried
func WithRetryableErrors(errors ...error) RetryOption {
	return func(c *RetryConfig) {
		c.RetryableErrors = errors
		c.ShouldRetry = func(err error) bool {
			for _, retryableErr := range errors {
				if err == retryableErr || err.Error() == retryableErr.Error() {
					return true
				}
			}
			return false
		}
	}
}

// WithShouldRetry sets a custom function to determine if an error should be retried
func WithShouldRetry(shouldRetry func(error) bool) RetryOption {
	return func(c *RetryConfig) {
		c.ShouldRetry = shouldRetry
	}
}

// Do executes the given function with retry logic
func (r *Retry) Do(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < r.config.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := r.calculateDelay(attempt)

			// Add jitter to prevent thundering herd
			jitter := time.Duration(rand.Float64() * float64(delay) * 0.1) // 10% jitter
			delay += jitter

			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry this error
		if !r.config.ShouldRetry(err) {
			break
		}

		// If this is the last attempt, don't wait
		if attempt == r.config.MaxAttempts-1 {
			break
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded, last error: %w", r.config.MaxAttempts, lastErr)
}

// DoWithValue executes the given function with retry logic and returns a value
func (r *Retry) DoWithValue[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt < r.config.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := r.calculateDelay(attempt)

			// Add jitter to prevent thundering herd
			jitter := time.Duration(rand.Float64() * float64(delay) * 0.1) // 10% jitter
			delay += jitter

			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return result, fmt.Errorf("retry cancelled: %w", ctx.Err())
			}
		}

		res, err := fn()
		if err == nil {
			return res, nil
		}

		lastErr = err
		result = res

		// Check if we should retry this error
		if !r.config.ShouldRetry(err) {
			break
		}

		// If this is the last attempt, don't wait
		if attempt == r.config.MaxAttempts-1 {
			break
		}
	}

	return result, fmt.Errorf("max retry attempts (%d) exceeded, last error: %w", r.config.MaxAttempts, lastErr)
}

// calculateDelay calculates the delay for a given attempt based on the strategy
func (r *Retry) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch r.config.Strategy {
	case BackoffStrategyLinear:
		delay = time.Duration(float64(r.config.InitialDelay) * float64(attempt))
	case BackoffStrategyExponential:
		delay = time.Duration(float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1)))
	case BackoffStrategyFixed:
		delay = r.config.InitialDelay
	default:
		delay = r.config.InitialDelay
	}

	// Cap the delay at MaxDelay
	if delay > r.config.MaxDelay {
		delay = r.config.MaxDelay
	}

	return delay
}

// GetConfig returns the current retry configuration
func (r *Retry) GetConfig() RetryConfig {
	return r.config
}

// IsRetryableError checks if an error is retryable based on the configuration
func (r *Retry) IsRetryableError(err error) bool {
	return r.config.ShouldRetry(err)
}

// Common retryable errors
var (
	ErrTimeout          = fmt.Errorf("timeout")
	ErrConnectionRefused = fmt.Errorf("connection refused")
	ErrTemporaryFailure  = fmt.Errorf("temporary failure")
	ErrRateLimited      = fmt.Errorf("rate limited")
	ErrServiceUnavailable = fmt.Errorf("service unavailable")
)

// DefaultRetryableErrors returns a list of commonly retryable errors
func DefaultRetryableErrors() []error {
	return []error{
		ErrTimeout,
		ErrConnectionRefused,
		ErrTemporaryFailure,
		ErrRateLimited,
		ErrServiceUnavailable,
	}
}

// IsTransientError checks if an error is likely transient (retryable)
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Common patterns for transient errors
	transientPatterns := []string{
		"timeout",
		"connection refused",
		"temporary failure",
		"rate limited",
		"service unavailable",
		"network is unreachable",
		"no route to host",
		"connection reset",
		"connection timed out",
		"deadline exceeded",
		"resource temporarily unavailable",
	}

	for _, pattern := range transientPatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		func() bool {
			sLower := toLower(s)
			substrLower := toLower(substr)
			for i := 1; i <= len(sLower)-len(substrLower); i++ {
				if sLower[i:i+len(substrLower)] == substrLower {
					return true
				}
			}
			return false
		}())))
}

// toLower converts a string to lowercase
func toLower(s string) string {
	result := make([]rune, len([]rune(s)))
	for i, r := range []rune(s) {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + ('a' - 'A')
		} else {
			result[i] = r
		}
	}
	return string(result)
}