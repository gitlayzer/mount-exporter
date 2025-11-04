package reliability

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name          string
	maxFailures   int
	resetTimeout  time.Duration
	mu            sync.RWMutex
	state         State
	failures      int
	lastFailTime  time.Time
	onStateChange func(name string, from State, to State)
}

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	Name         string
	MaxFailures  int
	ResetTimeout time.Duration
	OnStateChange func(name string, from State, to State)
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.MaxFailures <= 0 {
		config.MaxFailures = 5
	}
	if config.ResetTimeout <= 0 {
		config.ResetTimeout = 60 * time.Second
	}

	return &CircuitBreaker{
		name:          config.Name,
		maxFailures:   config.MaxFailures,
		resetTimeout:  config.ResetTimeout,
		state:         StateClosed,
		onStateChange: config.OnStateChange,
	}
}

// Execute executes the given function if the circuit breaker allows it
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return errors.New("circuit breaker is open")
	}

	err := fn()
	cb.recordResult(err == nil)
	return err
}

// allowRequest determines whether a request should be allowed
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		return time.Since(cb.lastFailTime) >= cb.resetTimeout
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of a request
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		cb.onSuccess()
	} else {
		cb.onFailure()
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	cb.failures = 0

	switch cb.state {
	case StateClosed:
		// Stay closed
	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailTime) >= cb.resetTimeout {
			cb.setState(StateHalfOpen)
		}
	case StateHalfOpen:
		cb.setState(StateClosed)
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.setState(StateOpen)
		}
	case StateHalfOpen:
		cb.setState(StateOpen)
	case StateOpen:
		// Stay open, already tracking failures
	}
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(newState State) {
	if cb.state != newState {
		oldState := cb.state
		cb.state = newState

		if cb.onStateChange != nil {
			go cb.onStateChange(cb.name, oldState, newState)
		}
	}
}

// State returns the current circuit breaker state
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Failures returns the current failure count
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.lastFailTime = time.Time{}
	cb.setState(StateClosed)
}

// Name returns the circuit breaker name
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// IsOpen returns true if the circuit breaker is open
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.State() == StateOpen
}

// IsHalfOpen returns true if the circuit breaker is half-open
func (cb *CircuitBreaker) IsHalfOpen() bool {
	return cb.State() == StateHalfOpen
}

// IsClosed returns true if the circuit breaker is closed
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.State() == StateClosed
}

// String returns a string representation of the circuit breaker state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateHalfOpen:
		return "HALF_OPEN"
	case StateOpen:
		return "OPEN"
	default:
		return "UNKNOWN"
	}
}