package metrics

import (
	"testing"
	"time"

	"github.com/mount-exporter/mount-exporter/config"
	"github.com/prometheus/client_golang/prometheus"
)

func TestNewCollector(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
			Path: "/metrics",
		},
		MountPoints: []string{"/test1", "/test2"},
		Interval:    30 * time.Second,
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	collector := NewCollector(cfg)

	if collector.config != cfg {
		t.Error("Expected collector config to be set")
	}

	if collector.findmnt == nil {
		t.Error("Expected findmnt wrapper to be initialized")
	}
}

func TestCollector_Describe(t *testing.T) {
	cfg := &config.Config{
		MountPoints: []string{"/test"},
		Interval:    30 * time.Second,
	}

	collector := NewCollector(cfg)

	ch := make(chan *prometheus.Desc, 10)
	collector.Describe(ch)

	close(ch)

	descCount := 0
	for desc := range ch {
		if desc == nil {
			t.Error("Received nil descriptor")
		}
		descCount++
	}

	// Should have 4 descriptors: mount_point_status, scrape_duration, scrape_success, up
	if descCount != 4 {
		t.Errorf("Expected 4 descriptors, got %d", descCount)
	}
}

func TestCollector_Collect(t *testing.T) {
	cfg := &config.Config{
		MountPoints: []string{"/definitely-nonexistent-mount-point-12345"},
		Interval:    5 * time.Second,
	}

	collector := NewCollector(cfg)

	ch := make(chan prometheus.Metric, 20)
	collector.Collect(ch)

	close(ch)

	metricCount := 0
	for metric := range ch {
		if metric == nil {
			t.Error("Received nil metric")
		}
		metricCount++
	}

	// Should have multiple metrics for the mount point:
	// - mount_point_status
	// - scrape_duration
	// - scrape_success (only on success, so might not be present)
	// - up
	// - total_scrape_duration
	if metricCount < 3 {
		t.Errorf("Expected at least 3 metrics, got %d", metricCount)
	}
}

func TestCollector_UpdateConfig(t *testing.T) {
	cfg1 := &config.Config{
		MountPoints: []string{"/test1"},
		Interval:    30 * time.Second,
	}

	cfg2 := &config.Config{
		MountPoints: []string{"/test2"},
		Interval:    60 * time.Second,
	}

	collector := NewCollector(cfg1)

	// Verify initial config
	if len(collector.config.MountPoints) != 1 || collector.config.MountPoints[0] != "/test1" {
		t.Error("Initial config not set correctly")
	}

	// Update config
	collector.UpdateConfig(cfg2)

	// Verify updated config
	if len(collector.config.MountPoints) != 1 || collector.config.MountPoints[0] != "/test2" {
		t.Error("Config not updated correctly")
	}

	if collector.config.Interval != 60*time.Second {
		t.Error("Interval not updated correctly")
	}
}

func TestCollector_GetFindmntWrapper(t *testing.T) {
	cfg := &config.Config{
		Interval: 30 * time.Second,
	}

	collector := NewCollector(cfg)
	wrapper := collector.GetFindmntWrapper()

	if wrapper == nil {
		t.Error("Expected findmnt wrapper, got nil")
	}

	if wrapper != collector.findmnt {
		t.Error("Expected returned wrapper to be the same as internal wrapper")
	}
}

func TestCollector_MetricFormats(t *testing.T) {
	cfg := &config.Config{
		MountPoints: []string{"/definitely-nonexistent-mount-point-12345"},
		Interval:    5 * time.Second,
	}

	collector := NewCollector(cfg)

	// Test mount point status metric
	statusDesc := collector.mountPointStatus
	if statusDesc == nil {
		t.Error("Expected mount point status descriptor to be set")
	}

	// Test scrape duration metric
	durationDesc := collector.scrapeDuration
	if durationDesc == nil {
		t.Error("Expected scrape duration descriptor to be set")
	}

	// Test scrape success metric
	successDesc := collector.scrapeSuccess
	if successDesc == nil {
		t.Error("Expected scrape success descriptor to be set")
	}

	// Test up metric
	upDesc := collector.up
	if upDesc == nil {
		t.Error("Expected up descriptor to be set")
	}

	// Test that all descriptors are properly created
	// (We can't easily inspect internal fields of prometheus.Desc,
	// but we can verify they're not nil and the collector works)
}

func TestCollector_MetricsCollectionErrorHandling(t *testing.T) {
	cfg := &config.Config{
		MountPoints: []string{"/definitely-nonexistent-mount-point-12345"},
		Interval:    1 * time.Millisecond, // Very short timeout to trigger errors
	}

	collector := NewCollector(cfg)

	ch := make(chan prometheus.Metric, 20)
	collector.Collect(ch)

	close(ch)

	metricCount := 0
	statusMetricFound := false

	for metric := range ch {
		metricCount++

		// Parse metric to check if it's a status metric
		desc := metric.Desc()
		if desc != nil && desc.String() == collector.mountPointStatus.String() {
			statusMetricFound = true
		}
	}

	if !statusMetricFound {
		t.Error("Expected to find mount point status metric")
	}

	if metricCount == 0 {
		t.Error("Expected at least one metric to be collected")
	}
}

func TestCollector_ConcurrentCollection(t *testing.T) {
	cfg := &config.Config{
		MountPoints: []string{"/definitely-nonexistent-mount-point-12345"},
		Interval:    5 * time.Second,
	}

	collector := NewCollector(cfg)

	// Test concurrent collection
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			ch := make(chan prometheus.Metric, 20)
			collector.Collect(ch)
			close(ch)

			// Count metrics
			count := 0
			for range ch {
				count++
			}

			if count == 0 {
				t.Errorf("Expected at least one metric in goroutine %d", i)
			}

			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out waiting for goroutines to complete")
		}
	}
}

func TestCollector_MultipleMountPoints(t *testing.T) {
	cfg := &config.Config{
		MountPoints: []string{
			"/definitely-nonexistent-mount-point-1",
			"/definitely-nonexistent-mount-point-2",
			"/definitely-nonexistent-mount-point-3",
		},
		Interval: 5 * time.Second,
	}

	collector := NewCollector(cfg)

	ch := make(chan prometheus.Metric, 50)
	collector.Collect(ch)

	close(ch)

	metricCount := 0
	statusMetrics := 0

	for metric := range ch {
		metricCount++

		// Count status metrics
		desc := metric.Desc()
		if desc != nil && desc.String() == collector.mountPointStatus.String() {
			statusMetrics++
		}
	}

	// Should have 3 status metrics (one for each mount point)
	if statusMetrics != 3 {
		t.Errorf("Expected 3 status metrics, got %d", statusMetrics)
	}

	// Should have multiple metrics: 3 status + 3 duration + up + total_duration + maybe some success metrics
	if metricCount < 8 {
		t.Errorf("Expected at least 8 total metrics for 3 mount points, got %d", metricCount)
	}
}

// Helper function to compare string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// BenchmarkCollect benchmarks the Collect method
func BenchmarkCollect(b *testing.B) {
	cfg := &config.Config{
		MountPoints: []string{
			"/definitely-nonexistent-mount-point-1",
			"/definitely-nonexistent-mount-point-2",
			"/definitely-nonexistent-mount-point-3",
		},
		Interval: 5 * time.Second,
	}

	collector := NewCollector(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch := make(chan prometheus.Metric, 50)
		collector.Collect(ch)
		close(ch)

		// Drain the channel
		for range ch {
		}
	}
}