package recovery

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestLogger implements Logger interface for testing
type TestLogger struct {
	messages []string
	mu       sync.Mutex
}

func (l *TestLogger) Printf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

func (l *TestLogger) GetMessages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.messages...)
}

func (l *TestLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = nil
}

func TestNewPanicHandler(t *testing.T) {
	logger := &TestLogger{}
	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled:        true,
		Logger:         logger,
		MaxStackFrames: 10,
	})

	if !handler.IsEnabled() {
		t.Error("Expected handler to be enabled")
	}

	stats := handler.GetStats()
	if stats["enabled"] != true {
		t.Error("Expected enabled to be true in stats")
	}
}

func TestPanicHandler_Recover(t *testing.T) {
	logger := &TestLogger{}
	handlerCalled := false

	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled: true,
		Logger:  logger,
		Handlers: []PanicHandlerFunc{
			func(info PanicInfo) {
				handlerCalled = true
				if info.PanicValue != "test panic" {
					t.Errorf("Expected panic value 'test panic', got %v", info.PanicValue)
				}
			},
		},
	})

	info := &PanicInfo{
		Timestamp:   time.Now(),
		GoroutineID: "123",
		PanicValue:  "test panic",
		Stack:       []byte("test stack"),
		Message:     "test message",
	}

	handler.Recover(info)

	if !handlerCalled {
		t.Error("Expected panic handler to be called")
	}

	messages := logger.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected panic to be logged")
	}

	// Check that panic was logged
	found := false
	for _, msg := range messages {
		if contains(msg, "PANIC RECOVERED") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'PANIC RECOVERED' in log messages")
	}
}

func TestPanicHandler_RecoverWithFunc(t *testing.T) {
	logger := &TestLogger{}
	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled: true,
		Logger:  logger,
	})

	// Test successful function
	err := handler.RecoverWithFunc(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test function that panics
	err = handler.RecoverWithFunc(func() error {
		panic("test panic")
	})

	if err == nil {
		t.Error("Expected error from recovered panic")
	}

	if !contains(err.Error(), "panic recovered") {
		t.Errorf("Expected 'panic recovered' in error message, got %s", err.Error())
	}

	messages := logger.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected panic to be logged")
	}
}

func TestPanicHandler_RecoverWithContext(t *testing.T) {
	logger := &TestLogger{}
	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled: true,
		Logger:  logger,
	})

	ctx := context.Background()

	// Test successful function
	err := handler.RecoverWithContext(ctx, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test function that panics
	err = handler.RecoverWithContext(ctx, func(ctx context.Context) error {
		panic("test panic")
	})

	if err == nil {
		t.Error("Expected error from recovered panic")
	}

	if !contains(err.Error(), "panic recovered") {
		t.Errorf("Expected 'panic recovered' in error message, got %s", err.Error())
	}
}

func TestPanicHandler_Disabled(t *testing.T) {
	logger := &TestLogger{}
	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled: false,
		Logger:  logger,
	})

	// Test that panics are not recovered when disabled
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when recovery is disabled")
		}
	}()

	handler.RecoverWithFunc(func() error {
		panic("test panic")
	})
}

func TestPanicHandler_GetStats(t *testing.T) {
	logger := &TestLogger{}
	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled: true,
		Logger:  logger,
	})

	// Initial stats
	stats := handler.GetStats()
	if stats["total_recovered"] != 0 {
		t.Errorf("Expected 0 total recovered initially, got %v", stats["total_recovered"])
	}

	// Recover a panic
	info := &PanicInfo{
		Timestamp:   time.Now(),
		GoroutineID: "123",
		PanicValue:  "test panic",
	}

	handler.Recover(info)

	// Updated stats
	stats = handler.GetStats()
	if stats["total_recovered"] != 1 {
		t.Errorf("Expected 1 total recovered, got %v", stats["total_recovered"])
	}

	goroutineCounts := stats["goroutine_counts"].(map[string]int64)
	if goroutineCounts["123"] != 1 {
		t.Errorf("Expected goroutine 123 to have 1 count, got %v", goroutineCounts["123"])
	}
}

func TestPanicHandler_ResetStats(t *testing.T) {
	logger := &TestLogger{}
	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled: true,
		Logger:  logger,
	})

	// Recover a panic
	info := &PanicInfo{
		Timestamp:   time.Now(),
		GoroutineID: "123",
		PanicValue:  "test panic",
	}

	handler.Recover(info)

	// Reset stats
	handler.ResetStats()

	stats := handler.GetStats()
	if stats["total_recovered"] != 0 {
		t.Errorf("Expected 0 total recovered after reset, got %v", stats["total_recovered"])
	}
}

func TestPanicHandler_AddHandler(t *testing.T) {
	logger := &TestLogger{}
	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled: true,
		Logger:  logger,
	})

	// Add a new handler
	newHandlerCalled := false
	handler.AddHandler(func(info PanicInfo) {
		newHandlerCalled = true
	})

	info := &PanicInfo{
		Timestamp:   time.Now(),
		GoroutineID: "123",
		PanicValue:  "test panic",
	}

	handler.Recover(info)

	// Give time for handler to run (it runs in a separate goroutine)
	time.Sleep(100 * time.Millisecond)

	if !newHandlerCalled {
		t.Error("Expected newly added handler to be called")
	}
}

func TestSafeGo(t *testing.T) {
	logger := &TestLogger{}
	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled: true,
		Logger:  logger,
	})

	// Test successful goroutine
	done := make(chan bool)
	SafeGo(handler, func() {
		done <- true
	})

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Expected goroutine to complete")
	}

	// Test goroutine that panics
	SafeGo(handler, func() {
		panic("test panic in goroutine")
	})

	// Give time for panic recovery
	time.Sleep(100 * time.Millisecond)

	messages := logger.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected panic to be logged")
	}
}

func TestSafeGoWithContext(t *testing.T) {
	logger := &TestLogger{}
	handler := NewPanicHandler(PanicRecoveryConfig{
		Enabled: true,
		Logger:  logger,
	})

	ctx := context.Background()
	done := make(chan bool)

	// Test successful goroutine with context
	SafeGoWithContext(handler, ctx, func(ctx context.Context) {
		done <- true
	})

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Expected goroutine to complete")
	}

	// Test goroutine that panics
	SafeGoWithContext(handler, ctx, func(ctx context.Context) {
		panic("test panic in goroutine with context")
	})

	// Give time for panic recovery
	time.Sleep(100 * time.Millisecond)

	messages := logger.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected panic to be logged")
	}
}

func TestSafeGo_NilHandler(t *testing.T) {
	// Test with nil handler - should work normally
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Expected no panic with nil handler, got %v", r)
		}
	}()

	done := make(chan bool)
	SafeGo(nil, func() {
		done <- true
	})

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Expected goroutine to complete")
	}
}

func TestDefaultPanicHandlers(t *testing.T) {
	handlers := DefaultPanicHandlers()

	if len(handlers) == 0 {
		t.Error("Expected at least one default panic handler")
	}

	// Test that handlers don't panic
	info := PanicInfo{
		Timestamp:   time.Now(),
		GoroutineID: "123",
		PanicValue:  "test panic",
	}

	for i, handler := range handlers {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Default handler %d panicked: %v", i, r)
			}
		}()
		handler(info)
	}
}

func TestNewDefaultPanicHandler(t *testing.T) {
	handler := NewDefaultPanicHandler()

	if !handler.IsEnabled() {
		t.Error("Expected default handler to be enabled")
	}

	// Test that it can recover panics
	err := handler.RecoverWithFunc(func() error {
		panic("test panic")
	})

	if err == nil {
		t.Error("Expected error from recovered panic")
	}
}

func TestGetGoroutineID(t *testing.T) {
	id := getGoroutineID()
	if id == "" {
		t.Error("Expected goroutine ID to be non-empty")
	}
	if id == "unknown" {
		t.Log("Warning: Could not extract goroutine ID")
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