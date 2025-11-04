package recovery

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// PanicHandler handles panic recovery
type PanicHandler struct {
	mu           sync.RWMutex
	recovered    map[string]int64
	handlers     []PanicHandlerFunc
	logger       Logger
	enabled      bool
	maxStackFrames int
}

// PanicInfo contains information about a recovered panic
type PanicInfo struct {
	Timestamp   time.Time
	GoroutineID string
	PanicValue  interface{}
	Stack       []byte
	Message     string
}

// PanicHandlerFunc is a function that handles recovered panics
type PanicHandlerFunc func(info PanicInfo)

// Logger interface for panic logging
type Logger interface {
	Printf(format string, args ...interface{})
}

// DefaultLogger implements a simple logger
type DefaultLogger struct{}

func (l *DefaultLogger) Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// PanicRecoveryConfig holds configuration for panic recovery
type PanicRecoveryConfig struct {
	Enabled         bool
	Logger          Logger
	Handlers        []PanicHandlerFunc
	MaxStackFrames  int
	LogLevel        string
}

// NewPanicHandler creates a new panic handler
func NewPanicHandler(config PanicRecoveryConfig) *PanicHandler {
	if config.Logger == nil {
		config.Logger = &DefaultLogger{}
	}

	if config.MaxStackFrames <= 0 {
		config.MaxStackFrames = 50
	}

	return &PanicHandler{
		recovered:      make(map[string]int64),
		handlers:       config.Handlers,
		logger:         config.Logger,
		enabled:        config.Enabled,
		maxStackFrames: config.MaxStackFrames,
	}
}

// Recover recovers from a panic and handles it
func (ph *PanicHandler) Recover(info *PanicInfo) {
	if !ph.enabled {
		return
	}

	ph.mu.Lock()
	ph.recovered[info.GoroutineID]++
	ph.mu.Unlock()

	// Log the panic
	ph.logPanic(info)

	// Call all handlers
	for _, handler := range ph.handlers {
		if handler != nil {
			// Run handlers in separate goroutines to avoid panics in handlers
			go func(h PanicHandlerFunc) {
				defer func() {
					if r := recover(); r != nil {
						ph.logger.Printf("Panic in panic handler: %v", r)
					}
				}()
				h(*info)
			}(handler)
		}
	}
}

// RecoverWithFunc recovers from a panic in a function and returns an error
func (ph *PanicHandler) RecoverWithFunc(fn func() error) (err error) {
	if !ph.enabled {
		return fn()
	}

	defer func() {
		if r := recover(); r != nil {
			info := ph.capturePanic(r)
			ph.Recover(info)
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()

	return fn()
}

// RecoverWithContext recovers from a panic in a function with context
func (ph *PanicHandler) RecoverWithContext(ctx context.Context, fn func(context.Context) error) (err error) {
	if !ph.enabled {
		return fn(ctx)
	}

	defer func() {
		if r := recover(); r != nil {
			info := ph.capturePanic(r)
			ph.Recover(info)
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()

	return fn(ctx)
}

// capturePanic captures panic information
func (ph *PanicHandler) capturePanic(panicValue interface{}) *PanicInfo {
	stack := make([]byte, 4096)
	length := runtime.Stack(stack, false)
	if length > len(stack) {
		length = len(stack)
	}

	// Limit stack frames
	if ph.maxStackFrames > 0 && length > ph.maxStackFrames*100 {
		length = ph.maxStackFrames * 100
	}

	goroutineID := getGoroutineID()

	return &PanicInfo{
		Timestamp:   time.Now(),
		GoroutineID: goroutineID,
		PanicValue:  panicValue,
		Stack:       stack[:length],
		Message:     fmt.Sprintf("Panic recovered: %v", panicValue),
	}
}

// logPanic logs panic information
func (ph *PanicHandler) logPanic(info *PanicInfo) {
	ph.logger.Printf("=== PANIC RECOVERED ===")
	ph.logger.Printf("Timestamp: %s", info.Timestamp.Format(time.RFC3339))
	ph.logger.Printf("GoroutineID: %s", info.GoroutineID)
	ph.logger.Printf("PanicValue: %v", info.PanicValue)
	ph.logger.Printf("Message: %s", info.Message)
	ph.logger.Printf("Stack trace:\n%s", string(info.Stack))
	ph.logger.Printf("========================")
}

// GetStats returns panic recovery statistics
func (ph *PanicHandler) GetStats() map[string]interface{} {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	stats := map[string]interface{}{
		"total_recovered": len(ph.recovered),
		"goroutine_counts": make(map[string]int64),
		"enabled": ph.enabled,
	}

	for goroutineID, count := range ph.recovered {
		goroutineCounts := stats["goroutine_counts"].(map[string]int64)
		goroutineCounts[goroutineID] = count
	}

	return stats
}

// ResetStats resets panic recovery statistics
func (ph *PanicHandler) ResetStats() {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.recovered = make(map[string]int64)
}

// IsEnabled returns whether panic recovery is enabled
func (ph *PanicHandler) IsEnabled() bool {
	return ph.enabled
}

// SetEnabled enables or disables panic recovery
func (ph *PanicHandler) SetEnabled(enabled bool) {
	ph.enabled = enabled
}

// AddHandler adds a panic handler function
func (ph *PanicHandler) AddHandler(handler PanicHandlerFunc) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.handlers = append(ph.handlers, handler)
}

// RemoveHandlers removes all panic handlers
func (ph *PanicHandler) RemoveHandlers() {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.handlers = nil
}

// getGoroutineID gets the current goroutine ID (simplified version)
func getGoroutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	if n == 0 {
		return "unknown"
	}

	// Extract goroutine ID from stack trace
	stack := string(buf[:n])
	// Format: "goroutine 123 [running]:"
	for i := 0; i < len(stack); i++ {
		if stack[i:i+9] == "goroutine " {
			start := i + 9
			end := start
			for end < len(stack) && stack[end] != ' ' && stack[end] != '[' {
				end++
			}
			if end > start {
				return stack[start:end]
			}
		}
	}

	return "unknown"
}

// SafeGo starts a goroutine with panic recovery
func SafeGo(ph *PanicHandler, fn func()) {
	if ph == nil || !ph.IsEnabled() {
		go fn()
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				info := ph.capturePanic(r)
				ph.Recover(info)
			}
		}()
		fn()
	}()
}

// SafeGoWithContext starts a goroutine with panic recovery and context
func SafeGoWithContext(ph *PanicHandler, ctx context.Context, fn func(context.Context)) {
	if ph == nil || !ph.IsEnabled() {
		go fn(ctx)
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				info := ph.capturePanic(r)
				ph.Recover(info)
			}
		}()
		fn(ctx)
	}()
}

// DefaultPanicHandlers returns some common panic handlers
func DefaultPanicHandlers() []PanicHandlerFunc {
	return []PanicHandlerFunc{
		// Log to a file or external service
		func(info PanicInfo) {
			// This could be replaced with actual logging to file/database
			fmt.Printf("ALERT: Panic occurred at %s in goroutine %s: %v\n",
				info.Timestamp.Format(time.RFC3339), info.GoroutineID, info.PanicValue)
		},
		// Send metrics to monitoring system
		func(info PanicInfo) {
			// This could be replaced with actual metrics collection
			fmt.Printf("METRIC: panic.recovered:1 goroutine=%s\n", info.GoroutineID)
		},
	}
}

// NewDefaultPanicHandler creates a panic handler with default configuration
func NewDefaultPanicHandler() *PanicHandler {
	return NewPanicHandler(PanicRecoveryConfig{
		Enabled:        true,
		Logger:         &DefaultLogger{},
		Handlers:       DefaultPanicHandlers(),
		MaxStackFrames: 50,
	})
}