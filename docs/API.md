# API Documentation

This document describes the HTTP API endpoints provided by Mount Exporter.

## Base URL

```
http://localhost:8080
```

The port and host can be configured in the `config.yaml` file.

## Endpoints

### 1. GET `/metrics`

**Description**: Prometheus metrics endpoint for scraping mount point status and application health metrics.

**Method**: `GET`

**Path**: `/metrics`

**Headers**:
- `Content-Type`: `text/plain; version=0.0.4; charset=utf-8`
- `X-Content-Type-Options`: `nosniff`
- `X-Frame-Options`: `DENY`
- `X-XSS-Protection`: `1; mode=block`
- `Referrer-Policy`: `strict-origin-when-cross-origin`

**Query Parameters**: None

**Response Format**: Prometheus text format

**Response Codes**:
- `200 OK`: Metrics successfully returned
- `500 Internal Server Error`: Application error occurred

**Response Body Example**:
```
# HELP mount_exporter_mount_point_status Mount point availability status (1=mounted, 0=not mounted)
# TYPE mount_exporter_mount_point_status gauge
mount_exporter_mount_point_status{mount_point="/",target="/dev/sda1",fs_type="ext4",source="/dev/sda1"} 1
mount_exporter_mount_point_status{mount_point="/data",target="/dev/sdb1",fs_type="ext4",source="/dev/sdb1"} 1
mount_exporter_mount_point_status{mount_point="/var/log",target="/dev/sda2",fs_type="ext4",source="/dev/sda2"} 1
mount_exporter_mount_point_status{mount_point="/mnt/backups",target="nas.example.com:/backups",fs_type="nfs4",source="nas.example.com:/backups"} 0

# HELP mount_exporter_scrape_duration_seconds Time spent scraping mount point status
# TYPE mount_exporter_scrape_duration_seconds gauge
mount_exporter_scrape_duration_seconds{mount_point="/"} 0.023
mount_exporter_scrape_duration_seconds{mount_point="/data"} 0.045
mount_exporter_scrape_duration_seconds{mount_point="/var/log"} 0.032
mount_exporter_scrape_duration_seconds{mount_point="/mnt/backups"} 1.234

# HELP mount_exporter_scrape_success_total Total number of successful scrapes
# TYPE mount_exporter_scrape_success_total counter
mount_exporter_scrape_success_total{mount_point="/"} 150
mount_exporter_scrape_success_total{mount_point="/data"} 150
mount_exporter_scrape_success_total{mount_point="/var/log"} 150
mount_exporter_scrape_success_total{mount_point="/mnt/backups"} 120

# HELP mount_exporter_up Whether the mount exporter is healthy (1=healthy, 0=unhealthy)
# TYPE mount_exporter_up gauge
mount_exporter_up 1

# HELP mount_exporter_total_scrape_duration_seconds Total time spent scraping all mount points
# TYPE mount_exporter_total_scrape_duration_seconds gauge
mount_exporter_total_scrape_duration_seconds 0.156
```

**Usage Example**:
```bash
curl http://localhost:8080/metrics
```

### 2. GET `/health`

**Description**: Health check endpoint for monitoring application health and availability.

**Method**: `GET`

**Path**: `/health`

**Headers**:
- `Content-Type`: `application/json`
- Security headers (same as `/metrics`)

**Query Parameters**: None

**Response Format**: JSON

**Response Codes**:
- `200 OK`: Application is healthy
- `503 Service Unavailable`: Application has issues

**Response Body (Healthy)**:
```json
{
  "status": "healthy"
}
```

**Response Body (Unhealthy)**:
```json
{
  "status": "unhealthy",
  "error": "findmnt command not available"
}
```

**Usage Example**:
```bash
curl http://localhost:8080/health
```

**Response Examples**:
```bash
# Healthy response
curl -s http://localhost:8080/health | jq .
{
  "status": "healthy"
}

# Unhealthy response
curl -s http://localhost:8080/health | jq .
{
  "status": "unhealthy",
  "error": "findmnt command not available"
}
```

### 3. GET `/healthz`

**Description**: Alternative health check endpoint (compatible with Kubernetes health checks).

**Method**: `GET`

**Path**: `/healthz`

**Response**: Same as `/health` endpoint

**Usage Example**:
```bash
curl http://localhost:8080/healthz
```

### 4. GET `/`

**Description**: Root endpoint providing basic application information.

**Method**: `GET`

**Path**: `/`

**Headers**:
- `Content-Type`: `text/html; charset=utf-8`
- Security headers (same as other endpoints)

**Query Parameters**: None

**Response Format**: HTML

**Response Codes**:
- `200 OK`: Information successfully returned

**Response Body Example**:
```html
<!DOCTYPE html>
<html>
<head>
    <title>Mount Exporter</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 800px; margin: 0 auto; }
        .metric { margin: 10px 0; }
        .label { font-weight: bold; }
        .status-healthy { color: green; }
        .status-unhealthy { color: red; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Mount Exporter</h1>
        <p>A Prometheus exporter for monitoring filesystem mount point availability.</p>

        <h2>Application Status</h2>
        <div class="metric">
            <span class="label">Status:</span>
            <span class="status-healthy">Healthy</span>
        </div>
        <div class="metric">
            <span class="label">Version:</span>
            v1.0.0
        </div>
        <div class="metric">
            <span class="label">Uptime:</span>
            2h30m45s
        </div>

        <h2>Available Endpoints</h2>
        <ul>
            <li><a href="/metrics">/metrics</a> - Prometheus metrics endpoint</li>
            <li><a href="/health">/health</a> - Health check endpoint</li>
            <li><a href="/healthz">/healthz</a> - Alternative health check endpoint</li>
        </ul>

        <h2>Documentation</h2>
        <p>For more information, see the <a href="https://github.com/mount-exporter/mount-exporter">GitHub repository</a>.</p>
    </div>
</body>
</html>
```

**Usage Example**:
```bash
curl http://localhost:8080/
```

## Metrics Reference

### Mount Point Metrics

#### `mount_exporter_mount_point_status`

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

**Example PromQL Queries**:
```promql
# Show all mount point statuses
mount_exporter_mount_point_status

# Count mounted mount points
sum(mount_exporter_mount_point_status)

# Show unmounted mount points
mount_exporter_mount_point_status == 0

# Count unmounted mount points
sum(mount_exporter_mount_point_status == 0)
```

#### `mount_exporter_scrape_duration_seconds`

**Type**: Gauge
**Description**: Time taken to check the status of each mount point.

**Labels**:
- `mount_point`: The path of the mount point being checked

**Example PromQL Queries**:
```promql
# Average scrape duration
avg(mount_exporter_scrape_duration_seconds)

# Maximum scrape duration
max(mount_exporter_scrape_duration_seconds)

# Scrape duration by mount point
mount_exporter_scrape_duration_seconds
```

#### `mount_exporter_scrape_success_total`

**Type**: Counter
**Description**: Total number of successful scrape operations for each mount point.

**Labels**:
- `mount_point`: The path of the mount point

**Example PromQL Queries**:
```promql
# Rate of successful scrapes
rate(mount_exporter_scrape_success_total[5m])

# Total successful scrapes per mount point
mount_exporter_scrape_success_total
```

### Application Metrics

#### `mount_exporter_up`

**Type**: Gauge
**Description**: Overall health status of the mount exporter application.

**Labels**: None

**Values**:
- `1`: Application is healthy and functioning normally
- `0`: Application has issues (e.g., findmnt not available)

**Example PromQL Queries**:
```promql
# Check if exporter is up
mount_exporter_up

# Alert if exporter is down
mount_exporter_up == 0
```

#### `mount_exporter_total_scrape_duration_seconds`

**Type**: Gauge
**Description**: Total time spent during a complete scrape cycle collecting all mount point metrics.

**Labels**: None

**Example PromQL Queries**:
```promql
# Total scrape duration over time
mount_exporter_total_scrape_duration_seconds

# Average total scrape duration
avg_over_time(mount_exporter_total_scrape_duration_seconds[1h])
```

## HTTP Status Codes

| Status Code | Description | Endpoints |
|-------------|-------------|-----------|
| 200 OK | Request successful | All endpoints |
| 400 Bad Request | Invalid request (unsupported method for health endpoints) | /health, /healthz |
| 404 Not Found | Endpoint not found | Any invalid path |
| 405 Method Not Allowed | Method not supported | /health, /healthz |
| 500 Internal Server Error | Application error | Any endpoint |
| 503 Service Unavailable | Service unavailable (unhealthy) | /health, /healthz |

## Request Headers

### Request Headers Supported

| Header | Description | Required |
|--------|-------------|----------|
| `User-Agent` | Client identification | Optional |
| `Accept` | Response format preference | Optional |
| `Accept-Encoding` | Supported encodings | Optional |

### Response Headers

| Header | Description | Value |
|--------|-------------|-------|
| `Content-Type` | Response format | Varies by endpoint |
| `Content-Length` | Response body length | Auto-generated |
| `Date` | Response timestamp | Auto-generated |
| `X-Content-Type-Options` | MIME type sniffing protection | `nosniff` |
| `X-Frame-Options` | Clickjacking protection | `DENY` |
| `X-XSS-Protection` | XSS protection | `1; mode=block` |
| `Referrer-Policy` | Referrer policy | `strict-origin-when-cross-origin` |

## Rate Limiting

Mount Exporter does not implement explicit rate limiting. However:

- The `/metrics` endpoint is designed for Prometheus scraping (typically every 30-60 seconds)
- Health check endpoints are lightweight and can be called frequently
- No authentication or authorization is required by default

## Security Considerations

### Authentication
By default, Mount Exporter does not require authentication. For production environments, consider:

1. **Network Security**:
   - Run behind firewall
   - Use network segmentation
   - Limit access to Prometheus servers only

2. **Reverse Proxy**:
   ```nginx
   location /metrics {
       auth_basic "Mount Exporter";
       auth_basic_user_file /etc/nginx/.htpasswd;
       proxy_pass http://localhost:8080;
   }
   ```

3. **TLS/HTTPS**:
   ```nginx
   server {
       listen 443 ssl;
       ssl_certificate /path/to/cert.pem;
       ssl_certificate_key /path/to/key.pem;

       location /metrics {
           proxy_pass http://localhost:8080;
       }
   }
   ```

### Authorization
No built-in authorization. Implement at network layer or via reverse proxy.

### Input Validation
All inputs are validated:
- Mount point paths must be absolute and valid
- Configuration is validated at startup
- HTTP requests are validated for proper methods and paths

## Client Libraries

### Prometheus
The metrics endpoint is designed for Prometheus scraping:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'mount-exporter'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 30s
    metrics_path: /metrics
```

### curl Examples

```bash
# Get metrics
curl -s http://localhost:8080/metrics

# Check health
curl -s http://localhost:8080/health | jq .

# Get application info
curl -s http://localhost:8080/

# Verbose request
curl -v http://localhost:8080/metrics

# With custom headers
curl -H "User-Agent: CustomClient/1.0" http://localhost:8080/health
```

### Python Examples

```python
import requests
import json

# Health check
response = requests.get('http://localhost:8080/health')
health_data = response.json()
print(f"Status: {health_data['status']}")

# Get metrics
response = requests.get('http://localhost:8080/metrics')
metrics = response.text
print(f"Metrics received: {len(metrics)} characters")

# Parse specific metric
import re
status_match = re.search(r'mount_exporter_up (\d+)', metrics)
if status_match:
    is_up = bool(int(status_match.group(1)))
    print(f"Exporter is up: {is_up}")
```

### Go Examples

```go
package main

import (
    "fmt"
    "io/ioutil"
    "net/http"
    "encoding/json"
)

type HealthResponse struct {
    Status string `json:"status"`
    Error  string `json:"error,omitempty"`
}

func main() {
    // Health check
    resp, err := http.Get("http://localhost:8080/health")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    var health HealthResponse
    if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
        panic(err)
    }
    fmt.Printf("Health Status: %s\n", health.Status)

    // Get metrics
    resp, err = http.Get("http://localhost:8080/metrics")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    metrics, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Metrics length: %d bytes\n", len(metrics))
}
```

## Error Handling

### HTTP Errors

All endpoints return appropriate HTTP status codes and error messages:

```bash
# Method not allowed
curl -X POST http://localhost:8080/health
# Returns: 405 Method Not Allowed

# Not found
curl http://localhost:8080/nonexistent
# Returns: 404 Not Found

# Service unavailable (if findmnt missing)
curl http://localhost:8080/health
# Returns: 503 Service Unavailable
```

### Application Errors

Errors are logged to the application log and reflected in health status:

- **findmnt unavailable**: Health endpoint returns 503
- **Configuration errors**: Application fails to start
- **Mount point errors**: Reflected in metric values and error labels

## Integration Examples

### Docker Health Check

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
```

### Kubernetes Liveness/Readiness Probes

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Systemd Service Monitoring

```ini
[Unit]
Description=Mount Exporter
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/mount-exporter
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Monitoring with Prometheus Alerts

```yaml
groups:
- name: mount-exporter
  rules:
  - alert: MountExporterDown
    expr: up{job="mount-exporter"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Mount exporter is down"
      description: "Mount exporter has been down for more than 1 minute."

  - alert: MountPointUnavailable
    expr: mount_exporter_mount_point_status == 0
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Mount point {{ $labels.mount_point }} is unavailable"
      description: "Mount point {{ $labels.mount_point }} has been unavailable for more than 2 minutes."
```

## Versioning

The API is versioned through the application version, which can be found:

1. **Root endpoint**: HTML page shows version
2. **Binary**: `mount-exporter -version`
3. **Build info**: Available in logs at startup

API changes follow semantic versioning:
- **Major**: Breaking changes to API structure
- **Minor**: New features, backward compatible
- **Patch**: Bug fixes, security updates

## Support

For API issues:

1. Check the [troubleshooting guide](TROUBLESHOOTING.md)
2. Review [GitHub issues](https://github.com/mount-exporter/mount-exporter/issues)
3. Create new issue with:
   - API endpoint used
   - Request/response details
   - Error messages
   - Expected vs actual behavior