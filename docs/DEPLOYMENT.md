# Deployment Guide

This guide covers different deployment strategies for Mount Exporter.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Binary Deployment](#binary-deployment)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Systemd Deployment](#systemd-deployment)
- [Monitoring and Alerting](#monitoring-and-alerting)
- [Troubleshooting](#troubleshooting)
- [Maintenance](#maintenance)

## Prerequisites

### System Requirements
- **Operating System**: Linux (Ubuntu 18.04+, RHEL 7+, CentOS 7+), macOS 10.15+, Windows 10+
- **Memory**: Minimum 64MB RAM
- **Disk**: 50MB free disk space
- **Network**: Port access for configured metrics port (default: 8080)

### Required System Commands
- `findmnt` command (part of util-linux package)
- Network access to Prometheus server (if pushing metrics)

### Installing findmnt

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install util-linux
```

**CentOS/RHEL:**
```bash
sudo yum install util-linux
# For RHEL 8+ or CentOS 8+
sudo dnf install util-linux
```

**macOS:**
```bash
# findmnt is typically available on macOS
# If not available, install coreutils
brew install coreutils
```

## Binary Deployment

### 1. Download the Binary

Download the appropriate binary for your platform from the [releases page](https://github.com/mount-exporter/mount-exporter/releases).

```bash
# Example for Linux AMD64
wget https://github.com/mount-exporter/mount-exporter/releases/latest/download/mount-exporter-linux-amd64.tar.gz
tar -xzf mount-exporter-linux-amd64.tar.gz
chmod +x mount-exporter
sudo mv mount-exporter /usr/local/bin/
```

### 2. Create Configuration

Create a configuration directory and config file:

```bash
sudo mkdir -p /etc/mount-exporter
sudo nano /etc/mount-exporter/config.yaml
```

Example configuration:
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  path: "/metrics"

mount_points:
  - "/"
  - "/data"
  - "/var/log"
  - "/mnt/backups"

interval: 30s

logging:
  level: "info"
  format: "json"
```

### 3. Test the Installation

```bash
mount-exporter -config /etc/mount-exporter/config.yaml -log-level debug
```

### 4. Verify Metrics

```bash
curl http://localhost:8080/metrics
```

## Docker Deployment

### 1. Pull the Image

```bash
docker pull ghcr.io/mount-exporter/mount-exporter:latest
```

### 2. Create Configuration

```bash
mkdir -p ./mount-exporter-config
cat > ./mount-exporter-config/config.yaml << EOF
server:
  host: "0.0.0.0"
  port: 8080
  path: "/metrics"

mount_points:
  - "/"
  - "/data"
  - "/var/log"

interval: 30s

logging:
  level: "info"
  format: "json"
EOF
```

### 3. Run the Container

```bash
docker run -d \
  --name mount-exporter \
  --restart unless-stopped \
  -p 8080:8080 \
  -v $(pwd)/mount-exporter-config:/etc/mount-exporter:ro \
  -v /:/host:ro \
  ghcr.io/mount-exporter/mount-exporter:latest
```

### 4. Docker Compose

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  mount-exporter:
    image: ghcr.io/mount-exporter/mount-exporter:latest
    container_name: mount-exporter
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config:/etc/mount-exporter:ro
      - /:/host:ro
    environment:
      - MOUNT_EXPORTER_LOG_LEVEL=info
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

Run with:
```bash
docker-compose up -d
```

## Kubernetes Deployment

### 1. Create ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mount-exporter-config
  namespace: monitoring
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      path: "/metrics"

    mount_points:
      - "/"
      - "/data"
      - "/var/log"
      - "/mnt/backups"

    interval: 30s

    logging:
      level: "info"
      format: "json"
```

### 2. Create Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mount-exporter
  namespace: monitoring
  labels:
    app: mount-exporter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mount-exporter
  template:
    metadata:
      labels:
        app: mount-exporter
    spec:
      containers:
      - name: mount-exporter
        image: ghcr.io/mount-exporter/mount-exporter:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
          name: metrics
        args:
        - "-config"
        - "/etc/mount-exporter/config.yaml"
        volumeMounts:
        - name: config
          mountPath: /etc/mount-exporter
        - name: host-root
          mountPath: /host
          readOnly: true
        resources:
          requests:
            memory: "32Mi"
            cpu: "10m"
          limits:
            memory: "128Mi"
            cpu: "100m"
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
      volumes:
      - name: config
        configMap:
          name: mount-exporter-config
      - name: host-root
        hostPath:
          path: /
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
```

### 3. Create Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mount-exporter
  namespace: monitoring
  labels:
    app: mount-exporter
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
spec:
  selector:
    app: mount-exporter
  ports:
  - name: metrics
    port: 8080
    targetPort: 8080
    protocol: TCP
  type: ClusterIP
```

### 4. Deploy

```bash
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
```

## Systemd Deployment

### 1. Create System User

```bash
sudo useradd -r -s /bin/false mount-exporter
sudo mkdir -p /etc/mount-exporter
sudo chown -R mount-exporter:mount-exporter /etc/mount-exporter
```

### 2. Install Binary

```bash
wget https://github.com/mount-exporter/mount-exporter/releases/latest/download/mount-exporter-linux-amd64.tar.gz
tar -xzf mount-exporter-linux-amd64.tar.gz
sudo mv mount-exporter /usr/local/bin/
sudo chmod +x /usr/local/bin/mount-exporter
```

### 3. Create Configuration

```bash
sudo nano /etc/mount-exporter/config.yaml
```

### 4. Create Systemd Service

```bash
sudo nano /etc/systemd/system/mount-exporter.service
```

Content:
```ini
[Unit]
Description=Mount Exporter - Prometheus exporter for filesystem mount points
Documentation=https://github.com/mount-exporter/mount-exporter
After=network.target
Wants=network.target

[Service]
Type=simple
User=mount-exporter
Group=mount-exporter
ExecStart=/usr/local/bin/mount-exporter -config /etc/mount-exporter/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=10
TimeoutStopSec=30

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ProtectKernelTunables=true
LockPersonality=true
RestrictRealtime=true
RestrictSUIDSGID=true
RemoveIPC=true

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mount-exporter

# Resource limits
LimitNOFILE=65536
MemoryMax=64M

[Install]
WantedBy=multi-user.target
```

### 5. Enable and Start Service

```bash
sudo systemctl daemon-reload
sudo systemctl enable mount-exporter
sudo systemctl start mount-exporter
sudo systemctl status mount-exporter
```

### 6. Check Logs

```bash
sudo journalctl -u mount-exporter -f
```

## Monitoring and Alerting

### Prometheus Configuration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'mount-exporter'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 30s
    scrape_timeout: 10s
    metrics_path: /metrics
```

### Grafana Dashboard

Example panel configurations:

**Mount Point Status:**
```json
{
  "title": "Mount Point Status",
  "type": "stat",
  "targets": [
    {
      "expr": "sum(mount_exporter_mount_point_status)",
      "legendFormat": "Mounted"
    }
  ]
}
```

**Individual Mount Points:**
```json
{
  "title": "Mount Point Details",
  "type": "table",
  "targets": [
    {
      "expr": "mount_exporter_mount_point_status",
      "format": "table"
    }
  ]
}
```

### Alerting Rules

Create `mount-exporter-rules.yml`:

```yaml
groups:
- name: mount-exporter
  rules:
  - alert: MountPointUnavailable
    expr: mount_exporter_mount_point_status == 0
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Mount point {{ $labels.mount_point }} is unavailable"
      description: "Mount point {{ $labels.mount_point }} has been unavailable for more than 2 minutes."

  - alert: MountExporterDown
    expr: up{job="mount-exporter"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Mount exporter is down"
      description: "Mount exporter has been down for more than 1 minute."

  - alert: MountExporterHighScrapeDuration
    expr: mount_exporter_scrape_duration_seconds > 5
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High scrape duration for mount point {{ $labels.mount_point }}"
      description: "Scrape duration for mount point {{ $labels.mount_point }} is {{ $value }}s."
```

## Troubleshooting

### Common Issues

**1. Permission Denied**
```bash
# Check binary permissions
ls -la /usr/local/bin/mount-exporter
sudo chmod +x /usr/local/bin/mount-exporter

# Check config file permissions
ls -la /etc/mount-exporter/
sudo chown -R mount-exporter:mount-exporter /etc/mount-exporter/
```

**2. findmnt Command Not Found**
```bash
# Install util-linux
sudo apt-get install util-linux  # Ubuntu/Debian
sudo yum install util-linux      # CentOS/RHEL
```

**3. Port Already in Use**
```bash
# Check what's using the port
sudo netstat -tulpn | grep :8080

# Change port in config
sudo nano /etc/mount-exporter/config.yaml
```

**4. Configuration Errors**
```bash
# Test configuration
mount-exporter -config /etc/mount-exporter/config.yaml -log-level debug

# Validate YAML
python -c "import yaml; yaml.safe_load(open('/etc/mount-exporter/config.yaml'))"
```

### Health Checks

```bash
# Check health endpoint
curl http://localhost:8080/health

# Check metrics endpoint
curl http://localhost:8080/metrics

# Check application logs
sudo journalctl -u mount-exporter -n 50
```

### Performance Tuning

**1. Collection Interval**
- Reduce interval for more frequent checks (may increase system load)
- Increase interval for less frequent checks (better for system resources)

**2. Mount Points**
- Only monitor critical mount points to reduce overhead
- Avoid monitoring network mounts with high latency

**3. Memory Usage**
- Monitor memory usage with `ps aux | grep mount-exporter`
- Adjust systemd MemoryMax if needed

## Maintenance

### Updates

1. **Stop the service:**
   ```bash
   sudo systemctl stop mount-exporter
   ```

2. **Backup configuration:**
   ```bash
   sudo cp /etc/mount-exporter/config.yaml /etc/mount-exporter/config.yaml.backup
   ```

3. **Update binary:**
   ```bash
   wget https://github.com/mount-exporter/mount-exporter/releases/latest/download/mount-exporter-linux-amd64.tar.gz
   tar -xzf mount-exporter-linux-amd64.tar.gz
   sudo mv mount-exporter /usr/local/bin/mount-exporter.new
   sudo mv /usr/local/bin/mount-exporter /usr/local/bin/mount-exporter.old
   sudo mv /usr/local/bin/mount-exporter.new /usr/local/bin/mount-exporter
   ```

4. **Restart service:**
   ```bash
   sudo systemctl start mount-exporter
   sudo systemctl status mount-exporter
   ```

### Backup and Restore

**Backup:**
```bash
# Backup configuration
sudo tar -czf mount-exporter-backup-$(date +%Y%m%d).tar.gz /etc/mount-exporter/

# Backup systemd service
sudo cp /etc/systemd/system/mount-exporter.service .
```

**Restore:**
```bash
# Restore configuration
sudo tar -xzf mount-exporter-backup-YYYYMMDD.tar.gz -C /

# Restore systemd service
sudo cp mount-exporter.service /etc/systemd/system/
sudo systemctl daemon-reload
```

### Log Rotation

Create `/etc/logrotate.d/mount-exporter`:

```
/var/log/mount-exporter/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 644 mount-exporter mount-exporter
    postrotate
        systemctl reload mount-exporter
    endscript
}
```

## Security Considerations

1. **Run as non-root user** - Use dedicated mount-exporter user
2. **File permissions** - Restrict access to configuration files
3. **Network exposure** - Only expose necessary ports
4. **Container security** - Use read-only filesystem where possible
5. **Resource limits** - Set memory and CPU limits
6. **TLS** - Use HTTPS for metrics in production environments
7. **Authentication** - Consider basic auth or token-based auth for metrics endpoint

## Performance Guidelines

1. **Monitor system resources** - CPU, memory, and I/O usage
2. **Optimize collection interval** - Balance between responsiveness and overhead
3. **Limit mount points** - Only monitor necessary mount points
4. **Use caching** - Enable metrics caching where appropriate
5. **Network considerations** - Consider latency for remote mount points
6. **Scaling** - Deploy multiple instances for large environments