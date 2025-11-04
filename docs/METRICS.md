# Mount Exporter Metrics Documentation

This document describes all metrics exposed by the Mount Exporter for monitoring filesystem mount point availability.

## Table of Contents

- [Overview](#overview)
- [Available Metrics](#available-metrics)
- [Metric Labels](#metric-labels)
- [PromQL Examples](#promql-examples)
- [Grafana Dashboard](#grafana-dashboard)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Overview

Mount Exporter exposes metrics in Prometheus format that provide comprehensive monitoring of filesystem mount points. The metrics are designed to help you:

- Monitor mount point availability and status
- Track performance of mount point checks
- Monitor the health of the exporter itself
- Set up alerts for storage infrastructure issues

## Available Metrics

### 1. Mount Point Status

**Metric Name**: `mount_exporter_mount_point_status`

**Type**: Gauge

**Description**: Indicates whether each configured mount point is currently mounted (1) or not mounted (0).

**Labels**:
- `mount_point`: The path of the mount point being monitored
- `target`: The actual target where the mount point is mounted (if available)
- `fs_type`: Filesystem type (e.g., ext4, xfs, nfs) (if available)
- `source`: Source device (e.g., /dev/sda1) (if available)
- `error`: Error message if checking failed (if applicable)

**Values**:
- `1`: Mount point is mounted and accessible
- `0`: Mount point is not mounted or inaccessible

**Example**:
```
# HELP mount_exporter_mount_point_status Mount point availability status (1=mounted, 0=not mounted)
# TYPE mount_exporter_mount_point_status gauge
mount_exporter_mount_point_status{mount_point="/",target="/dev/sda1",fs_type="ext4",source="/dev/sda1"} 1
mount_exporter_mount_point_status{mount_point="/data",target="/dev/sdb1",fs_type="ext4",source="/dev/sdb1"} 1
mount_exporter_mount_point_status{mount_point="/var/log",target="/dev/sda2",fs_type="ext4",source="/dev/sda2"} 1
mount_exporter_mount_point_status{mount_point="/mnt/backups",target="nas.example.com:/backups",fs_type="nfs4",source="nas.example.com:/backups"} 0
mount_exporter_mount_point_status{mount_point="/nonexistent",error="findmnt command failed: No such file or directory"} 0
```

**Use Cases**:
- Alert when critical mount points become unmounted
- Monitor storage infrastructure health
- Track mount point availability over time
- Create dashboards showing mount point status

### 2. Scrape Duration

**Metric Name**: `mount_exporter_scrape_duration_seconds`

**Type**: Gauge

**Description**: Time taken to check the status of each mount point.

**Labels**:
- `mount_point`: The path of the mount point being checked

**Example**:
```
# HELP mount_exporter_scrape_duration_seconds Time spent scraping mount point status
# TYPE mount_exporter_scrape_duration_seconds gauge
mount_exporter_scrape_duration_seconds{mount_point="/"} 0.045
mount_exporter_scrape_duration_seconds{mount_point="/data"} 0.032
mount_exporter_scrape_duration_seconds{mount_point="/var/log"} 0.028
mount_exporter_scrape_duration_seconds{mount_point="/mnt/backups"} 1.234
mount_exporter_scrape_duration_seconds{mount_point="/remote/nfs"} 2.456
```

**Use Cases**:
- Monitor performance of the exporter
- Identify slow or problematic mount points
- Set up alerts for scrape duration thresholds
- Optimize collection intervals

### 3. Scrape Success Count

**Metric Name**: `mount_exporter_scrape_success_total`

**Type**: Counter

**Description**: Total number of successful scrape operations for each mount point.

**Labels**:
- `mount_point`: The path of the mount point

**Example**:
```
# HELP mount_exporter_scrape_success_total Total number of successful scrapes
# TYPE mount_exporter_scrape_success_total counter
mount_exporter_scrape_success_total{mount_point="/"} 1250
mount_exporter_scrape_success_total{mount_point="/data"} 1250
mount_exporter_scrape_success_total{mount_point="/var/log"} 1250
mount_exporter_scrape_success_total{mount_point="/mnt/backups"} 1200
```

**Use Cases**:
- Track reliability of mount point monitoring
- Calculate success rates
- Identify problematic mount points
- Monitor overall system health

### 4. Application Health

**Metric Name**: `mount_exporter_up`

**Type**: Gauge

**Description**: Overall health status of the mount exporter application.

**Values**:
- `1`: Application is healthy and functioning normally
- `0`: Application has issues (e.g., findmnt not available)

**Labels**: None

**Example**:
```
# HELP mount_exporter_up Whether the mount exporter is healthy (1=healthy, 0=unhealthy)
# TYPE mount_exporter_up gauge
mount_exporter_up 1
```

**Use Cases**:
- Monitor application health
- Set up alerts for exporter downtime
- Track application uptime
- Include in service health dashboards

### 5. Total Scrape Duration

**Metric Name**: `mount_exporter_total_scrape_duration_seconds`

**Type**: Gauge

**Description**: Total time spent during a complete scrape cycle collecting all mount point metrics.

**Labels**: None

**Example**:
```
# HELP mount_exporter_total_scrape_duration_seconds Total time spent scraping all mount points
# TYPE mount_exporter_total_scrape_duration_seconds gauge
mount_exporter_total_scrape_duration_seconds 0.156
```

**Use Cases**:
- Monitor overall scrape performance
- Identify performance bottlenecks
- Optimize collection intervals
- Track impact of adding more mount points

## Metric Labels

### Common Labels

#### `mount_point`
- **Description**: The absolute path of the mount point
- **Example Values**: `/data`, `/var/log`, `/mnt/backups`
- **Cardinality**: High - depends on configuration
- **Usage**: Primary identifier for each mount point

#### `target`
- **Description**: Where the mount point is actually mounted
- **Example Values**: `/dev/sda1`, `nas.example.com:/data`
- **Cardinality**: Medium - depends on system
- **Usage**: Helps identify the actual mount target

#### `fs_type`
- **Description**: Filesystem type
- **Example Values**: `ext4`, `xfs`, `nfs4`, `cifs`
- **Cardinality**: Low - standard filesystem types
- **Usage**: Useful for grouping by filesystem type

#### `source`
- **Description**: Source device or network path
- **Example Values**: `/dev/sda1`, `server.example.com:/export`
- **Cardinality**: Medium - depends on configuration
- **Usage**: Helps identify mount source

#### `error`
- **Description**: Error message if mount point checking failed
- **Example Values**: `findmnt command failed`, `timeout`
- **Cardinality**: High - various error types
- **Usage**: Debugging and troubleshooting

### Label Cardinality Considerations

- **Mount Points**: High cardinality expected - monitor critical mount points only
- **Filesystem Types**: Low cardinality - suitable for grouping
- **Error Messages**: High cardinality - useful for debugging but may impact performance

## PromQL Examples

### Basic Monitoring

```promql
# Show all mount point statuses
mount_exporter_mount_point_status

# Count mounted mount points
sum(mount_exporter_mount_point_status)

# Count unmounted mount points
sum(mount_exporter_mount_point_status == 0)

# Check if exporter is healthy
mount_exporter_up

# Show total scrape duration
mount_exporter_total_scrape_duration_seconds
```

### Status Monitoring

```promql
# Show only unmounted mount points
mount_exporter_mount_point_status == 0

# Show mount points by filesystem type
sum by (fs_type) (mount_exporter_mount_point_status)

# Calculate mount point availability percentage
(sum(mount_exporter_mount_point_status) / count(count(mount_exporter_mount_point_status))) * 100

# Show mount points with errors
mount_exporter_mount_point_status{error!=""}
```

### Performance Monitoring

```promql
# Average scrape duration for all mount points
avg(mount_exporter_scrape_duration_seconds)

# Maximum scrape duration
max(mount_exporter_scrape_duration_seconds)

# Show slowest mount points
topk(5, mount_exporter_scrape_duration_seconds)

# 95th percentile scrape duration
histogram_quantile(0.95, rate(mount_exporter_scrape_duration_seconds_bucket[5m]))

# Total scrape duration trend
rate(mount_exporter_total_scrape_duration_seconds[5m])
```

### Reliability Monitoring

```promql
# Success rate for each mount point (last 5 minutes)
rate(mount_exporter_scrape_success_total[5m])

# Total successful scrapes per hour
increase(mount_exporter_scrape_success_total[1h])

# Monitor application uptime
avg_over_time(mount_exporter_up[1h])

# Check for exporter downtime
absent(mount_exporter_up)
```

### Advanced Queries

```promql
# Mount point availability over time (last hour)
avg_over_time(mount_exporter_mount_point_status{mount_point="/data"}[1h])

# Mount point status by filesystem type
sum by (fs_type) (mount_exporter_mount_point_status)

# Network mount point monitoring
mount_exporter_mount_point_status{fs_type=~"nfs.*|cifs.*|smb.*"}

# Local mount point monitoring
mount_exporter_mount_point_status{fs_type=~"ext.*|xfs|btrfs"}

# Success rate by mount point
rate(mount_exporter_scrape_success_total[5m]) / rate(mount_exporter_scrape_duration_seconds_count[5m])

# Identify mount points with high latency
mount_exporter_scrape_duration_seconds > 1

# Mount point availability heatmap (requires heatmap panel in Grafana)
mount_exporter_mount_point_status
```

## Grafana Dashboard

### Panel Configurations

#### Mount Point Status Overview
```json
{
  "title": "Mount Point Status Overview",
  "type": "stat",
  "targets": [
    {
      "expr": "sum(mount_exporter_mount_point_status)",
      "legendFormat": "Mounted Mount Points"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "color": {
        "mode": "thresholds"
      },
      "thresholds": {
        "steps": [
          {"color": "red", "value": 0},
          {"color": "green", "value": 1}
        ]
      }
    }
  }
}
```

#### Mount Point Status Table
```json
{
  "title": "Mount Point Status Details",
  "type": "table",
  "targets": [
    {
      "expr": "mount_exporter_mount_point_status",
      "format": "table"
    }
  ],
  "transformations": [
    {
      "id": "organize",
      "options": {
        "excludeByName": {"Time": true},
        "indexByName": {
          "mount_point": 0,
          "target": 1,
          "fs_type": 2,
          "source": 3,
          "error": 4,
          "Value": 5
        }
      }
    }
  ]
}
```

#### Scrape Duration Graph
```json
{
  "title": "Mount Point Check Duration",
  "type": "graph",
  "targets": [
    {
      "expr": "mount_exporter_scrape_duration_seconds",
      "legendFormat": "{{mount_point}}"
    }
  ],
  "yAxes": [
    {
      "label": "Duration (seconds)",
      "unit": "s"
    }
  ]
}
```

#### Application Health Panel
```json
{
  "title": "Application Health",
  "type": "singlestat",
  "targets": [
    {
      "expr": "mount_exporter_up",
      "legendFormat": "Health Status"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "mappings": [
        {"value": "0", "text": "UNHEALTHY", "color": "red"},
        {"value": "1", "text": "HEALTHY", "color": "green"}
      ]
    }
  }
}
```

### Dashboard Variables

```json
{
  "mount_point": {
    "query": "label_values(mount_exporter_mount_point_status, mount_point)",
    "multi": true,
    "includeAll": true
  },
  "fs_type": {
    "query": "label_values(mount_exporter_mount_point_status, fs_type)",
    "multi": true,
    "includeAll": true
  }
}
```

## Troubleshooting

### Metrics Not Appearing

1. **Check Configuration**: Ensure mount points are correctly configured
2. **Verify findmnt**: Make sure the `findmnt` command is available
3. **Check Permissions**: Verify the exporter can access mount points
4. **Review Logs**: Check for error messages in application logs

### Unexpected Values

1. **Always 0**: Mount point may not exist or be inaccessible
2. **No Labels**: Mount point checking failed (check error logs)
3. **High Duration**: Network mount points or system performance issues
4. **Missing Metrics**: Application startup issues (check health endpoint)

### Performance Issues

1. **High Scrape Duration**: Reduce collection interval or number of mount points
2. **Memory Usage**: Monitor memory usage with many mount points
3. **CPU Usage**: Optimize timeout values and collection frequency
4. **Network Issues**: Check network connectivity for remote mounts

### Debugging Metrics

```bash
# Check what metrics are available
curl -s http://localhost:8080/metrics | grep "^mount_exporter_"

# Verify specific metric values
curl -s http://localhost:8080/metrics | grep "mount_exporter_mount_point_status"

# Check application health
curl -s http://localhost:8080/health | jq .

# Monitor metric collection in real-time
watch -n 5 'curl -s http://localhost:8080/metrics | grep "mount_exporter_mount_point_status"'
```

## Best Practices

### Configuration

1. **Monitor Critical Mount Points**: Only monitor mount points that are critical for your infrastructure
2. **Use Descriptive Names**: Use clear mount point names for easier identification
3. **Set Appropriate Intervals**: Balance between responsiveness and system load
4. **Validate Configuration**: Test configuration before deploying to production

### Alerting

1. **Set Appropriate Thresholds**: Configure alerts based on your requirements
2. **Use Multi-level Alerts**: Different severity levels for different issues
3. **Include Context**: Add descriptive alert messages with mount point information
4. **Test Alerts**: Verify alerts work as expected during maintenance windows

### Monitoring

1. **Monitor the Exporter**: Include exporter health in your monitoring
2. **Track Performance**: Monitor scrape duration and success rates
3. **Use Labels Effectively**: Group and filter metrics using labels
4. **Create Meaningful Dashboards**: Design dashboards that provide actionable insights

### Performance

1. **Optimize Collection Intervals**: Don't collect too frequently
2. **Limit Mount Points**: Monitor only necessary mount points
3. **Network Mounts**: Be aware of latency for remote filesystems
4. **Resource Planning**: Monitor CPU and memory usage

### Documentation

1. **Document Configuration**: Keep configuration files under version control
2. **Document Dependencies**: Record system requirements and dependencies
3. **Maintain Alerting Rules**: Keep alerting rules in version control
4. **Update Documentation**: Keep metric documentation current with changes

## Integration Examples

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 30s
  evaluation_interval: 30s

scrape_configs:
  - job_name: 'mount-exporter'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 30s
    scrape_timeout: 10s
    metrics_path: /metrics
    honor_labels: true
```

### Alertmanager Rules

```yaml
# mount-exporter-rules.yml
groups:
  - name: mount-exporter
    rules:
      - alert: MountPointUnavailable
        expr: mount_exporter_mount_point_status == 0
        for: 2m
        labels:
          severity: critical
          service: storage
        annotations:
          summary: "Mount point {{ $labels.mount_point }} is unavailable"
          description: "Mount point {{ $labels.mount_point }} has been unavailable for more than 2 minutes."
          runbook_url: "https://docs.company.com/storage/troubleshooting"

      - alert: MountExporterDown
        expr: up{job="mount-exporter"} == 0
        for: 1m
        labels:
          severity: critical
          service: monitoring
        annotations:
          summary: "Mount exporter is down"
          description: "Mount exporter has been down for more than 1 minute."

      - alert: MountExporterHighLatency
        expr: mount_exporter_scrape_duration_seconds > 5
        for: 5m
        labels:
          severity: warning
          service: storage
        annotations:
          summary: "High scrape duration for mount point {{ $labels.mount_point }}"
          description: "Scrape duration for mount point {{ $labels.mount_point }} is {{ $value }}s."
```

### Docker Compose Integration

```yaml
# docker-compose.yml
version: '3.8'

services:
  mount-exporter:
    image: ghcr.io/mount-exporter/mount-exporter:latest
    container_name: mount-exporter
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/etc/mount-exporter/config.yaml:ro
      - /:/host:ro
    environment:
      - MOUNT_EXPORTER_LOG_LEVEL=info
    healthcheck:
      test: ["CMD", "curl", "-f", "-s", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    labels:
      - "prometheus.scrape=true"
      - "prometheus.port=8080"
      - "prometheus.path=/metrics"
```

This comprehensive metrics documentation provides everything needed to effectively monitor and troubleshoot mount point availability using Mount Exporter.