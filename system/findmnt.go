package system

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mount-exporter/mount-exporter/reliability"
)

// MountStatus represents the status of a mount point
type MountStatus int

const (
	MountStatusUnknown MountStatus = iota
	MountStatusMounted
	MountStatusNotMounted
)

// String returns the string representation of MountStatus
func (ms MountStatus) String() string {
	switch ms {
	case MountStatusMounted:
		return "mounted"
	case MountStatusNotMounted:
		return "not_mounted"
	default:
		return "unknown"
	}
}

// FindmntResult represents the result of a findmnt command
type FindmntResult struct {
	MountPoint string      `json:"mount_point"`
	Status     MountStatus `json:"status"`
	Target     string      `json:"target,omitempty"`
	FSType     string      `json:"fs_type,omitempty"`
	Options    string      `json:"options,omitempty"`
	Source     string      `json:"source,omitempty"`
	Error      error       `json:"error,omitempty"`
}

// FindmntWrapper provides a wrapper around the findmnt command
type FindmntWrapper struct {
	timeout        time.Duration
	circuitBreaker *reliability.CircuitBreaker
	retry          *reliability.Retry
	mu             sync.RWMutex
	stats          struct {
		totalCalls       int64
		successfulCalls  int64
		failedCalls      int64
		circuitBreakerTrips int64
		retryAttempts     int64
	}
}

// NewFindmntWrapper creates a new FindmntWrapper with the given timeout
func NewFindmntWrapper(timeout time.Duration) *FindmntWrapper {
	cb := reliability.NewCircuitBreaker(reliability.CircuitBreakerConfig{
		Name:         "findmnt-circuit-breaker",
		MaxFailures:  5,
		ResetTimeout: 60 * time.Second,
		OnStateChange: func(name string, from, to reliability.State) {
			// Log circuit breaker state changes
			if to == reliability.StateOpen {
				// Could increment circuit breaker trips counter here
			}
		},
	})

	retry := reliability.NewRetry(
		reliability.WithMaxAttempts(3),
		reliability.WithInitialDelay(100*time.Millisecond),
		reliability.WithMaxDelay(5*time.Second),
		reliability.WithBackoffStrategy(reliability.BackoffStrategyExponential),
		reliability.WithShouldRetry(reliability.IsTransientError),
	)

	return &FindmntWrapper{
		timeout:        timeout,
		circuitBreaker: cb,
		retry:          retry,
	}
}

// CheckMountPoint checks if a mount point is currently mounted using findmnt
func (f *FindmntWrapper) CheckMountPoint(ctx context.Context, mountPoint string) *FindmntResult {
	f.mu.Lock()
	f.stats.totalCalls++
	f.mu.Unlock()

	result := &FindmntResult{
		MountPoint: mountPoint,
		Status:     MountStatusUnknown,
	}

	// Check circuit breaker first
	if f.circuitBreaker.IsOpen() {
		f.mu.Lock()
		f.stats.failedCalls++
		f.mu.Unlock()
		result.Error = fmt.Errorf("circuit breaker is open - findmnt commands are temporarily disabled")
		result.Status = MountStatusUnknown
		return result
	}

	// Execute findmnt through circuit breaker
	err := f.circuitBreaker.Execute(func() error {
		return f.executeFindmnt(ctx, mountPoint, result)
	})

	if err != nil {
		f.mu.Lock()
		f.stats.failedCalls++
		f.mu.Unlock()

		if err.Error() == "circuit breaker is open" {
			result.Error = fmt.Errorf("circuit breaker is open - findmnt commands are temporarily disabled")
			result.Status = MountStatusUnknown
		} else {
			result.Error = err
		}
		return result
	}

	f.mu.Lock()
	f.stats.successfulCalls++
	f.mu.Unlock()

	return result
}

// executeFindmnt executes the actual findmnt command with retry logic
func (f *FindmntWrapper) executeFindmnt(ctx context.Context, mountPoint string, result *FindmntResult) error {
	return f.retry.Do(ctx, func() error {
		// Track retry attempts
		f.mu.Lock()
		f.stats.retryAttempts++
		f.mu.Unlock()

		// Create context with timeout
		cmdCtx, cancel := context.WithTimeout(ctx, f.timeout)
		defer cancel()

		// Execute findmnt command
		cmd := exec.CommandContext(cmdCtx, "findmnt", "-n", "-o", "TARGET,FSTYPE,OPTIONS,SOURCE", "--mountpoint", mountPoint)
		output, err := cmd.Output()

		if err != nil {
			if cmdCtx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("findmnt command timed out after %v", f.timeout)
			} else if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
				// Exit code 1 typically means mount point not found - this is not a failure
				result.Status = MountStatusNotMounted
				return nil
			} else {
				return fmt.Errorf("findmnt command failed: %w", err)
			}
		}

		// Parse the output
		outputStr := string(output)
		if len(strings.TrimSpace(outputStr)) == 0 {
			result.Status = MountStatusNotMounted
			return nil
		}

		// Parse findmnt output
		scanner := bufio.NewScanner(strings.NewReader(outputStr))
		if scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				fields := strings.Split(line, " ")
				if len(fields) >= 1 {
					result.Status = MountStatusMounted
					result.Target = fields[0]
					if len(fields) >= 2 {
						result.FSType = fields[1]
					}
					if len(fields) >= 3 {
						result.Options = fields[2]
					}
					if len(fields) >= 4 {
						result.Source = strings.Join(fields[3:], " ")
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to parse findmnt output: %w", err)
		}

		return nil
	})
}

// CheckMultipleMountPoints checks multiple mount points concurrently
func (f *FindmntWrapper) CheckMultipleMountPoints(ctx context.Context, mountPoints []string) []*FindmntResult {
	results := make([]*FindmntResult, len(mountPoints))
	resultChan := make(chan struct {
		index  int
		result *FindmntResult
	}, len(mountPoints))

	// Launch goroutines for concurrent checking
	for i, mountPoint := range mountPoints {
		go func(index int, mp string) {
			result := f.CheckMountPoint(ctx, mp)
			resultChan <- struct {
				index  int
				result *FindmntResult
			}{index: index, result: result}
		}(i, mountPoint)
	}

	// Collect results
	for i := 0; i < len(mountPoints); i++ {
		select {
		case res := <-resultChan:
			results[res.index] = res.result
		case <-ctx.Done():
			// Context cancelled, fill remaining results with timeout errors
			for j := i; j < len(mountPoints); j++ {
				if results[j] == nil {
					results[j] = &FindmntResult{
						MountPoint: mountPoints[j],
						Status:     MountStatusUnknown,
						Error:      fmt.Errorf("context cancelled"),
					}
				}
			}
			return results
		}
	}

	return results
}

// IsAvailable checks if the findmnt command is available on the system
func (f *FindmntWrapper) IsAvailable() bool {
	_, err := exec.LookPath("findmnt")
	return err == nil
}

// GetVersion returns the findmnt version if available
func (f *FindmntWrapper) GetVersion() (string, error) {
	cmd := exec.Command("findmnt", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get findmnt version: %w", err)
	}

	// Parse version from output
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "findmnt from util-linux") {
			return line, nil
		}
	}

	return string(output), nil
}

// GetStats returns statistics about the FindmntWrapper operations
func (f *FindmntWrapper) GetStats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	stats := map[string]interface{}{
		"total_calls":              f.stats.totalCalls,
		"successful_calls":         f.stats.successfulCalls,
		"failed_calls":            f.stats.failedCalls,
		"retry_attempts":          f.stats.retryAttempts,
		"success_rate":            float64(f.stats.successfulCalls) / float64(f.stats.totalCalls),
		"circuit_breaker_state":   f.circuitBreaker.State().String(),
		"circuit_breaker_failures": f.circuitBreaker.Failures(),
	}

	// Calculate retry rate
	if f.stats.totalCalls > 0 {
		stats["retry_rate"] = float64(f.stats.retryAttempts) / float64(f.stats.totalCalls)
	} else {
		stats["retry_rate"] = 0.0
	}

	return stats
}

// ResetCircuitBreaker resets the circuit breaker to closed state
func (f *FindmntWrapper) ResetCircuitBreaker() {
	f.circuitBreaker.Reset()
}

// GetCircuitBreakerState returns the current circuit breaker state
func (f *FindmntWrapper) GetCircuitBreakerState() reliability.State {
	return f.circuitBreaker.State()
}