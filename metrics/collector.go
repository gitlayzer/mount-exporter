package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/mount-exporter/mount-exporter/config"
	"github.com/mount-exporter/mount-exporter/system"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "mount_exporter"
	subsystem = ""
)

// Collector collects mount point metrics
type Collector struct {
	config     *config.Config
	findmnt    *system.FindmntWrapper
	mu         sync.RWMutex

	// Metrics
	mountPointStatus *prometheus.Desc
	scrapeDuration   *prometheus.Desc
	scrapeSuccess    *prometheus.Desc
	up               *prometheus.Desc
}

// NewCollector creates a new metrics collector
func NewCollector(cfg *config.Config) *Collector {
	return &Collector{
		config:  cfg,
		findmnt: system.NewFindmntWrapper(cfg.Interval),
		mountPointStatus: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "mount_point_status"),
			"Mount point availability status (1=mounted, 0=not mounted)",
			[]string{"mount_point", "target", "fs_type", "source", "error"},
			nil,
		),
		scrapeDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "scrape_duration_seconds"),
			"Time spent scraping mount point status",
			[]string{"mount_point"},
			nil,
		),
		scrapeSuccess: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "scrape_success_total"),
			"Total number of successful scrapes",
			[]string{"mount_point"},
			nil,
		),
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "up"),
			"Whether the mount exporter is healthy (1=healthy, 0=unhealthy)",
			nil,
			nil,
		),
	}
}

// Describe implements prometheus.Collector interface
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.mountPointStatus
	ch <- c.scrapeDuration
	ch <- c.scrapeSuccess
	ch <- c.up
}

// Collect implements prometheus.Collector interface
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	start := time.Now()
	healthy := 1

	// Check all mount points
	for _, mountPoint := range c.config.MountPoints {
		scrapeStart := time.Now()
		result := c.findmnt.CheckMountPoint(context.Background(), mountPoint)
		scrapeDuration := time.Since(scrapeStart).Seconds()

		var value float64
		var target, fsType, source, errorMsg string

		if result.Error != nil {
			healthy = 0
			value = 0
			errorMsg = result.Error.Error()
		} else {
			switch result.Status {
			case system.MountStatusMounted:
				value = 1
			case system.MountStatusNotMounted:
				value = 0
			default:
				value = 0
				errorMsg = "unknown status"
			}
		}

		target = result.Target
		fsType = result.FSType
		source = result.Source

		// Export mount point status metric
		ch <- prometheus.MustNewConstMetric(
			c.mountPointStatus,
			prometheus.GaugeValue,
			value,
			mountPoint, target, fsType, source, errorMsg,
		)

		// Export scrape duration metric
		ch <- prometheus.MustNewConstMetric(
			c.scrapeDuration,
			prometheus.GaugeValue,
			scrapeDuration,
			mountPoint,
		)

		// Export scrape success metric (increment on success)
		if result.Error == nil {
			ch <- prometheus.MustNewConstMetric(
				c.scrapeSuccess,
				prometheus.CounterValue,
				1,
				mountPoint,
			)
		}
	}

	// Export overall health metric
	ch <- prometheus.MustNewConstMetric(
		c.up,
		prometheus.GaugeValue,
		float64(healthy),
	)

	// Export total scrape duration
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "total_scrape_duration_seconds"),
			"Total time spent scraping all mount points",
			nil,
			nil,
		),
		prometheus.GaugeValue,
		time.Since(start).Seconds(),
	)
}

// UpdateConfig updates the collector configuration
func (c *Collector) UpdateConfig(cfg *config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.config = cfg
	c.findmnt = system.NewFindmntWrapper(cfg.Interval)
}

// GetFindmntWrapper returns the findmnt wrapper for external use
func (c *Collector) GetFindmntWrapper() *system.FindmntWrapper {
	return c.findmnt
}