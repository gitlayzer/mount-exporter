package reliability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetry_NewRetry(t *testing.T) {
	retry := NewRetry()

	config := retry.GetConfig()
	if config.MaxAttempts != 3 {
		t.Errorf("Expected default MaxAttempts to be 3, got %d", config.MaxAttempts)
	}

	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected default InitialDelay to be 100ms, got %v", config.InitialDelay)
	}

	if config.Strategy != BackoffStrategyExponential {
		t.Errorf("Expected default Strategy to be Exponential, got %v", config.Strategy)
	}
}

func TestRetry_WithMaxAttempts(t *testing.T) {
	retry := NewRetry(WithMaxAttempts(5))

	config := retry.GetConfig()
	if config.MaxAttempts != 5 {
		t.Errorf("Expected MaxAttempts to be 5, got %d", config.MaxAttempts)
	}
}

func TestRetry_WithInitialDelay(t *testing.T) {
	retry := NewRetry(WithInitialDelay(200 * time.Millisecond))

	config := retry.GetConfig()
	if config.InitialDelay != 200*time.Millisecond {
		t.Errorf("Expected InitialDelay to be 200ms, got %v", config.InitialDelay)
	}
}

func TestRetry_WithBackoffStrategy(t *testing.T) {
	strategies := []BackoffStrategy{
		BackoffStrategyLinear,
		BackoffStrategyExponential,
		BackoffStrategyFixed,
	}

	for _, strategy := range strategies {
		retry := NewRetry(WithBackoffStrategy(strategy))

		config := retry.GetConfig()
		if config.Strategy != strategy {
			t.Errorf("Expected Strategy to be %v, got %v", strategy, config.Strategy)
		}
	}
}

func TestRetry_SuccessOnFirstAttempt(t *testing.T) {
	retry := NewRetry()
	calls := 0

	err := retry.Do(context.Background(), func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if calls != 1 {
		t.Errorf("Expected 1 call, got %d", calls)
	}
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(3),
		WithInitialDelay(10*time.Millisecond),
		WithBackoffStrategy(BackoffStrategyFixed),
	)
	calls := 0

	err := retry.Do(context.Background(), func() error {
		calls++
		if calls < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error after retries, got %v", err)
	}

	if calls != 3 {
		t.Errorf("Expected 3 calls, got %d", calls)
	}
}

func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(3),
		WithInitialDelay(10*time.Millisecond),
		WithBackoffStrategy(BackoffStrategyFixed),
	)
	calls := 0

	err := retry.Do(context.Background(), func() error {
		calls++
		return errors.New("persistent failure")
	})

	if err == nil {
		t.Error("Expected error after max attempts")
	}

	if !contains(err.Error(), "max retry attempts") {
		t.Errorf("Expected max retry attempts error, got %s", err.Error())
	}

	if calls != 3 {
		t.Errorf("Expected 3 calls, got %d", calls)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(3),
		WithShouldRetry(func(err error) bool {
			return err.Error() != "non-retryable"
		}),
	)
	calls := 0

	err := retry.Do(context.Background(), func() error {
		calls++
		return errors.New("non-retryable")
	})

	if err == nil {
		t.Error("Expected error")
	}

	if calls != 1 {
		t.Errorf("Expected 1 call for non-retryable error, got %d", calls)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(5),
		WithInitialDelay(100*time.Millisecond),
		WithBackoffStrategy(BackoffStrategyFixed),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	calls := 0

	err := retry.Do(ctx, func() error {
		calls++
		return errors.New("temporary failure")
	})

	if err == nil {
		t.Error("Expected context cancellation error")
	}

	if !contains(err.Error(), "retry cancelled") {
		t.Errorf("Expected retry cancelled error, got %s", err.Error())
	}

	// Should have made at most 2 calls (initial + one retry before cancellation)
	if calls > 2 {
		t.Errorf("Expected at most 2 calls before cancellation, got %d", calls)
	}
}

func TestRetry_DoWithValue(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(3),
		WithInitialDelay(10*time.Millisecond),
		WithBackoffStrategy(BackoffStrategyFixed),
	)
	calls := 0

	result, err := retry.DoWithValue(context.Background(), func() (string, error) {
		calls++
		if calls < 3 {
			return "", errors.New("temporary failure")
		}
		return "success", nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got %s", result)
	}

	if calls != 3 {
		t.Errorf("Expected 3 calls, got %d", calls)
	}
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(4),
		WithInitialDelay(10*time.Millisecond),
		WithMultiplier(2.0),
		WithBackoffStrategy(BackoffStrategyExponential),
	)

	start := time.Now()
	calls := 0

	err := retry.Do(context.Background(), func() error {
		calls++
		if calls < 4 {
			return errors.New("temporary failure")
		}
		return nil
	})

	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should have taken approximately: 0ms (first) + 10ms + 20ms + 40ms = 70ms minimum
	// Allow some tolerance for jitter and processing time
	if duration < 60*time.Millisecond {
		t.Errorf("Expected duration to be at least 60ms, got %v", duration)
	}

	if calls != 4 {
		t.Errorf("Expected 4 calls, got %d", calls)
	}
}

func TestRetry_LinearBackoff(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(4),
		WithInitialDelay(10*time.Millisecond),
		WithBackoffStrategy(BackoffStrategyLinear),
	)

	start := time.Now()
	calls := 0

	err := retry.Do(context.Background(), func() error {
		calls++
		if calls < 4 {
			return errors.New("temporary failure")
		}
		return nil
	})

	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should have taken approximately: 0ms + 10ms + 20ms + 30ms = 60ms minimum
	if duration < 50*time.Millisecond {
		t.Errorf("Expected duration to be at least 50ms, got %v", duration)
	}

	if calls != 4 {
		t.Errorf("Expected 4 calls, got %d", calls)
	}
}

func TestRetry_MaxDelay(t *testing.T) {
	retry := NewRetry(
		WithMaxAttempts(5),
		WithInitialDelay(10*time.Millisecond),
		WithMaxDelay(20*time.Millisecond),
		WithMultiplier(10.0), // Very high multiplier to trigger max delay
		WithBackoffStrategy(BackoffStrategyExponential),
	)

	start := time.Now()
	calls := 0

	err := retry.Do(context.Background(), func() error {
		calls++
		if calls < 5 {
			return errors.New("temporary failure")
		}
		return nil
	})

	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Even with high multiplier, delay should be capped at MaxDelay
	// Should be approximately: 0ms + 10ms + 20ms + 20ms + 20ms = 70ms maximum
	if duration > 100*time.Millisecond {
		t.Errorf("Expected duration to be capped at ~70ms, got %v", duration)
	}

	if calls != 5 {
		t.Errorf("Expected 5 calls, got %d", calls)
	}
}

func TestRetry_WithRetryableErrors(t *testing.T) {
	retryableErr := errors.New("retryable error")
	nonRetryableErr := errors.New("non-retryable error")

	retry := NewRetry(
		WithMaxAttempts(3),
		WithInitialDelay(10*time.Millisecond),
		WithRetryableErrors(retryableErr),
	)

	// Test with retryable error
	calls := 0
	err := retry.Do(context.Background(), func() error {
		calls++
		return retryableErr
	})

	if calls != 3 {
		t.Errorf("Expected 3 calls for retryable error, got %d", calls)
	}

	// Test with non-retryable error
	calls = 0
	err = retry.Do(context.Background(), func() error {
		calls++
		return nonRetryableErr
	})

	if calls != 1 {
		t.Errorf("Expected 1 call for non-retryable error, got %d", calls)
	}
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"timeout error", errors.New("operation timed out"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"temporary failure", errors.New("temporary failure"), true},
		{"rate limited", errors.New("rate limited"), true},
		{"service unavailable", errors.New("service unavailable"), true},
		{"network error", errors.New("network is unreachable"), true},
		{"permanent error", errors.New("permanent failure"), false},
		{"nil error", nil, false},
		{"validation error", errors.New("invalid input"), false},
		{"mixed case", errors.New("CONNECTION REFUSED"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTransientError(tt.err); got != tt.want {
				t.Errorf("IsTransientError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRetry_IsRetryableError(t *testing.T) {
	retry := NewRetry(
		WithShouldRetry(func(err error) bool {
			return IsTransientError(err)
		}),
	)

	transientErr := errors.New("operation timed out")
	permanentErr := errors.New("invalid input")

	if !retry.IsRetryableError(transientErr) {
		t.Error("Expected transient error to be retryable")
	}

	if retry.IsRetryableError(permanentErr) {
		t.Error("Expected permanent error to not be retryable")
	}
}

func TestDefaultRetryableErrors(t *testing.T) {
	errors := DefaultRetryableErrors()

	if len(errors) == 0 {
		t.Error("Expected at least one default retryable error")
	}

	// Check that all expected errors are present
	expectedErrors := []error{
		ErrTimeout,
		ErrConnectionRefused,
		ErrTemporaryFailure,
		ErrRateLimited,
		ErrServiceUnavailable,
	}

	for _, expectedErr := range expectedErrors {
		found := false
		for _, err := range errors {
			if err == expectedErr {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error %v to be in default retryable errors", expectedErr)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		func() bool {
			for i := 1; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}