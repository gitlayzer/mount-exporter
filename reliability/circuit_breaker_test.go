package reliability

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_NewCircuitBreaker(t *testing.T) {
	config := CircuitBreakerConfig{
		Name:         "test-cb",
		MaxFailures:  3,
		ResetTimeout: 30 * time.Second,
	}

	cb := NewCircuitBreaker(config)

	if cb.Name() != "test-cb" {
		t.Errorf("Expected name 'test-cb', got %s", cb.Name())
	}

	if cb.State() != StateClosed {
		t.Errorf("Expected initial state to be CLOSED, got %s", cb.State())
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected initial failures to be 0, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_SuccessfulExecution(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "test-cb",
		MaxFailures:  3,
		ResetTimeout: 30 * time.Second,
	})

	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if cb.State() != StateClosed {
		t.Errorf("Expected state to remain CLOSED, got %s", cb.State())
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected failures to be 0, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_FailureThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "test-cb",
		MaxFailures:  3,
		ResetTimeout: 100 * time.Millisecond,
	})

	// Execute 3 failing operations to reach threshold
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			return errors.New("test error")
		})

		if err == nil {
			t.Errorf("Expected error for execution %d", i+1)
		}
	}

	// Circuit should now be open
	if !cb.IsOpen() {
		t.Error("Expected circuit breaker to be OPEN after threshold failures")
	}

	if cb.Failures() != 3 {
		t.Errorf("Expected failures to be 3, got %d", cb.Failures())
	}

	// Next execution should be blocked
	err := cb.Execute(func() error {
		return nil
	})

	if err == nil {
		t.Error("Expected execution to be blocked when circuit is open")
	}

	if err.Error() != "circuit breaker is open" {
		t.Errorf("Expected 'circuit breaker is open' error, got %s", err.Error())
	}
}

func TestCircuitBreaker_ResetTimeout(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "test-cb",
		MaxFailures:  2,
		ResetTimeout: 50 * time.Millisecond,
	})

	// Fail to open circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return errors.New("test error")
		})
	}

	if !cb.IsOpen() {
		t.Error("Expected circuit breaker to be OPEN")
	}

	// Wait for reset timeout
	time.Sleep(100 * time.Millisecond)

	// Next request should be allowed (half-open state)
	executed := false
	err := cb.Execute(func() error {
		executed = true
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !executed {
		t.Error("Expected execution to be allowed after reset timeout")
	}

	// Successful execution should close the circuit
	if !cb.IsClosed() {
		t.Error("Expected circuit breaker to be CLOSED after successful execution")
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "test-cb",
		MaxFailures:  2,
		ResetTimeout: 50 * time.Millisecond,
	})

	// Fail to open circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return errors.New("test error")
		})
	}

	if !cb.IsOpen() {
		t.Error("Expected circuit breaker to be OPEN")
	}

	// Wait for reset timeout
	time.Sleep(100 * time.Millisecond)

	// First execution in half-open state fails
	err := cb.Execute(func() error {
		return errors.New("test error")
	})

	if err == nil {
		t.Error("Expected error for execution in half-open state")
	}

	// Should return to open state
	if !cb.IsOpen() {
		t.Error("Expected circuit breaker to return to OPEN state after half-open failure")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "test-cb",
		MaxFailures:  2,
		ResetTimeout: 30 * time.Second,
	})

	// Fail to open circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return errors.New("test error")
		})
	}

	if !cb.IsOpen() {
		t.Error("Expected circuit breaker to be OPEN")
	}

	// Reset circuit breaker
	cb.Reset()

	if !cb.IsClosed() {
		t.Error("Expected circuit breaker to be CLOSED after reset")
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected failures to be 0 after reset, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	var mu sync.Mutex
	callbacks := []struct {
		name     string
		from     State
		to       State
		received bool
	}{}

	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "test-cb",
		MaxFailures:  2,
		ResetTimeout: 50 * time.Millisecond,
		OnStateChange: func(name string, from State, to State) {
			mu.Lock()
			defer mu.Unlock()
			callbacks = append(callbacks, struct {
				name     string
				from     State
				to       State
				received bool
			}{name: name, from: from, to: to, received: true})
		},
	})

	// Fail to open circuit (should trigger CLOSED -> OPEN)
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return errors.New("test error")
		})
	}

	// Wait for reset timeout
	time.Sleep(100 * time.Millisecond)

	// Successful execution (should trigger OPEN -> HALF_OPEN -> CLOSED)
	cb.Execute(func() error {
		return nil
	})

	// Give time for callbacks to execute
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(callbacks) < 2 {
		t.Errorf("Expected at least 2 callbacks, got %d", len(callbacks))
		return
	}

	// Check first callback (CLOSED -> OPEN)
	if callbacks[0].from != StateClosed || callbacks[0].to != StateOpen {
		t.Errorf("Expected callback CLOSED -> OPEN, got %s -> %s", callbacks[0].from, callbacks[0].to)
	}

	// Should have callbacks for OPEN -> HALF_OPEN and HALF_OPEN -> CLOSED
	hasHalfOpen := false
	hasClosed := false
	for _, cb := range callbacks {
		if cb.from == StateOpen && cb.to == StateHalfOpen {
			hasHalfOpen = true
		}
		if cb.from == StateHalfOpen && cb.to == StateClosed {
			hasClosed = true
		}
	}

	if !hasHalfOpen {
		t.Error("Expected OPEN -> HALF_OPEN callback")
	}

	if !hasClosed {
		t.Error("Expected HALF_OPEN -> CLOSED callback")
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:         "test-cb",
		MaxFailures:  10,
		ResetTimeout: 30 * time.Second,
	})

	var wg sync.WaitGroup
	numGoroutines := 10
	executionsPerGoroutine := 5

	// Execute concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < executionsPerGoroutine; j++ {
				cb.Execute(func() error {
					time.Sleep(1 * time.Millisecond) // Simulate work
					return nil
				})
			}
		}(i)
	}

	wg.Wait()

	// Should still be closed and have no failures
	if !cb.IsClosed() {
		t.Error("Expected circuit breaker to remain CLOSED under concurrent successful operations")
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected no failures, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_DefaultValues(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name: "test-cb",
	})

	// Should use default values
	if cb.IsOpen() {
		t.Error("Expected circuit breaker to be CLOSED with default config")
	}

	// Test with default max failures
	for i := 0; i < 5; i++ {
		cb.Execute(func() error {
			return errors.New("test error")
		})
	}

	if !cb.IsOpen() {
		t.Error("Expected circuit breaker to be OPEN after 5 failures (default)")
	}
}

func TestState_String(t *testing.T) {
	tests := map[State]string{
		StateClosed:   "CLOSED",
		StateOpen:     "OPEN",
		StateHalfOpen: "HALF_OPEN",
		State(999):    "UNKNOWN",
	}

	for state, expected := range tests {
		if state.String() != expected {
			t.Errorf("Expected %s, got %s", expected, state.String())
		}
	}
}