# Troubleshooting Guide & FAQ

This guide covers common issues, troubleshooting steps, and frequently asked questions for Mount Exporter.

## Table of Contents

- [Installation Issues](#installation-issues)
- [Configuration Problems](#configuration-problems)
- [Runtime Issues](#runtime-issues)
- [Network and Connectivity](#network-and-connectivity)
- [Docker and Container Issues](#docker-and-container-issues)
- [Kubernetes Issues](#kubernetes-issues)
- [Performance Issues](#performance-issues)
- [Monitoring and Alerting](#monitoring-and-alerting)
- [Frequently Asked Questions](#frequently-asked-questions)
- [Advanced Troubleshooting](#advanced-troubleshooting)

## Installation Issues

### Q: The binary doesn't work after download

**Symptoms:**
- `./mount-exporter: command not found`
- Permission denied errors
- Binary execution fails

**Solutions:**

1. **Make the binary executable:**
   ```bash
   chmod +x mount-exporter
   ```

2. **Check binary integrity:**
   ```bash
   file mount-exporter
   # Should show: ELF 64-bit LSB executable, x86-64

   # Check with sha256sum
   sha256sum mount-exporter
   # Compare with the checksum from GitHub releases
   ```

3. **Verify correct architecture:**
   ```bash
   uname -m
   # Download the matching binary (amd64, arm64, etc.)
   ```

### Q: "findmnt command not found" error

**Symptoms:**
- Application logs show "findmnt command not available"
- Health check returns unhealthy status

**Solutions:**

1. **Install util-linux package:**

   **Ubuntu/Debian:**
   ```bash
   sudo apt-get update
   sudo apt-get install util-linux
   ```

   **CentOS/RHEL:**
   ```bash
   sudo yum install util-linux
   # Or for RHEL 8+:
   sudo dnf install util-linux
   ```

   **macOS:**
   ```bash
   # findmnt should be included, but if missing:
   brew install coreutils
   ```

2. **Verify installation:**
   ```bash
   which findmnt
   findmnt --version
   ```

### Q: Permission denied accessing mount points

**Symptoms:**
- Errors about permission denied when checking certain mount points
- Metrics show status 0 for mount points that should be mounted

**Solutions:**

1. **Run with appropriate privileges:**
   ```bash
   # Check if mount point is accessible
   ls -la /path/to/mount/point

   # Run mount-exporter with same user that can access mount points
   # Usually no special privileges needed for read-only access
   ```

2. **Check mount point permissions:**
   ```bash
   # Check directory permissions
   stat /path/to/mount/point

   # Check mount options
   mount | grep /path/to/mount/point
   ```

3. **Use specific user for service:**
   ```bash
   # For systemd, create dedicated user
   sudo useradd -r -s /bin/false mount-exporter

   # Add user to appropriate groups if needed
   sudo usermod -a -G disk,adm mount-exporter
   ```

## Configuration Problems

### Q: Configuration file parsing errors

**Symptoms:**
- "failed to parse config file" errors
- YAML syntax errors
- Invalid configuration values

**Solutions:**

1. **Validate YAML syntax:**
   ```bash
   # Using Python
   python -c "import yaml; yaml.safe_load(open('/etc/mount-exporter/config.yaml'))"

   # Using yamllint
   pip install yamllint
   yamllint /etc/mount-exporter/config.yaml
   ```

2. **Check common YAML issues:**
   ```yaml
   # Correct syntax:
   server:
     host: "0.0.0.0"
     port: 8080

   # Common mistakes:
   # - Using tabs instead of spaces
   # - Missing quotes on string values
   # - Incorrect indentation
   ```

3. **Use debug mode:**
   ```bash
   mount-exporter -config /etc/mount-exporter/config.yaml -log-level debug
   ```

### Q: Port already in use

**Symptoms:**
- "address already in use" error
- Service fails to start

**Solutions:**

1. **Find what's using the port:**
   ```bash
   sudo netstat -tulpn | grep :8080
   # or
   sudo lsof -i :8080
   ```

2. **Kill the process:**
   ```bash
   sudo kill -9 <PID>
   ```

3. **Change port in configuration:**
   ```yaml
   server:
     host: "0.0.0.0"
     port: 9090  # Use different port
   ```

4. **Use environment variable:**
   ```bash
   export MOUNT_EXPORTER_PORT=9090
   mount-exporter
   ```

### Q: Invalid mount point paths

**Symptoms:**
- "mount point must be absolute path" error
- Configuration validation fails

**Solutions:**

1. **Use absolute paths:**
   ```yaml
   # Correct:
   mount_points:
     - "/data"
     - "/var/log"
     - "/mnt/backups"

   # Incorrect:
   mount_points:
     - "data"
     - "./var/log"
     - "../mnt/backups"
   ```

2. **Verify mount points exist:**
   ```bash
   # Check if paths exist
   for mp in /data /var/log /mnt/backups; do
     if [ ! -d "$mp" ]; then
       echo "Directory does not exist: $mp"
     fi
   done
   ```

## Runtime Issues

### Q: Service starts but metrics endpoint returns 404

**Symptoms:**
- Service appears to be running
- `curl localhost:8080/metrics` returns 404
- Health check works but metrics don't

**Solutions:**

1. **Check metrics path configuration:**
   ```yaml
   server:
     path: "/metrics"  # Ensure this matches what you're requesting
   ```

2. **Verify with correct path:**
   ```bash
   # Check what's actually configured
   curl http://localhost:8080/health
   curl http://localhost:8080/metrics
   curl http://localhost:8080/  # Root endpoint shows info
   ```

3. **Check logs for path configuration:**
   ```bash
   sudo journalctl -u mount-exporter | grep "Metrics available"
   ```

### Q: All mount points show as not mounted (status 0)

**Symptoms:**
- Metrics show all mount points with value 0
- Mount points that should be mounted show as unavailable

**Solutions:**

1. **Check if mount points are actually mounted:**
   ```bash
   # Manual check with findmnt
   findmnt /data
   findmnt /var/log

   # Or check with mount command
   mount | grep /data
   mount | grep /var/log
   ```

2. **Test findmnt directly:**
   ```bash
   # Test each mount point
   findmnt -n -o TARGET,FSTYPE,OPTIONS,SOURCE --mountpoint /data
   findmnt -n -o TARGET,FSTYPE,OPTIONS,SOURCE --mountpoint /var/log
   ```

3. **Check for timeout issues:**
   ```bash
   # Test with longer timeout
   timeout 10s findmnt /data
   ```

4. **Run with debug logging:**
   ```bash
   mount-exporter -log-level debug
   ```

### Q: Metrics are not being collected periodically

**Symptoms:**
- Metrics don't update after initial collection
- Stale values in Prometheus

**Solutions:**

1. **Check collection interval:**
   ```yaml
   interval: 30s  # Ensure this is set appropriately
   ```

2. **Check logs for collection activity:**
   ```bash
   sudo journalctl -u mount-exporter -f | grep "Collecting metrics"
   ```

3. **Check for circuit breaker activation:**
   ```bash
   # Look for circuit breaker messages
   sudo journalctl -u mount-exporter | grep "circuit breaker"
   ```

4. **Restart service:**
   ```bash
   sudo systemctl restart mount-exporter
   ```

## Network and Connectivity

### Q: Prometheus cannot scrape metrics

**Symptoms:**
- Prometheus shows "scrape failed" errors
- Connection timeout errors
- HTTP 403/404 errors

**Solutions:**

1. **Check network connectivity:**
   ```bash
   # From Prometheus server
   curl http://mount-exporter-host:8080/metrics
   telnet mount-exporter-host 8080
   ```

2. **Check firewall rules:**
   ```bash
   # On mount-exporter host
   sudo ufw status
   sudo iptables -L

   # Allow port if needed
   sudo ufw allow 8080/tcp
   ```

3. **Verify Prometheus configuration:**
   ```yaml
   scrape_configs:
     - job_name: 'mount-exporter'
       static_configs:
         - targets: ['mount-exporter-host:8080']
       metrics_path: /metrics
       scrape_interval: 30s
   ```

4. **Check service binding:**
   ```yaml
   server:
     host: "0.0.0.0"  # Bind to all interfaces
     # Don't use "127.0.0.1" if accessing from other hosts
   ```

### Q: CORS errors when accessing from browser

**Symptoms:**
- CORS errors in browser console
- Cannot access metrics endpoint from web UI

**Solutions:**

1. **Use reverse proxy with CORS headers:**
   ```nginx
   location /metrics {
     proxy_pass http://localhost:8080;
     add_header Access-Control-Allow-Origin *;
     add_header Access-Control-Allow-Methods GET;
   }
   ```

2. **Access via Prometheus:**
   - Best practice is to access metrics through Prometheus rather than direct browser access

## Docker and Container Issues

### Q: Container won't start

**Symptoms:**
- Docker container exits immediately
- No logs available

**Solutions:**

1. **Check logs:**
   ```bash
   docker logs mount-exporter
   docker logs -f mount-exporter
   ```

2. **Run in interactive mode:**
   ```bash
   docker run -it --rm \
     -v $(pwd)/config.yaml:/etc/mount-exporter/config.yaml:ro \
     ghcr.io/mount-exporter/mount-exporter:latest \
     -config /etc/mount-exporter/config.yaml -log-level debug
   ```

3. **Check configuration file mounting:**
   ```bash
   # Verify config is accessible inside container
   docker run --rm -v $(pwd)/config.yaml:/config.yaml:ro \
     alpine cat /config.yaml
   ```

### Q: Container cannot access host mount points

**Symptoms:**
- All mount points show as not mounted
- "No such file or directory" errors

**Solutions:**

1. **Mount host filesystem:**
   ```bash
   docker run -d \
     -v /:/host:ro \
     -v $(pwd)/config:/etc/mount-exporter:ro \
     ghcr.io/mount-exporter/mount-exporter:latest
   ```

2. **Update configuration for container paths:**
   ```yaml
   mount_points:
     - "/host/data"
     - "/host/var/log"
     - "/host/mnt/backups"
   ```

3. **Use privileged mode (not recommended for production):**
   ```bash
   docker run --privileged -d ...
   ```

### Q: Container health checks failing

**Symptoms:**
- Docker shows unhealthy status
- Container restarts repeatedly

**Solutions:**

1. **Check health endpoint manually:**
   ```bash
   docker exec mount-exporter wget -qO- http://localhost:8080/health
   ```

2. **Adjust health check timing:**
   ```yaml
   # In docker-compose.yml
   healthcheck:
     test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
     interval: 30s
     timeout: 10s
     retries: 3
     start_period: 40s
   ```

3. **Check for startup delays:**
   ```bash
   # Look at logs during startup
   docker logs mount-exporter | head -20
   ```

## Kubernetes Issues

### Q: Pod is stuck in CrashLoopBackOff

**Symptoms:**
- Pod status shows CrashLoopBackOff
- Container keeps restarting

**Solutions:**

1. **Check pod logs:**
   ```bash
   kubectl logs -n monitoring mount-exporter-xxxxx
   kubectl logs -n monitoring -f mount-exporter-xxxxx
   ```

2. **Describe pod for events:**
   ```bash
   kubectl describe pod -n monitoring mount-exporter-xxxxx
   ```

3. **Check configuration:**
   ```bash
   kubectl get configmap mount-exporter-config -o yaml
   ```

4. **Debug with interactive session:**
   ```bash
   kubectl exec -it -n monitoring mount-exporter-xxxxx -- /bin/sh
   ```

### Q: Mount points not accessible in pod

**Symptoms:**
- Metrics show all mount points as unavailable
- Permission errors in logs

**Solutions:**

1. **Check hostPath volume mounting:**
   ```yaml
   volumes:
   - name: host-root
     hostPath:
       path: /
   ```

2. **Verify pod can access host:**
   ```bash
   kubectl exec -it -n monitoring mount-exporter-xxxxx -- ls /host
   ```

3. **Check security context:**
   ```yaml
   securityContext:
     runAsUser: 1000
     runAsGroup: 1000
     fsGroup: 1000
   ```

### Q: Service discovery issues

**Symptoms:**
- Prometheus cannot find mount-exporter service
- Service endpoints not created

**Solutions:**

1. **Check service:**
   ```bash
   kubectl get svc -n monitoring mount-exporter
   kubectl describe svc -n monitoring mount-exporter
   ```

2. **Check endpoints:**
   ```bash
   kubectl get endpoints -n monitoring mount-exporter
   ```

3. **Verify pod labels:**
   ```bash
   kubectl get pods -n monitoring --show-labels
   ```

4. **Check Prometheus service discovery:**
   ```yaml
   # In Prometheus config
   - job_name: 'mount-exporter'
     kubernetes_sd_configs:
       - role: pod
         namespaces:
           names:
             - monitoring
     relabel_configs:
       - source_labels: [__meta_kubernetes_pod_label_app]
         action: keep
         regex: mount-exporter
   ```

## Performance Issues

### Q: High CPU usage

**Symptoms:**
- Mount exporter using excessive CPU
- System performance degradation

**Solutions:**

1. **Check collection frequency:**
   ```yaml
   interval: 60s  # Increase from default 30s
   ```

2. **Reduce number of mount points:**
   ```yaml
   mount_points:
     - "/"          # Keep only critical ones
     - "/data"
     # Remove less critical mount points
   ```

3. **Monitor resource usage:**
   ```bash
   top -p $(pgrep mount-exporter)
   ps aux | grep mount-exporter
   ```

4. **Check for stuck operations:**
   ```bash
   # Look for long-running findmnt operations
   sudo strace -p $(pgrep mount-exporter)
   ```

### Q: High memory usage

**Symptoms:**
- Memory usage continuously increasing
- OOM (Out of Memory) errors

**Solutions:**

1. **Check for memory leaks:**
   ```bash
   # Monitor memory over time
   watch -n 5 'ps aux | grep mount-exporter'
   ```

2. **Adjust resource limits:**
   ```yaml
   # In deployment.yaml
   resources:
     requests:
       memory: "32Mi"
     limits:
       memory: "128Mi"
   ```

3. **Restart service:**
   ```bash
   sudo systemctl restart mount-exporter
   ```

4. **Check circuit breaker and retry statistics:**
   ```bash
   # Look for retry attempts in logs
   sudo journalctl -u mount-exporter | grep "retry"
   ```

## Monitoring and Alerting

### Q: False alerts for mount point availability

**Symptoms:**
- Alerts firing for mount points that are actually mounted
- Alert flapping (going up and down rapidly)

**Solutions:**

1. **Increase alert duration:**
   ```yaml
   - alert: MountPointUnavailable
     expr: mount_exporter_mount_point_status == 0
     for: 5m  # Increase from 2m
   ```

2. **Add alert inhibition rules:**
   ```yaml
   - alert: MountPointUnavailable
     expr: mount_exporter_mount_point_status == 0
     for: 5m
     labels:
       severity: critical
     annotations:
       summary: "Mount point {{ $labels.mount_point }} is unavailable"
   # Add inhibition for maintenance windows
   ```

3. **Check network mount latency:**
   ```bash
   # For NFS mounts
   time ls /mnt/nfs-share
   ping nfs-server
   ```

### Q: Missing metrics in Prometheus

**Symptoms:**
- Some metrics not appearing in Prometheus
- Incomplete metric labels

**Solutions:**

1. **Check scrape interval:**
   ```yaml
   scrape_configs:
     - job_name: 'mount-exporter'
       scrape_interval: 30s  # Ensure this matches collection interval
   ```

2. **Verify metric names:**
   ```bash
   curl -s http://localhost:8080/metrics | grep mount_exporter
   ```

3. **Check for metric label cardinality issues:**
   ```bash
   # Count unique label combinations
   curl -s http://localhost:8080/metrics | grep mount_exporter_mount_point_status | wc -l
   ```

## Frequently Asked Questions

### Q: Does Mount Exporter require root privileges?

**A:** No, Mount Exporter typically does not require root privileges. It uses the `findmnt` command which is generally available to all users. However, some mount points may have restricted access that requires specific permissions.

### Q: Can Mount Exporter monitor network file systems (NFS, SMB)?

**A:** Yes, Mount Exporter can monitor network file systems as long as they are mounted and accessible via `findmnt`. However, be aware that network mount points may have higher latency and may cause timeouts.

### Q: How does Mount Exporter handle mount points with spaces or special characters?

**A:** Mount Exporter correctly handles mount points with spaces and special characters in their paths. Ensure the paths in your configuration are properly quoted in YAML.

### Q: Can I run multiple instances of Mount Exporter?

**A:** Yes, you can run multiple instances as long as they listen on different ports or different IP addresses. This can be useful for:
- High availability
- Monitoring different sets of mount points
- Redundancy

### Q: How secure is Mount Exporter?

**A:** Mount Exporter is designed with security in mind:
- Runs without root privileges by default
- Read-only access to mount information
- No external dependencies
- Minimal attack surface
- Security headers on HTTP endpoints

For additional security, consider:
- Running in containers with limited privileges
- Using reverse proxy with authentication
- Network segmentation

### Q: What happens if findmnt is unavailable?

**A:** If `findmnt` is unavailable, Mount Exporter will:
- Log an error message
- Set the `mount_exporter_up` metric to 0 (unhealthy)
- Continue attempting to run `findmnt` with circuit breaker and retry logic
- Return mount point status as unknown

### Q: Can Mount Exporter cause performance issues on the monitored system?

**A:** Mount Exporter is designed to be lightweight:
- Minimal CPU usage (typically < 1%)
- Small memory footprint (typically < 64MB)
- Efficient `findmnt` command usage
- Configurable collection intervals

Performance can be tuned by adjusting the collection interval and the number of monitored mount points.

### Q: How does Mount Exporter handle mount points that are temporarily unavailable?

**A:** Mount Exporter includes several reliability features:
- Circuit breaker to prevent repeated failures
- Retry logic for transient failures
- Configurable timeouts
- Graceful error handling

Temporarily unavailable mount points will be reflected in the metrics but won't cause the exporter to crash.

## Advanced Troubleshooting

### Debug Mode

Enable debug logging for detailed information:

```bash
mount-exporter -config /etc/mount-exporter/config.yaml -log-level debug
```

### Manual Testing

Test components individually:

```bash
# Test findmnt command
findmnt -n -o TARGET,FSTYPE,OPTIONS,SOURCE --mountpoint /data

# Test configuration parsing
python -c "import yaml; print(yaml.safe_load(open('/etc/mount-exporter/config.yaml')))"

# Test HTTP endpoints
curl -v http://localhost:8080/health
curl -v http://localhost:8080/metrics
```

### Performance Profiling

Enable profiling for performance analysis:

```bash
# Enable pprof endpoints
curl http://localhost:8080/debug/pprof/heap
curl http://localhost:8080/debug/pprof/profile
```

### Network Debugging

Debug network connectivity issues:

```bash
# Check port binding
sudo netstat -tulpn | grep :8080

# Test connectivity
telnet localhost 8080

# Check firewall
sudo iptables -L | grep 8080
```

### Container Debugging

Debug container issues:

```bash
# Enter container
docker exec -it mount-exporter /bin/sh

# Check processes
docker exec mount-exporter ps aux

# Check network
docker exec mount-exporter netstat -tulpn
```

### Getting Help

If you're still experiencing issues:

1. **Check the logs:**
   ```bash
   sudo journalctl -u mount-exporter -n 100
   ```

2. **Create an issue on GitHub:**
   - Include configuration (sanitized)
   - Include relevant logs
   - Include system information
   - Describe expected vs actual behavior

3. **Community support:**
   - Check existing GitHub issues
   - Join discussions
   - Review documentation

Remember to include sensitive information when sharing logs or configurations!